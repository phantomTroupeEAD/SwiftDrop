package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/repository"
)

var (
	// ErrRabbitMQUnavailable indicates that the RabbitMQ connection is down or unconfigured.
	ErrRabbitMQUnavailable = errors.New("rabbitmq broker unavailable")

	uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// SupportedTrackingEventTypes lists canonical event types relevant to Tracking API projections.
var SupportedTrackingEventTypes = map[string]bool{
	"parcel.booked.v1":           true,
	"parcel.assigned.v1":         true,
	"parcel.picked_up.v1":        true,
	"parcel.in_transit.v1":       true,
	"parcel.out_for_delivery.v1": true,
	"parcel.delivered.v1":        true,
	"parcel.delivery_failed.v1":  true,
	"parcel.returned.v1":         true,
	"parcel.cancelled.v1":        true,
}

// EventEnvelope matches Member 8's canonical event envelope published to RabbitMQ.
type EventEnvelope struct {
	EventID          string          `json:"event_id"`
	EventType        string          `json:"event_type"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateVersion int             `json:"aggregate_version"`
	SchemaVersion    string          `json:"schema_version"`
	OccurredAt       string          `json:"occurred_at"`
	CorrelationID    string          `json:"correlation_id"`
	Producer         string          `json:"producer"`
	Payload          json.RawMessage `json:"payload"`
}

// ParcelStatusChangedPayload matches Member 2's commit 1120834 payload structure.
type ParcelStatusChangedPayload struct {
	ParcelID        string    `json:"parcel_id"`
	TrackingNumber  string    `json:"tracking_number"`
	PreviousStatus  string    `json:"previous_status"`
	NewStatus       string    `json:"new_status"`
	StatusHistoryID string    `json:"status_history_id"`
	ActorID         *string   `json:"actor_id,omitempty"`
	ActorRole       string    `json:"actor_role,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	AttemptCount    int       `json:"delivery_attempts"`
	OccurredAt      time.Time `json:"occurred_at"`
}

// ParcelBookedPayload matches Member 2's initial parcel creation event payload.
type ParcelBookedPayload struct {
	ParcelID       string    `json:"parcel_id"`
	TrackingNumber string    `json:"tracking_number"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// TrackingEventPublisher abstracts the Redis Pub/Sub publishing sink.
type TrackingEventPublisher interface {
	Publish(ctx context.Context, event RedisTrackingEvent) error
}

// AMQPDelivery abstracts an incoming message delivery for testing.
type AMQPDelivery interface {
	GetBody() []byte
	GetDeliveryTag() uint64
	GetHeaders() amqp.Table
	IsRedelivered() bool
	Ack(multiple bool) error
	Nack(multiple, requeue bool) error
}

type realAMQPDelivery struct {
	d amqp.Delivery
}

func (r realAMQPDelivery) GetBody() []byte                   { return r.d.Body }
func (r realAMQPDelivery) GetDeliveryTag() uint64            { return r.d.DeliveryTag }
func (r realAMQPDelivery) GetHeaders() amqp.Table            { return r.d.Headers }
func (r realAMQPDelivery) IsRedelivered() bool               { return r.d.Redelivered }
func (r realAMQPDelivery) Ack(multiple bool) error           { return r.d.Ack(multiple) }
func (r realAMQPDelivery) Nack(multiple, requeue bool) error { return r.d.Nack(multiple, requeue) }

// RabbitMQConsumer consumes real-time parcel lifecycle events from queue q.tracking.updates.
type RabbitMQConsumer struct {
	amqpURL       string
	queueName     string
	prefetchCount int
	reconnectGap  time.Duration
	repo          repository.TrackingRepository
	publisher     TrackingEventPublisher
	dedup         *Deduplicator
	logger        *slog.Logger
	maxRetries    int
	retryMu       sync.Mutex
	retryCounts   map[string]int

	mu       sync.RWMutex
	conn     *amqp.Connection
	ch       *amqp.Channel
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewRabbitMQConsumer constructs a new production RabbitMQ consumer.
func NewRabbitMQConsumer(
	amqpURL string,
	queueName string,
	prefetchCount int,
	reconnectGap time.Duration,
	repo repository.TrackingRepository,
	publisher TrackingEventPublisher,
	logger *slog.Logger,
) *RabbitMQConsumer {
	if queueName == "" {
		queueName = "q.tracking.updates"
	}
	if prefetchCount <= 0 {
		prefetchCount = 10
	}
	if reconnectGap <= 0 {
		reconnectGap = 2 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &RabbitMQConsumer{
		amqpURL:       amqpURL,
		queueName:     queueName,
		prefetchCount: prefetchCount,
		reconnectGap:  reconnectGap,
		repo:          repo,
		publisher:     publisher,
		dedup:         NewDeduplicator(10000, 15*time.Minute),
		logger:        logger,
		stopChan:      make(chan struct{}),
		maxRetries:    3,
		retryCounts:   make(map[string]int),
	}
}

// SetMaxRetries overrides the maximum transient failure retry attempts (default: 3).
func (c *RabbitMQConsumer) SetMaxRetries(max int) {
	c.retryMu.Lock()
	defer c.retryMu.Unlock()
	if max > 0 {
		c.maxRetries = max
	}
}

func (c *RabbitMQConsumer) incrementRetry(eventID string) int {
	c.retryMu.Lock()
	defer c.retryMu.Unlock()
	if c.retryCounts == nil {
		c.retryCounts = make(map[string]int)
	}
	c.retryCounts[eventID]++
	return c.retryCounts[eventID]
}

func (c *RabbitMQConsumer) clearRetry(eventID string) {
	c.retryMu.Lock()
	defer c.retryMu.Unlock()
	if c.retryCounts != nil {
		delete(c.retryCounts, eventID)
	}
}

func extractDeliveryCount(headers amqp.Table) int64 {
	if headers == nil {
		return 0
	}
	val, ok := headers["x-delivery-count"]
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case uint64:
		return int64(v)
	case uint32:
		return int64(v)
	default:
		return 0
	}
}

func extractDeathCount(headers amqp.Table) int64 {
	if headers == nil {
		return 0
	}
	val, ok := headers["x-death"]
	if !ok || val == nil {
		return 0
	}
	deaths, ok := val.([]interface{})
	if !ok {
		return 0
	}
	var total int64
	for _, item := range deaths {
		tbl, ok := item.(amqp.Table)
		if !ok {
			continue
		}
		if countVal, found := tbl["count"]; found {
			switch cv := countVal.(type) {
			case int64:
				total += cv
			case int32:
				total += int64(cv)
			case int:
				total += int64(cv)
			}
		}
	}
	return total
}

func (c *RabbitMQConsumer) handleTransientError(d AMQPDelivery, eventID string, reason string, err error) {
	// 1. Inspect broker-level delivery metadata (authoritative across multiple replicas)
	headers := d.GetHeaders()
	brokerDeliveries := extractDeliveryCount(headers)
	brokerDeaths := extractDeathCount(headers)

	// In RabbitMQ Quorum Queues, x-delivery-count records delivery attempts across all pod replicas.
	if brokerDeliveries >= int64(c.maxRetries) || brokerDeaths >= int64(c.maxRetries) {
		c.logger.Error("broker delivery count exceeded retry ceiling across replicas; routing poison message to DLQ",
			"event_id", eventID,
			"broker_deliveries", brokerDeliveries,
			"broker_deaths", brokerDeaths,
			"max_retries", c.maxRetries,
			"reason", reason,
			"error", err.Error(),
		)
		c.clearRetry(eventID)
		_ = d.Nack(false, false) // Requeue=false sends to DLQ
		return
	}

	// 2. Process-local retry ceiling
	attempts := c.incrementRetry(eventID)
	if attempts > c.maxRetries {
		c.logger.Error("replica retry limit exceeded; routing poison message to DLQ",
			"event_id", eventID,
			"attempts", attempts,
			"max_retries", c.maxRetries,
			"reason", reason,
			"error", err.Error(),
		)
		c.clearRetry(eventID)
		_ = d.Nack(false, false) // requeue=false routes to DLQ
		return
	}

	c.logger.Warn("transient failure encountered; requeueing event for bounded retry",
		"event_id", eventID,
		"attempt", attempts,
		"broker_deliveries", brokerDeliveries,
		"max_retries", c.maxRetries,
		"reason", reason,
		"error", err.Error(),
	)

	// 3. Apply backoff throttle to prevent high-frequency busy-looping during broker or database outages
	time.Sleep(50 * time.Millisecond)

	_ = d.Nack(false, true) // requeue=true
}

// Start launches the consumer reconnection and ingestion loop in the background.
func (c *RabbitMQConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = true
	c.stopChan = make(chan struct{})
	c.mu.Unlock()

	c.wg.Add(1)
	go c.runLoop(ctx)

	c.logger.Info("RabbitMQ consumer started",
		"queue", c.queueName,
		"prefetch", c.prefetchCount,
	)
	return nil
}

func (c *RabbitMQConsumer) runLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		default:
		}

		if strings.TrimSpace(c.amqpURL) == "" {
			c.logger.Warn("RABBITMQ_URL unconfigured; consumer idle")
			select {
			case <-ctx.Done():
				return
			case <-c.stopChan:
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		conn, ch, deliveries, err := c.connectAndConsume(ctx)
		if err != nil {
			c.logger.Warn("RabbitMQ connection failed; will retry",
				"error", err.Error(),
				"retry_in", c.reconnectGap.String(),
			)
			select {
			case <-ctx.Done():
				return
			case <-c.stopChan:
				return
			case <-time.After(c.reconnectGap):
				continue
			}
		}

		c.mu.Lock()
		c.conn = conn
		c.ch = ch
		c.mu.Unlock()

		c.logger.Info("connected to RabbitMQ; consuming events", "queue", c.queueName)

		notifyConnClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		notifyChanClose := ch.NotifyClose(make(chan *amqp.Error, 1))

		active := true
		for active {
			select {
			case <-ctx.Done():
				active = false
			case <-c.stopChan:
				active = false
			case amqpErr := <-notifyConnClose:
				c.logger.Warn("RabbitMQ connection dropped", "error", fmt.Sprintf("%v", amqpErr))
				active = false
			case chErr := <-notifyChanClose:
				c.logger.Warn("RabbitMQ channel dropped", "error", fmt.Sprintf("%v", chErr))
				active = false
			case d, ok := <-deliveries:
				if !ok {
					c.logger.Warn("RabbitMQ delivery channel closed")
					active = false
					break
				}
				c.ProcessDelivery(ctx, realAMQPDelivery{d: d})
			}
		}

		_ = ch.Close()
		_ = conn.Close()

		c.mu.Lock()
		c.ch = nil
		c.conn = nil
		c.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-time.After(c.reconnectGap):
			// Backoff before reconnecting
		}
	}
}

func (c *RabbitMQConsumer) connectAndConsume(ctx context.Context) (*amqp.Connection, *amqp.Channel, <-chan amqp.Delivery, error) {
	conn, err := amqp.Dial(c.amqpURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("dial amqp: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("open amqp channel: %w", err)
	}

	// Apply QoS prefetch ceiling (Agreement §3)
	if err := ch.Qos(c.prefetchCount, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("set amqp qos: %w", err)
	}

	// Consume from pre-declared queue q.tracking.updates
	deliveries, err := ch.Consume(
		c.queueName,
		"",    // consumer tag (auto-generated)
		false, // autoAck = FALSE (explicit ACK/NACK required)
		false, // exclusive = FALSE (competing consumer group)
		false, // noLocal
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("consume queue %s: %w", c.queueName, err)
	}

	return conn, ch, deliveries, nil
}

// ProcessDelivery handles validation, history correlation, idempotency, and fan-out of a single AMQP delivery.
// Exposed for comprehensive unit testing with mock deliveries.
func (c *RabbitMQConsumer) ProcessDelivery(ctx context.Context, d AMQPDelivery) {
	defer func() {
		if rec := recover(); rec != nil {
			c.logger.Error("panic recovered while processing amqp delivery", "panic", fmt.Sprintf("%v", rec))
			_ = d.Nack(false, false) // Requeue=false sends to DLQ
		}
	}()

	body := d.GetBody()

	// 1. Unmarshal Envelope
	var env EventEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		c.logger.Warn("rejecting malformed JSON message to DLQ",
			"delivery_tag", d.GetDeliveryTag(),
			"error", err.Error(),
		)
		_ = d.Nack(false, false)
		return
	}

	// 2. Validate Event Envelope
	if !uuidRegex.MatchString(env.EventID) {
		c.logger.Warn("rejecting envelope with invalid event_id to DLQ", "event_id", env.EventID)
		_ = d.Nack(false, false)
		return
	}

	if env.SchemaVersion != "1.0.0" {
		c.logger.Warn("rejecting envelope with unsupported schema_version to DLQ",
			"event_id", env.EventID,
			"schema_version", env.SchemaVersion,
		)
		_ = d.Nack(false, false)
		return
	}

	if !strings.EqualFold(env.AggregateType, "parcel") {
		// Non-parcel aggregate (e.g. courier-only event): Ack and skip cleanly
		c.logger.Debug("ignoring non-parcel aggregate event",
			"event_id", env.EventID,
			"aggregate_type", env.AggregateType,
		)
		_ = d.Ack(false)
		return
	}

	if !uuidRegex.MatchString(env.AggregateID) {
		c.logger.Warn("rejecting envelope with invalid aggregate_id to DLQ",
			"event_id", env.EventID,
			"aggregate_id", env.AggregateID,
		)
		_ = d.Nack(false, false)
		return
	}

	// 3. Filter for tracking-relevant events
	if !SupportedTrackingEventTypes[env.EventType] {
		c.logger.Debug("ignoring unsupported event type",
			"event_id", env.EventID,
			"event_type", env.EventType,
		)
		_ = d.Ack(false)
		return
	}

	if len(env.Payload) == 0 {
		c.logger.Warn("rejecting envelope with empty payload to DLQ", "event_id", env.EventID)
		_ = d.Nack(false, false)
		return
	}

	// 4. Parse typed payload and extract tracking data
	var (
		trackingNumber  string
		parcelID        string
		status          string
		statusHistoryID string
		occurredAt      time.Time
	)

	if env.EventType == "parcel.booked.v1" {
		var bookedPayload ParcelBookedPayload
		if err := json.Unmarshal(env.Payload, &bookedPayload); err != nil {
			c.logger.Warn("rejecting malformed parcel.booked.v1 payload to DLQ",
				"event_id", env.EventID,
				"error", err.Error(),
			)
			_ = d.Nack(false, false)
			return
		}
		trackingNumber = bookedPayload.TrackingNumber
		parcelID = bookedPayload.ParcelID
		status = bookedPayload.Status
		if status == "" {
			status = "CREATED"
		}
		occurredAt = bookedPayload.CreatedAt
	} else {
		// All 8 lifecycle transitions from Member 2 (commit 1120834)
		var statusPayload ParcelStatusChangedPayload
		if err := json.Unmarshal(env.Payload, &statusPayload); err != nil {
			c.logger.Warn("rejecting malformed parcel status transition payload to DLQ",
				"event_id", env.EventID,
				"error", err.Error(),
			)
			_ = d.Nack(false, false)
			return
		}
		trackingNumber = statusPayload.TrackingNumber
		parcelID = statusPayload.ParcelID
		status = statusPayload.NewStatus
		statusHistoryID = strings.TrimSpace(statusPayload.StatusHistoryID)
		occurredAt = statusPayload.OccurredAt

		// Authoritative contract: status_history_id must be a valid UUID for lifecycle transitions
		if statusHistoryID == "" || !uuidRegex.MatchString(statusHistoryID) {
			c.logger.Warn("rejecting status change event missing valid status_history_id to DLQ",
				"event_id", env.EventID,
				"status_history_id", statusHistoryID,
			)
			_ = d.Nack(false, false)
			return
		}
	}

	// Validate tracking number syntax
	tn, err := domain.NewTrackingNumber(trackingNumber)
	if err != nil {
		c.logger.Warn("rejecting event with invalid tracking_number to DLQ",
			"event_id", env.EventID,
			"tracking_number", trackingNumber,
			"error", err.Error(),
		)
		_ = d.Nack(false, false)
		return
	}

	// Validate parcel status enum
	parsedStatus := domain.ParcelStatus(strings.TrimSpace(status))
	if err := parsedStatus.Validate(); err != nil {
		c.logger.Warn("rejecting event with invalid parcel status to DLQ",
			"event_id", env.EventID,
			"status", status,
			"error", err.Error(),
		)
		_ = d.Nack(false, false)
		return
	}

	if occurredAt.IsZero() {
		// Fallback to envelope occurred_at if payload timestamp is zero
		if parsedTime, err := time.Parse(time.RFC3339Nano, env.OccurredAt); err == nil {
			occurredAt = parsedTime
		} else if parsedTime, err := time.Parse(time.RFC3339, env.OccurredAt); err == nil {
			occurredAt = parsedTime
		} else {
			occurredAt = time.Now().UTC()
		}
	}

	// 5. Idempotency Check (In-memory Deduplication)
	dedupKey := statusHistoryID
	if dedupKey == "" {
		dedupKey = env.EventID
	}
	if c.dedup != nil && c.dedup.Seen(dedupKey) {
		c.logger.Debug("duplicate delivery detected; acknowledging without re-publishing",
			"event_id", env.EventID,
			"dedup_key", dedupKey,
		)
		c.clearRetry(env.EventID)
		_ = d.Ack(false)
		return
	}

	// 6. Correlate with Persisted History (PostgreSQL)
	if c.repo != nil {
		if statusHistoryID != "" {
			// Contract: payload.status_history_id == parcel_status_history.id
			record, err := c.repo.FindStatusHistoryByID(ctx, statusHistoryID)
			if err != nil {
				if errors.Is(err, domain.ErrTrackingNotFound) {
					c.handleTransientError(d, env.EventID, "status_history_id not found in postgres (replication delay)", err)
					return
				}
				c.handleTransientError(d, env.EventID, "database query error correlating status history", err)
				return
			}

			// Validate database record matches payload identifiers
			if record.ParcelID != parcelID && record.ParcelID != env.AggregateID {
				c.logger.Warn("mismatch between status_history record parcel_id and event aggregate_id; routing to DLQ",
					"event_id", env.EventID,
					"record_parcel_id", record.ParcelID,
					"event_parcel_id", parcelID,
				)
				c.clearRetry(env.EventID)
				_ = d.Nack(false, false)
				return
			}
		} else {
			// For parcel.booked.v1 without status_history_id, verify parcel exists
			_, err := c.repo.FindParcelByTrackingNumber(ctx, tn.String())
			if err != nil {
				if errors.Is(err, domain.ErrTrackingNotFound) {
					c.handleTransientError(d, env.EventID, "booked parcel not yet in postgres (replication delay)", err)
					return
				}
				c.handleTransientError(d, env.EventID, "database error verifying booked parcel", err)
				return
			}
		}
	}

	// 7. Dispatch Accepted Event to Redis Pub/Sub Fan-Out
	if c.publisher != nil {
		redisEvent := RedisTrackingEvent{
			EventID:         env.EventID,
			EventType:       env.EventType,
			TrackingNumber:  tn.String(),
			ParcelID:        parcelID,
			Status:          string(parsedStatus),
			StatusHistoryID: statusHistoryID,
			OccurredAt:      occurredAt.UTC(),
		}

		if err := c.publisher.Publish(ctx, redisEvent); err != nil {
			c.handleTransientError(d, env.EventID, "failed to publish event to Redis pubsub", err)
			return
		}
	}

	// 8. Mark seen in deduplicator, clear transient retries, and explicitly ACK
	if c.dedup != nil {
		c.dedup.SeenOrAdd(dedupKey)
		c.dedup.SeenOrAdd(env.EventID)
	}

	c.clearRetry(env.EventID)

	if err := d.Ack(false); err != nil {
		c.logger.Error("failed to ACK delivery", "event_id", env.EventID, "error", err.Error())
		return
	}

	c.logger.Info("successfully processed and distributed tracking update",
		"event_id", env.EventID,
		"event_type", env.EventType,
		"tracking_number", tn.String(),
		"status", string(parsedStatus),
		"status_history_id", statusHistoryID,
	)
}

// Ping checks RabbitMQ connectivity for readiness probes.
func (c *RabbitMQConsumer) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil || c.conn.IsClosed() {
		return ErrRabbitMQUnavailable
	}
	return nil
}

// IsAvailable reports whether the consumer connection is healthy.
func (c *RabbitMQConsumer) IsAvailable() bool {
	return c.Ping(context.Background()) == nil
}

// Close gracefully stops consumption and terminates AMQP connection.
func (c *RabbitMQConsumer) Close() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	close(c.stopChan)

	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.mu.Unlock()

	c.wg.Wait()
	return nil
}
