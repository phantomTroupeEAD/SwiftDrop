package delivery

import (
	"math/rand"
	"testing"
	"time"
)

func TestBackoffCalculator(t *testing.T) {
	schedule := []time.Duration{
		0 * time.Second,
		30 * time.Second,
		2 * time.Minute,
		10 * time.Minute,
		30 * time.Minute,
	}

	calc := NewBackoffCalculator(schedule, rand.New(rand.NewSource(42)))

	// Attempt 0 should be 0s without jitter
	d0 := calc.CalculateDelay(0)
	if d0 != 0 {
		t.Errorf("expected 0s for attempt 0, got %v", d0)
	}

	// Attempt 1 should be 30s ± 20% (between 24s and 36s)
	d1 := calc.CalculateDelay(1)
	if d1 < 24*time.Second || d1 > 36*time.Second {
		t.Errorf("expected 24s-36s for attempt 1, got %v", d1)
	}

	// Attempt 2 should be 2m ± 20% (between 96s and 144s)
	d2 := calc.CalculateDelay(2)
	if d2 < 96*time.Second || d2 > 144*time.Second {
		t.Errorf("expected 96s-144s for attempt 2, got %v", d2)
	}

	// Attempt beyond max should cap at last element (30m ± 20%)
	d5 := calc.CalculateDelay(10)
	if d5 < 24*time.Minute || d5 > 36*time.Minute {
		t.Errorf("expected 24m-36m for capped attempt, got %v", d5)
	}
}
