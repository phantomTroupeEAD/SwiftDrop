package application

import "time"

// SystemClock implements Clock using current UTC timestamps.
type SystemClock struct{}

// NewSystemClock constructs a production SystemClock.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now returns the current time in UTC.
func (c *SystemClock) Now() time.Time {
	return time.Now().UTC()
}
