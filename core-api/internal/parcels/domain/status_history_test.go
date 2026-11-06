package domain

import (
	"errors"
	"testing"
	"time"
)

func TestStatusHistoryEntry_InitialEntry(t *testing.T) {
	now := time.Date(2026, time.November, 6, 10, 0, 0, 0, time.UTC)
	entry, err := NewStatusHistoryEntry(
		"hist-001",
		"parcel-001",
		nil,
		StatusCreated,
		"user-001",
		"CUSTOMER",
		"Initial booking creation",
		now,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.ParcelID != "parcel-001" {
		t.Errorf("expected parcel id parcel-001, got %s", entry.ParcelID)
	}
	if entry.FromStatus != nil {
		t.Errorf("expected nil FromStatus, got %v", entry.FromStatus)
	}
	if entry.ToStatus != StatusCreated {
		t.Errorf("expected ToStatus CREATED, got %s", entry.ToStatus)
	}

	// Initial entry with non-CREATED status must fail.
	_, err = NewStatusHistoryEntry(
		"hist-002",
		"parcel-001",
		nil,
		StatusAssigned,
		"user-001",
		"DISPATCHER",
		"Invalid start",
		now,
		0,
	)
	if !errors.Is(err, ErrInvalidStatusHistory) {
		t.Fatalf("expected ErrInvalidStatusHistory for non-CREATED initial status, got %v", err)
	}
}

func TestStatusHistoryEntry_TransitionValidation(t *testing.T) {
	now := time.Date(2026, time.November, 6, 10, 0, 0, 0, time.UTC)
	fromStatus := StatusCreated

	// Legal transition
	entry, err := NewStatusHistoryEntry(
		"hist-003",
		"parcel-001",
		&fromStatus,
		StatusAssigned,
		"disp-001",
		"DISPATCHER",
		"Assigned to courier",
		now,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error for legal transition: %v", err)
	}
	if *entry.FromStatus != StatusCreated || entry.ToStatus != StatusAssigned {
		t.Errorf("unexpected statuses: from=%v, to=%s", entry.FromStatus, entry.ToStatus)
	}

	// Illegal transition (CREATED -> DELIVERED)
	_, err = NewStatusHistoryEntry(
		"hist-004",
		"parcel-001",
		&fromStatus,
		StatusDelivered,
		"disp-001",
		"DISPATCHER",
		"Illegal leap",
		now,
		0,
	)
	if !errors.Is(err, ErrInvalidStatusHistory) {
		t.Fatalf("expected ErrInvalidStatusHistory for illegal transition, got: %v", err)
	}

	// Empty parcel ID
	_, err = NewStatusHistoryEntry(
		"hist-005",
		"",
		&fromStatus,
		StatusAssigned,
		"disp-001",
		"DISPATCHER",
		"Missing parcel ID",
		now,
		0,
	)
	if !errors.Is(err, ErrInvalidStatusHistory) {
		t.Fatalf("expected ErrInvalidStatusHistory for empty parcel id, got: %v", err)
	}

	// Zero timestamp
	_, err = NewStatusHistoryEntry(
		"hist-006",
		"parcel-001",
		&fromStatus,
		StatusAssigned,
		"disp-001",
		"DISPATCHER",
		"Missing timestamp",
		time.Time{},
		0,
	)
	if !errors.Is(err, ErrInvalidStatusHistory) {
		t.Fatalf("expected ErrInvalidStatusHistory for zero timestamp, got: %v", err)
	}
}

