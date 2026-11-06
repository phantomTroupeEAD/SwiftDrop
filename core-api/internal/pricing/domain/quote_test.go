package domain

import (
	"testing"
	"time"
)

func validQuote() Quote {
	createdAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

	return Quote{
		ID:                 "quote-001",
		TariffID:           "tariff-001",
		TariffVersion:      1,
		ChargeableWeightKg: 2.5,
		ServiceLevel:       ServiceLevelStandard,
		Fee:                Money{Amount: 51000, Currency: CurrencyLKR},
		CreatedAt:          createdAt,
		ExpiresAt:          createdAt.Add(QuoteValidity),
	}
}

func validQuoteTariff() Tariff {
	tariff := validTariff()
	tariff.EffectiveFrom = time.Date(
		2026, 11, 1, 0, 0, 0, 0, time.UTC,
	)
	return tariff
}

func TestNewQuote(t *testing.T) {
	createdAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

	quote, err := NewQuote(
		"quote-001",
		validQuoteTariff(),
		Weight{ActualKg: 2, VolumetricKg: 2.5, ChargeableKg: 2.5},
		ServiceLevelStandard,
		Money{Amount: 51000, Currency: CurrencyLKR},
		createdAt,
	)

	if err != nil {
		t.Fatalf("expected quote creation to succeed, got: %v", err)
	}

	if quote.ID != "quote-001" {
		t.Errorf("expected quote ID quote-001, got %s", quote.ID)
	}

	if quote.TariffID != "tariff-001" {
		t.Errorf("expected tariff ID tariff-001, got %s", quote.TariffID)
	}

	if quote.TariffVersion != 1 {
		t.Errorf("expected tariff version 1, got %d", quote.TariffVersion)
	}

	if quote.ServiceLevel != ServiceLevelStandard {
		t.Errorf("expected service level %s, got %s", ServiceLevelStandard, quote.ServiceLevel)
	}

	if !quote.ExpiresAt.Equal(createdAt.Add(15 * time.Minute)) {
		t.Errorf("expected quote to expire after 15 minutes")
	}
}

func TestNewQuoteRejectsInactiveTariff(t *testing.T) {
	createdAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	tariff := validQuoteTariff()
	tariff.EffectiveFrom = createdAt.Add(time.Hour)

	_, err := NewQuote(
		"quote-001",
		tariff,
		Weight{ChargeableKg: 2},
		ServiceLevelStandard,
		Money{Amount: 51000, Currency: CurrencyLKR},
		createdAt,
	)

	if err != ErrInvalidQuote {
		t.Fatalf("expected ErrInvalidQuote, got: %v", err)
	}
}

func TestNewQuoteRejectsExcessiveWeight(t *testing.T) {
	createdAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

	_, err := NewQuote(
		"quote-001",
		validQuoteTariff(),
		Weight{ChargeableKg: 31},
		ServiceLevelStandard,
		Money{Amount: 51000, Currency: CurrencyLKR},
		createdAt,
	)

	if err != ErrInvalidQuote {
		t.Fatalf("expected ErrInvalidQuote, got: %v", err)
	}
}

func TestNewQuoteRejectsInvalidServiceLevel(t *testing.T) {
	createdAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

	_, err := NewQuote(
		"quote-001",
		validQuoteTariff(),
		Weight{ChargeableKg: 2},
		ServiceLevel("INVALID"),
		Money{Amount: 51000, Currency: CurrencyLKR},
		createdAt,
	)

	if err != ErrInvalidQuote {
		t.Fatalf("expected ErrInvalidQuote, got: %v", err)
	}
}

func TestQuoteValidate(t *testing.T) {
	quote := validQuote()

	if err := quote.Validate(); err != nil {
		t.Fatalf("expected valid quote, got: %v", err)
	}
}

func TestQuoteValidateRejectsInvalidExpiry(t *testing.T) {
	quote := validQuote()
	quote.ExpiresAt = quote.CreatedAt.Add(20 * time.Minute)

	if err := quote.Validate(); err != ErrInvalidQuote {
		t.Fatalf("expected ErrInvalidQuote, got: %v", err)
	}
}

func TestQuoteIsExpired(t *testing.T) {
	quote := validQuote()

	tests := []struct {
		name     string
		at       time.Time
		expected bool
	}{
		{"before expiry", quote.ExpiresAt.Add(-time.Second), false},
		{"exactly at expiry", quote.ExpiresAt, true},
		{"after expiry", quote.ExpiresAt.Add(time.Second), true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := quote.IsExpired(test.at)

			if actual != test.expected {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestQuoteIsUsableAt(t *testing.T) {
	quote := validQuote()

	if !quote.IsUsableAt(quote.CreatedAt.Add(5 * time.Minute)) {
		t.Error("expected quote to be usable before expiry")
	}

	if quote.IsUsableAt(quote.ExpiresAt) {
		t.Error("expected quote to be unusable at expiry")
	}
}

func TestQuoteMarkUsed(t *testing.T) {
	quote := validQuote()
	usedAt := quote.CreatedAt.Add(5 * time.Minute)

	if err := quote.MarkUsed(usedAt); err != nil {
		t.Fatalf("expected quote to be marked as used, got: %v", err)
	}

	if quote.UsedAt == nil || !quote.UsedAt.Equal(usedAt) {
		t.Error("expected UsedAt to be set")
	}

	if quote.IsUsableAt(usedAt) {
		t.Error("expected used quote to be unusable")
	}
}

func TestQuoteMarkUsedRejectsReuse(t *testing.T) {
	quote := validQuote()
	usedAt := quote.CreatedAt.Add(5 * time.Minute)

	if err := quote.MarkUsed(usedAt); err != nil {
		t.Fatalf("unexpected first-use error: %v", err)
	}

	err := quote.MarkUsed(usedAt.Add(time.Minute))

	if err != ErrQuoteAlreadyUsed {
		t.Fatalf("expected ErrQuoteAlreadyUsed, got: %v", err)
	}
}

func TestQuoteMarkUsedRejectsExpiredQuote(t *testing.T) {
	quote := validQuote()

	err := quote.MarkUsed(quote.ExpiresAt)

	if err != ErrQuoteExpired {
		t.Fatalf("expected ErrQuoteExpired, got: %v", err)
	}
}
