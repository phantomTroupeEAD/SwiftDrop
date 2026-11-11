package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// PostgresTokenRepository implements application.TokenRepository using PostgreSQL.
type PostgresTokenRepository struct {
	db *sql.DB
}

// NewPostgresTokenRepository constructs a repository backed by PostgreSQL.
func NewPostgresTokenRepository(db *sql.DB) *PostgresTokenRepository {
	return &PostgresTokenRepository{db: db}
}

// SaveToken inserts or updates a refresh token session record.
func (r *PostgresTokenRepository) SaveToken(ctx context.Context, session *domain.RefreshTokenSession) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, family_id, is_revoked, expires_at, used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (token_hash) DO UPDATE
		SET is_revoked = EXCLUDED.is_revoked,
		    used_at = EXCLUDED.used_at;
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.FamilyID,
		session.IsRevoked,
		session.ExpiresAt,
		session.UsedAt,
		session.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres SaveToken failed: %w", err)
	}
	return nil
}

// FindByTokenHash looks up a refresh token session by its SHA-256 token hash.
func (r *PostgresTokenRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshTokenSession, error) {
	query := `
		SELECT id, user_id, token_hash, family_id, is_revoked, expires_at, used_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, tokenHash)

	var (
		id, userID, hash, familyID string
		isRevoked                  bool
		expiresAt, createdAt       time.Time
		usedAt                     sql.NullTime
	)

	err := row.Scan(
		&id,
		&userID,
		&hash,
		&familyID,
		&isRevoked,
		&expiresAt,
		&usedAt,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTokenExpired
		}
		return nil, fmt.Errorf("postgres FindByTokenHash failed: %w", err)
	}

	var usedAtPtr *time.Time
	if usedAt.Valid {
		t := usedAt.Time
		usedAtPtr = &t
	}

	return &domain.RefreshTokenSession{
		ID:        id,
		UserID:    userID,
		TokenHash: hash,
		FamilyID:  familyID,
		IsRevoked: isRevoked,
		ExpiresAt: expiresAt,
		UsedAt:    usedAtPtr,
		CreatedAt: createdAt,
	}, nil
}

// MarkTokenUsed marks a single token as used for rotation (used_at = now).
func (r *PostgresTokenRepository) MarkTokenUsed(ctx context.Context, id string) error {
	query := `
		UPDATE refresh_tokens
		SET used_at = $1
		WHERE id = $2;
	`
	_, err := r.db.ExecContext(ctx, query, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("postgres MarkTokenUsed failed: %w", err)
	}
	return nil
}

// RevokeTokenFamily marks all tokens in a family as revoked (is_revoked = true).
func (r *PostgresTokenRepository) RevokeTokenFamily(ctx context.Context, familyID string) error {
	query := `
		UPDATE refresh_tokens
		SET is_revoked = TRUE
		WHERE family_id = $1;
	`
	_, err := r.db.ExecContext(ctx, query, familyID)
	if err != nil {
		return fmt.Errorf("postgres RevokeTokenFamily failed: %w", err)
	}
	return nil
}
