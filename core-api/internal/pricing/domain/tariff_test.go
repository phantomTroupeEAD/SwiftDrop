package domain

import (
	"testing"
	"time"
)

func validTariff() Tariff {
	return Tariff{
		ID:                "tariff-001",
		OriginZoneID:      "COLOMBO",
		DestinationZoneID: "GAMPAHA",
		BaseFee:           Money{Amount: 35000, Currency: CurrencyLKR},
		PerKgRate:         Money{Amount: 8000, Currency: CurrencyLKR},
		EffectiveFrom:     time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		IsActive:          true,
		Version:           1,
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
	tariff.OriginZoneID = ""

	if err := tariff.Validate(); err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff for empty origin, got: %v", err)
	}

	tariff = validTariff()
	tariff.DestinationZoneID = ""

	if err := tariff.Validate(); err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff for empty destination, got: %v", err)
	}
}

func TestServiceLevelValidate(t *testing.T) {
	if err := ServiceLevelStandard.Validate(); err != nil {
		t.Errorf("expected ServiceLevelStandard to be valid, got: %v", err)
	}
	if err := ServiceLevelExpress.Validate(); err != nil {
		t.Errorf("expected ServiceLevelExpress to be valid, got: %v", err)
	}
	if err := ServiceLevelSameDay.Validate(); err != nil {
		t.Errorf("expected ServiceLevelSameDay to be valid, got: %v", err)
	}
	invalid := ServiceLevel("OVERNIGHT")
	if err := invalid.Validate(); err != ErrInvalidServiceLevel {
		t.Errorf("expected ErrInvalidServiceLevel, got: %v", err)
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

func TestTariffValidateTimestampOrdering(t *testing.T) {
	tariff := validTariff()
	now := time.Now().UTC()
	tariff.CreatedAt = now
	tariff.UpdatedAt = now.Add(-time.Hour)

	if err := tariff.Validate(); err != ErrInvalidTariff {
		t.Fatalf("expected ErrInvalidTariff when UpdatedAt is before CreatedAt, got: %v", err)
	}

	tariff.UpdatedAt = now.Add(time.Hour)
	if err := tariff.Validate(); err != nil {
		t.Fatalf("expected valid tariff with forward UpdatedAt, got: %v", err)
	}
}

func TestServiceLevelMultiplier(t *testing.T) {
	tests := []struct {
		serviceLevel ServiceLevel
		expected     float64
		expectedNum  int64
		expectedDen  int64
	}{
		{ServiceLevelStandard, 1.0, 1, 1},
		{ServiceLevelExpress, 1.5, 3, 2},
		{ServiceLevelSameDay, 2.0, 2, 1},
	}

	for _, test := range tests {
		t.Run(string(test.serviceLevel), func(t *testing.T) {
			actual, err := test.serviceLevel.Multiplier()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if actual != test.expected {
				t.Errorf("expected multiplier %v, got %v", test.expected, actual)
			}

			num, den, err := test.serviceLevel.MultiplierRatio()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if num != test.expectedNum || den != test.expectedDen {
				t.Errorf("expected ratio %d/%d, got %d/%d", test.expectedNum, test.expectedDen, num, den)
			}
		})
	}

	invalid := ServiceLevel("INVALID")
	if _, err := invalid.Multiplier(); err != ErrInvalidServiceLevel {
		t.Errorf("expected ErrInvalidServiceLevel, got: %v", err)
	}
	if _, _, err := invalid.MultiplierRatio(); err != ErrInvalidServiceLevel {
		t.Errorf("expected ErrInvalidServiceLevel, got: %v", err)
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
		isActive bool
		at       time.Time
		expected bool
	}{
		{"before start", true, start.Add(-time.Second), false},
		{"exactly at start", true, start, true},
		{"during period", true, start.AddDate(0, 0, 10), true},
		{"exactly at end", true, end, false},
		{"after end", true, end.Add(time.Second), false},
		{"inactive during period", false, start.AddDate(0, 0, 10), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tariff.IsActive = test.isActive
			actual := tariff.IsEffectiveAt(test.at)

			if actual != test.expected {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}
