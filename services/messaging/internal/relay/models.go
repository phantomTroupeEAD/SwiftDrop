package relay

import (
	"encoding/json"
	"time"
)

// EventRecord represents a staged outbox row in the database.
type EventRecord struct {
	ID               string
	EventID          string
	EventType        string
	AggregateType    string
	AggregateID      string
	AggregateVersion int
	SchemaVersion    string
	OccurredAt       time.Time
	CorrelationID    string
	Producer         string
	Payload          json.RawMessage
	AttemptCount     int
}

// OutboxEnvelope represents the full event envelope published to the AMQP broker.
type OutboxEnvelope struct {
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

// ToEnvelope converts an EventRecord to an OutboxEnvelope.
func (r *EventRecord) ToEnvelope() OutboxEnvelope {
	return OutboxEnvelope{
		EventID:          r.EventID,
		EventType:        r.EventType,
		AggregateType:    r.AggregateType,
		AggregateID:      r.AggregateID,
		AggregateVersion: r.AggregateVersion,
		SchemaVersion:    r.SchemaVersion,
		OccurredAt:       r.OccurredAt.UTC().Format(time.RFC3339Nano),
		CorrelationID:    r.CorrelationID,
		Producer:         r.Producer,
		Payload:          r.Payload,
	}
}
