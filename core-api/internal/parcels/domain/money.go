package domain

import "fmt"

// CurrencyLKR defines the authoritative currency for all SwiftDrop transactions.
const CurrencyLKR = "LKR"

// Money represents a monetary value in exact integer minor units (LKR cents).
type Money struct {
	Amount   int64
	Currency string
}

// NewMoney creates and validates a Money value object.
func NewMoney(amount int64, currency string) (Money, error) {
	m := Money{Amount: amount, Currency: currency}
	if err := m.Validate(); err != nil {
		return Money{}, err
	}
	return m, nil
}

// Validate verifies that the monetary amount is non-negative and uses the approved currency.
func (m Money) Validate() error {
	if m.Currency != CurrencyLKR {
		return fmt.Errorf("%w: unsupported currency %q (must be %s)", ErrInvalidDeliveryFee, m.Currency, CurrencyLKR)
	}
	if m.Amount < 0 {
		return fmt.Errorf("%w: amount cannot be negative (%d)", ErrInvalidDeliveryFee, m.Amount)
	}
	return nil
}

