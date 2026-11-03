package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Courier is the aggregate root governing a courier's operational fleet profile, shift status, and capacity.
type Courier struct {
	UserID        uuid.UUID
	EmployeeCode  string
	Vehicle       VehicleType
	MaxCapacity   int
	Status        ShiftStatus
	CurrentZoneID string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewCourier creates and validates a new operational courier profile.
func NewCourier(userID uuid.UUID, employeeCode string, vehicle VehicleType, maxCapacity int, zoneID string) (*Courier, error) {
	trimmedCode := strings.TrimSpace(employeeCode)
	if trimmedCode == "" {
		return nil, ErrInvalidEmployeeCode
	}

	if err := vehicle.Validate(); err != nil {
		return nil, err
	}

	if maxCapacity == 0 {
		maxCapacity = vehicle.DefaultCapacity()
	}

	if maxCapacity < 1 || maxCapacity > 200 {
		return nil, ErrInvalidCapacity
	}

	trimmedZone := strings.TrimSpace(zoneID)
	if trimmedZone == "" {
		return nil, ErrInvalidZoneID
	}

	now := time.Now().UTC()
	return &Courier{
		UserID:        userID,
		EmployeeCode:  trimmedCode,
		Vehicle:       vehicle,
		MaxCapacity:   maxCapacity,
		Status:        ShiftStatusOffShift,
		CurrentZoneID: trimmedZone,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// CanAcceptAssignment evaluates whether this courier is currently eligible to take on an additional parcel delivery.
func (c *Courier) CanAcceptAssignment(currentActiveWorkload int) error {
	if !c.IsActive {
		return ErrCourierInactive
	}

	if c.Status == ShiftStatusSuspended {
		return ErrCourierSuspended
	}

	if c.Status != ShiftStatusOnShift {
		return ErrCourierNotOnShift
	}

	if currentActiveWorkload+1 > c.MaxCapacity {
		return ErrCourierOverloaded
	}

	return nil
}

// ToggleShift updates the courier's shift state in accordance with the shift lifecycle state machine.
func (c *Courier) ToggleShift(target ShiftStatus, isSelfService bool) error {
	if !c.IsActive {
		return ErrCourierInactive
	}

	if !c.Status.CanTransitionTo(target, isSelfService) {
		if isSelfService && (target == ShiftStatusSuspended || c.Status == ShiftStatusSuspended) {
			return ErrUnauthorizedShiftChange
		}
		return ErrInvalidShiftTransition
	}

	c.Status = target
	c.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateCapacity adjusts the maximum concurrent workload permitted for this courier.
func (c *Courier) UpdateCapacity(newCapacity int) error {
	if newCapacity < 1 || newCapacity > 200 {
		return ErrInvalidCapacity
	}

	c.MaxCapacity = newCapacity
	c.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateZone reassigns the primary operating delivery zone for this courier.
func (c *Courier) UpdateZone(zoneID string) error {
	trimmed := strings.TrimSpace(zoneID)
	if trimmed == "" {
		return ErrInvalidZoneID
	}

	c.CurrentZoneID = trimmed
	c.UpdatedAt = time.Now().UTC()
	return nil
}

// Suspend places an administrative operational hold on the courier.
func (c *Courier) Suspend() error {
	c.Status = ShiftStatusSuspended
	c.UpdatedAt = time.Now().UTC()
	return nil
}

// Reactivate restores a suspended or inactive courier back to an active OFF_SHIFT baseline.
func (c *Courier) Reactivate() error {
	c.IsActive = true
	c.Status = ShiftStatusOffShift
	c.UpdatedAt = time.Now().UTC()
	return nil
}

