package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/messaging/internal/metrics"
	"github.com/swiftdrop/messaging/internal/relay"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// MessageHandler processes a parsed and validated domain event envelope.
type MessageHandler func(ctx context.Context, d amqp.Delivery, env *relay.OutboxEnvelope) error

// Runner manages consumption of messages from an AMQP queue delivery channel.
type Runner struct {
	name    string
	handler MessageHandler
	logger  *slog.Logger
}

// NewRunner creates a new consumer runner.
func NewRunner(name string, handler MessageHandler, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		name:    name,
		handler: handler,
		logger:  logger,
	}
}

// Start consumes messages from delivery channel until ctx is cancelled or channel closes.
func (r *Runner) Start(ctx context.Context, deliveries <-chan amqp.Delivery) error {
	r.logger.Info("starting consumer runner", "consumer", r.name)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("consumer runner stopping", "consumer", r.name)
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				r.logger.Info("delivery channel closed", "consumer", r.name)
				return nil
			}
			r.handleDelivery(ctx, d)
		}
	}
}

func (r *Runner) handleDelivery(ctx context.Context, d amqp.Delivery) {
	defer func() {
		if rec := recover(); rec != nil {
			r.logger.Error("panic recovered in consumer runner",
				"consumer", r.name,
				"panic", fmt.Sprintf("%v", rec),
			)
			metrics.ConsumerFailuresTotal.WithLabelValues(r.name).Inc()
			metrics.DLQMessagesRoutedTotal.Inc()
			_ = d.Nack(false, false) // Requeue=false routes to DLQ
		}
	}()

	var env relay.OutboxEnvelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		r.logger.Warn("rejecting malformed JSON message to DLQ",
			"consumer", r.name,
			"delivery_tag", d.DeliveryTag,
			"error", err.Error(),
		)
		metrics.DLQMessagesRoutedTotal.Inc()
		_ = d.Nack(false, false)
		return
	}

	// Validate required fields
	if !uuidRegex.MatchString(env.EventID) || env.EventType == "" || !uuidRegex.MatchString(env.AggregateID) || len(env.Payload) == 0 {
		r.logger.Warn("rejecting envelope with invalid required fields to DLQ",
			"consumer", r.name,
			"event_id", env.EventID,
			"event_type", env.EventType,
			"aggregate_id", env.AggregateID,
		)
		metrics.DLQMessagesRoutedTotal.Inc()
		_ = d.Nack(false, false)
		return
	}

	if err := r.handler(ctx, d, &env); err != nil {
		r.logger.Error("handler error processing message",
			"consumer", r.name,
			"event_id", env.EventID,
			"error", err.Error(),
		)
		metrics.ConsumerFailuresTotal.WithLabelValues(r.name).Inc()
		// If handler failed to stage row, Nack without requeue to DLQ to avoid tight loop
		metrics.DLQMessagesRoutedTotal.Inc()
		_ = d.Nack(false, false)
		return
	}

	_ = d.Ack(false)
}
