package application

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

// CourierRepository governs persistence operations for Courier operational profiles.
type CourierRepository interface {
	GetByID(ctx context.Context, userID uuid.UUID) (*domain.Courier, error)
	GetByIDWithLock(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*domain.Courier, error)
	Save(ctx context.Context, tx *sql.Tx, courier *domain.Courier) error
	List(ctx context.Context, zoneID string, status *domain.ShiftStatus, vehicleType *domain.VehicleType) ([]*domain.Courier, error)
}

// AssignmentRepository governs persistence operations for CourierAssignment aggregates.
type AssignmentRepository interface {
	Create(ctx context.Context, tx *sql.Tx, asgn *domain.CourierAssignment) error
	GetActiveByParcelID(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*domain.CourierAssignment, error)
	GetActiveByParcelIDReadOnly(ctx context.Context, parcelID uuid.UUID) (*domain.CourierAssignment, error)
	CountActiveByCourierID(ctx context.Context, tx *sql.Tx, courierID uuid.UUID) (int, error)
	Update(ctx context.Context, tx *sql.Tx, asgn *domain.CourierAssignment) error
}

// UnassignedParcelDTO represents an eligible parcel waiting for dispatch on the Dispatcher Board.
type UnassignedParcelDTO struct {
	ID             string    `json:"id"`
	TrackingNumber string    `json:"tracking_number"`
	WeightKg       float64   `json:"weight_kg"`
	RecipientCity  string    `json:"recipient_city"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// CourierLoadDTO represents active workload and capacity metrics for a courier on the Dispatcher Board.
type CourierLoadDTO struct {
	CourierID         string             `json:"courier_id"`
	Name              string             `json:"name"`
	EmployeeCode      string             `json:"employee_code"`
	CurrentZoneID     string             `json:"current_zone_id"`
	ActiveAssignments int                `json:"active_assignments"`
	MaxCapacity       int                `json:"max_capacity"`
	Status            domain.ShiftStatus `json:"status"`
	VehicleType       domain.VehicleType `json:"vehicle_type"`
}

// DispatchBoardReader provides optimized, read-only queries for the Dispatcher Board.
type DispatchBoardReader interface {
	GetUnassignedParcels(ctx context.Context, zoneID string) ([]UnassignedParcelDTO, error)
	GetCourierLoads(ctx context.Context, zoneID string) ([]CourierLoadDTO, error)
}

