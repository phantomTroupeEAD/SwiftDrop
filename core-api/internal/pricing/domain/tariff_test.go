package domain

import (
	"testing"
	"time"
)

func validTariff() Tariff {
	return Tariff{
		ID:              "tariff-001",
		PickupZone:      "COLOMBO",
		DestinationZone: "GAMPAHA",
		BaseFee:         Money{Amount: 35000, Currency: CurrencyLKR},
		PerKgRate:       Money{Amount: 8000, Currency: CurrencyLKR},
		ServiceLevel:    ServiceLevelStandard,
		EffectiveFrom:   time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Version:         1,
	}
}

func TestTariffValidate(t *testing.T) {
	tariff := validTariff()

	if err := tariff.Validate(); err != nil {
		t.Fatalf("expected valid tariff, got error: %v", err)
	}
}

func TestTariffValidateRejectsMissingZone(t *testing.T) {
	tariff := validTariff()
	tariff.PickupZone = ""

	if err := tariff.Validate(); err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff, got: %v", err)
	}
}

func TestTariffValidateRejectsInvalidServiceLevel(t *testing.T) {
	tariff := validTariff()
	tariff.ServiceLevel = "OVERNIGHT"

	if err := tariff.Validate(); err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff, got: %v", err)
	}
}

func TestTariffValidateRejectsInvalidVersion(t *testing.T) {
	tariff := validTariff()
	tariff.Version = 0

	if err := tariff.Validate(); err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff, got: %v", err)
	}
}

func TestTariffValidateRejectsInvalidEffectivePeriod(t *testing.T) {
	tariff := validTariff()

	end := tariff.EffectiveFrom
	tariff.EffectiveUntil = &end

	if err := tariff.Validate(); err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff, got: %v", err)
	}
}

func TestTariffServiceMultiplier(t *testing.T) {
	tests := []struct {
		serviceLevel ServiceLevel
		expected     float64
	}{
		{ServiceLevelStandard, 1.0},
		{ServiceLevelExpress, 1.5},
		{ServiceLevelSameDay, 2.0},
	}

	for _, test := range tests {
		t.Run(string(test.serviceLevel), func(t *testing.T) {
			tariff := validTariff()
			tariff.ServiceLevel = test.serviceLevel

			actual, err := tariff.ServiceMultiplier()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if actual != test.expected {
				t.Errorf("expected multiplier %v, got %v",
					test.expected, actual)
			}
		})
	}
}

func TestTariffIsEffectiveAt(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	tariff := validTariff()
	tariff.EffectiveFrom = start
	tariff.EffectiveUntil = &end

	tests := []struct {
		name     string
		at       time.Time
		expected bool
	}{
		{"before start", start.Add(-time.Second), false},
		{"exactly at start", start, true},
		{"during period", start.AddDate(0, 0, 10), true},
		{"exactly at end", end, false},
		{"after end", end.Add(time.Second), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := tariff.IsEffectiveAt(test.at)

			if actual != test.expected {
				t.Errorf("expected %v, got %v",
					test.expected, actual)
			}
		})
	}
}
