package domain

import (
	"fmt"
	"math"
)

// MaxChargeableWeightKg defines the maximum allowed parcel weight across all service tiers.
const MaxChargeableWeightKg = 30.0

// Weight captures the actual, volumetric, and chargeable weight of a parcel in kilograms.
type Weight struct {
	ActualKg     float64
	VolumetricKg float64
	ChargeableKg float64
}

// NewWeight creates and validates a Weight value object.
func NewWeight(actualKg, volumetricKg, chargeableKg float64) (Weight, error) {
	w := Weight{
		ActualKg:     actualKg,
		VolumetricKg: volumetricKg,
		ChargeableKg: chargeableKg,
	}
	if err := w.Validate(); err != nil {
		return Weight{}, err
	}
	return w, nil
}

// Validate ensures weight components are positive, within system limits, and internally consistent.
func (w Weight) Validate() error {
	values := []struct {
		name  string
		value float64
	}{
		{"actual weight", w.ActualKg},
		{"volumetric weight", w.VolumetricKg},
		{"chargeable weight", w.ChargeableKg},
	}

	for _, v := range values {
		if math.IsNaN(v.value) || math.IsInf(v.value, 0) || v.value <= 0 {
			return fmt.Errorf("%w: %s must be a positive finite number, got %v", ErrInvalidWeight, v.name, v.value)
		}
	}

	if w.ChargeableKg > MaxChargeableWeightKg {
		return fmt.Errorf("%w: chargeable weight %.2f kg exceeds maximum limit of %.2f kg",
			ErrInvalidWeight, w.ChargeableKg, MaxChargeableWeightKg)
	}

	expectedChargeable := math.Max(w.ActualKg, w.VolumetricKg)
	if math.Abs(w.ChargeableKg-expectedChargeable) > 1e-6 {
		return fmt.Errorf("%w: chargeable weight %.2f does not match expected maximum of actual (%.2f) and volumetric (%.2f)",
			ErrInvalidWeight, w.ChargeableKg, w.ActualKg, w.VolumetricKg)
	}

	return nil
}
