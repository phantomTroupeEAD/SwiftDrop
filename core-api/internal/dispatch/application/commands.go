package application

import (
	"errors"
	"time"

	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

var (
	ErrParcelNotEligibleForAssignment   = errors.New("parcel is not eligible for assignment (must be in CREATED or ATTEMPT_FAILED status)")
	ErrCourierNotFound                  = errors.New("courier operational profile not found")
	ErrAssignmentNotFound               = errors.New("active courier assignment not found for parcel")
	ErrParcelNotFound                   = errors.New("target parcel not found")
	ErrInvalidCommand                   = errors.New("invalid application command input")
	ErrDuplicateActiveAssignment        = errors.New("parcel already has an active courier assignment")
	ErrParcelNotAssigned                = errors.New("parcel is not currently assigned to a courier")
	ErrInvalidParcelStatusForAssignment = errors.New("parcel is not in a valid state for assignment")
	ErrMaxAttemptsExceeded              = errors.New("parcel has reached the maximum permitted delivery attempts")
	ErrUnauthorized                     = errors.New("unauthorized dispatch operation")
	ErrUserNotFound                     = errors.New("user account not found")
	ErrUserNotCourier                   = errors.New("user account does not possess the COURIER role")
	ErrCourierProfileExists             = errors.New("courier operational profile already exists for this user")
)

// AssignCourierCommand carries input parameters for parcel dispatch.
type AssignCourierCommand struct {
	ParcelID   string `json:"parcel_id"`
	CourierID  string `json:"courier_id"`
	AssignedBy string `json:"assigned_by"`
	Notes      string `json:"notes"`
}

// AssignmentResultDTO represents the outcome of an assignment operation.
type AssignmentResultDTO struct {
	AssignmentID string    `json:"assignment_id"`
	ParcelID     string    `json:"parcel_id"`
	CourierID    string    `json:"courier_id"`
	AssignedAt   time.Time `json:"assigned_at"`
	AssignedBy   string    `json:"assigned_by"`
	Status       string    `json:"status"`
}

// CancelAssignmentCommand carries input parameters for cancelling an active courier assignment.
type CancelAssignmentCommand struct {
	ParcelID    string `json:"parcel_id"`
	CancelledBy string `json:"cancelled_by"`
	Reason      string `json:"reason"`
}

// RescheduleParcelCommand carries dispatcher authorization to reattempt a failed parcel.
type RescheduleParcelCommand struct {
	ParcelID    string `json:"parcel_id"`
	CourierID   string `json:"courier_id"`
	ScheduledBy string `json:"scheduled_by"`
	Notes       string `json:"notes"`
}

// ToggleShiftCommand carries self-service or administrative shift transition requests.
type ToggleShiftCommand struct {
	CourierID     string             `json:"courier_id"`
	TargetStatus  domain.ShiftStatus `json:"target_status"`
	IsSelfService bool               `json:"is_self_service"`
}

// RegisterCourierCommand carries onboarding parameters for courier operational fleet profiles.
type RegisterCourierCommand struct {
	UserID        string             `json:"user_id"`
	EmployeeCode  string             `json:"employee_code"`
	VehicleType   domain.VehicleType `json:"vehicle_type"`
	MaxCapacity   int                `json:"max_capacity"`
	CurrentZoneID string             `json:"current_zone_id"`
}

// DispatchBoardDTO is the response envelope for the dispatcher board query.
type DispatchBoardDTO struct {
	ZoneID            string                `json:"zone_id"`
	UnassignedParcels []UnassignedParcelDTO `json:"unassigned_parcels"`
	ActiveCouriers    []CourierLoadDTO      `json:"active_couriers"`
}
