package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// PostgresOAuthRepository implements application.OAuthRepository using PostgreSQL.
type PostgresOAuthRepository struct {
	db *sql.DB
}

// NewPostgresOAuthRepository constructs a repository backed by PostgreSQL.
func NewPostgresOAuthRepository(db *sql.DB) *PostgresOAuthRepository {
	return &PostgresOAuthRepository{db: db}
}

// SaveLink stores or updates an OAuth federated identity link.
func (r *PostgresOAuthRepository) SaveLink(ctx context.Context, identity *domain.OAuthIdentity) error {
	query := `
		INSERT INTO oauth_identities (user_id, provider, provider_subject, email, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider_subject) DO UPDATE
		SET email = EXCLUDED.email;
	`
	createdAt := identity.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		identity.UserID,
		identity.Provider,
		identity.ProviderSubject,
		identity.Email,
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("postgres SaveLink failed: %w", err)
	}
	return nil
}

// FindByProviderSubject finds an identity link by provider and provider subject ID.
func (r *PostgresOAuthRepository) FindByProviderSubject(ctx context.Context, provider, subject string) (*domain.OAuthIdentity, error) {
	query := `
		SELECT id, user_id, provider, provider_subject, email, created_at
		FROM oauth_identities
		WHERE provider = $1 AND provider_subject = $2
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, provider, subject)

	var (
		id, userID, prov, sub, email string
		createdAt                    time.Time
	)
	err := row.Scan(&id, &userID, &prov, &sub, &email, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("oauth link not found")
		}
		return nil, fmt.Errorf("postgres query failed: %w", err)
	}

	return &domain.OAuthIdentity{
		ID:              id,
		UserID:          userID,
		Provider:        prov,
		ProviderSubject: sub,
		Email:           email,
		CreatedAt:       createdAt,
	}, nil
}

// FindByUserID retrieves all OAuth links associated with a user.
func (r *PostgresOAuthRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.OAuthIdentity, error) {
	query := `
		SELECT id, user_id, provider, provider_subject, email, created_at
		FROM oauth_identities
		WHERE user_id = $1
		ORDER BY created_at ASC;
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres query failed: %w", err)
	}
	defer rows.Close()

	var links []*domain.OAuthIdentity
	for rows.Next() {
		var (
			id, uid, prov, sub, email string
			createdAt                 time.Time
		)
		if err := rows.Scan(&id, &uid, &prov, &sub, &email, &createdAt); err != nil {
			return nil, fmt.Errorf("postgres scan failed: %w", err)
		}
		links = append(links, &domain.OAuthIdentity{
			ID:              id,
			UserID:          uid,
			Provider:        prov,
			ProviderSubject: sub,
			Email:           email,
			CreatedAt:       createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres rows iteration failed: %w", err)
	}

	return links, nil
}
