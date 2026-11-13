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

func TestGeoCoordinate_BoundaryValues(t *testing.T) {
	// Exact lower bounds (-90, -180) -> valid
	c1, err := domain.NewGeoCoordinate(-90.0, -180.0)
	if err != nil || c1.Latitude != -90.0 || c1.Longitude != -180.0 {
		t.Errorf("expected valid for exact min boundaries (-90, -180), got err: %v", err)
	}

	// Exact upper bounds (+90, +180) -> valid
	c2, err := domain.NewGeoCoordinate(90.0, 180.0)
	if err != nil || c2.Latitude != 90.0 || c2.Longitude != 180.0 {
		t.Errorf("expected valid for exact max boundaries (90, 180), got err: %v", err)
	}

	// Center point (0, 0) -> valid
	c3, err := domain.NewGeoCoordinate(0.0, 0.0)
	if err != nil || c3.Latitude != 0.0 || c3.Longitude != 0.0 {
		t.Errorf("expected valid for center point (0, 0), got err: %v", err)
	}

	// Just below lower boundary for lat (-90.0001) -> error
	_, err = domain.NewGeoCoordinate(-90.0001, 0.0)
	if !errors.Is(err, domain.ErrInvalidCoordinates) {
		t.Errorf("expected ErrInvalidCoordinates for lat -90.0001, got %v", err)
	}

	// Just above upper boundary for lat (+90.0001) -> error
	_, err = domain.NewGeoCoordinate(90.0001, 0.0)
	if !errors.Is(err, domain.ErrInvalidCoordinates) {
		t.Errorf("expected ErrInvalidCoordinates for lat 90.0001, got %v", err)
	}

	// Just below lower boundary for lng (-180.0001) -> error
	_, err = domain.NewGeoCoordinate(0.0, -180.0001)
	if !errors.Is(err, domain.ErrInvalidCoordinates) {
		t.Errorf("expected ErrInvalidCoordinates for lng -180.0001, got %v", err)
	}

	// Just above upper boundary for lng (+180.0001) -> error
	_, err = domain.NewGeoCoordinate(0.0, 180.0001)
	if !errors.Is(err, domain.ErrInvalidCoordinates) {
		t.Errorf("expected ErrInvalidCoordinates for lng 180.0001, got %v", err)
	}
}
