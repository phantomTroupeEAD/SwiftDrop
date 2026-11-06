package domain

import (
	"math"
	"testing"
)

func calculatorTestTariff() Tariff {
	return Tariff{
		ID:              "tariff-001",
		PickupZone:      "COLOMBO",
		DestinationZone: "GAMPAHA",
		BaseFee:         Money{Amount: 35000, Currency: CurrencyLKR},
		PerKgRate:       Money{Amount: 8000, Currency: CurrencyLKR},
		ServiceLevel:    ServiceLevelStandard,
		EffectiveFrom:   validTariff().EffectiveFrom,
		Version:         1,
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tariff := calculatorTestTariff()
			tariff.ServiceLevel = test.serviceLevel

			actual, err := CalculateDeliveryFee(tariff, test.weight)
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
	tariff.ServiceLevel = ServiceLevelExpress

	weight := Weight{
		ActualKg:     1,
		VolumetricKg: 1,
		ChargeableKg: 1,
	}

	actual, err := CalculateDeliveryFee(tariff, weight)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 cent × 1.5 = 1.5 cents, rounded half-up to 2 cents.
	if actual.Amount != 2 {
		t.Errorf("expected 2 cents, got %d cents", actual.Amount)
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
	tariff.ServiceLevel = "OVERNIGHT"

	weight := Weight{
		ActualKg: 1, VolumetricKg: 1, ChargeableKg: 1,
	}

	_, err := CalculateDeliveryFee(tariff, weight)

	if err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff, got: %v", err)
	}
}

func TestCalculateDeliveryFeeRejectsOverflow(t *testing.T) {
	tariff := calculatorTestTariff()
	tariff.BaseFee = Money{
		Amount: maxInt64, Currency: CurrencyLKR,
	}
	tariff.ServiceLevel = ServiceLevelSameDay

	weight := Weight{
		ActualKg: 1, VolumetricKg: 1, ChargeableKg: 1,
	}

	_, err := CalculateDeliveryFee(tariff, weight)

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

	_, err := CalculateDeliveryFee(tariff, weight)

	if err != ErrMoneyOverflow {
		t.Fatalf("expected ErrMoneyOverflow, got: %v", err)
	}
}
