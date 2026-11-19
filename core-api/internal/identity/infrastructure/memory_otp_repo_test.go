package infrastructure

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

func TestMemoryOTPRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("save, cooldown, and single-use consume", func(t *testing.T) {
		repo := NewMemoryOTPRepository()
		email := "user@swiftdrop.local"

		if err := repo.CheckCooldown(ctx, email); err != nil {
			t.Fatalf("expected no cooldown initially, got %v", err)
		}

		if err := repo.SaveOTP(ctx, email, "123456"); err != nil {
			t.Fatalf("SaveOTP failed: %v", err)
		}

		if err := repo.CheckCooldown(ctx, email); err != domain.ErrOTPCooldownActive {
			t.Errorf("expected ErrOTPCooldownActive, got %v", err)
		}

		if err := repo.VerifyAndConsumeOTP(ctx, email, "123456"); err != nil {
			t.Fatalf("expected valid OTP to verify, got %v", err)
		}

		// Second use must fail (single-use)
		if err := repo.VerifyAndConsumeOTP(ctx, email, "123456"); err != domain.ErrOTPInvalid {
			t.Errorf("expected ErrOTPInvalid after consumption, got %v", err)
		}
	})

	t.Run("unknown email returns ErrOTPInvalid", func(t *testing.T) {
		repo := NewMemoryOTPRepository()
		if err := repo.VerifyAndConsumeOTP(ctx, "nobody@swiftdrop.local", "000000"); err != domain.ErrOTPInvalid {
			t.Errorf("expected ErrOTPInvalid, got %v", err)
		}
	})

	t.Run("enforces max attempts and deletes record", func(t *testing.T) {
		repo := NewMemoryOTPRepository()
		email := "attempts@swiftdrop.local"
		_ = repo.SaveOTP(ctx, email, "654321")

		if err := repo.VerifyAndConsumeOTP(ctx, email, "111111"); err != domain.ErrOTPInvalid {
			t.Errorf("attempt 1: expected ErrOTPInvalid, got %v", err)
		}
		if err := repo.VerifyAndConsumeOTP(ctx, email, "222222"); err != domain.ErrOTPInvalid {
			t.Errorf("attempt 2: expected ErrOTPInvalid, got %v", err)
		}
		if err := repo.VerifyAndConsumeOTP(ctx, email, "333333"); err != domain.ErrOTPMaxAttempts {
			t.Errorf("attempt 3: expected ErrOTPMaxAttempts, got %v", err)
		}
		// Record deleted after max attempts
		if err := repo.VerifyAndConsumeOTP(ctx, email, "654321"); err != domain.ErrOTPInvalid {
			t.Errorf("after lockout: expected ErrOTPInvalid, got %v", err)
		}
	})

	t.Run("expired OTP returns ErrOTPExpired", func(t *testing.T) {
		repo := NewMemoryOTPRepository()
		email := "expired@swiftdrop.local"
		repo.records[email] = &otpRecord{
			code:        "999999",
			expiresAt:   time.Now().Add(-1 * time.Second),
			cooldownEnd: time.Now().Add(-1 * time.Second),
		}

		if err := repo.VerifyAndConsumeOTP(ctx, email, "999999"); err != domain.ErrOTPExpired {
			t.Errorf("expected ErrOTPExpired, got %v", err)
		}
		if err := repo.CheckCooldown(ctx, email); err != nil {
			t.Errorf("expected expired cooldown to pass, got %v", err)
		}
	})

	t.Run("concurrent access is race-free", func(t *testing.T) {
		repo := NewMemoryOTPRepository()
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				email := "concurrent@swiftdrop.local"
				_ = repo.SaveOTP(ctx, email, "123456")
				_ = repo.CheckCooldown(ctx, email)
				_ = repo.VerifyAndConsumeOTP(ctx, email, "123456")
			}(i)
		}
		wg.Wait()
	})
}
