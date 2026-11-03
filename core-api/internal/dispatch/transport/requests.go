package transport

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// RegisterCourierRequest represents the JSON payload to onboard a courier into the operational fleet.
type RegisterCourierRequest struct {
	UserID        string `json:"user_id"`
	EmployeeCode  string `json:"employee_code"`
	VehicleType   string `json:"vehicle_type"`
	MaxCapacity   int    `json:"max_capacity"`
	CurrentZoneID string `json:"current_zone_id"`
}

// Validate checks request invariants.
func (r *RegisterCourierRequest) Validate() []httpx.ErrorDetail {
	var details []httpx.ErrorDetail
	if _, err := uuid.Parse(strings.TrimSpace(r.UserID)); err != nil {
		details = append(details, httpx.ErrorDetail{Field: "user_id", Issue: "Must be a valid UUID"})
	}
	if strings.TrimSpace(r.EmployeeCode) == "" {
		details = append(details, httpx.ErrorDetail{Field: "employee_code", Issue: "Employee code is required"})
	}
	if strings.TrimSpace(r.VehicleType) == "" {
		details = append(details, httpx.ErrorDetail{Field: "vehicle_type", Issue: "Vehicle type is required (MOTORCYCLE, VAN, TRUCK)"})
	}
	if r.MaxCapacity < 0 || r.MaxCapacity > 200 {
		details = append(details, httpx.ErrorDetail{Field: "max_capacity", Issue: "Max capacity must be between 1 and 200 (or 0 for vehicle default)"})
	}
	return details
}

// AssignParcelRequest represents the JSON payload to assign a parcel to a courier.
type AssignParcelRequest struct {
	ParcelID  string `json:"parcel_id"`
	CourierID string `json:"courier_id"`
	Notes     string `json:"notes"`
}

// Validate checks assign parcel request fields.
func (r *AssignParcelRequest) Validate() []httpx.ErrorDetail {
	var details []httpx.ErrorDetail
	if _, err := uuid.Parse(strings.TrimSpace(r.ParcelID)); err != nil {
		details = append(details, httpx.ErrorDetail{Field: "parcel_id", Issue: "Must be a valid parcel UUID"})
	}
	if _, err := uuid.Parse(strings.TrimSpace(r.CourierID)); err != nil {
		details = append(details, httpx.ErrorDetail{Field: "courier_id", Issue: "Must be a valid courier UUID"})
	}
	return details
}

// CancelAssignmentRequest represents the JSON payload to cancel an active assignment.
type CancelAssignmentRequest struct {
	Reason string `json:"reason"`
}

// Validate checks cancel assignment request fields.
func (r *CancelAssignmentRequest) Validate() error {
	if strings.TrimSpace(r.Reason) == "" {
		return errors.New("cancellation reason is required")
	}
	return nil
}

// RescheduleParcelRequest represents the JSON payload for dispatcher-supervised reattempts.
type RescheduleParcelRequest struct {
	CourierID string `json:"courier_id"`
	Notes     string `json:"notes"`
}

// Validate checks reschedule request fields.
func (r *RescheduleParcelRequest) Validate() []httpx.ErrorDetail {
	var details []httpx.ErrorDetail
	if _, err := uuid.Parse(strings.TrimSpace(r.CourierID)); err != nil {
		details = append(details, httpx.ErrorDetail{Field: "courier_id", Issue: "Must be a valid courier UUID"})
	}
	return details
}

// ToggleShiftRequest represents the JSON payload to update a courier's shift state.
type ToggleShiftRequest struct {
	TargetStatus string `json:"target_status"`
	CourierID    string `json:"courier_id,omitempty"`
}

// Validate checks toggle shift request fields.
func (r *ToggleShiftRequest) Validate() []httpx.ErrorDetail {
	var details []httpx.ErrorDetail
	if strings.TrimSpace(r.TargetStatus) == "" {
		details = append(details, httpx.ErrorDetail{Field: "target_status", Issue: "Target shift status is required"})
	}
	return details
}

