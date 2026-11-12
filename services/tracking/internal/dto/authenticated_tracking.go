package dto

import "time"

// AuthenticatedTrackingResponse encapsulates the detailed parcel tracking payload
// returned to an authorized parcel owner (customer or merchant) or operations staff.
// Conforms strictly to Document 4 §4.3 and api-conventions.md §2.1.
type AuthenticatedTrackingResponse struct {
	TrackingNumber    string                   `json:"tracking_number"`
	ParcelID          string                   `json:"parcel_id"`
	Status            string                   `json:"status"`
	StatusDescription string                   `json:"status_description"`
	Destination       AuthenticatedDestination `json:"destination"`
	Milestones        []AuthenticatedMilestone `json:"milestones"`
}

// AuthenticatedDestination exposes destination delivery routing information
// (city, postal code, delivery zone ID) to authorized callers while still
// withholding exact recipient street address line 1 and personal contact details per SEC-009.
type AuthenticatedDestination struct {
	City       string `json:"city"`
	PostalCode string `json:"postal_code,omitempty"`
	ZoneID     string `json:"zone_id,omitempty"`
}

// AuthenticatedMilestone exposes chronological audit events for authorized parcel owners,
// including operational actor roles and non-confidential transition reason codes.
type AuthenticatedMilestone struct {
	Status      string    `json:"status"`
	Description string    `json:"description"`
	ActorRole   string    `json:"actor_role,omitempty"`
	ReasonCode  string    `json:"reason_code,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}
