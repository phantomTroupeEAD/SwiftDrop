package events

import (
	"context"
	"log/slog"
	"sync"

	"github.com/swiftdrop/tracking/internal/dto"
)

const (
	// DefaultBufferSize is the default channel buffer depth allocated per client subscription.
	DefaultBufferSize = 16

	// DefaultMaxSubscribersPerKey limits concurrent subscribers for a single tracking number.
	DefaultMaxSubscribersPerKey = 100

	// DefaultMaxTotalSubscribers limits total concurrent subscriptions across all tracking numbers.
	DefaultMaxTotalSubscribers = 10000
)

type clientSub struct {
	ch     chan dto.PublicStatusUpdateEvent
	once   sync.Once
	closed bool
}

func (s *clientSub) Close() {
	s.once.Do(func() {
		s.closed = true
		close(s.ch)
	})
}

// HubOption configures an EventHub instance.
type HubOption func(*EventHub)

// WithBufferSize overrides the subscriber channel buffer size.
func WithBufferSize(size int) HubOption {
	return func(h *EventHub) {
		if size > 0 {
			h.bufferSize = size
		}
	}
}

// WithMaxSubscribersPerKey sets the maximum subscribers allowed per tracking number.
func WithMaxSubscribersPerKey(max int) HubOption {
	return func(h *EventHub) {
		if max > 0 {
			h.maxSubsPerKey = max
		}
	}
}

// WithMaxTotalSubscribers sets the maximum global subscriptions allowed across all tracking numbers.
func WithMaxTotalSubscribers(max int) HubOption {
	return func(h *EventHub) {
		if max > 0 {
			h.maxTotalSubs = max
		}
	}
}

// WithLogger sets the structured logger for the hub.
func WithLogger(logger *slog.Logger) HubOption {
	return func(h *EventHub) {
		h.logger = logger
	}
}

// EventHub manages in-memory concurrent client subscriptions and backpressure fan-out.
// In Phase 4, it provides an in-memory event source for deterministic testing and local fan-out foundation.
type EventHub struct {
	mu            sync.RWMutex
	subscribers   map[string][]*clientSub
	bufferSize    int
	maxSubsPerKey int
	maxTotalSubs  int
	totalSubs     int
	available     bool
	closed        bool
	logger        *slog.Logger
}

// NewEventHub constructs a new EventHub.
func NewEventHub(options ...HubOption) *EventHub {
	h := &EventHub{
		subscribers:   make(map[string][]*clientSub),
		bufferSize:    DefaultBufferSize,
		maxSubsPerKey: DefaultMaxSubscribersPerKey,
		maxTotalSubs:  DefaultMaxTotalSubscribers,
		available:     true,
	}

	for _, opt := range options {
		opt(h)
	}

	return h
}

// Subscribe registers a subscriber channel for a given tracking number.
// Returns the event channel, an idempotent unsubscription function, or an error.
func (h *EventHub) Subscribe(ctx context.Context, trackingNumber string) (<-chan dto.PublicStatusUpdateEvent, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil, nil, ErrEventSourceClosed
	}

	if !h.available {
		return nil, nil, ErrEventSourceUnavailable
	}

	if h.totalSubs >= h.maxTotalSubs {
		return nil, nil, ErrSubscriptionLimitExceeded
	}

	currentKeySubs := len(h.subscribers[trackingNumber])
	if currentKeySubs >= h.maxSubsPerKey {
		return nil, nil, ErrSubscriptionLimitExceeded
	}

	sub := &clientSub{ch: make(chan dto.PublicStatusUpdateEvent, h.bufferSize)}
	h.subscribers[trackingNumber] = append(h.subscribers[trackingNumber], sub)
	h.totalSubs++

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		channels := h.subscribers[trackingNumber]
		for i, c := range channels {
			if c == sub {
				// Remove channel preserving order
				h.subscribers[trackingNumber] = append(channels[:i], channels[i+1:]...)
				h.totalSubs--
				c.Close()
				break
			}
		}

		if len(h.subscribers[trackingNumber]) == 0 {
			delete(h.subscribers, trackingNumber)
		}
	}

	return sub.ch, unsubscribe, nil
}

// Broadcast distributes an event to all active subscribers for event.TrackingNumber.
// If a subscriber channel buffer overflows (slow client), the stalled connection is terminated
// to trigger the browser EventSource auto-reconnect and database milestone resynchronization,
// rather than silently dropping updates and leaving the client in an inconsistent state.
// Returns the count of successfully queued deliveries.
func (h *EventHub) Broadcast(event dto.PublicStatusUpdateEvent) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed || !h.available {
		return 0
	}

	channels, found := h.subscribers[event.TrackingNumber]
	if !found || len(channels) == 0 {
		return 0
	}

	delivered := 0
	for _, sub := range channels {
		select {
		case sub.ch <- event:
			delivered++
		default:
			// Buffer full for this subscriber: slow client is lagging behind.
			// Terminate the stalled channel to force client reconnect and milestone replay.
			if h.logger != nil {
				h.logger.Warn("Slow client buffer overflowed; terminating subscription for resynchronization",
					"tracking_number", event.TrackingNumber,
					"status", event.Status,
				)
			}
			sub.Close()
		}
	}

	return delivered
}

// IsAvailable reports whether the event source is active.
func (h *EventHub) IsAvailable() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.available && !h.closed
}

// SetAvailable toggles availability for testing or runtime state changes.
func (h *EventHub) SetAvailable(available bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.available = available
}

// SubscriberCount returns the current number of active subscribers for a tracking number.
func (h *EventHub) SubscriberCount(trackingNumber string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers[trackingNumber])
}

// TotalSubscribers returns the total active subscribers across all tracking numbers.
func (h *EventHub) TotalSubscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.totalSubs
}

// Close terminates the hub, marks it unavailable, and cleanly closes all active subscriber channels.
func (h *EventHub) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	h.closed = true
	h.available = false

	for _, channels := range h.subscribers {
		for _, sub := range channels {
			sub.Close()
		}
	}

	h.subscribers = make(map[string][]*clientSub)
	h.totalSubs = 0
	return nil
}
