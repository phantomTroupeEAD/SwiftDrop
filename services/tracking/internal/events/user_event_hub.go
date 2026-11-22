package events

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/swiftdrop/tracking/internal/dto"
)

var (
	// ErrUserHubKeyCapacityExceeded indicates too many active subscriptions for one user key.
	ErrUserHubKeyCapacityExceeded = errors.New("user event hub subscriber limit exceeded for key")

	// ErrUserHubTotalCapacityExceeded indicates global capacity reached.
	ErrUserHubTotalCapacityExceeded = errors.New("user event hub total subscriber limit reached")
)

type userClientSub struct {
	mu        sync.RWMutex
	ch        chan dto.InAppNotificationDTO
	closed    bool
	uncounted atomic.Bool
}

func (s *userClientSub) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

func (s *userClientSub) TrySend(event dto.InAppNotificationDTO) (delivered bool, closed bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return false, true
	}

	select {
	case s.ch <- event:
		return true, false
	default:
		return false, false // channel buffer full (slow consumer)
	}
}

// UserEventHub coordinates concurrent in-memory real-time notification streaming by user ID and role keys.
type UserEventHub struct {
	mu            sync.RWMutex
	subscribers   map[string][]*userClientSub
	totalSubs     atomic.Int64
	maxSubsPerKey int
	maxTotalSubs  int
	bufferSize    int
	logger        *slog.Logger
}

// NewUserEventHub constructs a new UserEventHub.
func NewUserEventHub(logger *slog.Logger) *UserEventHub {
	if logger == nil {
		logger = slog.Default()
	}
	return &UserEventHub{
		subscribers:   make(map[string][]*userClientSub),
		maxSubsPerKey: 20,
		maxTotalSubs:  5000,
		bufferSize:    16,
		logger:        logger,
	}
}

// Subscribe registers a client channel to receive events for the provided subscription keys (e.g. user_id, role:DISPATCHER).
// Returns the event channel, an idempotent unsubscribe function, and any capacity errors.
func (h *UserEventHub) Subscribe(ctx context.Context, keys ...string) (<-chan dto.InAppNotificationDTO, func(), error) {
	if len(keys) == 0 {
		return nil, nil, errors.New("at least one subscription key is required")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if int(h.totalSubs.Load()) >= h.maxTotalSubs {
		return nil, nil, ErrUserHubTotalCapacityExceeded
	}

	for _, k := range keys {
		if len(h.subscribers[k]) >= h.maxSubsPerKey {
			return nil, nil, ErrUserHubKeyCapacityExceeded
		}
	}

	sub := &userClientSub{
		ch: make(chan dto.InAppNotificationDTO, h.bufferSize),
	}
	h.totalSubs.Add(1)

	for _, k := range keys {
		h.subscribers[k] = append(h.subscribers[k], sub)
	}

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()

			for _, k := range keys {
				list := h.subscribers[k]
				for i, existing := range list {
					if existing == sub {
						h.subscribers[k] = append(list[:i], list[i+1:]...)
						break
					}
				}
				if len(h.subscribers[k]) == 0 {
					delete(h.subscribers, k)
				}
			}

			if !sub.uncounted.Swap(true) {
				h.totalSubs.Add(-1)
			}
			sub.Close()
		})
	}

	return sub.ch, unsub, nil
}

// Broadcast dispatches a notification to all active subscribers for the specified key.
func (h *UserEventHub) Broadcast(key string, event dto.InAppNotificationDTO) {
	h.mu.RLock()
	subs := make([]*userClientSub, len(h.subscribers[key]))
	copy(subs, h.subscribers[key])
	h.mu.RUnlock()

	for _, sub := range subs {
		sub.TrySend(event)
	}
}

// BroadcastMulti dispatches a notification to all subscribers matching any of the provided keys.
func (h *UserEventHub) BroadcastMulti(keys []string, event dto.InAppNotificationDTO) {
	seen := make(map[*userClientSub]bool)

	h.mu.RLock()
	var targets []*userClientSub
	for _, k := range keys {
		for _, s := range h.subscribers[k] {
			if !seen[s] {
				seen[s] = true
				targets = append(targets, s)
			}
		}
	}
	h.mu.RUnlock()

	for _, sub := range targets {
		sub.TrySend(event)
	}
}

// ActiveSubscriberCount returns total live subscriptions.
func (h *UserEventHub) ActiveSubscriberCount() int {
	return int(h.totalSubs.Load())
}
