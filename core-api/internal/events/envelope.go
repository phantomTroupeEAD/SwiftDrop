package events

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	eventTypeRegex = regexp.MustCompile(`^[a-z_]+\.[a-z_]+\.v[0-9]+$`)
	uuidRegex      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// AllowedAggregateTypes defines the strict domain bounded context entities that may emit events.
var AllowedAggregateTypes = map[string]bool{
	"parcel":   true,
	"courier":  true,
	"merchant": true,
	"dispatch": true,
}

// AnyEnvelope defines the universal metadata interface for an event envelope.
// This interface allows generic envelopes to be accepted by the untyped OutboxWriter.
type AnyEnvelope interface {
	GetEventID() string
	GetEventType() string
	GetAggregateType() string
	GetAggregateID() string
	GetAggregateVersion() int
	GetSchemaVersion() string
	GetOccurredAt() time.Time
	GetProducer() string
	GetCorrelationID() string
	GetPayload() any
	SerializePayload() ([]byte, error)
}

// EventEnvelope is the canonical generic event envelope for domain and integration events.
type EventEnvelope[T any] struct {
	EventID          string    `json:"event_id"`
	EventType        string    `json:"event_type"`
	AggregateType    string    `json:"aggregate_type"`
	AggregateID      string    `json:"aggregate_id"`
	AggregateVersion int       `json:"aggregate_version"`
	SchemaVersion    string    `json:"schema_version"`
	OccurredAt       time.Time `json:"occurred_at"`
	CorrelationID    string    `json:"correlation_id"`
	Producer         string    `json:"producer"`
	Payload          T         `json:"payload"`
}

// Ensure EventEnvelope[T] implements AnyEnvelope.
var _ AnyEnvelope = (*EventEnvelope[any])(nil)

func (e EventEnvelope[T]) GetEventID() string       { return e.EventID }
func (e EventEnvelope[T]) GetEventType() string     { return e.EventType }
func (e EventEnvelope[T]) GetAggregateType() string { return e.AggregateType }
func (e EventEnvelope[T]) GetAggregateID() string   { return e.AggregateID }
func (e EventEnvelope[T]) GetAggregateVersion() int { return e.AggregateVersion }
func (e EventEnvelope[T]) GetSchemaVersion() string { return e.SchemaVersion }
func (e EventEnvelope[T]) GetOccurredAt() time.Time { return e.OccurredAt }
func (e EventEnvelope[T]) GetProducer() string      { return e.Producer }
func (e EventEnvelope[T]) GetCorrelationID() string { return e.CorrelationID }
func (e EventEnvelope[T]) GetPayload() any          { return e.Payload }

// SerializePayload converts the typed payload into JSON bytes.
func (e EventEnvelope[T]) SerializePayload() ([]byte, error) {
	bytes, err := json.Marshal(e.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize event payload: %w", err)
	}
	if len(bytes) == 0 || string(bytes) == "null" {
		return nil, ErrNilPayload
	}
	return bytes, nil
}

// GenerateUUIDv7 generates an RFC 9562 compliant UUIDv7 using the standard library.
// Format: 8-4-4-4-12 hex string.
// - 48-bit big-endian millisecond timestamp
// - 4-bit version (0b0111 = 7)
// - 12-bit pseudo-random data
// - 2-bit variant (0b10 = RFC 4122/9562)
// - 62-bit pseudo-random data
func GenerateUUIDv7() (string, error) {
	var raw [16]byte

	// 48-bit timestamp in milliseconds
	//nolint:gosec // G115: UnixMilli timestamp is guaranteed non-negative
	milli := uint64(time.Now().UnixMilli())
	raw[0] = byte(milli >> 40)
	raw[1] = byte(milli >> 32)
	raw[2] = byte(milli >> 24)
	raw[3] = byte(milli >> 16)
	raw[4] = byte(milli >> 8)
	raw[5] = byte(milli)

	// Fill remaining 10 bytes with cryptographic randomness
	if _, err := rand.Read(raw[6:]); err != nil {
		return "", fmt.Errorf("failed to read random bytes for UUIDv7: %w", err)
	}

	// Set version 7 in high nibble of byte 6
	raw[6] = (raw[6] & 0x0f) | 0x70

	// Set variant 1 (0b10) in high 2 bits of byte 8
	raw[8] = (raw[8] & 0x3f) | 0x80

	dst := make([]byte, 36)
	hex.Encode(dst[0:8], raw[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], raw[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], raw[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], raw[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], raw[10:16])

	return string(dst), nil
}

// IsValidUUID checks if the given string is a valid canonical 36-character UUID.
func IsValidUUID(u string) bool {
	if len(u) != 36 {
		return false
	}
	return uuidRegex.MatchString(u)
}

// ValidateEnvelope checks the invariants of any event envelope.
func ValidateEnvelope(env AnyEnvelope) error {
	if env == nil {
		return ErrNilPayload
	}

	if !IsValidUUID(env.GetEventID()) {
		return ErrInvalidEventID
	}

	if !eventTypeRegex.MatchString(env.GetEventType()) {
		return ErrInvalidEventType
	}

	aggType := strings.ToLower(env.GetAggregateType())
	if !AllowedAggregateTypes[aggType] {
		return ErrInvalidAggregateType
	}

	if !IsValidUUID(env.GetAggregateID()) {
		return ErrInvalidAggregateID
	}

	if env.GetAggregateVersion() < 1 {
		return ErrInvalidAggregateVersion
	}

	corrID := env.GetCorrelationID()
	if strings.TrimSpace(corrID) == "" || len(corrID) > 128 {
		return ErrMissingCorrelationID
	}

	payloadBytes, err := env.SerializePayload()
	if err != nil || len(payloadBytes) == 0 || string(payloadBytes) == "null" {
		return ErrNilPayload
	}

	return nil
}

// NewEnvelope creates a new typed EventEnvelope with validated fields, a freshly
// generated UUIDv7 EventID, UTC occurred_at timestamp, and schema_version "1.0.0".
func NewEnvelope[T any](
	eventType string,
	aggregateType string,
	aggregateID string,
	aggregateVersion int,
	correlationID string,
	producer string,
	payload T,
) (EventEnvelope[T], error) {
	eventID, err := GenerateUUIDv7()
	if err != nil {
		return EventEnvelope[T]{}, fmt.Errorf("failed to generate event id: %w", err)
	}

	if strings.TrimSpace(producer) == "" {
		producer = "core-api"
	}

	env := EventEnvelope[T]{
		EventID:          eventID,
		EventType:        eventType,
		AggregateType:    aggregateType,
		AggregateID:      aggregateID,
		AggregateVersion: aggregateVersion,
		SchemaVersion:    "1.0.0",
		OccurredAt:       time.Now().UTC(),
		CorrelationID:    correlationID,
		Producer:         producer,
		Payload:          payload,
	}

	if valErr := ValidateEnvelope(env); valErr != nil {
		return EventEnvelope[T]{}, valErr
	}

	return env, nil
}
