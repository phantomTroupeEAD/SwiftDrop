package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/messaging/internal/delivery"
	"github.com/swiftdrop/messaging/internal/idempotency"
	"github.com/swiftdrop/messaging/internal/metrics"
	"github.com/swiftdrop/messaging/internal/relay"
)

// SupportedNotificationMilestones lists event types that trigger customer email notifications (Decision D21).
var SupportedNotificationMilestones = map[string]bool{
	"parcel.booked.v1":           true,
	"parcel.out_for_delivery.v1": true,
	"parcel.delivered.v1":        true,
	"parcel.delivery_failed.v1":  true,
	"parcel.returned.v1":         true,
}

// NewNotificationHandler creates a handler for q.notifications.worker.
func NewNotificationHandler(repo delivery.Repository, idemp idempotency.Store, logger *slog.Logger) MessageHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(ctx context.Context, d amqp.Delivery, env *relay.OutboxEnvelope) error {
		// 1. Check if event is a customer milestone (Decision D21)
		if !SupportedNotificationMilestones[env.EventType] {
			metrics.NotificationsSkippedTotal.Inc()
			return nil
		}

		// 2. Check deduplication ledger
		seen, err := idemp.Seen(ctx, "notifier", env.EventID)
		if err != nil {
			return fmt.Errorf("idempotency check error: %w", err)
		}
		if seen {
			logger.Debug("notification already processed, skipping", "event_id", env.EventID)
			return nil
		}

		// 3. Parse payload to extract notify block (Decision D10)
		var payloadMap map[string]interface{}
		if err := json.Unmarshal(env.Payload, &payloadMap); err != nil {
			return fmt.Errorf("failed to unmarshal payload for notify: %w", err)
		}

		var recipientEmail string
		if notifyRaw, ok := payloadMap["notify"].(map[string]interface{}); ok {
			if emailRaw, ok := notifyRaw["recipient_email"].(string); ok {
				recipientEmail = strings.TrimSpace(emailRaw)
			}
		}

		// If notify block or recipient is missing, record FAILED row per Decision D10
		if recipientEmail == "" {
			task := delivery.NotificationTask{
				EventID:          env.EventID,
				EventType:        env.EventType,
				AggregateID:      env.AggregateID,
				AggregateVersion: env.AggregateVersion,
				Payload:          env.Payload,
				Recipient:        "-",
				Channel:          "EMAIL",
				TemplateName:     "milestone_notice",
			}
			if err := repo.InsertNotification(ctx, task); err != nil {
				return fmt.Errorf("failed to record no-recipient notification: %w", err)
			}
			logger.Warn("event has no recipient email; marked notification as FAILED",
				"event_id", env.EventID,
				"event_type", env.EventType,
			)
			return nil
		}

		task := delivery.NotificationTask{
			EventID:          env.EventID,
			EventType:        env.EventType,
			AggregateID:      env.AggregateID,
			AggregateVersion: env.AggregateVersion,
			Payload:          env.Payload,
			Recipient:        recipientEmail,
			Channel:          "EMAIL",
			TemplateName:     "milestone_notice",
		}

		if err := repo.InsertNotification(ctx, task); err != nil {
			return fmt.Errorf("failed to insert notification task: %w", err)
		}

		logger.Info("staged customer notification task",
			"event_id", env.EventID,
			"event_type", env.EventType,
		)
		return nil
	}
}

// NewWebhookFanoutHandler creates a handler for q.webhooks.dispatcher.
func NewWebhookFanoutHandler(repo delivery.Repository, idemp idempotency.Store, logger *slog.Logger) MessageHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(ctx context.Context, d amqp.Delivery, env *relay.OutboxEnvelope) error {
		// 1. Extract merchant_id from payload (Decision D10)
		var payloadMap map[string]interface{}
		if err := json.Unmarshal(env.Payload, &payloadMap); err != nil {
			return fmt.Errorf("failed to unmarshal payload for webhook fanout: %w", err)
		}

		merchantIDRaw, ok := payloadMap["merchant_id"].(string)
		merchantID := strings.TrimSpace(merchantIDRaw)
		if !ok || merchantID == "" {
			logger.Debug("event has no merchant_id; skipping webhooks", "event_id", env.EventID, "event_type", env.EventType)
			return nil
		}

		// 2. Query active subscribed targets
		targets, err := repo.GetMerchantTargets(ctx, merchantID, env.EventType)
		if err != nil {
			return fmt.Errorf("failed to query merchant webhook targets: %w", err)
		}

		if len(targets) == 0 {
			logger.Debug("no active webhook targets subscribed", "merchant_id", merchantID, "event_type", env.EventType)
			return nil
		}

		// 3. Strip notify from payload before storing for webhooks (Decisions D11, D19)
		strippedPayload := make(map[string]interface{}, len(payloadMap))
		for k, v := range payloadMap {
			if k != "notify" {
				strippedPayload[k] = v
			}
		}
		sanitizedBytes, err := json.Marshal(strippedPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal sanitized webhook payload: %w", err)
		}

		occurredAt, err := time.Parse(time.RFC3339Nano, env.OccurredAt)
		if err != nil {
			occurredAt, err = time.Parse(time.RFC3339, env.OccurredAt)
			if err != nil {
				occurredAt = time.Now().UTC()
			}
		}

		// 4. Fan out to each subscribed endpoint
		for _, target := range targets {
			consumerName := fmt.Sprintf("webhook:%s", target.EndpointID)
			seen, err := idemp.Seen(ctx, consumerName, env.EventID)
			if err != nil {
				return fmt.Errorf("idempotency check error for %s: %w", consumerName, err)
			}
			if seen {
				continue
			}

			task := delivery.WebhookTask{
				EndpointID:       target.EndpointID,
				EventID:          env.EventID,
				EventType:        env.EventType,
				AggregateID:      env.AggregateID,
				AggregateVersion: env.AggregateVersion,
				OccurredAt:       occurredAt,
				Payload:          sanitizedBytes,
			}

			if err := repo.InsertWebhook(ctx, task); err != nil {
				return fmt.Errorf("failed to insert webhook task for endpoint %s: %w", target.EndpointID, err)
			}

			logger.Info("staged webhook delivery task",
				"endpoint_id", target.EndpointID,
				"event_id", env.EventID,
				"event_type", env.EventType,
			)
		}

		return nil
	}
}
