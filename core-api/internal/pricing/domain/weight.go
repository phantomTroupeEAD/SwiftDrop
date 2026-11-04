package domain

import "math"

const (
	// VolumetricDivisor converts parcel volume (cm³) into volumetric weight (kg).
	VolumetricDivisor = 5000.0

	// MaxChargeableWeightKg is the maximum permitted chargeable weight.
	MaxChargeableWeightKg = 30.0
)

// Weight represents the actual, volumetric, and chargeable weight of a parcel.
type Weight struct {
	ActualKg     float64
	VolumetricKg float64
	ChargeableKg float64
}

// CalculateWeight calculates the volumetric and chargeable weights.
//
// Dimensions must always be provided in centimetres.
// Chargeable weight is always the greater of actual and volumetric weight.
func CalculateWeight(
	actualKg float64,
	lengthCm float64,
	widthCm float64,
	heightCm float64,
) (Weight, error) {

	// Validate actual weight.
	if math.IsNaN(actualKg) || math.IsInf(actualKg, 0) || actualKg <= 0 {
		return Weight{}, ErrInvalidWeight
	}

	// Validate dimensions.
	dimensions := []float64{lengthCm, widthCm, heightCm}

	for _, dimension := range dimensions {
		if math.IsNaN(dimension) ||
			math.IsInf(dimension, 0) ||
			dimension <= 0 {
			return Weight{}, ErrInvalidDimensions
		}
	}

	// Calculate volumetric weight.
	volumeCm3 := lengthCm * widthCm * heightCm
	volumetricKg := volumeCm3 / VolumetricDivisor

	// Calculate chargeable weight.
	chargeableKg := math.Max(actualKg, volumetricKg)

	// Enforce maximum chargeable weight.
	if chargeableKg > MaxChargeableWeightKg {
		return Weight{}, ErrChargeableWeightExceeded
	}

	return Weight{
		ActualKg:     actualKg,
		VolumetricKg: volumetricKg,
		ChargeableKg: chargeableKg,
	}, nil
}
