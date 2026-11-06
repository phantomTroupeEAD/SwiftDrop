package domain

import (
	"fmt"
	"strings"
	"time"
)

// StatusHistoryEntry represents an immutable chronological audit record of a parcel state change.
type StatusHistoryEntry struct {
	ID         string
	ParcelID   string
	FromStatus *ParcelStatus // nil for the initial booking creation event
	ToStatus   ParcelStatus
	ActorID    string
	ActorRole  string
	Reason     string
	Timestamp  time.Time
}

// NewStatusHistoryEntry creates and validates a status history audit record.
func NewStatusHistoryEntry(
	id string,
	parcelID string,
	fromStatus *ParcelStatus,
	toStatus ParcelStatus,
	actorID string,
	actorRole string,
	reason string,
	timestamp time.Time,
	attemptCount int,
) (StatusHistoryEntry, error) {
	entry := StatusHistoryEntry{
		ID:         strings.TrimSpace(id),
		ParcelID:   strings.TrimSpace(parcelID),
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		ActorID:    strings.TrimSpace(actorID),
		ActorRole:  strings.TrimSpace(actorRole),
		Reason:     strings.TrimSpace(reason),
		Timestamp:  timestamp,
	}

	if err := entry.Validate(attemptCount); err != nil {
		return StatusHistoryEntry{}, err
	}

	return entry, nil
}

// Validate ensures that the history record represents a valid, legal state transition.
func (e StatusHistoryEntry) Validate(attemptCount int) error {
	if e.ParcelID == "" {
		return fmt.Errorf("%w: parcel id is required", ErrInvalidStatusHistory)
	}

	if err := e.ToStatus.Validate(); err != nil {
		return fmt.Errorf("%w: invalid target status %q", ErrInvalidStatusHistory, e.ToStatus)
	}

	if e.Timestamp.IsZero() {
		return fmt.Errorf("%w: timestamp is required", ErrInvalidStatusHistory)
	}

	// If there is a previous status, the transition from FromStatus to ToStatus must be legal.
	if e.FromStatus != nil {
		if err := e.FromStatus.Validate(); err != nil {
			return fmt.Errorf("%w: invalid source status %q", ErrInvalidStatusHistory, *e.FromStatus)
		}
		if err := ValidateTransition(*e.FromStatus, e.ToStatus, attemptCount); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidStatusHistory, err)
		}
	} else {
		// If FromStatus is nil, ToStatus MUST be CREATED (initial aggregate creation).
		if e.ToStatus != StatusCreated {
			return fmt.Errorf("%w: initial status without prior history must be CREATED, got %s",
				ErrInvalidStatusHistory, e.ToStatus)
		}
	}

	return nil
}

