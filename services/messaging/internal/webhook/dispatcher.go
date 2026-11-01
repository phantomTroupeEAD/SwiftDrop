package webhook

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/swiftdrop/messaging/internal/config"
	"github.com/swiftdrop/messaging/internal/delivery"
	"github.com/swiftdrop/messaging/internal/logging"
	"github.com/swiftdrop/messaging/internal/metrics"
)

// HTTPClient defines the outbound dispatch interface.
type HTTPClient interface {
	Do(ctx context.Context, targetURL string, headers map[string]string, body []byte) DispatchResult
}

// Dispatcher polls pending webhook deliveries and dispatches them to merchant endpoints.
type Dispatcher struct {
	repo      delivery.Repository
	client    HTTPClient
	decryptor *Decryptor
	calc      *delivery.BackoffCalculator
	cfg       *config.Config
	logger    *slog.Logger
}

// NewDispatcher constructs a new Webhook Dispatcher.
func NewDispatcher(
	repo delivery.Repository,
	client HTTPClient,
	decryptor *Decryptor,
	calc *delivery.BackoffCalculator,
	cfg *config.Config,
	logger *slog.Logger,
) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if calc == nil {
		calc = delivery.NewBackoffCalculator(cfg.WebhookRetrySchedule, nil)
	}
	return &Dispatcher{
		repo:      repo,
		client:    client,
		decryptor: decryptor,
		calc:      calc,
		cfg:       cfg,
		logger:    logger,
	}
}

// Run starts the polling loop until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) error {
	d.logger.Info("starting webhook dispatcher daemon")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("webhook dispatcher stopping")
			return ctx.Err()
		case <-ticker.C:
			processed, err := d.PollAndDispatch(ctx)
			if err != nil {
				d.logger.Error("webhook dispatcher poll error", "error", err.Error())
			} else if processed > 0 {
				d.logger.Debug("processed webhook batch", "count", processed)
			}
		}
	}
}

// PollAndDispatch claims due webhook rows and dispatches them.
func (d *Dispatcher) PollAndDispatch(ctx context.Context) (int, error) {
	leaseDuration := time.Duration(d.cfg.DeliveryLeaseSeconds) * time.Second
	tasks, err := d.repo.ClaimWebhooks(ctx, 20, leaseDuration)
	if err != nil {
		return 0, fmt.Errorf("failed to claim webhooks: %w", err)
	}

	if len(tasks) == 0 {
		return 0, nil
	}

	for _, task := range tasks {
		// 1. Load target fresh by endpoint_id to verify it is still enabled
		target, err := d.repo.GetEndpointByID(ctx, task.EndpointID)
		if err != nil {
			d.logger.Error("failed to load endpoint target", "endpoint_id", task.EndpointID, "error", err.Error())
			continue
		}

		if target == nil {
			d.logger.Warn("endpoint target not found or disabled; marking delivery as FAILED",
				"delivery_id", task.ID,
				"endpoint_id", task.EndpointID,
			)
			_ = d.repo.RecordWebhookTerminalFailure(
				ctx,
				task.ID,
				task.EndpointID,
				task.EventID,
				task.AttemptCount,
				0,
				"",
				"ENDPOINT_DISABLED",
				"endpoint configuration was deleted, disabled, or merchant unapproved",
			)
			metrics.WebhookDeliveryAttemptsTotal.WithLabelValues("failed").Inc()
			continue
		}

		// 2. Decrypt endpoint secret
		secret, err := d.decryptor.Decrypt(target.SecretCiphertext, target.EndpointID, target.SecretKeyVersion)
		if err != nil {
			d.logger.Error("failed to decrypt endpoint secret",
				"delivery_id", task.ID,
				"endpoint_id", task.EndpointID,
				"key_version", target.SecretKeyVersion,
				"error", err.Error(),
			)
			_ = d.repo.RecordWebhookTerminalFailure(
				ctx,
				task.ID,
				task.EndpointID,
				task.EventID,
				task.AttemptCount,
				0,
				"",
				"KEY_UNAVAILABLE",
				"failed to decrypt webhook secret with configured key version",
			)
			metrics.WebhookDeliveryAttemptsTotal.WithLabelValues("failed").Inc()
			continue
		}

		// 3. Build sanitized payload body
		bodyBytes, err := BuildWebhookBody(
			task.EventID,
			task.EventType,
			task.AggregateID,
			task.AggregateVersion,
			task.OccurredAt,
			task.Payload,
		)
		if err != nil {
			d.logger.Error("failed to build webhook body", "delivery_id", task.ID, "error", err.Error())
			continue
		}

		// 4. Sign payload
		nowUnix := time.Now().Unix()
		sig := Sign(nowUnix, bodyBytes, secret)

		headers := map[string]string{
			"Content-Type":            "application/json; charset=utf-8",
			"User-Agent":              "SwiftDrop-Webhook-Delivery/1.0",
			"X-SwiftDrop-Delivery-ID": task.ID,
			"X-SwiftDrop-Event-ID":    task.EventID,
			"X-SwiftDrop-Timestamp":   strconv.FormatInt(nowUnix, 10),
			"X-SwiftDrop-Signature":   sig,
		}

		// 5. Execute HTTP POST request outside any database transaction
		dispatchCtx, cancel := context.WithTimeout(ctx, d.cfg.WebhookHTTPTimeout+1*time.Second)
		res := d.client.Do(dispatchCtx, target.URL, headers, bodyBytes)
		cancel()

		maskedHost := logging.HostOnly(target.URL)

		if res.IsSuccess {
			_ = d.repo.RecordWebhookSuccess(
				ctx,
				task.ID,
				task.EndpointID,
				task.EventID,
				task.AttemptCount,
				res.StatusCode,
				res.Snippet,
			)
			metrics.WebhookDeliveryAttemptsTotal.WithLabelValues("success").Inc()
			d.logger.Info("webhook delivered successfully",
				"delivery_id", task.ID,
				"endpoint_id", task.EndpointID,
				"target_host", maskedHost,
				"status_code", res.StatusCode,
			)
			continue
		}

		if res.IsTerminal || task.AttemptCount >= d.calc.MaxAttempts() {
			_ = d.repo.RecordWebhookTerminalFailure(
				ctx,
				task.ID,
				task.EndpointID,
				task.EventID,
				task.AttemptCount,
				res.StatusCode,
				res.Snippet,
				res.ErrorClass,
				res.ErrorMessage,
			)
			metrics.WebhookDeliveryAttemptsTotal.WithLabelValues("failed").Inc()
			d.logger.Warn("webhook delivery reached terminal failure",
				"delivery_id", task.ID,
				"endpoint_id", task.EndpointID,
				"target_host", maskedHost,
				"status_code", res.StatusCode,
				"error_class", res.ErrorClass,
			)
			continue
		}

		// Retryable failure
		delay := d.calc.CalculateDelay(task.AttemptCount)
		nextAttemptAt := time.Now().Add(delay)
		_ = d.repo.RecordWebhookRetry(
			ctx,
			task.ID,
			task.AttemptCount,
			res.StatusCode,
			res.Snippet,
			res.ErrorClass,
			res.ErrorMessage,
			nextAttemptAt,
		)
		metrics.WebhookDeliveryAttemptsTotal.WithLabelValues("retry").Inc()
		d.logger.Info("webhook delivery scheduled for retry",
			"delivery_id", task.ID,
			"endpoint_id", task.EndpointID,
			"target_host", maskedHost,
			"status_code", res.StatusCode,
			"retry_delay", delay,
		)
	}

	return len(tasks), nil
}
