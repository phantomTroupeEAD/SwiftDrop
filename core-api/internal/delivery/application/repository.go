package application

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

// ActiveAssignmentDTO provides operational assignment details needed for verification.
type ActiveAssignmentDTO struct {
	AssignmentID uuid.UUID
	ParcelID     uuid.UUID
	CourierID    uuid.UUID
	Status       string
}

// ActiveAssignmentFinder abstracts cross-context queries against courier_assignments
// to verify courier assignment and prevent IDOR attacks.
type ActiveAssignmentFinder interface {
	GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*ActiveAssignmentDTO, error)
	CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error
}

// DeliveryAttemptRepository governs persistence of immutable delivery attempts in delivery_attempts.
type DeliveryAttemptRepository interface {
	InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *domain.DeliveryAttempt) error
	ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*domain.DeliveryAttempt, error)
	GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error)
}

// CourierManifestItemDTO represents a single delivery task on a courier's personalized manifest.
// Strictly adheres to data minimization: customer PII and wholesale merchant fees are redacted.
type CourierManifestItemDTO struct {
	ParcelID             string    `json:"parcel_id"`
	TrackingNumber       string    `json:"tracking_number"`
	CurrentStatus        string    `json:"current_status"`
	DestinationZone      string    `json:"destination_zone"`
	RecipientName        string    `json:"recipient_name"`
	RecipientPhone       string    `json:"recipient_phone"`
	DeliveryStreet       string    `json:"delivery_street"`
	DeliveryCity         string    `json:"delivery_city"`
	WeightKg             float64   `json:"weight_kg"`
	HandlingInstructions string    `json:"handling_instructions"`
	AttemptCount         int       `json:"attempt_count"`
	AssignedAt           time.Time `json:"assigned_at"`
}

// CourierManifestReader retrieves the active delivery manifest for a specific courier.
type CourierManifestReader interface {
	GetActiveManifest(ctx context.Context, courierID uuid.UUID) ([]CourierManifestItemDTO, error)
}

