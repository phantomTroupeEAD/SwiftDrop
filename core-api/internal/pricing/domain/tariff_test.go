package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validTariff() Tariff {
	return Tariff{
		ID:                "tariff-001",
		OriginZoneID:      "ZONE_COLOMBO_01",
		DestinationZoneID: "ZONE_GAMPAHA_01",
		BaseFee:           Money{Amount: 35000, Currency: CurrencyLKR},
		PerKgRate:         Money{Amount: 8000, Currency: CurrencyLKR},
		EffectiveFrom:     time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		IsActive:          true,
		Version:           1,
	}
}

func TestValidateZoneID(t *testing.T) {
	tests := []struct {
		name    string
		zoneID  string
		wantErr bool
	}{
		{name: "valid standard zone", zoneID: "ZONE_COLOMBO_01", wantErr: false},
		{name: "valid min length 7 chars", zoneID: "ZONE_AB", wantErr: false},
		{name: "valid max length 32 chars", zoneID: "ZONE_" + strings.Repeat("A", 27), wantErr: false},
		{name: "valid digits and underscores", zoneID: "ZONE_WEST_01_A", wantErr: false},
		{name: "empty string", zoneID: "", wantErr: true},
		{name: "whitespace only", zoneID: "   ", wantErr: true},
		{name: "surrounding whitespace", zoneID: " ZONE_COLOMBO_01 ", wantErr: true},
		{name: "missing ZONE prefix", zoneID: "COLOMBO", wantErr: true},
		{name: "hyphenated legacy format", zoneID: "ZONE-WESTERN-01", wantErr: true},
		{name: "lowercase characters", zoneID: "zone_colombo_01", wantErr: true},
		{name: "too short suffix", zoneID: "ZONE_A", wantErr: true},
		{name: "too long 33 chars", zoneID: "ZONE_" + strings.Repeat("A", 28), wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateZoneID(tc.zoneID)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidZoneID) {
					t.Fatalf("expected ErrInvalidZoneID for %q, got: %v", tc.zoneID, err)
				}
			} else if err != nil {
				t.Fatalf("expected valid zone ID %q, got error: %v", tc.zoneID, err)
			}
		})
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

func TestTariffValidateRejectsInvalidZoneFormat(t *testing.T) {
	invalidZones := []string{
		"COLOMBO",
		"ZONE-WESTERN-01",
		"zone_colombo_01",
		"ZONE_A",
		" ZONE_COLOMBO_01 ",
	}

	for _, z := range invalidZones {
		tariff := validTariff()
		tariff.OriginZoneID = z
		if err := tariff.Validate(); err != ErrInvalidTariff {
			t.Fatalf("expected ErrInvalidTariff for invalid origin zone %q, got: %v", z, err)
		}

		tariff = validTariff()
		tariff.DestinationZoneID = z
		if err := tariff.Validate(); err != ErrInvalidTariff {
			t.Fatalf("expected ErrInvalidTariff for invalid destination zone %q, got: %v", z, err)
		}
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
