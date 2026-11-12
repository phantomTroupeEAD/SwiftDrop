package events

import "errors"

var (
	// ErrMissingTransaction is returned when OutboxWriter.Write is called with a nil tx.
	ErrMissingTransaction = errors.New("events: missing transaction (*sql.Tx required)")

	// ErrInvalidEventType is returned when event_type does not match ^[a-z_]+\.[a-z_]+\.v[0-9]+$.
	ErrInvalidEventType = errors.New("events: invalid event_type (must match ^[a-z_]+\\.[a-z_]+\\.v[0-9]+$)")

	// ErrMissingCorrelationID is returned when correlation_id is empty or exceeds 128 characters.
	ErrMissingCorrelationID = errors.New("events: missing or invalid correlation_id (must be non-empty and <= 128 chars)")

	// ErrInvalidAggregateID is returned when aggregate_id is not a valid UUID.
	ErrInvalidAggregateID = errors.New("events: invalid aggregate_id (must be a valid UUID)")

	// ErrInvalidAggregateType is returned when aggregate_type is not one of parcel, courier, merchant, or dispatch.
	ErrInvalidAggregateType = errors.New("events: invalid aggregate_type (must be parcel, courier, merchant, or dispatch)")

	// ErrInvalidAggregateVersion is returned when aggregate_version is less than 1.
	ErrInvalidAggregateVersion = errors.New("events: invalid aggregate_version (must be >= 1)")

	// ErrInvalidEventID is returned when event_id is not a valid UUID.
	ErrInvalidEventID = errors.New("events: invalid event_id (must be a valid UUID)")

	// ErrNilPayload is returned when payload is nil or serializes to empty/invalid JSON.
	ErrNilPayload = errors.New("events: payload cannot be nil or serialize to empty JSON")

	// ErrEmptyEnvelopes is returned when no envelopes are provided to Write.
	ErrEmptyEnvelopes = errors.New("events: no envelopes provided to Write")
)
