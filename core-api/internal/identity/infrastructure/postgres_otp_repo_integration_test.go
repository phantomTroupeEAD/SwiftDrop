package infrastructure

import (
	"context"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// TestPostgresOTPRepository_VerifyOTP_Integration checks that VerifyOTP keeps a correct
// code for the final registration step and shares the 3-attempt limit with
// VerifyAndConsumeOTP. Skips unless DATABASE_URL points at a reachable PostgreSQL.
func TestPostgresOTPRepository_VerifyOTP_Integration(t *testing.T) {
	db := openTestPostgresDB(t)
	ctx := context.Background()
	repo := NewPostgresOTPRepository(db)

	suffix := time.Now().UTC().Format("20060102150405.000000")
	keepEmail := "otp.keep." + suffix + "@swiftdrop.local"
	lockEmail := "otp.lock." + suffix + "@swiftdrop.local"
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM email_otps WHERE email IN ($1, $2)", keepEmail, lockEmail)
	})

	t.Run("a correct code is kept so the final step can consume it", func(t *testing.T) {
		if err := repo.SaveOTP(ctx, keepEmail, "123456"); err != nil {
			t.Fatalf("SaveOTP failed: %v", err)
		}
		if err := repo.VerifyOTP(ctx, keepEmail, "123456"); err != nil {
			t.Fatalf("expected correct code to verify, got %v", err)
		}
		if err := repo.VerifyOTP(ctx, keepEmail, "123456"); err != nil {
			t.Fatalf("expected second check to pass, got %v", err)
		}
		if err := repo.VerifyAndConsumeOTP(ctx, keepEmail, "123456"); err != nil {
			t.Fatalf("expected the final step to consume the kept code, got %v", err)
		}
		if err := repo.VerifyOTP(ctx, keepEmail, "123456"); err != domain.ErrOTPInvalid {
			t.Errorf("expected ErrOTPInvalid once consumed, got %v", err)
		}
	})

	t.Run("wrong codes share one 3-attempt limit with VerifyAndConsumeOTP", func(t *testing.T) {
		if err := repo.SaveOTP(ctx, lockEmail, "654321"); err != nil {
			t.Fatalf("SaveOTP failed: %v", err)
		}
		if err := repo.VerifyOTP(ctx, lockEmail, "111111"); err != domain.ErrOTPInvalid {
			t.Errorf("attempt 1: expected ErrOTPInvalid, got %v", err)
		}
		if err := repo.VerifyAndConsumeOTP(ctx, lockEmail, "222222"); err != domain.ErrOTPInvalid {
			t.Errorf("attempt 2: expected ErrOTPInvalid, got %v", err)
		}
		if err := repo.VerifyOTP(ctx, lockEmail, "333333"); err != domain.ErrOTPMaxAttempts {
			t.Errorf("attempt 3: expected ErrOTPMaxAttempts, got %v", err)
		}
		if err := repo.VerifyOTP(ctx, lockEmail, "654321"); err != domain.ErrOTPInvalid {
			t.Errorf("after lockout: expected ErrOTPInvalid, got %v", err)
		}
	})
}
