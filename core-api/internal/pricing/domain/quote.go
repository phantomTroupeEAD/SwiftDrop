package domain

import (
	"math"
	"time"
)

const QuoteValidity = 15 * time.Minute

type Quote struct {
	ID                 string
	TariffID           string
	TariffVersion      int
	ChargeableWeightKg float64
	ServiceLevel       ServiceLevel
	Fee                Money
	CreatedAt          time.Time
	ExpiresAt          time.Time
	UsedAt             *time.Time
}

// NewQuote creates a quote using the selected tariff and calculated weight.
func NewQuote(
	id string,
	tariff Tariff,
	weight Weight,
	fee Money,
	createdAt time.Time,
) (Quote, error) {
	if err := tariff.Validate(); err != nil {
		return Quote{}, ErrInvalidQuote
	}

	if id == "" ||
		math.IsNaN(weight.ChargeableKg) ||
		math.IsInf(weight.ChargeableKg, 0) ||
		weight.ChargeableKg <= 0 ||
		weight.ChargeableKg > MaxChargeableWeightKg ||
		createdAt.IsZero() ||
		!tariff.IsEffectiveAt(createdAt) {
		return Quote{}, ErrInvalidQuote
	}

	if err := fee.Validate(); err != nil {
		return Quote{}, ErrInvalidQuote
	}

	quote := Quote{
		ID:                 id,
		TariffID:           tariff.ID,
		TariffVersion:      tariff.Version,
		ChargeableWeightKg: weight.ChargeableKg,
		ServiceLevel:       tariff.ServiceLevel,
		Fee:                fee,
		CreatedAt:          createdAt,
		ExpiresAt:          createdAt.Add(QuoteValidity),
	}

	if err := quote.Validate(); err != nil {
		return Quote{}, err
	}

	return quote, nil
}

// Validate checks the quote's required data and expiry configuration.
func (q Quote) Validate() error {
	if q.ID == "" || q.TariffID == "" || q.TariffVersion <= 0 {
		return ErrInvalidQuote
	}

	if math.IsNaN(q.ChargeableWeightKg) ||
		math.IsInf(q.ChargeableWeightKg, 0) ||
		q.ChargeableWeightKg <= 0 ||
		q.ChargeableWeightKg > MaxChargeableWeightKg {
		return ErrInvalidQuote
	}

	switch q.ServiceLevel {
	case ServiceLevelStandard, ServiceLevelExpress, ServiceLevelSameDay:
	default:
		return ErrInvalidQuote
	}

	if err := q.Fee.Validate(); err != nil {
		return ErrInvalidQuote
	}

	if q.CreatedAt.IsZero() ||
		q.ExpiresAt.IsZero() ||
		!q.ExpiresAt.Equal(q.CreatedAt.Add(QuoteValidity)) {
		return ErrInvalidQuote
	}

	if q.UsedAt != nil && q.UsedAt.Before(q.CreatedAt) {
		return ErrInvalidQuote
	}

	return nil
}

// IsExpired reports whether the quote has reached its expiry time.
func (q Quote) IsExpired(at time.Time) bool {
	return !at.Before(q.ExpiresAt)
}

// IsUsableAt reports whether the quote can be accepted at the given time.
func (q Quote) IsUsableAt(at time.Time) bool {
	if q.Validate() != nil {
		return false
	}

	return q.UsedAt == nil && !q.IsExpired(at)
}

// MarkUsed consumes the quote so it cannot be accepted again.
func (q *Quote) MarkUsed(at time.Time) error {
	if q == nil || q.Validate() != nil {
		return ErrInvalidQuote
	}

	if q.UsedAt != nil {
		return ErrQuoteAlreadyUsed
	}

	if q.IsExpired(at) {
		return ErrQuoteExpired
	}

	if at.Before(q.CreatedAt) {
		return ErrInvalidQuote
	}

	q.UsedAt = &at
	return nil
}
