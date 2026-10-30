package application

import (
	"context"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// Principal represents the authenticated security context of an actor.
type Principal struct {
	UserID   string      `json:"user_id"`
	Email    string      `json:"email"`
	Role     domain.Role `json:"role"`
	TenantID *string     `json:"tenant_id,omitempty"`
}

// UserRepository defines the persistence port for User aggregates.
type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Save(ctx context.Context, user *domain.User) error
	ListUsers(ctx context.Context, role *domain.Role, limit int, offset int) ([]*domain.User, int, error)
}

// TokenRepository defines persistence for refresh token sessions.
type TokenRepository interface {
	SaveToken(ctx context.Context, session *domain.RefreshTokenSession) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshTokenSession, error)
	MarkTokenUsed(ctx context.Context, id string) error
	RevokeTokenFamily(ctx context.Context, familyID string) error
}

// PasswordHasher defines the port for cryptographic password hashing and verification.
type PasswordHasher interface {
	HashPassword(plaintext string) (string, error)
	VerifyPassword(plaintext string, hash string) bool
}

// TokenProvider defines the port for issuing and validating signed JWT access tokens.
type TokenProvider interface {
	GenerateAccessToken(principal Principal) (string, error)
	ValidateAccessToken(tokenString string) (*Principal, error)
}
