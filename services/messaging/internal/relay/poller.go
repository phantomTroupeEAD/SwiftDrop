package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/messaging/internal/broker"
	"github.com/swiftdrop/messaging/internal/config"
	"github.com/swiftdrop/messaging/internal/metrics"
)

// Publisher abstracts the AMQP publishing operation.
type Publisher interface {
	Publish(ctx context.Context, exchange, routingKey string, msg amqp.Publishing) error
}

// Relay polls pending outbox records and publishes them to RabbitMQ.
type Relay struct {
	repo      OutboxRepository
	publisher Publisher
	cfg       *config.Config
	logger    *slog.Logger

	rngMu sync.Mutex
	rng   *rand.Rand
}

// NewRelay constructs a new outbox relay.
func NewRelay(repo OutboxRepository, publisher Publisher, cfg *config.Config, logger *slog.Logger) *Relay {
	if logger == nil {
		logger = slog.Default()
	}
	return &Relay{
		repo:      repo,
		publisher: publisher,
		cfg:       cfg,
		logger:    logger,
		//nolint:gosec // math/rand is used exclusively for non-cryptographic poll interval jitter
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetRNG sets a custom random generator for deterministic testing.
func (r *Relay) SetRNG(rng *rand.Rand) {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	r.rng = rng
}

// Run starts the main polling loop and metrics background updater.
func (r *Relay) Run(ctx context.Context) error {
	leaseDuration := time.Duration(r.cfg.OutboxLeaseSeconds) * time.Second
	r.logger.Info("starting outbox relay daemon",
		"poll_interval", r.cfg.OutboxPollInterval,
		"batch_size", r.cfg.OutboxBatchSize,
		"lease_duration", leaseDuration,
	)

	// Start metrics updater
	go r.startMetricsUpdater(ctx)

	ticker := time.NewTicker(r.cfg.OutboxPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("outbox relay stopping on context cancellation")
			return ctx.Err()
		case <-ticker.C:
			processed, err := r.PollAndPublish(ctx)
			if err != nil {
				r.logger.Error("outbox relay poll error", "error", err.Error())
			} else if processed > 0 {
				r.logger.Debug("processed outbox batch", "count", processed)
			}
		}
	}
}

// PollAndPublish claims a batch of events and publishes them with publisher confirms.
func (r *Relay) PollAndPublish(ctx context.Context) (int, error) {
	leaseDuration := time.Duration(r.cfg.OutboxLeaseSeconds) * time.Second
	records, err := r.repo.ClaimBatch(ctx, r.cfg.OutboxBatchSize, leaseDuration)
	if err != nil {
		return 0, fmt.Errorf("failed to claim outbox batch: %w", err)
	}

	if len(records) == 0 {
		return 0, nil
	}

	metrics.OutboxPolledTotal.Add(float64(len(records)))

	for _, rec := range records {
		envelope := rec.ToEnvelope()
		bodyBytes, err := json.Marshal(envelope)
		if err != nil {
			r.logger.Error("failed to marshal outbox envelope", "event_id", rec.EventID, "error", err.Error())
			r.handleFailure(ctx, rec, fmt.Errorf("marshal envelope: %w", err))
			continue
		}

		msg := amqp.Publishing{
			MessageId:    rec.EventID,
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Headers: amqp.Table{
				"event_id":       rec.EventID,
				"correlation_id": rec.CorrelationID,
			},
			Body: bodyBytes,
		}

		pubErr := r.publisher.Publish(ctx, broker.TopicExchange, rec.EventType, msg)
		if pubErr != nil {
			r.logger.Warn("failed to publish outbox event to amqp",
				"event_id", rec.EventID,
				"event_type", rec.EventType,
				"error", pubErr.Error(),
			)
			r.handleFailure(ctx, rec, pubErr)
			continue
		}

		// Mark successful publish
		if err := r.repo.MarkPublished(ctx, rec.ID); err != nil {
			r.logger.Error("failed to mark outbox event published", "id", rec.ID, "event_id", rec.EventID, "error", err.Error())
			continue
		}
		metrics.OutboxPublishedTotal.Inc()
	}

	return len(records), nil
}

func (r *Relay) handleFailure(ctx context.Context, rec EventRecord, pubErr error) {
	metrics.OutboxPublishFailuresTotal.Inc()

	newAttempt := rec.AttemptCount + 1
	finalFailure := newAttempt >= 5

	backoff := r.computeBackoff(rec.AttemptCount)
	nextAttemptAt := time.Now().Add(backoff)

	if err := r.repo.RecordFailure(ctx, rec.ID, pubErr.Error(), nextAttemptAt, finalFailure); err != nil {
		r.logger.Error("failed to record outbox failure", "id", rec.ID, "error", err.Error())
	}
}

// computeBackoff calculates exponential backoff: 1s, 2s, 4s, 8s, 16s with ±20% jitter.
func (r *Relay) computeBackoff(attempt int) time.Duration {
	baseDelays := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}

	idx := attempt
	if idx >= len(baseDelays) {
		idx = len(baseDelays) - 1
	}
	base := baseDelays[idx]

	r.rngMu.Lock()
	// ±20% jitter: range [0.8, 1.2]
	jitterFactor := 0.8 + (r.rng.Float64() * 0.4)
	r.rngMu.Unlock()

	return time.Duration(float64(base) * jitterFactor)
}

func (r *Relay) startMetricsUpdater(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pending, oldestAge, err := r.repo.GetOutboxStats(ctx)
			if err != nil {
				r.logger.Debug("failed to update outbox metrics", "error", err.Error())
				continue
			}
			metrics.OutboxPendingEvents.Set(float64(pending))
			metrics.OutboxOldestPendingAgeSeconds.Set(oldestAge)
		}
	}
}
