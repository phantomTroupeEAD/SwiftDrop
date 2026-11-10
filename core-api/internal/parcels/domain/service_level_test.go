package domain

import (
	"errors"
	"testing"
)

func TestServiceLevel_Validate(t *testing.T) {
	validTiers := []ServiceLevel{
		ServiceLevelStandard,
		ServiceLevelExpress,
		ServiceLevelSameDay,
	}

	for _, tier := range validTiers {
		if err := tier.Validate(); err != nil {
			t.Errorf("expected valid tier %q, got: %v", tier, err)
		}
	}

	invalidTiers := []ServiceLevel{
		"",
		"OVERNIGHT",
		"ECONOMY",
		"standard",
	}

	for _, tier := range invalidTiers {
		if err := tier.Validate(); !errors.Is(err, ErrInvalidServiceLevel) {
			t.Errorf("expected ErrInvalidServiceLevel for %q, got: %v", tier, err)
		}
	}
}
