package domain

import (
	"math"
	"testing"
)

func calculatorTestTariff() Tariff {
	return Tariff{
		ID:                "tariff-001",
		OriginZoneID:      "ZONE_COLOMBO_01",
		DestinationZoneID: "ZONE_GAMPAHA_01",
		BaseFee:           Money{Amount: 35000, Currency: CurrencyLKR},
		PerKgRate:         Money{Amount: 8000, Currency: CurrencyLKR},
		EffectiveFrom:     validTariff().EffectiveFrom,
		Version:           1,
		IsActive:          true,
	}
}

func TestCalculateDeliveryFee(t *testing.T) {
	tests := []struct {
		name           string
		weight         Weight
		serviceLevel   ServiceLevel
		expectedAmount int64
	}{
		{
			name: "standard service at 2.5 kg",
			weight: Weight{
				ActualKg: 2, VolumetricKg: 2.5, ChargeableKg: 2.5,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 47000,
		},
		{
			name: "express service at 2.5 kg",
			weight: Weight{
				ActualKg: 2, VolumetricKg: 2.5, ChargeableKg: 2.5,
			},
			serviceLevel:   ServiceLevelExpress,
			expectedAmount: 70500,
		},
		{
			name: "same day service at 2.5 kg",
			weight: Weight{
				ActualKg: 2, VolumetricKg: 2.5, ChargeableKg: 2.5,
			},
			serviceLevel:   ServiceLevelSameDay,
			expectedAmount: 94000,
		},
		{
			name: "below one kilogram",
			weight: Weight{
				ActualKg: 0.5, VolumetricKg: 0.8, ChargeableKg: 0.8,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 35000,
		},
		{
			name: "exactly one kilogram",
			weight: Weight{
				ActualKg: 1, VolumetricKg: 0.8, ChargeableKg: 1,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 35000,
		},
		{
			name: "exactly two kilograms",
			weight: Weight{
				ActualKg: 2, VolumetricKg: 1.5, ChargeableKg: 2,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 43000,
		},
		{
			name: "fractional additional weight",
			weight: Weight{
				ActualKg: 2, VolumetricKg: 2.1, ChargeableKg: 2.1,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 43800,
		},
		{
			name: "one and a half additional kilograms",
			weight: Weight{
				ActualKg: 2.5, VolumetricKg: 2, ChargeableKg: 2.5,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 47000,
		},
		{
			name: "fractional weight 2.15 kg standard",
			weight: Weight{
				ActualKg: 2.15, VolumetricKg: 1.5, ChargeableKg: 2.15,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 44200,
		},
		{
			name: "fractional weight 2.15 kg express",
			weight: Weight{
				ActualKg: 2.15, VolumetricKg: 1.5, ChargeableKg: 2.15,
			},
			serviceLevel:   ServiceLevelExpress,
			expectedAmount: 66300,
		},
		{
			name: "four-decimal minimum extra weight 1.0001 kg",
			weight: Weight{
				ActualKg: 1.0001, VolumetricKg: 1, ChargeableKg: 1.0001,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 35001,
		},
		{
			name: "maximum weight 30 kg standard",
			weight: Weight{
				ActualKg: 30, VolumetricKg: 25, ChargeableKg: 30,
			},
			serviceLevel:   ServiceLevelStandard,
			expectedAmount: 267000,
		},
		{
			name: "maximum weight 30 kg express",
			weight: Weight{
				ActualKg: 20, VolumetricKg: 30, ChargeableKg: 30,
			},
			serviceLevel:   ServiceLevelExpress,
			expectedAmount: 400500,
		},
		{
			name: "maximum weight 30 kg same day",
			weight: Weight{
				ActualKg: 30, VolumetricKg: 30, ChargeableKg: 30,
			},
			serviceLevel:   ServiceLevelSameDay,
			expectedAmount: 534000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tariff := calculatorTestTariff()

			actual, err := CalculateDeliveryFee(tariff, test.weight, test.serviceLevel)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if actual.Amount != test.expectedAmount {
				t.Errorf(
					"expected %d cents, got %d cents",
					test.expectedAmount,
					actual.Amount,
				)
			}

			if actual.Currency != CurrencyLKR {
				t.Errorf("expected currency LKR, got %s", actual.Currency)
			}
		})
	}
}

func TestCalculateDeliveryFeeRoundsHalfUp(t *testing.T) {
	tariff := calculatorTestTariff()
	tariff.BaseFee = Money{Amount: 1, Currency: CurrencyLKR}
	tariff.PerKgRate = Money{Amount: 0, Currency: CurrencyLKR}

	weight := Weight{
		ActualKg:     1,
		VolumetricKg: 1,
		ChargeableKg: 1,
	}

	actual, err := CalculateDeliveryFee(tariff, weight, ServiceLevelExpress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 cent × 1.5 = 1.5 cents, rounded half-up to 2 cents.
	if actual.Amount != 2 {
		t.Errorf("expected 2 cents, got %d cents", actual.Amount)
	}

	// Fractional decimal weight 1.15 kg (extra 0.15 kg) × 10 cents = 1.5 cents,
	// testing decimal half-up rounding without float64 binary representation drift.
	fractionalTariff := calculatorTestTariff()
	fractionalTariff.BaseFee = Money{Amount: 0, Currency: CurrencyLKR}
	fractionalTariff.PerKgRate = Money{Amount: 10, Currency: CurrencyLKR}

	fractionalWeight := Weight{
		ActualKg:     1.15,
		VolumetricKg: 1.0,
		ChargeableKg: 1.15,
	}

	fractionalFee, err := CalculateDeliveryFee(fractionalTariff, fractionalWeight, ServiceLevelStandard)
	if err != nil {
		t.Fatalf("unexpected error for fractional half-up rounding: %v", err)
	}
	if fractionalFee.Amount != 2 {
		t.Errorf("expected 2 cents for 0.15 * 10 half-up, got %d cents", fractionalFee.Amount)
	}
}

func TestCalculateDeliveryFee_FromCalculateWeightVolumetricAndMax(t *testing.T) {
	tariff := calculatorTestTariff()

	// Volumetric: (25.5 * 20.2 * 15.4) / 5000 = 1.586508 kg (> 1.0 kg actual)
	// Extra: 0.586508 * 8000 = 4692.064 -> 4692; Subtotal: 35000 + 4692 = 39692
	volWeight, err := CalculateWeight(1.0, 25.5, 20.2, 15.4)
	if err != nil {
		t.Fatalf("CalculateWeight failed for volumetric fractional test: %v", err)
	}
	fee, err := CalculateDeliveryFee(tariff, volWeight, ServiceLevelStandard)
	if err != nil {
		t.Fatalf("CalculateDeliveryFee failed: %v", err)
	}
	if fee.Amount != 39692 {
		t.Errorf("expected 39692 cents for volumetric fractional weight, got %d", fee.Amount)
	}

	// Maximum volumetric weight: (50 * 60 * 50) / 5000 = 30 kg -> Express = 400500
	maxWeight, err := CalculateWeight(12.0, 50, 60, 50)
	if err != nil {
		t.Fatalf("CalculateWeight failed for max 30 kg volumetric parcel: %v", err)
	}
	maxFee, err := CalculateDeliveryFee(tariff, maxWeight, ServiceLevelExpress)
	if err != nil {
		t.Fatalf("CalculateDeliveryFee failed at 30 kg: %v", err)
	}
	if maxFee.Amount != 400500 {
		t.Errorf("expected 400500 cents for 30 kg express, got %d", maxFee.Amount)
	}
}

func TestCalculateDeliveryFeeRejectsInvalidWeight(t *testing.T) {
	tests := []struct {
		name   string
		weight Weight
	}{
		{
			name: "zero chargeable weight",
			weight: Weight{
				ActualKg: 1, VolumetricKg: 1, ChargeableKg: 0,
			},
		},
		{
			name: "inconsistent chargeable weight",
			weight: Weight{
				ActualKg: 2, VolumetricKg: 1, ChargeableKg: 1,
			},
		},
		{
			name: "weight above maximum",
			weight: Weight{
				ActualKg: 31, VolumetricKg: 20, ChargeableKg: 31,
			},
		},
		{
			name: "NaN chargeable weight",
			weight: Weight{
				ActualKg: 1, VolumetricKg: 1, ChargeableKg: math.NaN(),
			},
		},
		{
			name: "infinite actual weight",
			weight: Weight{
				ActualKg:     math.Inf(1),
				VolumetricKg: 1,
				ChargeableKg: math.Inf(1),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CalculateDeliveryFee(
				calculatorTestTariff(),
				test.weight,
				ServiceLevelStandard,
			)

			if err != ErrInvalidWeight &&
				err != ErrChargeableWeightExceeded {
				t.Fatalf("expected a weight validation error, got: %v", err)
			}
		})
	}
}

func TestCalculateDeliveryFeeRejectsInvalidTariff(t *testing.T) {
	tariff := calculatorTestTariff()
	tariff.OriginZoneID = ""

	weight := Weight{
		ActualKg: 1, VolumetricKg: 1, ChargeableKg: 1,
	}

	_, err := CalculateDeliveryFee(tariff, weight, ServiceLevelStandard)

	if err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff, got: %v", err)
	}
}

func TestCalculateDeliveryFeeRejectsInvalidServiceLevel(t *testing.T) {
	tariff := calculatorTestTariff()
	weight := Weight{
		ActualKg: 1, VolumetricKg: 1, ChargeableKg: 1,
	}

	_, err := CalculateDeliveryFee(tariff, weight, ServiceLevel("INVALID"))

	if err != ErrInvalidServiceLevel {
		t.Fatalf("expected ErrInvalidServiceLevel, got: %v", err)
	}
}

func TestCalculateDeliveryFeeRejectsOverflow(t *testing.T) {
	tariff := calculatorTestTariff()
	tariff.BaseFee = Money{
		Amount: maxInt64, Currency: CurrencyLKR,
	}

	weight := Weight{
		ActualKg: 1, VolumetricKg: 1, ChargeableKg: 1,
	}

	_, err := CalculateDeliveryFee(tariff, weight, ServiceLevelSameDay)

	if err != ErrMoneyOverflow {
		t.Fatalf("expected ErrMoneyOverflow, got: %v", err)
	}
}

func TestCalculateDeliveryFeeRejectsWeightFeeOverflow(t *testing.T) {
	tariff := calculatorTestTariff()
	tariff.PerKgRate = Money{
		Amount: maxInt64, Currency: CurrencyLKR,
	}

	weight := Weight{
		ActualKg: 2, VolumetricKg: 2, ChargeableKg: 2,
	}

	_, err := CalculateDeliveryFee(tariff, weight, ServiceLevelStandard)

	if err != ErrMoneyOverflow {
		t.Fatalf("expected ErrMoneyOverflow, got: %v", err)
	}
}
