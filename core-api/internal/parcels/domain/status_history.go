package domain

import (
	"fmt"
	"strings"
	"time"
)

// ActorType identifies the category of actor that initiated a parcel status transition.
type ActorType string

const (
	ActorTypeUser        ActorType = "USER"
	ActorTypeSystem      ActorType = "SYSTEM"
	ActorTypeMerchantAPI ActorType = "MERCHANT_API"
)

// Validate verifies that the actor type is one of the supported enum values.
func (t ActorType) Validate() error {
	switch t {
	case ActorTypeUser, ActorTypeSystem, ActorTypeMerchantAPI:
		return nil
	default:
		return fmt.Errorf("%w: unsupported actor type %q", ErrInvalidActorType, t)
	}
}

func normalizeStatusActor(actorID *string, actorType ActorType) (*string, error) {
	if err := actorType.Validate(); err != nil {
		return nil, err
	}

	if actorType == ActorTypeSystem {
		if actorID != nil {
			return nil, fmt.Errorf("%w: SYSTEM transitions must have nil actor_id", ErrInvalidActorID)
		}
		return nil, nil
	}

	if actorID == nil {
		return nil, fmt.Errorf("%w: actor_id is required for actor type %s", ErrInvalidActorID, actorType)
	}

	trimmed := strings.TrimSpace(*actorID)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: actor_id cannot be blank for actor type %s", ErrInvalidActorID, actorType)
	}
	if strings.EqualFold(trimmed, string(ActorTypeSystem)) {
		return nil, fmt.Errorf("%w: literal %q is not a valid actor_id", ErrInvalidActorID, trimmed)
	}

	return &trimmed, nil
}

// StatusHistoryEntry represents an immutable chronological audit record of a parcel state change.
type StatusHistoryEntry struct {
	ID         string
	ParcelID   string
	FromStatus *ParcelStatus // nil for the initial booking creation event
	ToStatus   ParcelStatus
	ActorID    *string
	ActorType  ActorType
	ActorRole  string
	Reason     string
	OccurredAt time.Time
}

// NewStatusHistoryEntry creates and validates a status history audit record.
func NewStatusHistoryEntry(
	id string,
	parcelID string,
	fromStatus *ParcelStatus,
	toStatus ParcelStatus,
	actorID *string,
	actorType ActorType,
	actorRole string,
	reason string,
	occurredAt time.Time,
	attemptCount int,
) (StatusHistoryEntry, error) {
	var actorIDCopy *string
	if actorID != nil {
		trimmed := strings.TrimSpace(*actorID)
		actorIDCopy = &trimmed
	}

	entry := StatusHistoryEntry{
		ID:         strings.TrimSpace(id),
		ParcelID:   strings.TrimSpace(parcelID),
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		ActorID:    actorIDCopy,
		ActorType:  actorType,
		ActorRole:  strings.TrimSpace(actorRole),
		Reason:     strings.TrimSpace(reason),
		OccurredAt: occurredAt,
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

	if _, err := normalizeStatusActor(e.ActorID, e.ActorType); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidStatusHistory, err)
	}

	if e.OccurredAt.IsZero() {
		return fmt.Errorf("%w: occurred_at is required", ErrInvalidStatusHistory)
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
