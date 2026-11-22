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
	"github.com/swiftdrop/messaging/internal/relay"
)

// SupportedInAppEvents maps parcel lifecycle events handled by in-app notifications.
var SupportedInAppEvents = map[string]bool{
	"parcel.booked.v1":           true,
	"parcel.assigned.v1":         true,
	"parcel.picked_up.v1":        true,
	"parcel.in_transit.v1":       true,
	"parcel.out_for_delivery.v1": true,
	"parcel.delivered.v1":        true,
	"parcel.delivery_failed.v1":  true,
	"parcel.returned.v1":         true,
	"parcel.cancelled.v1":        true,
	"courier.unassigned.v1":      true,
	"merchant.approved.v1":       true,
	"merchant.status_changed.v1": true,
}

// InAppNotificationPayload extracts common metadata from incoming payloads.
type InAppNotificationPayload struct {
	ParcelID       string  `json:"parcel_id"`
	TrackingNumber string  `json:"tracking_number"`
	SenderID       *string `json:"sender_id,omitempty"`
	CustomerID     *string `json:"customer_id,omitempty"`
	MerchantID     *string `json:"merchant_id,omitempty"`
	CourierID      *string `json:"courier_id,omitempty"`
	OwnerUserID    *string `json:"owner_user_id,omitempty"`
	PreviousStatus string  `json:"previous_status,omitempty"`
	NewStatus      string  `json:"new_status,omitempty"`
	Status         string  `json:"status,omitempty"`
	Reason         string  `json:"reason,omitempty"`
}

// NewInAppNotificationHandler creates a MessageHandler for q.inapp.notifications.
func NewInAppNotificationHandler(repo delivery.Repository, idemp idempotency.Store, logger *slog.Logger) MessageHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(ctx context.Context, _ amqp.Delivery, env *relay.OutboxEnvelope) error {
		// 1. Check supported event
		if !SupportedInAppEvents[env.EventType] {
			logger.Debug("unsupported event for in-app notifications, skipping", "event_type", env.EventType)
			return nil
		}

		// 2. Check deduplication ledger
		seen, err := idemp.Seen(ctx, "inapp_notifier", env.EventID)
		if err != nil {
			return fmt.Errorf("idempotency check error: %w", err)
		}
		if seen {
			logger.Debug("in-app notification already processed, skipping", "event_id", env.EventID)
			return nil
		}

		// 3. Parse payload
		var p InAppNotificationPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		trackingNum := p.TrackingNumber
		if trackingNum == "" {
			trackingNum = env.AggregateID
		}

		// 4. Resolve sender/merchant from DB if not in payload
		senderID := p.SenderID
		if senderID == nil {
			senderID = p.CustomerID
		}
		merchantID := p.MerchantID

		if senderID == nil && merchantID == nil && strings.HasPrefix(env.EventType, "parcel.") {
			dbSender, dbMerchant, err := repo.GetParcelOwnership(ctx, env.AggregateID)
			if err == nil {
				if senderID == nil {
					senderID = dbSender
				}
				if merchantID == nil {
					merchantID = dbMerchant
				}
			}
		}

		// 5. Generate notifications for relevant parties
		notifications := buildInAppNotifications(env, p, trackingNum, senderID, merchantID)

		// 6. Persist all generated notifications
		for _, notif := range notifications {
			if err := repo.InsertInAppNotification(ctx, notif); err != nil {
				return fmt.Errorf("failed to insert in-app notification for user %s: %w", notif.UserID, err)
			}
		}

		// 7. Mark event as processed in idempotency ledger
		_ = idemp.MarkDone(ctx, nil, "inapp_notifier", env.EventID)
		logger.Info("in-app notification(s) created",
			"event_id", env.EventID,
			"event_type", env.EventType,
			"count", len(notifications),
		)

		return nil
	}
}

func buildInAppNotifications(
	env *relay.OutboxEnvelope,
	p InAppNotificationPayload,
	trackingNum string,
	senderID *string,
	merchantID *string,
) []delivery.InAppNotification {
	var notifs []delivery.InAppNotification
	now := time.Now().UTC()

	metaMap := map[string]interface{}{
		"tracking_number": trackingNum,
		"aggregate_id":    env.AggregateID,
		"event_type":      env.EventType,
	}
	if p.Status != "" {
		metaMap["status"] = p.Status
	} else if p.NewStatus != "" {
		metaMap["status"] = p.NewStatus
	}
	metaBytes, _ := json.Marshal(metaMap)

	switch env.EventType {
	case "parcel.booked.v1":
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Parcel Booked",
				Body:        fmt.Sprintf("Your parcel %s has been booked successfully.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "parcel.assigned.v1":
		if p.CourierID != nil && *p.CourierID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *p.CourierID,
				TenantID:    nil,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "New Delivery Assigned",
				Body:        fmt.Sprintf("You have been assigned parcel %s for delivery.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Courier Assigned",
				Body:        fmt.Sprintf("A courier has been assigned to deliver parcel %s.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "parcel.picked_up.v1":
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Parcel Picked Up",
				Body:        fmt.Sprintf("Parcel %s has been picked up by the courier.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "parcel.in_transit.v1":
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Parcel In Transit",
				Body:        fmt.Sprintf("Parcel %s is now in transit.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "parcel.out_for_delivery.v1":
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Out for Delivery",
				Body:        fmt.Sprintf("Parcel %s is out for doorstep delivery today.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "parcel.delivered.v1":
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Parcel Delivered",
				Body:        fmt.Sprintf("Parcel %s has been successfully delivered.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "parcel.delivery_failed.v1":
		failReason := p.Reason
		if failReason == "" {
			failReason = "Delivery attempt unsuccessful"
		}
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Delivery Attempt Failed",
				Body:        fmt.Sprintf("Delivery attempt for parcel %s failed: %s.", trackingNum, failReason),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}
		// Also broadcast exception to dispatchers
		notifs = append(notifs, delivery.InAppNotification{
			UserID:      "role:DISPATCHER",
			TenantID:    nil,
			EventType:   env.EventType,
			AggregateID: env.AggregateID,
			Title:       "Delivery Failed Exception",
			Body:        fmt.Sprintf("Delivery attempt failed for parcel %s (%s).", trackingNum, failReason),
			Metadata:    metaBytes,
			CreatedAt:   now,
		})

	case "parcel.returned.v1":
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Parcel Returned",
				Body:        fmt.Sprintf("Parcel %s has been returned to sender.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}
		notifs = append(notifs, delivery.InAppNotification{
			UserID:      "role:DISPATCHER",
			TenantID:    nil,
			EventType:   env.EventType,
			AggregateID: env.AggregateID,
			Title:       "Parcel Returned",
			Body:        fmt.Sprintf("Parcel %s marked returned to origin.", trackingNum),
			Metadata:    metaBytes,
			CreatedAt:   now,
		})

	case "parcel.cancelled.v1":
		if senderID != nil && *senderID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *senderID,
				TenantID:    merchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Parcel Cancelled",
				Body:        fmt.Sprintf("Parcel %s has been cancelled.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "courier.unassigned.v1":
		if p.CourierID != nil && *p.CourierID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *p.CourierID,
				TenantID:    nil,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Parcel Unassigned",
				Body:        fmt.Sprintf("You have been unassigned from parcel %s.", trackingNum),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "merchant.approved.v1":
		if p.OwnerUserID != nil && *p.OwnerUserID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *p.OwnerUserID,
				TenantID:    p.MerchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Merchant Account Approved",
				Body:        "Your merchant profile has been approved. You may now book bulk deliveries and configure webhooks.",
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}

	case "merchant.status_changed.v1":
		if p.OwnerUserID != nil && *p.OwnerUserID != "" {
			notifs = append(notifs, delivery.InAppNotification{
				UserID:      *p.OwnerUserID,
				TenantID:    p.MerchantID,
				EventType:   env.EventType,
				AggregateID: env.AggregateID,
				Title:       "Merchant Status Updated",
				Body:        fmt.Sprintf("Your merchant account status has changed to %s.", p.Status),
				Metadata:    metaBytes,
				CreatedAt:   now,
			})
		}
	}

	return notifs
}
