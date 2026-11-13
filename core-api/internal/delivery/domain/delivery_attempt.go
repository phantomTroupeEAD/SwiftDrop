package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type AttemptOutcome string

const (
	AttemptOutcomeSuccess AttemptOutcome = "SUCCESS"
	AttemptOutcomeFailed  AttemptOutcome = "FAILED"
)

// DeliveryAttempt is an immutable doorstep attempt event record.
type DeliveryAttempt struct {
	ID            uuid.UUID
	ParcelID      uuid.UUID
	CourierID     uuid.UUID
	AttemptNumber int
	Outcome       AttemptOutcome
	FailureReason *FailureReasonCode
	Proof         *ProofOfDelivery
	Coordinates   *GeoCoordinate
	Notes         string
	AttemptedAt   time.Time
}

// NewSuccessfulAttempt creates and validates an attempt record representing successful delivery with proof.
func NewSuccessfulAttempt(
	parcelID, courierID uuid.UUID,
	attemptNumber int,
	proof *ProofOfDelivery,
	coordinates *GeoCoordinate,
	notes string,
) (*DeliveryAttempt, error) {
	if attemptNumber < 1 || attemptNumber > 3 {
		return nil, ErrAttemptLimitExceeded
	}

	if proof == nil {
		return nil, ErrMissingDeliveryProof
	}

	if err := proof.Validate(); err != nil {
		return nil, err
	}

	return &DeliveryAttempt{
		ID:            uuid.New(),
		ParcelID:      parcelID,
		CourierID:     courierID,
		AttemptNumber: attemptNumber,
		Outcome:       AttemptOutcomeSuccess,
		FailureReason: nil,
		Proof:         proof,
		Coordinates:   coordinates,
		Notes:         strings.TrimSpace(notes),
		AttemptedAt:   time.Now().UTC(),
	}, nil
}

// NewFailedAttempt creates and validates an attempt record representing an unsuccessful doorstep delivery attempt.
func NewFailedAttempt(
	parcelID, courierID uuid.UUID,
	attemptNumber int,
	reasonCode FailureReasonCode,
	coordinates *GeoCoordinate,
	notes string,
) (*DeliveryAttempt, error) {
	if attemptNumber < 1 || attemptNumber > 3 {
		return nil, ErrAttemptLimitExceeded
	}

	trimmedNotes := strings.TrimSpace(notes)
	if err := reasonCode.Validate(trimmedNotes); err != nil {
		return nil, err
	}

	return &DeliveryAttempt{
		ID:            uuid.New(),
		ParcelID:      parcelID,
		CourierID:     courierID,
		AttemptNumber: attemptNumber,
		Outcome:       AttemptOutcomeFailed,
		FailureReason: &reasonCode,
		Proof:         nil,
		Coordinates:   coordinates,
		Notes:         trimmedNotes,
		AttemptedAt:   time.Now().UTC(),
	}, nil
}

// IsTerminal evaluates whether this attempt terminates the parcel delivery lifecycle,
// triggering an automatic transition to RETURNED (on 3rd failure or recipient refusal).
func (d *DeliveryAttempt) IsTerminal() bool {
	if d.Outcome == AttemptOutcomeSuccess {
		return false
	}

	if d.AttemptNumber >= 3 {
		return true
	}

	if d.FailureReason != nil && d.FailureReason.IsImmediateReturn() {
		return true
	}

	return false
}

// Validate verifies cross-field invariant consistency on the attempt entity.
func (d *DeliveryAttempt) Validate() error {
	if d.AttemptNumber < 1 || d.AttemptNumber > 3 {
		return ErrAttemptLimitExceeded
	}

	switch d.Outcome {
	case AttemptOutcomeSuccess:
		if d.Proof == nil {
			return ErrMissingDeliveryProof
		}
		if d.FailureReason != nil {
			return ErrOutcomeConflict
		}
		return d.Proof.Validate()

	case AttemptOutcomeFailed:
		if d.FailureReason == nil {
			return ErrInvalidFailureReasonCode
		}
		if d.Proof != nil {
			return ErrFailureConflict
		}
		return d.FailureReason.Validate(d.Notes)

	default:
		return ErrInvalidAttemptOutcome
	}
}

func (o AttemptOutcome) String() string {
	return string(o)
}
