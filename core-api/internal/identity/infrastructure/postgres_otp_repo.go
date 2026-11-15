package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

const (
	pgOTPTTL         = 10 * time.Minute
	pgOTPCooldown    = 60 * time.Second
	pgOTPMaxAttempts = 3
)

// PostgresOTPRepository is a PostgreSQL-backed implementation of application.OTPRepository.
type PostgresOTPRepository struct {
	db *sql.DB
}

// NewPostgresOTPRepository constructs a PostgresOTPRepository.
func NewPostgresOTPRepository(db *sql.DB) *PostgresOTPRepository {
	return &PostgresOTPRepository{db: db}
}

// CheckCooldown returns domain.ErrOTPCooldownActive if a resend is requested
// within 60 seconds of the previous send.
func (r *PostgresOTPRepository) CheckCooldown(ctx context.Context, email string) error {
	var cooldownEnd time.Time
	err := r.db.QueryRowContext(
		ctx,
		`SELECT cooldown_end FROM email_otps WHERE email = $1`,
		email,
	).Scan(&cooldownEnd)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check otp cooldown: %w", err)
	}
	if time.Now().UTC().Before(cooldownEnd) {
		return domain.ErrOTPCooldownActive
	}
	return nil
}

// SaveOTP stores a fresh OTP code for the given email, resetting the cooldown
// and attempt counter. Any previous record for the same email is overwritten.
func (r *PostgresOTPRepository) SaveOTP(ctx context.Context, email, code string) error {
	now := time.Now().UTC()
	expiresAt := now.Add(pgOTPTTL)
	cooldownEnd := now.Add(pgOTPCooldown)

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO email_otps (email, code, attempts, expires_at, cooldown_end, updated_at)
		 VALUES ($1, $2, 0, $3, $4, NOW())
		 ON CONFLICT (email) DO UPDATE
		 SET code = EXCLUDED.code,
		     attempts = 0,
		     expires_at = EXCLUDED.expires_at,
		     cooldown_end = EXCLUDED.cooldown_end,
		     updated_at = NOW()`,
		email, code, expiresAt, cooldownEnd,
	)
	if err != nil {
		return fmt.Errorf("failed to save otp: %w", err)
	}
	return nil
}

// VerifyAndConsumeOTP validates the provided code against the stored record,
// enforces expiry and attempt limits, and deletes the record on success.
func (r *PostgresOTPRepository) VerifyAndConsumeOTP(ctx context.Context, email, code string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin otp verification tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var storedCode string
	var attempts int
	var expiresAt time.Time

	err = tx.QueryRowContext(
		ctx,
		`SELECT code, attempts, expires_at FROM email_otps WHERE email = $1 FOR UPDATE`,
		email,
	).Scan(&storedCode, &attempts, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrOTPInvalid
	}
	if err != nil {
		return fmt.Errorf("failed to query otp: %w", err)
	}

	if time.Now().UTC().After(expiresAt) {
		if _, delErr := tx.ExecContext(ctx, `DELETE FROM email_otps WHERE email = $1`, email); delErr != nil {
			return fmt.Errorf("failed to delete expired otp: %w", delErr)
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("failed to commit expired otp cleanup: %w", commitErr)
		}
		return domain.ErrOTPExpired
	}

	if attempts >= pgOTPMaxAttempts {
		if _, delErr := tx.ExecContext(ctx, `DELETE FROM email_otps WHERE email = $1`, email); delErr != nil {
			return fmt.Errorf("failed to delete max-attempt otp: %w", delErr)
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("failed to commit max-attempt otp cleanup: %w", commitErr)
		}
		return domain.ErrOTPMaxAttempts
	}

	if storedCode != code {
		nextAttempts := attempts + 1
		if nextAttempts >= pgOTPMaxAttempts {
			if _, delErr := tx.ExecContext(ctx, `DELETE FROM email_otps WHERE email = $1`, email); delErr != nil {
				return fmt.Errorf("failed to delete max-attempt otp: %w", delErr)
			}
			if commitErr := tx.Commit(); commitErr != nil {
				return fmt.Errorf("failed to commit max-attempt otp cleanup: %w", commitErr)
			}
			return domain.ErrOTPMaxAttempts
		}
		if _, updErr := tx.ExecContext(
			ctx,
			`UPDATE email_otps SET attempts = $2, updated_at = NOW() WHERE email = $1`,
			email, nextAttempts,
		); updErr != nil {
			return fmt.Errorf("failed to increment otp attempts: %w", updErr)
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("failed to commit otp attempt increment: %w", commitErr)
		}
		return domain.ErrOTPInvalid
	}

	if _, delErr := tx.ExecContext(ctx, `DELETE FROM email_otps WHERE email = $1`, email); delErr != nil {
		return fmt.Errorf("failed to consume otp: %w", delErr)
	}
	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("failed to commit otp consumption: %w", commitErr)
	}
	return nil
}
