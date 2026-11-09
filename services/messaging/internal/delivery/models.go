package delivery

import (
	"encoding/json"
	"time"
)

// NotificationTask represents a claimed customer notification delivery row.
type NotificationTask struct {
	ID               string          `json:"id"`
	EventID          string          `json:"event_id"`
	EventType        string          `json:"event_type"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateVersion int             `json:"aggregate_version"`
	Payload          json.RawMessage `json:"payload"`
	Recipient        string          `json:"recipient"`
	Channel          string          `json:"channel"`
	TemplateName     string          `json:"template_name"`
	AttemptCount     int             `json:"attempt_count"`
	Status           string          `json:"status,omitempty"`
	LastError        string          `json:"last_error,omitempty"`
}

// WebhookTask represents a claimed merchant webhook delivery row.
type WebhookTask struct {
	ID               string          `json:"id"`
	EndpointID       string          `json:"endpoint_id"`
	EventID          string          `json:"event_id"`
	EventType        string          `json:"event_type"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateVersion int             `json:"aggregate_version"`
	OccurredAt       time.Time       `json:"occurred_at"`
	Payload          json.RawMessage `json:"payload"`
	AttemptCount     int             `json:"attempt_count"`
}

// WebhookAttemptRecord represents an entry in webhook_delivery_attempts audit table.
type WebhookAttemptRecord struct {
	DeliveryID          string
	AttemptNo           int
	StartedAt           time.Time
	ResponseStatusCode  int
	ResponseBodySnippet string
	ErrorClass          string
	ErrorMessage        string
	NextAttemptAt       *time.Time
}
