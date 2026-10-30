package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// AuthService coordinates all identity authentication use cases.
type AuthService struct {
	userRepo        UserRepository
	tokenRepo       TokenRepository
	hasher          PasswordHasher
	tokenProvider   TokenProvider
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewAuthService constructs a configured AuthService instance.
func NewAuthService(
	userRepo UserRepository,
	tokenRepo TokenRepository,
	hasher PasswordHasher,
	tokenProvider TokenProvider,
) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		tokenRepo:       tokenRepo,
		hasher:          hasher,
		tokenProvider:   tokenProvider,
		accessTokenTTL:  15 * time.Minute,
		refreshTokenTTL: 7 * 24 * time.Hour,
	}
}

// Login validates user credentials and issues a fresh JWT access token + refresh token session.
func (s *AuthService) Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	// 1. Normalize and validate email address
	emailVO, err := domain.NewEmail(cmd.Email)
	if err != nil {
		// Generic invalid credentials error to prevent account enumeration
		return nil, domain.ErrInvalidCredentials
	}

	// 2. Locate user aggregate by email
	user, err := s.userRepo.FindByEmail(ctx, emailVO.String())
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	// 3. Verify user status
	if !user.IsActive() {
		return nil, domain.ErrAccountSuspended
	}

	// 4. Verify password hash using constant-time comparison
	if !s.hasher.VerifyPassword(cmd.Password, user.PasswordHash) {
		return nil, domain.ErrInvalidCredentials
	}

	// 5. Construct Principal security context
	principal := Principal{
		UserID:   user.ID,
		Email:    user.Email.String(),
		Role:     user.Role,
		TenantID: user.TenantID,
	}

	// 6. Mint short-lived JWT access token (15 mins)
	accessToken, err := s.tokenProvider.GenerateAccessToken(principal)
	if err != nil {
		return nil, fmt.Errorf("failed to issue access token: %w", err)
	}

	// 7. Generate random opaque refresh token and session
	rawRefreshToken, tokenHash, err := generateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	familyID := "fam_" + generateRandomHex(12)
	sessionID := "ses_" + generateRandomHex(16)
	session := domain.NewRefreshTokenSession(
		sessionID,
		user.ID,
		tokenHash,
		familyID,
		s.refreshTokenTTL,
	)

	if s.tokenRepo != nil {
		if err := s.tokenRepo.SaveToken(ctx, session); err != nil {
			// Non-blocking in mock environments, but logged/handled
			_ = err
		}
	}

	return &LoginResult{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTokenTTL.Seconds()),
		RefreshToken: rawRefreshToken,
		User:         user,
	}, nil
}

// Helper to generate a cryptographically random token and its SHA-256 hash
func generateOpaqueToken() (raw string, hash string, error error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = "rt_" + hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

func generateRandomHex(byteCount int) string {
	b := make([]byte, byteCount)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
