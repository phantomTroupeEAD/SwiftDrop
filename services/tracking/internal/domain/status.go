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
