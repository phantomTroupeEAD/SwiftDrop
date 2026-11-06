package domain

import (
	"math"
	"math/big"
	"strconv"
)

// CalculateDeliveryFee calculates the final delivery fee using
// proportional additional-weight charging.
func CalculateDeliveryFee(tariff Tariff, weight Weight) (Money, error) {
	if err := tariff.Validate(); err != nil {
		return Money{}, ErrInvalidTariff
	}

	if err := validateCalculatedWeight(weight); err != nil {
		return Money{}, err
	}

	// The base fee covers the first 1 kg.
	// Additional weight is charged proportionally.
	extraKg := math.Max(0, weight.ChargeableKg-1)

	// Convert the decimal weight representation into an exact rational.
	extraWeight, ok := new(big.Rat).SetString(
		strconv.FormatFloat(extraKg, 'f', -1, 64),
	)
	if !ok {
		return Money{}, ErrInvalidWeight
	}

	// Calculate the additional weight fee.
	perKgRate := new(big.Rat).SetInt64(tariff.PerKgRate.Amount)
	weightFee := new(big.Rat).Mul(extraWeight, perKgRate)

	// Add the weight fee to the base fee.
	baseFee := new(big.Rat).SetInt64(tariff.BaseFee.Amount)
	subtotal := new(big.Rat).Add(baseFee, weightFee)

	// Obtain the exact service-level multiplier.
	numerator, denominator, err := tariff.ServiceMultiplierRatio()
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
