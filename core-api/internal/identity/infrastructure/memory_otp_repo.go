package infrastructure

import (
	"context"
	"sync"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

const (
	otpTTL         = 10 * time.Minute
	otpCooldown    = 60 * time.Second
	otpMaxAttempts = 3
)

// otpRecord stores a single pending OTP entry.
type otpRecord struct {
	code        string
	attempts    int
	expiresAt   time.Time
	cooldownEnd time.Time
}

// MemoryOTPRepository is a thread-safe in-memory OTP store.
// It satisfies application.OTPRepository and is intended for development and testing.
// In production, replace with a Redis-backed implementation.
type MemoryOTPRepository struct {
	mu      sync.Mutex
	records map[string]*otpRecord // keyed by normalised email
}

// NewMemoryOTPRepository constructs an empty in-memory OTP repository.
func NewMemoryOTPRepository() *MemoryOTPRepository {
	return &MemoryOTPRepository{
		records: make(map[string]*otpRecord),
	}
}

// CheckCooldown returns domain.ErrOTPCooldownActive if a resend is requested
// within 60 seconds of the previous send.
func (r *MemoryOTPRepository) CheckCooldown(_ context.Context, email string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.records[email]
	if !ok {
		return nil // no record — first request
	}
	if time.Now().Before(rec.cooldownEnd) {
		return domain.ErrOTPCooldownActive
	}
	return nil
}

// SaveOTP stores a fresh OTP code for the given email, resetting the cooldown
// and attempt counter. Any previous record for the same email is overwritten.
func (r *MemoryOTPRepository) SaveOTP(_ context.Context, email, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.records[email] = &otpRecord{
		code:        code,
		attempts:    0,
		expiresAt:   now.Add(otpTTL),
		cooldownEnd: now.Add(otpCooldown),
	}
	return nil
}

// VerifyAndConsumeOTP validates the provided code against the stored record,
// enforces expiry and attempt limits, and deletes the record on success.
func (r *MemoryOTPRepository) VerifyAndConsumeOTP(_ context.Context, email, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.records[email]
	if !ok {
		return domain.ErrOTPInvalid
	}

	// Check expiry
	if time.Now().After(rec.expiresAt) {
		delete(r.records, email)
		return domain.ErrOTPExpired
	}

	// Check max attempts
	if rec.attempts >= otpMaxAttempts {
		delete(r.records, email)
		return domain.ErrOTPMaxAttempts
	}

	// Verify code
	if rec.code != code {
		rec.attempts++
		if rec.attempts >= otpMaxAttempts {
			delete(r.records, email)
			return domain.ErrOTPMaxAttempts
		}
		return domain.ErrOTPInvalid
	}

	// Valid — single-use: consume and delete
	delete(r.records, email)
	return nil
}
