package domain_test

import (
	"errors"
	"testing"

	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

func TestGeoCoordinate_Validation(t *testing.T) {
	// 1. Valid coordinates (Colombo, Sri Lanka)
	coord, err := domain.NewGeoCoordinate(6.9271, 79.8612)
	if err != nil {
		t.Fatalf("unexpected error for valid coordinates: %v", err)
	}
	if coord.Latitude != 6.9271 || coord.Longitude != 79.8612 {
		t.Errorf("coordinate values mismatch")
	}

	// 2. Latitude out of range
	_, err = domain.NewGeoCoordinate(95.0, 79.8612)
	if !errors.Is(err, domain.ErrInvalidCoordinates) {
		t.Errorf("expected ErrInvalidCoordinates for lat 95.0, got %v", err)
	}

	_, err = domain.NewGeoCoordinate(-91.0, 79.8612)
	if !errors.Is(err, domain.ErrInvalidCoordinates) {
		t.Errorf("expected ErrInvalidCoordinates for lat -91.0, got %v", err)
	}

	// 3. Longitude out of range
	_, err = domain.NewGeoCoordinate(6.9271, 185.0)
	if !errors.Is(err, domain.ErrInvalidCoordinates) {
		t.Errorf("expected ErrInvalidCoordinates for lng 185.0, got %v", err)
	}

	_, err = domain.NewGeoCoordinate(6.9271, -181.0)
	if !errors.Is(err, domain.ErrInvalidCoordinates) {
		t.Errorf("expected ErrInvalidCoordinates for lng -181.0, got %v", err)
	}
}

