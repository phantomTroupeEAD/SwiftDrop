package domain

import (
	"errors"
	"math"
	"testing"
)

func TestWeight_Validate(t *testing.T) {
	w, err := NewWeight(2.0, 3.5, 3.5)
	if err != nil {
		t.Fatalf("unexpected error for valid weight: %v", err)
	}
	if w.ChargeableKg != 3.5 {
		t.Errorf("expected chargeable weight 3.5, got %v", w.ChargeableKg)
	}

	invalidTests := []struct {
		name                    string
		actual, vol, chargeable float64
	}{
		{"zero actual", 0, 1.0, 1.0},
		{"negative volumetric", 1.0, -2.0, 1.0},
		{"exceeds max limit 30kg", 31.0, 20.0, 31.0},
		{"inconsistent chargeable weight", 2.0, 5.0, 2.0}, // max is 5.0, but chargeable is 2.0
		{"NaN actual weight", math.NaN(), 1.0, 1.0},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWeight(tt.actual, tt.vol, tt.chargeable)
			if !errors.Is(err, ErrInvalidWeight) {
				t.Fatalf("expected ErrInvalidWeight for %s, got: %v", tt.name, err)
			}
		})
	}
}
