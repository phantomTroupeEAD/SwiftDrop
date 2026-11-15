package domain

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

// Validate checks whether the parcel status is one of the agreed 9 states.
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

// SafeStatusDescription maps internal parcel status codes into user-friendly,
// allowlisted milestone descriptions per SEC-009 privacy rules.
// Internal reason strings and operational notes are NEVER exposed to public consumers.
func SafeStatusDescription(status string) string {
	switch ParcelStatus(status) {
	case StatusCreated:
		return "Shipment booked and delivery label generated."
	case StatusAssigned:
		return "Courier assigned and dispatched for pickup."
	case StatusPickedUp:
		return "Parcel collected by courier from sender."
	case StatusInTransit:
		return "Parcel sorted and in transit between hubs."
	case StatusOutForDelivery:
		return "Courier is out for final doorstep delivery."
	case StatusDelivered:
		return "Parcel successfully delivered to recipient."
	case StatusAttemptFailed:
		return "Delivery attempt was unsuccessful; reattempt will be scheduled."
	case StatusReturned:
		return "Parcel returned to sender."
	case StatusCancelled:
		return "Shipment cancelled by sender or system."
	default:
		// Safe fallback: never reflect arbitrary or internal state strings to public callers
		return "Parcel status updated."
	}
}

// IsTerminal reports whether the status represents a final, immutable lifecycle state.
// Once reached, parcel status can never regress or transition further.
func (s ParcelStatus) IsTerminal() bool {
	switch s {
	case StatusDelivered, StatusReturned, StatusCancelled:
		return true
	default:
		return false
	}
}

// IsValidTransition reports whether transitioning directly from 'from' to 'to'
// is permitted under Member 2's canonical 9-state machine.
//
// Canonical transitions:
//   - CREATED -> ASSIGNED or CANCELLED
//   - ASSIGNED -> PICKED_UP
//   - PICKED_UP -> IN_TRANSIT
//   - IN_TRANSIT -> OUT_FOR_DELIVERY
//   - OUT_FOR_DELIVERY -> DELIVERED or ATTEMPT_FAILED
//   - ATTEMPT_FAILED -> OUT_FOR_DELIVERY (reattempt) or RETURNED
//
// Terminal states (DELIVERED, RETURNED, CANCELLED) allow no further transitions.
func IsValidTransition(from, to ParcelStatus) bool {
	if err := from.Validate(); err != nil {
		return false
	}
	if err := to.Validate(); err != nil {
		return false
	}
	if from.IsTerminal() {
		return false
	}
	if from == to {
		return false
	}

	// Cancellation is permitted ONLY while the parcel is in CREATED status.
	if to == StatusCancelled {
		return from == StatusCreated
	}

	switch from {
	case StatusCreated:
		return to == StatusAssigned
	case StatusAssigned:
		return to == StatusPickedUp
	case StatusPickedUp:
		return to == StatusInTransit
	case StatusInTransit:
		return to == StatusOutForDelivery
	case StatusOutForDelivery:
		return to == StatusDelivered || to == StatusAttemptFailed
	case StatusAttemptFailed:
		return to == StatusOutForDelivery || to == StatusReturned
	default:
		return false
	}
}

// CanReach reports whether 'to' is legally reachable from 'from' in the state machine,
// permitting forward transitions across skipped intermediate milestones while strictly
// blocking backward regressions and transitions from terminal states.
func CanReach(from, to ParcelStatus) bool {
	if err := from.Validate(); err != nil {
		return false
	}
	if err := to.Validate(); err != nil {
		return false
	}
	if from.IsTerminal() {
		return false
	}
	if from == to {
		return false
	}
	if to == StatusCancelled {
		return from == StatusCreated
	}

	switch from {
	case StatusCreated:
		// From CREATED, can reach any valid non-CREATED status
		return to != StatusCreated
	case StatusAssigned:
		// Cannot regress to CREATED or CANCELLED
		return to != StatusCreated && to != StatusCancelled
	case StatusPickedUp:
		// Cannot regress to CREATED, ASSIGNED, or CANCELLED
		return to != StatusCreated && to != StatusAssigned && to != StatusCancelled
	case StatusInTransit:
		// Cannot regress to CREATED, ASSIGNED, PICKED_UP, or CANCELLED
		return to == StatusOutForDelivery || to == StatusDelivered || to == StatusAttemptFailed || to == StatusReturned
	case StatusOutForDelivery:
		// Can transition to DELIVERED, ATTEMPT_FAILED, or RETURNED (via failed attempt).
		// Cannot regress to CREATED, ASSIGNED, PICKED_UP, IN_TRANSIT, CANCELLED.
		return to == StatusDelivered || to == StatusAttemptFailed || to == StatusReturned
	case StatusAttemptFailed:
		// Can transition to OUT_FOR_DELIVERY (reattempt), RETURNED, or DELIVERED (via reattempt).
		// Cannot regress to CREATED, ASSIGNED, PICKED_UP, IN_TRANSIT, CANCELLED.
		return to == StatusOutForDelivery || to == StatusReturned || to == StatusDelivered
	default:
		return false
	}
}

// LifecycleRank returns a legacy monotonically increasing sequence order for standard linear parcel milestones.
// NOTE: Numerical rankings cannot model cyclical or branching states (such as delivery attempt failure
// and rescheduling). IsValidTransition and CanReach must be preferred for lifecycle state validation.
func (s ParcelStatus) LifecycleRank() int {
	switch s {
	case StatusCreated:
		return 10
	case StatusAssigned:
		return 20
	case StatusPickedUp:
		return 30
	case StatusInTransit:
		return 40
	case StatusOutForDelivery:
		return 50
	case StatusAttemptFailed:
		return 55
	case StatusDelivered:
		return 100
	case StatusReturned:
		return 100
	case StatusCancelled:
		return 100
	default:
		return 0
	}
}
