package transport

import (
	"time"

	"github.com/swiftdrop/core-api/internal/dispatch/application"
)

// CourierProfileResponse represents the outbound courier view.
type CourierProfileResponse struct {
	UserID        string    `json:"user_id"`
	EmployeeCode  string    `json:"employee_code"`
	VehicleType   string    `json:"vehicle_type"`
	MaxCapacity   int       `json:"max_capacity"`
	Status        string    `json:"status"`
	CurrentZoneID string    `json:"current_zone_id,omitempty"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
}

// CourierListItemResponse represents an active courier with workload metrics in a zone list.
type CourierListItemResponse struct {
	UserID            string `json:"user_id"`
	CourierID         string `json:"courier_id"`
	CourierName       string `json:"courier_name"`
	Name              string `json:"name"`
	EmployeeCode      string `json:"employee_code"`
	CurrentZoneID     string `json:"current_zone_id"`
	ActiveCount       int    `json:"active_count"`
	ActiveAssignments int    `json:"active_assignments"`
	MaxCapacity       int    `json:"max_capacity"`
	Status            string `json:"status"`
	VehicleType       string `json:"vehicle_type"`
}

// AssignmentResponse represents the created or updated courier assignment.
type AssignmentResponse struct {
	AssignmentID string    `json:"assignment_id"`
	ParcelID     string    `json:"parcel_id"`
	CourierID    string    `json:"courier_id"`
	AssignedBy   string    `json:"assigned_by"`
	Status       string    `json:"status"`
	AssignedAt   time.Time `json:"assigned_at"`
}

// CancelAssignmentResponse describes the canceled assignment.
type CancelAssignmentResponse struct {
	AssignmentID string    `json:"assignment_id"`
	ParcelID     string    `json:"parcel_id"`
	Status       string    `json:"status"`
	EndedAt      time.Time `json:"ended_at"`
	Reason       string    `json:"reason"`
}

// DispatchBoardResponse wraps unassigned parcels, active assignments, and courier workloads.
type DispatchBoardResponse struct {
	ZoneID            string                            `json:"zone_id"`
	UnassignedParcels []application.UnassignedParcelDTO `json:"unassigned_parcels"`
	AssignedParcels   []application.AssignedParcelDTO   `json:"assigned_parcels"`
	CourierLoads      []application.CourierLoadDTO      `json:"courier_loads"`
}

// ShiftStatusResponse describes a courier's new shift state.
type ShiftStatusResponse struct {
	CourierID string `json:"courier_id"`
	NewStatus string `json:"new_status"`
	UpdatedAt string `json:"updated_at"`
}
