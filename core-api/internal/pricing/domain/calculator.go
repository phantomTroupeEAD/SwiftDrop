package domain

import (
	"math"
	"math/big"
)

const (
	// weightDecimalScale preserves six fractional decimal digits (1e-6 kg = 1 mg),
	// covering both NUMERIC(8,4) persistence and volumetric weight calculations without float string parsing.
	weightDecimalScale int64 = 1_000_000
)

// CalculateDeliveryFee calculates the final delivery fee using
// proportional additional-weight charging and the requested service-level multiplier.
func CalculateDeliveryFee(tariff Tariff, weight Weight, serviceLevel ServiceLevel) (Money, error) {
	if err := tariff.Validate(); err != nil {
		return Money{}, ErrInvalidTariff
	}

	if err := serviceLevel.Validate(); err != nil {
		return Money{}, err
	}

	if err := validateCalculatedWeight(weight); err != nil {
		return Money{}, err
	}

	// The base fee covers the first 1 kg.
	// Derive an exact rational from the bounded decimal weight without string formatting/parsing.
	// validateCalculatedWeight bounds ChargeableKg to (0, MaxChargeableWeightKg] (30 kg).
	chargeableScaled := int64(math.Round(weight.ChargeableKg * float64(weightDecimalScale)))
	var extraScaled int64
	if chargeableScaled > weightDecimalScale {
		extraScaled = chargeableScaled - weightDecimalScale
	}
	extraWeight := big.NewRat(extraScaled, weightDecimalScale)

	// Calculate the additional weight fee.
	perKgRate := new(big.Rat).SetInt64(tariff.PerKgRate.Amount)
	weightFee := new(big.Rat).Mul(extraWeight, perKgRate)

	// Add the weight fee to the base fee.
	baseFee := new(big.Rat).SetInt64(tariff.BaseFee.Amount)
	subtotal := new(big.Rat).Add(baseFee, weightFee)

	// Obtain the exact service-level multiplier.
	numerator, denominator, err := serviceLevel.MultiplierRatio()
	if err != nil {
		return Money{}, err
	}

	multiplier := new(big.Rat).SetFrac(
		big.NewInt(numerator),
		big.NewInt(denominator),
	)

	// Apply the multiplier before rounding the final monetary amount.
	finalAmount := new(big.Rat).Mul(subtotal, multiplier)

	// Round the final amount to the nearest cent using half-up rounding.
	roundedAmount, ok := roundRatHalfUp(finalAmount)
	if !ok {
		return Money{}, ErrMoneyOverflow
	}

	return NewMoney(roundedAmount, CurrencyLKR)
}

// roundRatHalfUp rounds a non-negative rational number to the nearest
// integer, with exact half values rounded upward.
func roundRatHalfUp(value *big.Rat) (int64, bool) {
	if value.Sign() < 0 {
		return 0, false
	}

	numerator := value.Num()
	denominator := value.Denom()

	quotient := new(big.Int)
	remainder := new(big.Int)

	quotient.QuoRem(numerator, denominator, remainder)

	// If the fractional part is at least 0.5, round upward.
	twiceRemainder := new(big.Int).Lsh(remainder, 1)

	if twiceRemainder.Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}

	if !quotient.IsInt64() {
		return 0, false
	}

	return quotient.Int64(), true
}

// validateCalculatedWeight ensures the calculator receives
// a consistent and valid Weight value.
func validateCalculatedWeight(weight Weight) error {
	values := []float64{
		weight.ActualKg,
		weight.VolumetricKg,
		weight.ChargeableKg,
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return ErrInvalidWeight
		}
	}

	if weight.ChargeableKg > MaxChargeableWeightKg {
		return ErrChargeableWeightExceeded
	}

	expectedChargeableWeight := math.Max(
		weight.ActualKg,
		weight.VolumetricKg,
	)

	if weight.ChargeableKg != expectedChargeableWeight {
		return ErrInvalidWeight
	}

	return nil
}
