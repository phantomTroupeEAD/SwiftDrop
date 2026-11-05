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

type Tariff struct {
	ID              string
	PickupZone      string
	DestinationZone string
	BaseFee         Money
	PerKgRate       Money
	ServiceLevel    ServiceLevel
	EffectiveFrom   time.Time
	EffectiveUntil  *time.Time
	Version         int
}

// Validate checks whether the tariff contains valid pricing configuration.
func (t Tariff) Validate() error {
	if strings.TrimSpace(t.ID) == "" ||
		strings.TrimSpace(t.PickupZone) == "" ||
		strings.TrimSpace(t.DestinationZone) == "" {
		return ErrInvalidTariff
	}

	if err := t.BaseFee.Validate(); err != nil {
		return ErrInvalidTariff
	}

	if err := t.PerKgRate.Validate(); err != nil {
		return ErrInvalidTariff
	}

	if !t.isSupportedServiceLevel() {
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

	return nil
}

func (t Tariff) isSupportedServiceLevel() bool {
	switch t.ServiceLevel {
	case ServiceLevelStandard, ServiceLevelExpress, ServiceLevelSameDay:
		return true
	default:
		return false
	}
}

// ServiceMultiplier returns the multiplier defined for the service level.
func (t Tariff) ServiceMultiplier() (float64, error) {
	switch t.ServiceLevel {
	case ServiceLevelStandard:
		return 1.0, nil
	case ServiceLevelExpress:
		return 1.5, nil
	case ServiceLevelSameDay:
		return 2.0, nil
	default:
		return 0, ErrInvalidTariff
	}
}

// IsEffectiveAt checks whether the tariff is active at the given time.
// The start is inclusive and the end is exclusive.
func (t Tariff) IsEffectiveAt(at time.Time) bool {
	if at.Before(t.EffectiveFrom) {
		return false
	}

	if t.EffectiveUntil != nil &&
		!at.Before(*t.EffectiveUntil) {
		return false
	}

	return true
}
