package events

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type samplePayload struct {
	ParcelID       string  `json:"parcel_id"`
	TrackingNumber string  `json:"tracking_number"`
	MerchantID     string  `json:"merchant_id"`
	Status         string  `json:"status"`
	WeightKG       float64 `json:"weight_kg"`
}

func TestGenerateUUIDv7(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id, err := GenerateUUIDv7()
		if err != nil {
			t.Fatalf("GenerateUUIDv7 returned error: %v", err)
		}

		if len(id) != 36 {
			t.Errorf("expected UUID length 36, got %d for %s", len(id), id)
		}

		if !IsValidUUID(id) {
			t.Errorf("IsValidUUID(%s) returned false", id)
		}

		// Check version 7 bit (char at index 14 is '7')
		if id[14] != '7' {
			t.Errorf("expected version '7' at index 14, got %c in %s", id[14], id)
		}

		// Check variant bit (char at index 19 is '8', '9', 'a', or 'b')
		variantChar := id[19]
		if variantChar != '8' && variantChar != '9' && variantChar != 'a' && variantChar != 'b' {
			t.Errorf("expected variant 1 at index 19 (8, 9, a, b), got %c in %s", variantChar, id)
		}

		if seen[id] {
			t.Fatalf("duplicate UUIDv7 generated: %s", id)
		}
		seen[id] = true
	}
}

func TestNewEnvelope_Valid(t *testing.T) {
	aggID, _ := GenerateUUIDv7()
	payload := samplePayload{
		ParcelID:       aggID,
		TrackingNumber: "SD-20261101-0001",
		MerchantID:     "0192e4a1-5555-7abc-8def-0123456789ab",
		Status:         "CREATED",
		WeightKG:       2.5,
	}

	env, err := NewEnvelope(
		ParcelBookedV1,
		"parcel",
		aggID,
		1,
		"req-trace-12345",
		"core-api.parcels",
		payload,
	)
	if err != nil {
		t.Fatalf("expected valid envelope, got error: %v", err)
	}

	if env.EventType != ParcelBookedV1 {
		t.Errorf("expected event_type %s, got %s", ParcelBookedV1, env.EventType)
	}
	if env.AggregateType != "parcel" {
		t.Errorf("expected aggregate_type parcel, got %s", env.AggregateType)
	}
	if env.AggregateID != aggID {
		t.Errorf("expected aggregate_id %s, got %s", aggID, env.AggregateID)
	}
	if env.AggregateVersion != 1 {
		t.Errorf("expected aggregate_version 1, got %d", env.AggregateVersion)
	}
	if env.SchemaVersion != "1.0.0" {
		t.Errorf("expected schema_version 1.0.0, got %s", env.SchemaVersion)
	}
	if env.CorrelationID != "req-trace-12345" {
		t.Errorf("expected correlation_id req-trace-12345, got %s", env.CorrelationID)
	}
	if env.Producer != "core-api.parcels" {
		t.Errorf("expected producer core-api.parcels, got %s", env.Producer)
	}
	if env.OccurredAt.IsZero() {
		t.Error("expected non-zero occurred_at timestamp")
	}

	// Verify AnyEnvelope interface
	var anyEnv AnyEnvelope = env
	if anyEnv.GetEventType() != ParcelBookedV1 {
		t.Errorf("AnyEnvelope.GetEventType returned %s", anyEnv.GetEventType())
	}
	if anyEnv.GetSchemaVersion() != "1.0.0" {
		t.Errorf("AnyEnvelope.GetSchemaVersion returned %s", anyEnv.GetSchemaVersion())
	}

	serialized, err := anyEnv.SerializePayload()
	if err != nil {
		t.Fatalf("SerializePayload failed: %v", err)
	}
	if !strings.Contains(string(serialized), "SD-20261101-0001") {
		t.Errorf("expected tracking number in serialized payload: %s", string(serialized))
	}
}

func TestValidateEnvelope_Table(t *testing.T) {
	validUUID, _ := GenerateUUIDv7()

	tests := []struct {
		name        string
		modify      func(e *EventEnvelope[samplePayload])
		expectedErr error
	}{
		{
			name:        "Valid envelope",
			modify:      func(e *EventEnvelope[samplePayload]) {},
			expectedErr: nil,
		},
		{
			name: "Invalid event_id",
			modify: func(e *EventEnvelope[samplePayload]) {
				e.EventID = "not-a-uuid"
			},
			expectedErr: ErrInvalidEventID,
		},
		{
			name: "Invalid event_type - missing version",
			modify: func(e *EventEnvelope[samplePayload]) {
				e.EventType = "parcel.booked"
			},
			expectedErr: ErrInvalidEventType,
		},
		{
			name: "Invalid event_type - uppercase characters",
			modify: func(e *EventEnvelope[samplePayload]) {
				e.EventType = "Parcel.Booked.v1"
			},
			expectedErr: ErrInvalidEventType,
		},
		{
			name: "Invalid aggregate_type",
			modify: func(e *EventEnvelope[samplePayload]) {
				e.AggregateType = "order"
			},
			expectedErr: ErrInvalidAggregateType,
		},
		{
			name: "Invalid aggregate_id - not a uuid",
			modify: func(e *EventEnvelope[samplePayload]) {
				e.AggregateID = "prcl_12345"
			},
			expectedErr: ErrInvalidAggregateID,
		},
		{
			name: "Invalid aggregate_version - zero",
			modify: func(e *EventEnvelope[samplePayload]) {
				e.AggregateVersion = 0
			},
			expectedErr: ErrInvalidAggregateVersion,
		},
		{
			name: "Invalid correlation_id - empty",
			modify: func(e *EventEnvelope[samplePayload]) {
				e.CorrelationID = "   "
			},
			expectedErr: ErrMissingCorrelationID,
		},
		{
			name: "Invalid correlation_id - exceeds 128 chars",
			modify: func(e *EventEnvelope[samplePayload]) {
				e.CorrelationID = strings.Repeat("x", 129)
			},
			expectedErr: ErrMissingCorrelationID,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := EventEnvelope[samplePayload]{
				EventID:          validUUID,
				EventType:        ParcelBookedV1,
				AggregateType:    "parcel",
				AggregateID:      validUUID,
				AggregateVersion: 1,
				SchemaVersion:    "1.0.0",
				OccurredAt:       time.Now().UTC(),
				CorrelationID:    "corr-1234",
				Producer:         "core-api.parcels",
				Payload: samplePayload{
					ParcelID:       validUUID,
					TrackingNumber: "SD-1234",
				},
			}

			tc.modify(&env)
			err := ValidateEnvelope(env)

			if tc.expectedErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tc.expectedErr != nil && err != tc.expectedErr {
				t.Fatalf("expected error %v, got: %v", tc.expectedErr, err)
			}
		})
	}
}

func TestCanonicalJSONGoldenSchema(t *testing.T) {
	fixedUUID := "0192e4a1-0001-7abc-8def-0123456789ab"
	occurredAt := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	env := EventEnvelope[samplePayload]{
		EventID:          fixedUUID,
		EventType:        ParcelBookedV1,
		AggregateType:    "parcel",
		AggregateID:      fixedUUID,
		AggregateVersion: 1,
		SchemaVersion:    "1.0.0",
		OccurredAt:       occurredAt,
		CorrelationID:    "corr-test-golden",
		Producer:         "core-api.parcels",
		Payload: samplePayload{
			ParcelID:       fixedUUID,
			TrackingNumber: "SD-20261101-99812",
			MerchantID:     "0192e4a1-5555-7abc-8def-0123456789ab",
			Status:         "CREATED",
			WeightKG:       2.5,
		},
	}

	rawJSON, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("failed to marshal envelope: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated JSON: %v", err)
	}

	requiredCanonicalFields := []string{
		"event_id",
		"event_type",
		"aggregate_type",
		"aggregate_id",
		"aggregate_version",
		"schema_version",
		"occurred_at",
		"correlation_id",
		"producer",
		"payload",
	}

	for _, field := range requiredCanonicalFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("golden schema missing canonical field: %q", field)
		}
	}

	if parsed["event_id"] != fixedUUID {
		t.Errorf("expected event_id %q, got %v", fixedUUID, parsed["event_id"])
	}
	if parsed["event_type"] != "parcel.booked.v1" {
		t.Errorf("expected event_type 'parcel.booked.v1', got %v", parsed["event_type"])
	}
}
