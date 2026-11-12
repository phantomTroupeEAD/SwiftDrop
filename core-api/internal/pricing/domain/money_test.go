package domain

import (
	"errors"
	"testing"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name        string
		amount      int64
		currency    string
		expectedErr error
	}{
		{
			name:     "valid LKR amount",
			amount:   125075,
			currency: CurrencyLKR,
		},
		{
			name:        "negative amount",
			amount:      -100,
			currency:    CurrencyLKR,
			expectedErr: ErrInvalidMoney,
		},
		{
			name:        "unsupported currency",
			amount:      10000,
			currency:    "USD",
			expectedErr: ErrInvalidCurrency,
		},
		{
			name:     "zero amount",
			amount:   0,
			currency: CurrencyLKR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMoney(tt.amount, tt.currency)

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("error = %v, want %v", err, tt.expectedErr)
			}

			if err == nil {
				if got.Amount != tt.amount {
					t.Errorf("amount = %d, want %d", got.Amount, tt.amount)
				}

				if got.Currency != tt.currency {
					t.Errorf("currency = %s, want %s", got.Currency, tt.currency)
				}
			}
		})
	}
}

func TestMoneyAdd(t *testing.T) {
	first := Money{Amount: 50000, Currency: CurrencyLKR}
	second := Money{Amount: 25050, Currency: CurrencyLKR}

	got, err := first.Add(second)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Amount != 75050 {
		t.Errorf("amount = %d, want 75050", got.Amount)
	}

	if got.Currency != CurrencyLKR {
		t.Errorf("currency = %s, want LKR", got.Currency)
	}
}

func TestMoneyAddDifferentCurrencies(t *testing.T) {
	first := Money{Amount: 50000, Currency: CurrencyLKR}
	second := Money{Amount: 10000, Currency: "USD"}

	_, err := first.Add(second)

	if !errors.Is(err, ErrInvalidCurrency) {
		t.Errorf("error = %v, want %v", err, ErrInvalidCurrency)
	}
}

func TestMoneyAddOverflow(t *testing.T) {
	first := Money{Amount: maxInt64, Currency: CurrencyLKR}
	second := Money{Amount: 1, Currency: CurrencyLKR}

	_, err := first.Add(second)

	if !errors.Is(err, ErrMoneyOverflow) {
		t.Errorf("error = %v, want %v", err, ErrMoneyOverflow)
	}
}

func TestMoneySubtract(t *testing.T) {
	first := Money{Amount: 100000, Currency: CurrencyLKR}
	second := Money{Amount: 25000, Currency: CurrencyLKR}

	got, err := first.Subtract(second)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Amount != 75000 {
		t.Errorf("amount = %d, want 75000", got.Amount)
	}
}

func TestMoneySubtractNegativeResult(t *testing.T) {
	first := Money{Amount: 50000, Currency: CurrencyLKR}
	second := Money{Amount: 70000, Currency: CurrencyLKR}

	_, err := first.Subtract(second)

	if !errors.Is(err, ErrInvalidMoney) {
		t.Errorf("error = %v, want %v", err, ErrInvalidMoney)
	}
}

func TestMoneyIsZeroAndNegative(t *testing.T) {
	zero := Money{Amount: 0, Currency: CurrencyLKR}
	positive := Money{Amount: 100, Currency: CurrencyLKR}
	negative := Money{Amount: -100, Currency: CurrencyLKR}

	if !zero.IsZero() {
		t.Error("expected zero amount")
	}

	if positive.IsZero() {
		t.Error("expected non-zero amount")
	}

	if !negative.IsNegative() {
		t.Error("expected negative amount")
	}
}
