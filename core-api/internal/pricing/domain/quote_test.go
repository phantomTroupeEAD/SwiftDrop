package domain

import (
	"testing"
	"time"
)

func ptrString(s string) *string {
	return &s
}

func validQuote() Quote {
	createdAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

	return Quote{
		ID:                 "quote-001",
		TariffID:           "tariff-001",
		TariffVersion:      1,
		RequesterID:        ptrString("018f4a7c-1005-7000-8000-000000000005"),
		MerchantID:         nil,
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
	reqID := "  018f4a7c-1005-7000-8000-000000000005  "

	quote, err := NewQuote(
		"quote-001",
		validQuoteTariff(),
		Weight{ActualKg: 2, VolumetricKg: 2.5, ChargeableKg: 2.5},
		ServiceLevelStandard,
		Money{Amount: 51000, Currency: CurrencyLKR},
		createdAt,
		&reqID,
		nil,
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

	if quote.RequesterID == nil || *quote.RequesterID != "018f4a7c-1005-7000-8000-000000000005" {
		t.Errorf("expected trimmed RequesterID, got %v", quote.RequesterID)
	}

	if quote.MerchantID != nil {
		t.Errorf("expected nil MerchantID, got %v", quote.MerchantID)
	}

	if quote.ServiceLevel != ServiceLevelStandard {
		t.Errorf("expected service level %s, got %s", ServiceLevelStandard, quote.ServiceLevel)
	}

	if !quote.ExpiresAt.Equal(createdAt.Add(15 * time.Minute)) {
		t.Errorf("expected quote to expire after 15 minutes")
	}
}

func TestNewQuoteOwnershipModes(t *testing.T) {
	createdAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	weight := Weight{ActualKg: 2, VolumetricKg: 2.5, ChargeableKg: 2.5}
	fee := Money{Amount: 51000, Currency: CurrencyLKR}

	t.Run("customer-only quote", func(t *testing.T) {
		quote, err := NewQuote(
			"quote-cust-001",
			validQuoteTariff(),
			weight,
			ServiceLevelStandard,
			fee,
			createdAt,
			ptrString("usr-123"),
			nil,
		)
		if err != nil {
			t.Fatalf("expected customer-only quote to succeed, got: %v", err)
		}
		if quote.RequesterID == nil || *quote.RequesterID != "usr-123" || quote.MerchantID != nil {
			t.Fatalf("unexpected ownership: requester=%v merchant=%v", quote.RequesterID, quote.MerchantID)
		}
	})

	t.Run("merchant-only quote with whitespace trimming", func(t *testing.T) {
		quote, err := NewQuote(
			"quote-merch-001",
			validQuoteTariff(),
			weight,
			ServiceLevelExpress,
			fee,
			createdAt,
			nil,
			ptrString("  mrc-456  "),
		)
		if err != nil {
			t.Fatalf("expected merchant-only quote to succeed, got: %v", err)
		}
		if quote.RequesterID != nil || quote.MerchantID == nil || *quote.MerchantID != "mrc-456" {
			t.Fatalf("unexpected ownership: requester=%v merchant=%v", quote.RequesterID, quote.MerchantID)
		}
	})

	t.Run("customer-plus-merchant quote", func(t *testing.T) {
		quote, err := NewQuote(
			"quote-both-001",
			validQuoteTariff(),
			weight,
			ServiceLevelSameDay,
			fee,
			createdAt,
			ptrString(" usr-123 "),
			ptrString(" mrc-456 "),
		)
		if err != nil {
			t.Fatalf("expected customer+merchant quote to succeed, got: %v", err)
		}
		if quote.RequesterID == nil || *quote.RequesterID != "usr-123" {
			t.Fatalf("expected trimmed RequesterID, got %v", quote.RequesterID)
		}
		if quote.MerchantID == nil || *quote.MerchantID != "mrc-456" {
			t.Fatalf("expected trimmed MerchantID, got %v", quote.MerchantID)
		}
	})

	t.Run("rejects both nil ownership", func(t *testing.T) {
		_, err := NewQuote(
			"quote-none-001",
			validQuoteTariff(),
			weight,
			ServiceLevelStandard,
			fee,
			createdAt,
			nil,
			nil,
		)
		if err != ErrInvalidQuote {
			t.Fatalf("expected ErrInvalidQuote when both owners nil, got: %v", err)
		}
	})

	t.Run("rejects non-nil empty or whitespace requester ID", func(t *testing.T) {
		for _, bad := range []string{"", "   ", "\t\n"} {
			_, err := NewQuote(
				"quote-bad-req",
				validQuoteTariff(),
				weight,
				ServiceLevelStandard,
				fee,
				createdAt,
				ptrString(bad),
				ptrString("mrc-456"),
			)
			if err != ErrInvalidQuote {
				t.Fatalf("expected ErrInvalidQuote for requester %q, got: %v", bad, err)
			}
		}
	})

	t.Run("rejects non-nil empty or whitespace merchant ID", func(t *testing.T) {
		for _, bad := range []string{"", "   ", "\t\n"} {
			_, err := NewQuote(
				"quote-bad-merch",
				validQuoteTariff(),
				weight,
				ServiceLevelStandard,
				fee,
				createdAt,
				ptrString("usr-123"),
				ptrString(bad),
			)
			if err != ErrInvalidQuote {
				t.Fatalf("expected ErrInvalidQuote for merchant %q, got: %v", bad, err)
			}
		}
	})
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
		ptrString("usr-001"),
		nil,
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
		ptrString("usr-001"),
		nil,
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
		ptrString("usr-001"),
		nil,
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

func TestQuoteValidateOwnershipRules(t *testing.T) {
	t.Run("rejects both nil owners", func(t *testing.T) {
		q := validQuote()
		q.RequesterID = nil
		q.MerchantID = nil
		if err := q.Validate(); err != ErrInvalidQuote {
			t.Fatalf("expected ErrInvalidQuote, got: %v", err)
		}
	})

	t.Run("rejects whitespace-only RequesterID even with valid MerchantID", func(t *testing.T) {
		q := validQuote()
		q.RequesterID = ptrString("   ")
		q.MerchantID = ptrString("mrc-001")
		if err := q.Validate(); err != ErrInvalidQuote {
			t.Fatalf("expected ErrInvalidQuote, got: %v", err)
		}
	})

	t.Run("rejects whitespace-only MerchantID even with valid RequesterID", func(t *testing.T) {
		q := validQuote()
		q.RequesterID = ptrString("usr-001")
		q.MerchantID = ptrString("")
		if err := q.Validate(); err != ErrInvalidQuote {
			t.Fatalf("expected ErrInvalidQuote, got: %v", err)
		}
	})

	t.Run("accepts merchant-only quote", func(t *testing.T) {
		q := validQuote()
		q.RequesterID = nil
		q.MerchantID = ptrString("mrc-001")
		if err := q.Validate(); err != nil {
			t.Fatalf("expected valid merchant-only quote, got: %v", err)
		}
	})
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
