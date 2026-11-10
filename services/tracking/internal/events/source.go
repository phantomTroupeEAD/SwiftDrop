package events

import (
	"context"
	"errors"

	"github.com/swiftdrop/tracking/internal/dto"
)

var (
	// ErrEventSourceUnavailable indicates that the event transport/broker is unreachable or unconfigured.
	ErrEventSourceUnavailable = errors.New("event source unavailable")

	// ErrSubscriptionLimitExceeded indicates that the maximum allowable concurrent subscriptions has been reached.
	ErrSubscriptionLimitExceeded = errors.New("subscription limit exceeded")

	// ErrEventSourceClosed indicates that the event source has been terminated.
	ErrEventSourceClosed = errors.New("event source closed")
)

// TrackingEventSource defines the narrow interface for subscribing to parcel tracking events.
// Decoupled from transport (Redis Pub/Sub, RabbitMQ, in-memory test fixtures).
type TrackingEventSource interface {
	// Subscribe registers a listener for real-time status updates for a specific tracking number.
	// Returns a channel yielding events and an idempotent cleanup function to unsubscribe.
	Subscribe(ctx context.Context, trackingNumber string) (<-chan dto.PublicStatusUpdateEvent, func(), error)

	// IsAvailable indicates whether the underlying event broker/transport is active and healthy.
	IsAvailable() bool
}
