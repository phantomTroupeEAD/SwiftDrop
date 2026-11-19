package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
)

var (
	// ErrRedisUnavailable indicates that Redis is unreachable or closed.
	ErrRedisUnavailable = errors.New("redis pubsub unavailable")
)

// RedisTrackingEvent represents the distributed real-time event wire schema
// published to and received from Redis Pub/Sub channel "tracking:updates".
type RedisTrackingEvent struct {
	EventID         string    `json:"event_id"`
	EventType       string    `json:"event_type"`
	TrackingNumber  string    `json:"tracking_number"`
	ParcelID        string    `json:"parcel_id"`
	Status          string    `json:"status"`
	StatusHistoryID string    `json:"status_history_id,omitempty"`
	OccurredAt      time.Time `json:"occurred_at"`
	SourceInstance  string    `json:"source_instance,omitempty"`
}

// RedisClientInterface abstracts go-redis operations for testability.
type RedisClientInterface interface {
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	Ping(ctx context.Context) *redis.StatusCmd
	Close() error
}

// RedisPubSub coordinates horizontal multi-pod event fan-out across Tracking API replicas.
type RedisPubSub struct {
	client       RedisClientInterface
	channelName  string
	hub          *EventHub
	dedup        *Deduplicator
	logger       *slog.Logger
	instanceID   string
	reconnectGap time.Duration

	mu       sync.RWMutex
	pubsub   *redis.PubSub
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewRedisPubSub constructs a new RedisPubSub distributor.
func NewRedisPubSub(
	client RedisClientInterface,
	channelName string,
	hub *EventHub,
	logger *slog.Logger,
) *RedisPubSub {
	if channelName == "" {
		channelName = "tracking:updates"
	}
	if logger == nil {
		logger = slog.Default()
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = fmt.Sprintf("pod-%d", time.Now().UnixNano())
	}

	return &RedisPubSub{
		client:       client,
		channelName:  channelName,
		hub:          hub,
		dedup:        NewDeduplicator(10000, 15*time.Minute),
		logger:       logger,
		instanceID:   hostname,
		reconnectGap: 1 * time.Second,
		stopChan:     make(chan struct{}),
	}
}

// Publish broadcasts an approved tracking update across all Tracking API replicas via Redis Pub/Sub.
func (r *RedisPubSub) Publish(ctx context.Context, event RedisTrackingEvent) error {
	if r.client == nil {
		return ErrRedisUnavailable
	}

	if event.SourceInstance == "" {
		event.SourceInstance = r.instanceID
	}

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal redis tracking event: %w", err)
	}

	cmd := r.client.Publish(ctx, r.channelName, payloadBytes)
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("%w: failed to publish to channel %s: %v", ErrRedisUnavailable, r.channelName, err)
	}

	// Mark in local dedup ONLY after publication actually succeeds
	dedupKey := event.StatusHistoryID
	if dedupKey == "" {
		dedupKey = event.EventID
	}
	if dedupKey != "" {
		r.dedup.SeenOrAdd(dedupKey)
	}

	// If the publishing pod has local SSE subscribers for this parcel, deliver directly to local hub
	if r.hub != nil {
		update := dto.PublicStatusUpdateEvent{
			TrackingNumber:  event.TrackingNumber,
			Status:          event.Status,
			Description:     domain.SafeStatusDescription(event.Status),
			Timestamp:       event.OccurredAt.UTC(),
			StatusHistoryID: event.StatusHistoryID,
		}
		r.hub.Broadcast(update)
	}

	return nil
}

// StartListening launches the background Redis subscription loop.
func (r *RedisPubSub) StartListening(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = true
	r.stopChan = make(chan struct{})
	r.mu.Unlock()

	r.wg.Add(1)
	go r.listenLoop(ctx)

	r.logger.Info("Redis Pub/Sub subscription started", "channel", r.channelName, "instance", r.instanceID)
	return nil
}

func (r *RedisPubSub) listenLoop(ctx context.Context) {
	defer r.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		default:
		}

		if r.client == nil {
			r.logger.Warn("Redis client nil; delaying reconnection attempt")
			select {
			case <-ctx.Done():
				return
			case <-r.stopChan:
				return
			case <-time.After(r.reconnectGap):
				continue
			}
		}

		// Subscribe to Redis channel
		pubsub := r.client.Subscribe(ctx, r.channelName)
		r.mu.Lock()
		r.pubsub = pubsub
		r.mu.Unlock()

		msgChan := pubsub.Channel()
		r.logger.Debug("subscribed to Redis channel", "channel", r.channelName)

		shouldReconnect := false
		for !shouldReconnect {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return
			case <-r.stopChan:
				_ = pubsub.Close()
				return
			case msg, ok := <-msgChan:
				if !ok {
					r.logger.Warn("Redis pubsub channel closed, reconnecting...", "channel", r.channelName)
					shouldReconnect = true
					break
				}
				r.handleRedisMessage(msg.Payload)
			}
		}

		_ = pubsub.Close()

		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case <-time.After(r.reconnectGap):
			// Backoff before reconnecting
		}
	}
}

func (r *RedisPubSub) handleRedisMessage(payload string) {
	var event RedisTrackingEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		r.logger.Warn("received malformed message from Redis pubsub", "payload", payload, "error", err.Error())
		return
	}

	tn, err := domain.NewTrackingNumber(event.TrackingNumber)
	if err != nil {
		r.logger.Warn("invalid tracking number in Redis message", "tracking_number", event.TrackingNumber)
		return
	}

	status := domain.ParcelStatus(strings.TrimSpace(event.Status))
	if err := status.Validate(); err != nil {
		r.logger.Warn("invalid status in Redis message", "status", event.Status)
		return
	}

	// Instance loopback suppression: if published by this pod instance,
	// it was already delivered directly to local subscribers during Publish().
	if event.SourceInstance != "" && event.SourceInstance == r.instanceID {
		return
	}

	// Deduplication: check if already delivered locally (e.g. from local publish or duplicate broadcast)
	dedupKey := event.StatusHistoryID
	if dedupKey == "" {
		dedupKey = event.EventID
	}
	if dedupKey != "" && r.dedup.SeenOrAdd(dedupKey) {
		r.logger.Debug("skipping duplicate redis event", "dedup_key", dedupKey, "tracking_number", tn.String())
		return
	}

	if r.hub == nil {
		return
	}

	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	update := dto.PublicStatusUpdateEvent{
		TrackingNumber:  tn.String(),
		Status:          string(status),
		Description:     domain.SafeStatusDescription(string(status)),
		Timestamp:       occurredAt.UTC(),
		StatusHistoryID: event.StatusHistoryID,
	}

	r.hub.Broadcast(update)
}

// Ping checks Redis connectivity for health/readiness probes.
func (r *RedisPubSub) Ping(ctx context.Context) error {
	if r.client == nil {
		return ErrRedisUnavailable
	}
	status := r.client.Ping(ctx)
	if err := status.Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrRedisUnavailable, err)
	}
	return nil
}

// IsAvailable reports whether Redis is operational.
func (r *RedisPubSub) IsAvailable() bool {
	if r.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.Ping(ctx) == nil
}

// Close gracefully shuts down the subscription loop and terminates the pubsub connection.
func (r *RedisPubSub) Close() error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	close(r.stopChan)

	if r.pubsub != nil {
		_ = r.pubsub.Close()
	}
	r.mu.Unlock()

	r.wg.Wait()

	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
