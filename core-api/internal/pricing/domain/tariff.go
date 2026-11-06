package domain

import (
	"strings"
	"time"
)

type ServiceLevel string

const (
	ServiceLevelStandard ServiceLevel = "STANDARD"
	ServiceLevelExpress  ServiceLevel = "EXPRESS"
	ServiceLevelSameDay  ServiceLevel = "SAME_DAY"
)

// Validate checks whether the service level is a supported operational tier.
func (s ServiceLevel) Validate() error {
	switch s {
	case ServiceLevelStandard, ServiceLevelExpress, ServiceLevelSameDay:
		return nil
	default:
		return ErrInvalidServiceLevel
	}
}

// MultiplierRatio returns the exact integer numerator and denominator for the service level.
func (s ServiceLevel) MultiplierRatio() (int64, int64, error) {
	switch s {
	case ServiceLevelStandard:
		return 1, 1, nil
	case ServiceLevelExpress:
		return 3, 2, nil
	case ServiceLevelSameDay:
		return 2, 1, nil
	default:
		return 0, 0, ErrInvalidServiceLevel
	}
}

// Multiplier returns the decimal representation of the multiplier for display purposes.
func (s ServiceLevel) Multiplier() (float64, error) {
	numerator, denominator, err := s.MultiplierRatio()
	if err != nil {
		return 0, err
	}

	return float64(numerator) / float64(denominator), nil
}

type Tariff struct {
	ID                string
	OriginZoneID      string
	DestinationZoneID string
	BaseFee           Money
	PerKgRate         Money
	EffectiveFrom     time.Time
	EffectiveUntil    *time.Time
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Version           int
}

// Validate checks whether the tariff contains valid pricing configuration.
func (t Tariff) Validate() error {
	if strings.TrimSpace(t.ID) == "" ||
		strings.TrimSpace(t.OriginZoneID) == "" ||
		strings.TrimSpace(t.DestinationZoneID) == "" {
		return ErrInvalidTariff
	}

	if err := t.BaseFee.Validate(); err != nil {
		return ErrInvalidTariff
	}

	if err := t.PerKgRate.Validate(); err != nil {
		return ErrInvalidTariff
	}

	if t.EffectiveFrom.IsZero() {
		return ErrInvalidTariff
	}

	if t.EffectiveUntil != nil &&
		!t.EffectiveUntil.After(t.EffectiveFrom) {
		return ErrInvalidTariff
	}

	if t.Version <= 0 {
		return ErrInvalidTariff
	}

	if !t.CreatedAt.IsZero() && !t.UpdatedAt.IsZero() && t.UpdatedAt.Before(t.CreatedAt) {
		return ErrInvalidTariff
	}

	return nil
}

// IsEffectiveAt checks whether the tariff is active at the given time.
// The start is inclusive, the end is exclusive, and IsActive must be true.
func (t Tariff) IsEffectiveAt(at time.Time) bool {
	if !t.IsActive {
		return false
	}

	if at.Before(t.EffectiveFrom) {
		return false
	}

	if t.EffectiveUntil != nil &&
		!at.Before(*t.EffectiveUntil) {
		return false
	}

	return true
}
