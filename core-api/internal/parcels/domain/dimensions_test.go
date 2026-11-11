package domain

import (
	"errors"
	"math"
	"testing"
)

func TestDimensions_Validate(t *testing.T) {
	d, err := NewDimensions(30.0, 20.0, 15.0)
	if err != nil {
		t.Fatalf("unexpected error for valid dimensions: %v", err)
	}
	if d.LengthCm != 30.0 || d.WidthCm != 20.0 || d.HeightCm != 15.0 {
		t.Errorf("unexpected dimension values: %+v", d)
	}

	invalidTests := []struct {
		name    string
		l, w, h float64
	}{
		{"zero length", 0, 10, 10},
		{"negative width", 10, -5, 10},
		{"NaN height", 10, 10, math.NaN()},
		{"Inf length", math.Inf(1), 10, 10},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDimensions(tt.l, tt.w, tt.h)
			if !errors.Is(err, ErrInvalidDimensions) {
				t.Fatalf("expected ErrInvalidDimensions for %s, got: %v", tt.name, err)
			}
		})
	}
}
