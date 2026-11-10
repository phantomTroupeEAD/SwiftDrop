package dto

import "time"

// PublicTrackingResponse encapsulates the allowlisted public tracking payload
// strictly conforming to SEC-009 privacy standards.
// PII, addresses, zone IDs, fees, and operational notes are strictly forbidden.
type PublicTrackingResponse struct {
	TrackingNumber    string            `json:"tracking_number"`
	Status            string            `json:"status"`
	StatusDescription string            `json:"status_description"`
	Destination       PublicDestination `json:"destination"`
	EstimatedDelivery *time.Time        `json:"estimated_delivery,omitempty"`
	Milestones        []PublicMilestone `json:"milestones"`
}

// PublicDestination exposes ONLY the destination city. Street addresses,
// postal codes, and internal zone IDs are strictly redacted per SEC-009.
type PublicDestination struct {
	City string `json:"city"`
}

// PublicMilestone exposes safe chronological milestone events. Internal actor IDs,
// raw reasons, courier notes, and POD proofs are strictly redacted per SEC-009.
type PublicMilestone struct {
	Status      string    `json:"status"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}
