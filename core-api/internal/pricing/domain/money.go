package domain

const (
	// CurrencyLKR is the currency supported by SwiftDrop.
	CurrencyLKR = "LKR"

	maxInt64 = int64(1<<63 - 1)
)

// Money represents a monetary amount in integer minor units.
type Money struct {
	Amount   int64
	Currency string
}

// NewMoney creates and validates a monetary value.
func NewMoney(amount int64, currency string) (Money, error) {
	money := Money{
		Amount:   amount,
		Currency: currency,
	}

	if err := money.Validate(); err != nil {
		return Money{}, err
	}

	return money, nil
}

// Validate checks whether the monetary value is valid.
func (m Money) Validate() error {
	if m.Amount < 0 {
		return ErrInvalidMoney
	}

	if m.Currency != CurrencyLKR {
		return ErrInvalidCurrency
	}

	return nil
}

// Add adds another monetary value to this value.
func (m Money) Add(other Money) (Money, error) {
	if err := m.Validate(); err != nil {
		return Money{}, err
	}

	if err := other.Validate(); err != nil {
		return Money{}, err
	}

	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}

	// Prevent int64 overflow.
	if other.Amount > maxInt64-m.Amount {
		return Money{}, ErrMoneyOverflow
	}

	return Money{
		Amount:   m.Amount + other.Amount,
		Currency: m.Currency,
	}, nil
}

// Subtract subtracts another monetary value from this value.
func (m Money) Subtract(other Money) (Money, error) {
	if err := m.Validate(); err != nil {
		return Money{}, err
	}

	if err := other.Validate(); err != nil {
		return Money{}, err
	}

	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}

	// Delivery fees cannot become negative.
	if other.Amount > m.Amount {
		return Money{}, ErrInvalidMoney
	}

	return Money{
		Amount:   m.Amount - other.Amount,
		Currency: m.Currency,
	}, nil
}

// IsNegative reports whether the amount is negative.
func (m Money) IsNegative() bool {
	return m.Amount < 0
}

// IsZero reports whether the amount is zero.
func (m Money) IsZero() bool {
	return m.Amount == 0
}
