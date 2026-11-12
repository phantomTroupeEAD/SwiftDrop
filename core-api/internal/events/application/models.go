package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

var (
	ErrNotFound           = errors.New("events: resource not found")
	ErrInvalidState       = errors.New("events: resource is not in FAILED state and cannot be retried")
	ErrTenantUnauthorized = errors.New("events: tenant is not authorized to access this resource")
)

// OutboxFilter defines criteria for querying outbox events.
type OutboxFilter struct {
	Status    string
	EventType string
	From      *time.Time
	To        *time.Time
}

// OutboxItem represents a recorded event in outbox_events.
type OutboxItem struct {
	ID               string          `json:"id"`
	EventID          string          `json:"event_id"`
	EventType        string          `json:"event_type"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateVersion int             `json:"aggregate_version"`
	SchemaVersion    string          `json:"schema_version"`
	OccurredAt       time.Time       `json:"occurred_at"`
	CorrelationID    string          `json:"correlation_id"`
	Producer         string          `json:"producer"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	Status           string          `json:"status"`
	AttemptCount     int             `json:"attempt_count"`
	LastError        *string         `json:"last_error,omitempty"`
	NextAttemptAt    *time.Time      `json:"next_attempt_at,omitempty"`
	PublishedAt      *time.Time      `json:"published_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

// OutboxStats represents statistical summary counts for outbox events.
type OutboxStats struct {
	PendingCount            int64   `json:"pending_count"`
	PublishedCount          int64   `json:"published_count"`
	FailedCount             int64   `json:"failed_count"`
	OldestPendingAgeSeconds float64 `json:"oldest_pending_age_seconds"`
}

// DeliveryFilter defines criteria for querying deliveries.
type DeliveryFilter struct {
	Kind   string // "notification" or "webhook"
	Status string // "PENDING", "IN_PROGRESS", "SUCCESS", "FAILED"
}

// UnifiedDeliveryItem represents a delivery task from either notifications or webhooks.
type UnifiedDeliveryItem struct {
	ID                  string     `json:"id"`
	Kind                string     `json:"kind"`                  // "notification" or "webhook"
	EndpointOrRecipient string     `json:"endpoint_or_recipient"` // Masked email or endpoint UUID
	EventID             string     `json:"event_id"`
	EventType           string     `json:"event_type"`
	Status              string     `json:"status"`
	AttemptCount        int        `json:"attempt_count"`
	NextAttemptAt       *time.Time `json:"next_attempt_at,omitempty"`
	LastError           *string    `json:"last_error,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	DeliveredAt         *time.Time `json:"delivered_at,omitempty"`
}

// MerchantDeliveryItem represents a merchant-owned webhook delivery.
type MerchantDeliveryItem struct {
	ID                  string     `json:"id"`
	EndpointID          string     `json:"endpoint_id"`
	EventID             string     `json:"event_id"`
	EventType           string     `json:"event_type"`
	Status              string     `json:"status"`
	AttemptCount        int        `json:"attempt_count"`
	NextAttemptAt       *time.Time `json:"next_attempt_at,omitempty"`
	ResponseStatusCode  *int       `json:"response_status_code,omitempty"`
	ResponseBodySnippet *string    `json:"response_body_snippet,omitempty"`
	LastError           *string    `json:"last_error,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	DeliveredAt         *time.Time `json:"delivered_at,omitempty"`
}

// WebhookAttemptItem represents an individual HTTP delivery attempt audit record.
type WebhookAttemptItem struct {
	AttemptNo           int        `json:"attempt_no"`
	StartedAt           time.Time  `json:"started_at"`
	ResponseStatusCode  *int       `json:"response_status_code,omitempty"`
	ResponseBodySnippet *string    `json:"response_body_snippet,omitempty"`
	ErrorClass          *string    `json:"error_class,omitempty"`
	ErrorMessage        *string    `json:"error_message,omitempty"`
	NextAttemptAt       *time.Time `json:"next_attempt_at,omitempty"`
}

// EventsRepository abstracts read-model queries and retry mutations.
type EventsRepository interface {
	ListOutbox(ctx context.Context, filter OutboxFilter, cursor httpx.Cursor, limit int) ([]OutboxItem, *httpx.Cursor, error)
	GetOutboxStats(ctx context.Context) (*OutboxStats, error)
	RetryOutbox(ctx context.Context, id string) error

	ListDeliveries(ctx context.Context, filter DeliveryFilter, cursor httpx.Cursor, limit int) ([]UnifiedDeliveryItem, *httpx.Cursor, error)
	RetryWebhookDelivery(ctx context.Context, id string) error

	ListMerchantWebhookDeliveries(ctx context.Context, merchantID string, cursor httpx.Cursor, limit int) ([]MerchantDeliveryItem, *httpx.Cursor, error)
	ListMerchantWebhookAttempts(ctx context.Context, merchantID, deliveryID string) ([]WebhookAttemptItem, error)
}
