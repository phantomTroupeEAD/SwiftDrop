package domain

type ShiftStatus string

const (
	ShiftStatusOffShift  ShiftStatus = "OFF_SHIFT"
	ShiftStatusOnShift   ShiftStatus = "ON_SHIFT"
	ShiftStatusOnBreak   ShiftStatus = "ON_BREAK"
	ShiftStatusSuspended ShiftStatus = "SUSPENDED"
)

// Validate checks whether the status string is a recognized shift status.
func (s ShiftStatus) Validate() error {
	switch s {
	case ShiftStatusOffShift, ShiftStatusOnShift, ShiftStatusOnBreak, ShiftStatusSuspended:
		return nil
	default:
		return ErrInvalidShiftTransition
	}
}

// CanTransitionTo evaluates the shift lifecycle state machine.
// isSelfService is true when triggered directly by the authenticated courier,
// preventing couriers from self-suspending or self-reinstating.
func (s ShiftStatus) CanTransitionTo(target ShiftStatus, isSelfService bool) bool {
	if s == target {
		return false
	}

	if isSelfService {
		// Couriers cannot administratively suspend themselves or clear suspensions
		if target == ShiftStatusSuspended || s == ShiftStatusSuspended {
			return false
		}

		switch s {
		case ShiftStatusOffShift:
			return target == ShiftStatusOnShift
		case ShiftStatusOnShift:
			return target == ShiftStatusOffShift || target == ShiftStatusOnBreak
		case ShiftStatusOnBreak:
			return target == ShiftStatusOnShift || target == ShiftStatusOffShift
		default:
			return false
		}
	}

	// Administrative rules (Dispatcher or Admin)
	if target == ShiftStatusSuspended {
		return true // Admin can suspend any courier from any state
	}

	if s == ShiftStatusSuspended {
		return target == ShiftStatusOffShift // Admin reinstates courier to OFF_SHIFT
	}

	switch s {
	case ShiftStatusOffShift:
		return target == ShiftStatusOnShift
	case ShiftStatusOnShift:
		return target == ShiftStatusOffShift || target == ShiftStatusOnBreak
	case ShiftStatusOnBreak:
		return target == ShiftStatusOnShift || target == ShiftStatusOffShift
	default:
		return false
	}
}

func (s ShiftStatus) String() string {
	return string(s)
}
