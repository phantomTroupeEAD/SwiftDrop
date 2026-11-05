package domain

import (
	"errors"
	"math"
	"testing"
)

func TestCalculateWeight(t *testing.T) {
	tests := []struct {
		name               string
		actualKg           float64
		lengthCm           float64
		widthCm            float64
		heightCm           float64
		expectedVolumetric float64
		expectedChargeable float64
	}{
		{
			name:               "volumetric weight is greater",
			actualKg:           2,
			lengthCm:           30,
			widthCm:            25,
			heightCm:           20,
			expectedVolumetric: 3,
			expectedChargeable: 3,
		},
		{
			name:               "actual weight is greater",
			actualKg:           5,
			lengthCm:           20,
			widthCm:            20,
			heightCm:           20,
			expectedVolumetric: 1.6,
			expectedChargeable: 5,
		},
		{
			name:               "actual and volumetric weights are equal",
			actualKg:           2,
			lengthCm:           10,
			widthCm:            20,
			heightCm:           50,
			expectedVolumetric: 2,
			expectedChargeable: 2,
		},
		{
			name:               "actual weight below one kilogram",
			actualKg:           0.5,
			lengthCm:           20,
			widthCm:            20,
			heightCm:           20,
			expectedVolumetric: 1.6,
			expectedChargeable: 1.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateWeight(
				tt.actualKg,
				tt.lengthCm,
				tt.widthCm,
				tt.heightCm,
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(got.VolumetricKg-tt.expectedVolumetric) > 0.000001 {
				t.Errorf(
					"volumetric weight = %v, want %v",
					got.VolumetricKg,
					tt.expectedVolumetric,
				)
			}

			if math.Abs(got.ChargeableKg-tt.expectedChargeable) > 0.000001 {
				t.Errorf(
					"chargeable weight = %v, want %v",
					got.ChargeableKg,
					tt.expectedChargeable,
				)
			}
		})
	}
}

func TestCalculateWeightInvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		actualKg    float64
		lengthCm    float64
		widthCm     float64
		heightCm    float64
		expectedErr error
	}{
		{
			name:        "zero actual weight",
			actualKg:    0,
			lengthCm:    10,
			widthCm:     10,
			heightCm:    10,
			expectedErr: ErrInvalidWeight,
		},
		{
			name:        "negative actual weight",
			actualKg:    -2,
			lengthCm:    10,
			widthCm:     10,
			heightCm:    10,
			expectedErr: ErrInvalidWeight,
		},
		{
			name:        "zero length",
			actualKg:    2,
			lengthCm:    0,
			widthCm:     10,
			heightCm:    10,
			expectedErr: ErrInvalidDimensions,
		},
		{
			name:        "negative width",
			actualKg:    2,
			lengthCm:    10,
			widthCm:     -5,
			heightCm:    10,
			expectedErr: ErrInvalidDimensions,
		},
		{
			name:        "invalid infinite height",
			actualKg:    2,
			lengthCm:    10,
			widthCm:     10,
			heightCm:    math.Inf(1),
			expectedErr: ErrInvalidDimensions,
		},
		{
			name:        "NaN actual weight",
			actualKg:    math.NaN(),
			lengthCm:    10,
			widthCm:     10,
			heightCm:    10,
			expectedErr: ErrInvalidWeight,
		},
		{
			name:        "chargeable weight exceeds maximum",
			actualKg:    20,
			lengthCm:    100,
			widthCm:     100,
			heightCm:    100,
			expectedErr: ErrChargeableWeightExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CalculateWeight(
				tt.actualKg,
				tt.lengthCm,
				tt.widthCm,
				tt.heightCm,
			)

			if !errors.Is(err, tt.expectedErr) {
				t.Errorf(
					"error = %v, want %v",


					
					err,
					tt.expectedErr,
				)
			}
		})
	}
}
