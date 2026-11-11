package domain

import "fmt"

// MaxDeliveryAttempts defines the maximum number of failed delivery attempts
// permitted before reattempt scheduling is locked.
const MaxDeliveryAttempts = 3

// ParcelStatus represents the authoritative lifecycle state of a parcel.
type ParcelStatus string

const (
	StatusCreated        ParcelStatus = "CREATED"
	StatusAssigned       ParcelStatus = "ASSIGNED"
	StatusPickedUp       ParcelStatus = "PICKED_UP"
	StatusInTransit      ParcelStatus = "IN_TRANSIT"
	StatusOutForDelivery ParcelStatus = "OUT_FOR_DELIVERY"
	StatusDelivered      ParcelStatus = "DELIVERED"
	StatusAttemptFailed  ParcelStatus = "ATTEMPT_FAILED"
	StatusReturned       ParcelStatus = "RETURNED"
	StatusCancelled      ParcelStatus = "CANCELLED"
)

// Validate checks whether the parcel status is one of the nine agreed states.
func (s ParcelStatus) Validate() error {
	switch s {
	case StatusCreated,
		StatusAssigned,
		StatusPickedUp,
		StatusInTransit,
		StatusOutForDelivery,
		StatusDelivered,
		StatusAttemptFailed,
		StatusReturned,
		StatusCancelled:
		return nil
	default:
		return ErrInvalidParcelStatus
	}
}

// IsTerminal reports whether the status represents a final, unalterable state.
func (s ParcelStatus) IsTerminal() bool {
	switch s {
	case StatusDelivered, StatusReturned, StatusCancelled:
		return true
	default:
		return false
	}
}

// ValidateTransition verifies whether a status change from one state to another
// is permitted by the canonical 9-state machine and business invariants.
func ValidateTransition(from, to ParcelStatus, attemptCount int) error {
	if err := from.Validate(); err != nil {
		return fmt.Errorf("%w: current status %q", ErrInvalidParcelStatus, from)
	}
	if err := to.Validate(); err != nil {
		return fmt.Errorf("%w: target status %q", ErrInvalidParcelStatus, to)
	}

	if from.IsTerminal() {
		return fmt.Errorf("%w: cannot transition from terminal status %s", ErrTerminalState, from)
	}

	if from == to {
		return fmt.Errorf("%w: transition to identical status %s", ErrIllegalStateTransition, to)
	}

	// Cancellation is permitted ONLY while the parcel is in CREATED status.
	if to == StatusCancelled {
		if from != StatusCreated {
			return ErrCancellationNotAllowed
		}
		return nil
	}

	switch from {
	case StatusCreated:
		if to == StatusAssigned {
			return nil
		}

	case StatusAssigned:
		if to == StatusPickedUp {
			return nil
		}

	case StatusPickedUp:
		if to == StatusInTransit {
			return nil
		}

	case StatusInTransit:
		if to == StatusOutForDelivery {
			return nil
		}

	case StatusOutForDelivery:
		if to == StatusDelivered || to == StatusAttemptFailed {
			return nil
		}

	case StatusAttemptFailed:
		if to == StatusOutForDelivery {
			if attemptCount >= MaxDeliveryAttempts {
				return ErrMaxDeliveryAttemptsExceeded
			}
			return nil
		}
		if to == StatusReturned {
			return nil
		}
	}

	return fmt.Errorf("%w: cannot transition from %s to %s", ErrIllegalStateTransition, from, to)
}
