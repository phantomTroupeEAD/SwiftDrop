package domain_test

import (
	"errors"
	"testing"

	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

func TestVehicleType_DefaultCapacity(t *testing.T) {
	tests := []struct {
		vehicle  domain.VehicleType
		expected int
	}{
		{domain.VehicleTypeMotorcycle, 25},
		{domain.VehicleTypeVan, 60},
		{domain.VehicleTypeTruck, 120},
		{domain.VehicleType("UNKNOWN"), 25},
	}

	for _, tc := range tests {
		actual := tc.vehicle.DefaultCapacity()
		if actual != tc.expected {
			t.Errorf("expected default capacity %d for %s, got %d", tc.expected, tc.vehicle, actual)
		}
	}
}

func TestVehicleType_Validate(t *testing.T) {
	valid := []domain.VehicleType{
		domain.VehicleTypeMotorcycle,
		domain.VehicleTypeVan,
		domain.VehicleTypeTruck,
	}

	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("expected valid vehicle type for %s, got error: %v", v, err)
		}
	}

	invalid := []domain.VehicleType{
		"BICYCLE",
		"CAR",
		"DRONE",
		"",
	}

	for _, v := range invalid {
		err := v.Validate()
		if err == nil {
			t.Errorf("expected error for invalid vehicle type %s, got nil", v)
		}
		if !errors.Is(err, domain.ErrInvalidVehicleType) {
			t.Errorf("expected ErrInvalidVehicleType for %s, got %v", v, err)
		}
	}
}
