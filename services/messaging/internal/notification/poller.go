package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/swiftdrop/messaging/internal/config"
	"github.com/swiftdrop/messaging/internal/delivery"
	"github.com/swiftdrop/messaging/internal/logging"
	"github.com/swiftdrop/messaging/internal/metrics"
)

// Poller polls pending notification delivery rows and dispatches them via SMTP.
type Poller struct {
	repo   delivery.Repository
	sender EmailSender
	calc   *delivery.BackoffCalculator
	cfg    *config.Config
	logger *slog.Logger
}

// NewPoller initializes a new notification poller.
func NewPoller(repo delivery.Repository, sender EmailSender, calc *delivery.BackoffCalculator, cfg *config.Config, logger *slog.Logger) *Poller {
	if logger == nil {
		logger = slog.Default()
	}
	if calc == nil {
		calc = delivery.NewBackoffCalculator(cfg.WebhookRetrySchedule, nil)
	}
	return &Poller{
		repo:   repo,
		sender: sender,
		calc:   calc,
		cfg:    cfg,
		logger: logger,
	}
}

// Run starts the polling loop until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) error {
	p.logger.Info("starting notification delivery poller")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("notification delivery poller stopping")
			return ctx.Err()
		case <-ticker.C:
			processed, err := p.PollAndSend(ctx)
			if err != nil {
				p.logger.Error("notification poller error", "error", err.Error())
			} else if processed > 0 {
				p.logger.Debug("processed notification batch", "count", processed)
			}
		}
	}
}

// PollAndSend claims due notification rows and delivers them via SMTP.
func (p *Poller) PollAndSend(ctx context.Context) (int, error) {
	leaseDuration := time.Duration(p.cfg.DeliveryLeaseSeconds) * time.Second
	tasks, err := p.repo.ClaimNotifications(ctx, 20, leaseDuration)
	if err != nil {
		return 0, fmt.Errorf("failed to claim notifications: %w", err)
	}

	if len(tasks) == 0 {
		return 0, nil
	}

	for _, task := range tasks {
		data := ExtractMilestoneData(task.Payload)
		subject, textBody, htmlBody, err := RenderNotification(task.EventType, data)
		if err != nil {
			p.logger.Error("failed to render notification template",
				"id", task.ID,
				"event_type", task.EventType,
				"error", err.Error(),
			)
			_ = p.repo.RecordNotificationTerminalFailure(ctx, task.ID, task.EventID, fmt.Sprintf("template render: %v", err))
			metrics.NotificationAttemptsTotal.WithLabelValues("failed").Inc()
			continue
		}

		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		providerMsgID, sendErr := p.sender.Send(sendCtx, task.Recipient, subject, textBody, htmlBody)
		cancel()

		maskedEmail := logging.MaskEmail(task.Recipient)

		if sendErr != nil {
			p.logger.Warn("smtp send failed",
				"id", task.ID,
				"recipient", maskedEmail,
				"attempt", task.AttemptCount,
				"error", sendErr.Error(),
			)

			if IsTerminalSMTPError(sendErr) || task.AttemptCount >= p.calc.MaxAttempts() {
				_ = p.repo.RecordNotificationTerminalFailure(ctx, task.ID, task.EventID, sendErr.Error())
				metrics.NotificationAttemptsTotal.WithLabelValues("failed").Inc()
			} else {
				delay := p.calc.CalculateDelay(task.AttemptCount)
				nextAttemptAt := time.Now().Add(delay)
				_ = p.repo.RecordNotificationRetry(ctx, task.ID, sendErr.Error(), nextAttemptAt)
				metrics.NotificationAttemptsTotal.WithLabelValues("retry").Inc()
			}
			continue
		}

		if err := p.repo.RecordNotificationSuccess(ctx, task.ID, task.EventID, providerMsgID); err != nil {
			p.logger.Error("failed to record notification success", "id", task.ID, "error", err.Error())
			continue
		}

		metrics.NotificationAttemptsTotal.WithLabelValues("success").Inc()
		p.logger.Info("notification email delivered successfully",
			"id", task.ID,
			"recipient", maskedEmail,
			"provider_msg_id", providerMsgID,
		)
	}

	return len(tasks), nil
}
