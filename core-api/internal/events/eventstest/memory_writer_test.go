package eventstest

import (
	"context"
	"database/sql"
	"testing"

	"github.com/swiftdrop/core-api/internal/events"
)

type dummyPayload struct {
	Message string `json:"message"`
}

func TestInMemoryWriter(t *testing.T) {
	writer := NewInMemoryWriter()
	ctx := context.Background()

	// 1. Nil transaction must return ErrMissingTransaction
	err := writer.Write(ctx, nil)
	if err != events.ErrMissingTransaction {
		t.Fatalf("expected ErrMissingTransaction for nil tx, got: %v", err)
	}

	// 2. Dummy *sql.Tx (non-nil pointer for testing interface behavior)
	dummyTx := &sql.Tx{}

	// 3. Writing zero envelopes is a no-op
	if err := writer.Write(ctx, dummyTx); err != nil {
		t.Fatalf("expected nil error for empty envelopes, got: %v", err)
	}
	if writer.Count() != 0 {
		t.Errorf("expected count 0, got %d", writer.Count())
	}

	// 4. Writing invalid envelope returns validation error
	invalidEnv := events.EventEnvelope[dummyPayload]{
		EventID:   "invalid-uuid",
		EventType: "bad_type",
	}
	if err := writer.Write(ctx, dummyTx, invalidEnv); err == nil {
		t.Fatal("expected validation error for invalid envelope, got nil")
	}

	// 5. Writing valid envelopes succeeds
	aggID1, _ := events.GenerateUUIDv7()
	aggID2, _ := events.GenerateUUIDv7()

	env1, err := events.NewEnvelope(
		events.ParcelBookedV1,
		"parcel",
		aggID1,
		1,
		"req-001",
		"core-api.parcels",
		dummyPayload{Message: "parcel booked"},
	)
	if err != nil {
		t.Fatalf("failed to create env1: %v", err)
	}

	env2, err := events.NewEnvelope(
		events.ParcelAssignedV1,
		"parcel",
		aggID1,
		2,
		"req-002",
		"core-api.dispatch",
		dummyPayload{Message: "courier assigned"},
	)
	if err != nil {
		t.Fatalf("failed to create env2: %v", err)
	}

	env3, err := events.NewEnvelope(
		events.CourierUnassignedV1,
		"courier",
		aggID2,
		1,
		"req-003",
		"core-api.dispatch",
		dummyPayload{Message: "courier unassigned"},
	)
	if err != nil {
		t.Fatalf("failed to create env3: %v", err)
	}

	if err := writer.Write(ctx, dummyTx, env1, env2, env3); err != nil {
		t.Fatalf("expected successful write, got: %v", err)
	}

	if writer.Count() != 3 {
		t.Errorf("expected count 3, got %d", writer.Count())
	}

	// Check filters
	bookedEvents := writer.FindByEventType(events.ParcelBookedV1)
	if len(bookedEvents) != 1 {
		t.Errorf("expected 1 booked event, got %d", len(bookedEvents))
	}

	agg1Events := writer.FindByAggregateID(aggID1)
	if len(agg1Events) != 2 {
		t.Errorf("expected 2 events for aggregate %s, got %d", aggID1, len(agg1Events))
	}

	// Snapshot check
	allEvents := writer.Events()
	if len(allEvents) != 3 {
		t.Errorf("expected 3 events in snapshot, got %d", len(allEvents))
	}

	// Reset check
	writer.Reset()
	if writer.Count() != 0 {
		t.Errorf("expected count 0 after reset, got %d", writer.Count())
	}
}
