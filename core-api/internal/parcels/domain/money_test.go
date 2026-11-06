package domain

import (
	"errors"
	"testing"
)

func TestMoney_Validate(t *testing.T) {
	m, err := NewMoney(47000, CurrencyLKR)
	if err != nil {
		t.Fatalf("unexpected error for valid money: %v", err)
	}
	if m.Amount != 47000 || m.Currency != CurrencyLKR {
		t.Errorf("unexpected money values: %+v", m)
	}

	// Negative amount
	_, err = NewMoney(-100, CurrencyLKR)
	if !errors.Is(err, ErrInvalidDeliveryFee) {
		t.Fatalf("expected ErrInvalidDeliveryFee for negative amount, got: %v", err)
	}

	// Unsupported currency
	_, err = NewMoney(5000, "USD")
	if !errors.Is(err, ErrInvalidDeliveryFee) {
		t.Fatalf("expected ErrInvalidDeliveryFee for unsupported currency, got: %v", err)
	}
}

