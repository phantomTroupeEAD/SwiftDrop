package delivery

import (
	"math/rand"
	"sync"
	"time"
)

var DefaultRetrySchedule = []time.Duration{
	0 * time.Second,
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

// BackoffCalculator computes jittered retry delays according to a schedule.
type BackoffCalculator struct {
	schedule []time.Duration
	mu       sync.Mutex
	rng      *rand.Rand
}

// NewBackoffCalculator creates a new calculator with the given schedule (or DefaultRetrySchedule).
func NewBackoffCalculator(schedule []time.Duration, rng *rand.Rand) *BackoffCalculator {
	if len(schedule) == 0 {
		schedule = DefaultRetrySchedule
	}
	if rng == nil {
		//nolint:gosec // exponential backoff jitter does not require cryptographic randomness
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return &BackoffCalculator{
		schedule: schedule,
		rng:      rng,
	}
}

// CalculateDelay returns the delay for a given attempt index (0-indexed).
// If delay base > 0, applies ±20% jitter (range [0.8, 1.2]).
func (b *BackoffCalculator) CalculateDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt >= len(b.schedule) {
		attempt = len(b.schedule) - 1
	}

	base := b.schedule[attempt]
	if base <= 0 {
		return 0
	}

	b.mu.Lock()
	factor := 0.8 + (b.rng.Float64() * 0.4) // ±20% jitter
	b.mu.Unlock()

	return time.Duration(float64(base) * factor)
}

// MaxAttempts returns the total number of attempts in the schedule.
func (b *BackoffCalculator) MaxAttempts() int {
	return len(b.schedule)
}
