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

// OTPRepository defines the port for managing short-lived One-Time Password codes
// used to verify email ownership before account creation.
type OTPRepository interface {
	SaveOTP(ctx context.Context, email, code string) error
	VerifyAndConsumeOTP(ctx context.Context, email, code string) error
	CheckCooldown(ctx context.Context, email string) error
}

// EmailSender defines the port for dispatching transactional email messages.
type EmailSender interface {
	SendOTPEmail(ctx context.Context, recipientEmail, code string) error
	SendPasswordResetEmail(ctx context.Context, recipientEmail, code string) error
}

// RoleAssigner enables other bounded contexts (e.g., Member 4 Merchant Integration,
// Member 7 Admin Approval) to grant or promote user roles.
type RoleAssigner interface {
	AssignRole(ctx context.Context, userID string, newRole domain.Role, tenantID *string) error
}

// AuditEntry encapsulates an immutable audit record for sensitive operations.
type AuditEntry struct {
	ActorID    string                 `json:"actor_id"`
	ActorRole  string                 `json:"actor_role"`
	Action     string                 `json:"action"`
	EntityType string                 `json:"entity_type"`
	EntityID   string                 `json:"entity_id"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
}

// AuditWriter defines the port for recording compliance and lifecycle audit logs.
type AuditWriter interface {
	WriteAuditLog(ctx context.Context, entry AuditEntry) error
}

// OAuthRepository defines persistence for federated third-party identity links.
type OAuthRepository interface {
	SaveLink(ctx context.Context, identity *domain.OAuthIdentity) error
	FindByProviderSubject(ctx context.Context, provider, subject string) (*domain.OAuthIdentity, error)
	FindByUserID(ctx context.Context, userID string) ([]*domain.OAuthIdentity, error)
}

// OIDCClaims encapsulates the claims decoded and verified from a federated identity provider token.
type OIDCClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// OIDCClient defines the port for validating third-party OIDC ID tokens (e.g. Google ID tokens).
type OIDCClient interface {
	VerifyIDToken(ctx context.Context, idToken string) (*OIDCClaims, error)
}
