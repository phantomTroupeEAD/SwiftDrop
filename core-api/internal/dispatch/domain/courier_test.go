package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

func TestNewCourier_Validation(t *testing.T) {
	validID := uuid.New()

	// 1. Success with default capacity
	c, err := domain.NewCourier(validID, "CR-0012", domain.VehicleTypeMotorcycle, 0, "ZONE_COLOMBO_01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.MaxCapacity != 25 {
		t.Errorf("expected default capacity 25 for motorcycle, got %d", c.MaxCapacity)
	}
	if c.Status != domain.ShiftStatusOffShift {
		t.Errorf("expected initial status OFF_SHIFT, got %s", c.Status)
	}

	// 2. Invalid vehicle
	_, err = domain.NewCourier(validID, "CR-0012", "BICYCLE", 25, "ZONE_COLOMBO_01")
	if !errors.Is(err, domain.ErrInvalidVehicleType) {
		t.Errorf("expected ErrInvalidVehicleType, got %v", err)
	}

	// 3. Invalid capacity bounds
	_, err = domain.NewCourier(validID, "CR-0012", domain.VehicleTypeVan, 250, "ZONE_COLOMBO_01")
	if !errors.Is(err, domain.ErrInvalidCapacity) {
		t.Errorf("expected ErrInvalidCapacity, got %v", err)
	}

	// 4. Blank employee code
	_, err = domain.NewCourier(validID, "   ", domain.VehicleTypeTruck, 120, "ZONE_COLOMBO_01")
	if !errors.Is(err, domain.ErrInvalidEmployeeCode) {
		t.Errorf("expected ErrInvalidEmployeeCode, got %v", err)
	}
}

func TestCourier_CanAcceptAssignment(t *testing.T) {
	c, err := domain.NewCourier(uuid.New(), "CR-0012", domain.VehicleTypeMotorcycle, 25, "ZONE_COLOMBO_01")
	if err != nil {
		t.Fatalf("failed to initialize courier: %v", err)
	}

	// 1. Off-shift courier cannot accept assignments
	err = c.CanAcceptAssignment(0)
	if !errors.Is(err, domain.ErrCourierNotOnShift) {
		t.Errorf("expected ErrCourierNotOnShift, got %v", err)
	}

	// 2. Put courier ON_SHIFT
	err = c.ToggleShift(domain.ShiftStatusOnShift, true)
	if err != nil {
		t.Fatalf("failed to toggle shift to ON_SHIFT: %v", err)
	}

	// 3. Workload under max capacity (24 active + 1 = 25 <= 25) -> Success
	err = c.CanAcceptAssignment(24)
	if err != nil {
		t.Errorf("expected success for 24 workload, got error: %v", err)
	}

	// 4. Overload check (25 active + 1 = 26 > 25) -> ErrCourierOverloaded
	err = c.CanAcceptAssignment(25)
	if !errors.Is(err, domain.ErrCourierOverloaded) {
		t.Errorf("expected ErrCourierOverloaded, got %v", err)
	}

	// 5. Suspended courier cannot accept assignments
	_ = c.Suspend()
	err = c.CanAcceptAssignment(0)
	if !errors.Is(err, domain.ErrCourierSuspended) {
		t.Errorf("expected ErrCourierSuspended, got %v", err)
	}

	// 6. Inactive courier cannot accept assignments
	c.IsActive = false
	err = c.CanAcceptAssignment(0)
	if !errors.Is(err, domain.ErrCourierInactive) {
		t.Errorf("expected ErrCourierInactive, got %v", err)
	}
}

func TestCourier_ToggleShift(t *testing.T) {
	c, _ := domain.NewCourier(uuid.New(), "CR-0012", domain.VehicleTypeMotorcycle, 25, "ZONE_COLOMBO_01")

	// 1. Courier clocks in
	err := c.ToggleShift(domain.ShiftStatusOnShift, true)
	if err != nil || c.Status != domain.ShiftStatusOnShift {
		t.Errorf("expected ON_SHIFT, got %s (err: %v)", c.Status, err)
	}

	// 2. Courier tries to self-suspend -> Rejected
	err = c.ToggleShift(domain.ShiftStatusSuspended, true)
	if !errors.Is(err, domain.ErrUnauthorizedShiftChange) {
		t.Errorf("expected ErrUnauthorizedShiftChange on self-suspension, got %v", err)
	}

	// 3. Admin suspends courier
	err = c.Suspend()
	if err != nil || c.Status != domain.ShiftStatusSuspended {
		t.Errorf("expected SUSPENDED, got %s", c.Status)
	}

	// 4. Suspended courier tries to clock in -> Rejected
	err = c.ToggleShift(domain.ShiftStatusOnShift, true)
	if !errors.Is(err, domain.ErrUnauthorizedShiftChange) {
		t.Errorf("expected ErrUnauthorizedShiftChange, got %v", err)
	}

	// 5. Admin reactivates
	err = c.Reactivate()
	if err != nil || c.Status != domain.ShiftStatusOffShift {
		t.Errorf("expected OFF_SHIFT after reactivation, got %s", c.Status)
	}
}

func TestCourier_UpdateCapacity_And_Zone(t *testing.T) {
	c, err := domain.NewCourier(uuid.New(), "CR-0012", domain.VehicleTypeMotorcycle, 25, "ZONE_COLOMBO_01")
	if err != nil {
		t.Fatalf("failed to initialize courier: %v", err)
	}

	// Boundary Value Analysis for Capacity [1, 200]
	// Below lower bound (0) -> error
	err = c.UpdateCapacity(0)
	if !errors.Is(err, domain.ErrInvalidCapacity) {
		t.Errorf("expected ErrInvalidCapacity for capacity 0, got %v", err)
	}

	// Negative value (-10) -> error
	err = c.UpdateCapacity(-10)
	if !errors.Is(err, domain.ErrInvalidCapacity) {
		t.Errorf("expected ErrInvalidCapacity for capacity -10, got %v", err)
	}

	// Above upper bound (201) -> error
	err = c.UpdateCapacity(201)
	if !errors.Is(err, domain.ErrInvalidCapacity) {
		t.Errorf("expected ErrInvalidCapacity for capacity 201, got %v", err)
	}

	// Valid lower boundary (1) -> success
	err = c.UpdateCapacity(1)
	if err != nil || c.MaxCapacity != 1 {
		t.Errorf("expected capacity 1, got %d (err: %v)", c.MaxCapacity, err)
	}

	// Valid upper boundary (200) -> success
	err = c.UpdateCapacity(200)
	if err != nil || c.MaxCapacity != 200 {
		t.Errorf("expected capacity 200, got %d (err: %v)", c.MaxCapacity, err)
	}

	// Zone update validation
	// Empty zone -> error
	err = c.UpdateZone("")
	if !errors.Is(err, domain.ErrInvalidZoneID) {
		t.Errorf("expected ErrInvalidZoneID for empty zone, got %v", err)
	}

	// Whitespace only zone -> error
	err = c.UpdateZone("    ")
	if !errors.Is(err, domain.ErrInvalidZoneID) {
		t.Errorf("expected ErrInvalidZoneID for whitespace zone, got %v", err)
	}

	// Valid zone with whitespace trimming -> success
	err = c.UpdateZone("  ZONE_KANDY_02  ")
	if err != nil || c.CurrentZoneID != "ZONE_KANDY_02" {
		t.Errorf("expected zone 'ZONE_KANDY_02', got '%s' (err: %v)", c.CurrentZoneID, err)
	}
}
