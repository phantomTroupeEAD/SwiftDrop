package domain

import (
	"errors"
	"testing"
	"time"
)

func strPtr(s string) *string {
	return &s
}

func TestStatusHistoryEntry_InitialEntry(t *testing.T) {
	now := time.Date(2026, time.November, 6, 10, 0, 0, 0, time.UTC)
	entry, err := NewStatusHistoryEntry(
		"hist-001",
		"parcel-001",
		nil,
		StatusCreated,
		strPtr(" user-001 "),
		ActorTypeUser,
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
	if entry.ActorType != ActorTypeUser {
		t.Errorf("expected ActorType USER, got %s", entry.ActorType)
	}
	if entry.ActorID == nil || *entry.ActorID != "user-001" {
		t.Errorf("expected trimmed ActorID user-001, got %v", entry.ActorID)
	}
	if !entry.OccurredAt.Equal(now) {
		t.Errorf("expected OccurredAt %v, got %v", now, entry.OccurredAt)
	}

	// Initial entry with non-CREATED status must fail.
	_, err = NewStatusHistoryEntry(
		"hist-002",
		"parcel-001",
		nil,
		StatusAssigned,
		strPtr("user-001"),
		ActorTypeUser,
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

	// Legal transition by USER
	entry, err := NewStatusHistoryEntry(
		"hist-003",
		"parcel-001",
		&fromStatus,
		StatusAssigned,
		strPtr("disp-001"),
		ActorTypeUser,
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

	// Legal transition by MERCHANT_API
	merchantEntry, err := NewStatusHistoryEntry(
		"hist-003b",
		"parcel-001",
		nil,
		StatusCreated,
		strPtr("merchant-001"),
		ActorTypeMerchantAPI,
		"MERCHANT",
		"Booked via merchant API",
		now,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error for MERCHANT_API actor: %v", err)
	}
	if merchantEntry.ActorType != ActorTypeMerchantAPI || merchantEntry.ActorID == nil || *merchantEntry.ActorID != "merchant-001" {
		t.Errorf("unexpected merchant entry actor metadata: %+v", merchantEntry)
	}

	// Legal transition by SYSTEM with nil ActorID
	sysEntry, err := NewStatusHistoryEntry(
		"hist-003c",
		"parcel-001",
		&fromStatus,
		StatusCancelled,
		nil,
		ActorTypeSystem,
		"SYSTEM",
		"Auto-expired booking",
		now,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error for SYSTEM transition with nil ActorID: %v", err)
	}
	if sysEntry.ActorID != nil || sysEntry.ActorType != ActorTypeSystem {
		t.Errorf("expected nil ActorID and SYSTEM ActorType, got id=%v type=%s", sysEntry.ActorID, sysEntry.ActorType)
	}

	// Illegal transition (CREATED -> DELIVERED)
	_, err = NewStatusHistoryEntry(
		"hist-004",
		"parcel-001",
		&fromStatus,
		StatusDelivered,
		strPtr("disp-001"),
		ActorTypeUser,
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
		strPtr("disp-001"),
		ActorTypeUser,
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
		strPtr("disp-001"),
		ActorTypeUser,
		"DISPATCHER",
		"Missing timestamp",
		time.Time{},
		0,
	)
	if !errors.Is(err, ErrInvalidStatusHistory) {
		t.Fatalf("expected ErrInvalidStatusHistory for zero timestamp, got: %v", err)
	}
}

func TestStatusHistoryEntry_ActorValidation(t *testing.T) {
	now := time.Date(2026, time.November, 6, 10, 0, 0, 0, time.UTC)
	fromStatus := StatusCreated

	tests := []struct {
		name      string
		actorID   *string
		actorType ActorType
		wantErr   error
	}{
		{
			name:      "SYSTEM with literal SYSTEM actor id rejected",
			actorID:   strPtr("SYSTEM"),
			actorType: ActorTypeSystem,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "SYSTEM with non-nil blank actor id rejected",
			actorID:   strPtr(""),
			actorType: ActorTypeSystem,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "SYSTEM with non-nil arbitrary actor id rejected",
			actorID:   strPtr("cron-worker"),
			actorType: ActorTypeSystem,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "USER with nil actor id rejected",
			actorID:   nil,
			actorType: ActorTypeUser,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "USER with whitespace actor id rejected",
			actorID:   strPtr("   "),
			actorType: ActorTypeUser,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "USER with literal SYSTEM actor id rejected",
			actorID:   strPtr("SYSTEM"),
			actorType: ActorTypeUser,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "MERCHANT_API with nil actor id rejected",
			actorID:   nil,
			actorType: ActorTypeMerchantAPI,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "MERCHANT_API with whitespace actor id rejected",
			actorID:   strPtr(" "),
			actorType: ActorTypeMerchantAPI,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "MERCHANT_API with literal system actor id rejected",
			actorID:   strPtr("system"),
			actorType: ActorTypeMerchantAPI,
			wantErr:   ErrInvalidActorID,
		},
		{
			name:      "unsupported actor type rejected",
			actorID:   strPtr("bot-001"),
			actorType: ActorType("WEBHOOK"),
			wantErr:   ErrInvalidActorType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStatusHistoryEntry(
				"hist-err",
				"parcel-001",
				&fromStatus,
				StatusCancelled,
				tt.actorID,
				tt.actorType,
				"ROLE",
				"reason",
				now,
				0,
			)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
			if !errors.Is(err, ErrInvalidStatusHistory) {
				t.Errorf("expected error wrapping ErrInvalidStatusHistory, got %v", err)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error wrapping %v, got %v", tt.wantErr, err)
			}
		})
	}
}
