package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// e164Regex validates E.164 international phone numbers, e.g. +94771234567.
var e164Regex = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)

// AuthService coordinates all identity authentication use cases.
type AuthService struct {
	userRepo        UserRepository
	tokenRepo       TokenRepository
	otpRepo         OTPRepository
	emailSender     EmailSender
	oauthRepo       OAuthRepository
	oidcClient      OIDCClient
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

// WithOTP attaches OTP and email-sender dependencies for registration.
// Call this after NewAuthService when OTP-based registration is required.
func (s *AuthService) WithOTP(otpRepo OTPRepository, emailSender EmailSender) *AuthService {
	s.otpRepo = otpRepo
	s.emailSender = emailSender
	return s
}

// WithOAuth attaches federated OAuth identity repository and OIDC verifier.
func (s *AuthService) WithOAuth(oauthRepo OAuthRepository, oidcClient OIDCClient) *AuthService {
	s.oauthRepo = oauthRepo
	s.oidcClient = oidcClient
	return s
}

// ─────────────────────────────────────────────
// Registration Use Cases
// ─────────────────────────────────────────────

// SendRegistrationOTP checks whether the email is available, enforces a resend
// cooldown, generates a 6-digit code, persists it, and dispatches it via email.
func (s *AuthService) SendRegistrationOTP(ctx context.Context, cmd SendRegistrationOTPCommand) (*SendRegistrationOTPResult, error) {
	// 1. Validate and normalize email format
	emailVO, err := domain.NewEmail(cmd.Email)
	if err != nil {
		return nil, domain.ErrInvalidEmail
	}

	// 2. Confirm the email is not already registered
	if _, err := s.userRepo.FindByEmail(ctx, emailVO.String()); err == nil {
		return nil, domain.ErrEmailAlreadyExists
	}

	// 3. Enforce resend cooldown (60 s)
	if s.otpRepo != nil {
		if err := s.otpRepo.CheckCooldown(ctx, emailVO.String()); err != nil {
			return nil, err
		}
	}

	// 4. Generate secure 6-digit numeric code
	code, err := generateSixDigitOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate otp: %w", err)
	}

	// 5. Persist OTP (TTL: 10 min, cooldown: 60 s, max attempts: 3)
	if s.otpRepo != nil {
		if err := s.otpRepo.SaveOTP(ctx, emailVO.String(), code); err != nil {
			return nil, fmt.Errorf("failed to save otp: %w", err)
		}
	}

	// 6. Dispatch email (dev mode: printed to stdout)
	if s.emailSender != nil {
		if err := s.emailSender.SendOTPEmail(ctx, emailVO.String(), code); err != nil {
			_ = err
		}
	}

	return &SendRegistrationOTPResult{
		Email:           emailVO.String(),
		ExpiresIn:       600, // 10 minutes
		CooldownSeconds: 60,
	}, nil
}

// Register verifies the OTP, hashes the password, and creates the customer account.
func (s *AuthService) Register(ctx context.Context, cmd RegisterCustomerCommand) (*RegisterResult, error) {
	// 1. Validate and normalize email
	emailVO, err := domain.NewEmail(cmd.Email)
	if err != nil {
		return nil, domain.ErrInvalidEmail
	}

	// 2. Require terms acceptance
	if !cmd.TermsAccepted {
		return nil, domain.ErrTermsNotAccepted
	}

	// 3. Validate password complexity
	if err := domain.ValidatePlaintextPassword(cmd.Password); err != nil {
		return nil, err
	}

	// 4. Validate first & last name
	firstName := strings.TrimSpace(cmd.FirstName)
	lastName := strings.TrimSpace(cmd.LastName)
	if len(firstName) < 1 || len(lastName) < 1 {
		return nil, fmt.Errorf("first_name and last_name are required")
	}

	// 5. Validate phone (E.164)
	phone := strings.TrimSpace(cmd.Phone)
	if !e164Regex.MatchString(phone) {
		return nil, domain.ErrInvalidPhone
	}

	// 6. Verify & consume OTP (single-use, replay-safe)
	if s.otpRepo != nil {
		if err := s.otpRepo.VerifyAndConsumeOTP(ctx, emailVO.String(), strings.TrimSpace(cmd.OTP)); err != nil {
			return nil, err
		}
	}

	// 7. Final uniqueness guard
	if _, err := s.userRepo.FindByEmail(ctx, emailVO.String()); err == nil {
		return nil, domain.ErrEmailAlreadyExists
	}

	// 8. Hash password
	passwordHash, err := s.hasher.HashPassword(cmd.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 9. Build user aggregate
	userID := "usr_" + generateRandomHex(16)
	displayName := firstName + " " + lastName
	newUser := domain.NewUser(
		userID,
		emailVO,
		passwordHash,
		displayName,
		phone,
		domain.RoleCustomer,
		nil,
		domain.StatusActive,
		time.Now().UTC(),
		time.Now().UTC(),
	)
	newUser.FirstName = firstName
	newUser.LastName = lastName
	newUser.EmailVerified = true
	newUser.TermsAcceptedAt = func() *time.Time { t := time.Now().UTC(); return &t }()

	// 10. Persist
	if err := s.userRepo.Save(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	return &RegisterResult{
		User:          newUser,
		IsNewUser:     true,
		RequiresPhone: false,
	}, nil
}

// UpdateProfile allows an authenticated user to complete their profile.
func (s *AuthService) UpdateProfile(ctx context.Context, cmd UpdateProfileCommand) (*domain.User, error) {
	user, err := s.userRepo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	phone := strings.TrimSpace(cmd.Phone)
	if !e164Regex.MatchString(phone) {
		return nil, domain.ErrInvalidPhone
	}

	if !cmd.TermsAccepted && user.TermsAcceptedAt == nil {
		return nil, domain.ErrTermsNotAccepted
	}

	user.Phone = phone
	if cmd.TermsAccepted && user.TermsAcceptedAt == nil {
		t := time.Now().UTC()
		user.TermsAcceptedAt = &t
	}
	user.UpdatedAt = time.Now().UTC()

	if err := s.userRepo.Save(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	return user, nil
}

// ─────────────────────────────────────────────
// Login & Session Lifecycle Use Cases
// ─────────────────────────────────────────────

// Login validates user credentials and issues a fresh JWT access token + refresh token session.
func (s *AuthService) Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	emailVO, err := domain.NewEmail(cmd.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	user, err := s.userRepo.FindByEmail(ctx, emailVO.String())
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if !user.IsActive() {
		return nil, domain.ErrAccountSuspended
	}

	if !s.hasher.VerifyPassword(cmd.Password, user.PasswordHash) {
		return nil, domain.ErrInvalidCredentials
	}

	principal := Principal{
		UserID:   user.ID,
		Email:    user.Email.String(),
		Role:     user.Role,
		TenantID: user.TenantID,
	}

	accessToken, err := s.tokenProvider.GenerateAccessToken(principal)
	if err != nil {
		return nil, fmt.Errorf("failed to issue access token: %w", err)
	}

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

// RefreshSession validates an existing refresh token, checks for replay attacks,
// rotates the token family, and mints a fresh JWT access token (ADR-003).
func (s *AuthService) RefreshSession(ctx context.Context, cmd RefreshSessionCommand) (*RefreshResult, error) {
	rawToken := strings.TrimSpace(cmd.RawRefreshToken)
	if rawToken == "" {
		return nil, domain.ErrInvalidToken
	}

	if s.tokenRepo == nil {
		return nil, fmt.Errorf("token repository not configured")
	}

	tokenHash := hashToken(rawToken)
	session, err := s.tokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, domain.ErrTokenExpired
	}

	// ── Replay Attack Detection ──
	// If a token that was already used is presented again, someone intercepted or cloned it!
	// Invalidate the ENTIRE token family immediately to protect the legitimate user.
	if session.UsedAt != nil {
		_ = s.tokenRepo.RevokeTokenFamily(ctx, session.FamilyID)
		return nil, domain.ErrTokenReused
	}

	if session.IsRevoked {
		return nil, domain.ErrTokenRevoked
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		return nil, domain.ErrTokenExpired
	}

	// Load user to ensure status is active
	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}
	if !user.IsActive() {
		return nil, domain.ErrAccountSuspended
	}

	// 1. Mark the current token as used (single-use rotation)
	if err := s.tokenRepo.MarkTokenUsed(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("failed to mark token as used: %w", err)
	}

	// 2. Mint fresh rotated refresh token belonging to the SAME family
	newRawRefreshToken, newHash, err := generateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate rotated refresh token: %w", err)
	}

	newSessionID := "ses_" + generateRandomHex(16)
	newSession := domain.NewRefreshTokenSession(
		newSessionID,
		user.ID,
		newHash,
		session.FamilyID, // preserves token family lineage for ADR-003
		s.refreshTokenTTL,
	)

	if err := s.tokenRepo.SaveToken(ctx, newSession); err != nil {
		return nil, fmt.Errorf("failed to persist rotated session: %w", err)
	}

	// 3. Mint fresh short-lived JWT access token
	principal := Principal{
		UserID:   user.ID,
		Email:    user.Email.String(),
		Role:     user.Role,
		TenantID: user.TenantID,
	}
	newAccessToken, err := s.tokenProvider.GenerateAccessToken(principal)
	if err != nil {
		return nil, fmt.Errorf("failed to issue access token: %w", err)
	}

	return &RefreshResult{
		AccessToken:     newAccessToken,
		TokenType:       "Bearer",
		ExpiresIn:       int(s.accessTokenTTL.Seconds()),
		NewRefreshToken: newRawRefreshToken,
		User:            user,
	}, nil
}

// Logout revokes the refresh token session family, preventing any further refreshes.
func (s *AuthService) Logout(ctx context.Context, cmd LogoutCommand) error {
	rawToken := strings.TrimSpace(cmd.RawRefreshToken)
	if rawToken == "" || s.tokenRepo == nil {
		return nil // Idempotent logout succeeds even with no token
	}

	tokenHash := hashToken(rawToken)
	session, err := s.tokenRepo.FindByTokenHash(ctx, tokenHash)
	if err == nil && session != nil {
		_ = s.tokenRepo.RevokeTokenFamily(ctx, session.FamilyID)
	}

	return nil
}

// ─────────────────────────────────────────────
// Google OAuth Use Cases
// ─────────────────────────────────────────────

// LoginWithGoogle handles federated authentication using Google Sign-In.
// If the user already has a linked Google identity, they are logged in directly.
// If an existing account with the same email exists, the Google identity is linked.
// If the user is entirely new, an account is created with role CUSTOMER, EmailVerified=true,
// and marked as requiring phone onboarding (Approach A: Progressive Onboarding).
func (s *AuthService) LoginWithGoogle(ctx context.Context, cmd GoogleLoginCommand) (*GoogleLoginResult, error) {
	if s.oidcClient == nil || s.oauthRepo == nil {
		return nil, fmt.Errorf("oauth provider is not configured")
	}

	idToken := strings.TrimSpace(cmd.IDToken)
	if idToken == "" {
		return nil, domain.ErrInvalidToken
	}

	// 1. Verify token with OIDC client
	claims, err := s.oidcClient.VerifyIDToken(ctx, idToken)
	if err != nil || claims == nil || strings.TrimSpace(claims.Subject) == "" {
		return nil, domain.ErrInvalidToken
	}

	emailStr := strings.TrimSpace(strings.ToLower(claims.Email))
	emailVO, err := domain.NewEmail(emailStr)
	if err != nil {
		return nil, domain.ErrInvalidEmail
	}

	var user *domain.User
	var isNewUser bool

	// 2. Check if this Google subject is already linked
	existingLink, err := s.oauthRepo.FindByProviderSubject(ctx, domain.ProviderGoogle, claims.Subject)
	if err == nil && existingLink != nil {
		// Existing link found -> fetch user
		user, err = s.userRepo.FindByID(ctx, existingLink.UserID)
		if err != nil {
			return nil, domain.ErrInvalidCredentials
		}
	} else {
		// No link for this provider+subject yet.
		// Check if an account already exists with this email
		existingUser, err := s.userRepo.FindByEmail(ctx, emailVO.String())
		if err == nil && existingUser != nil {
			// Account exists -> link Google identity
			user = existingUser
			link := domain.NewOAuthIdentity(
				"oai_"+generateRandomHex(16),
				user.ID,
				domain.ProviderGoogle,
				claims.Subject,
				emailVO.String(),
			)
			if err := s.oauthRepo.SaveLink(ctx, link); err != nil {
				return nil, fmt.Errorf("failed to link oauth identity: %w", err)
			}
		} else {
			// Brand new user registration via Google
			isNewUser = true
			firstName := strings.TrimSpace(claims.GivenName)
			lastName := strings.TrimSpace(claims.FamilyName)
			if firstName == "" && lastName == "" {
				name := strings.TrimSpace(claims.Name)
				parts := strings.Fields(name)
				if len(parts) >= 2 {
					firstName = parts[0]
					lastName = strings.Join(parts[1:], " ")
				} else if len(parts) == 1 {
					firstName = parts[0]
					lastName = ""
				} else {
					firstName = strings.Split(emailVO.String(), "@")[0]
					lastName = ""
				}
			}

			// Generate random unguessable password hash for OAuth-only users
			randomSecret := generateRandomHex(32)
			pwdHash, err := s.hasher.HashPassword(randomSecret)
			if err != nil {
				return nil, fmt.Errorf("failed to hash password: %w", err)
			}

			userID := "usr_" + generateRandomHex(16)
			displayName := strings.TrimSpace(firstName + " " + lastName)
			if displayName == "" {
				displayName = firstName
			}

			newUser := domain.NewUser(
				userID,
				emailVO,
				pwdHash,
				displayName,
				"", // phone empty -> requires phone onboarding
				domain.RoleCustomer,
				nil,
				domain.StatusActive,
				time.Now().UTC(),
				time.Now().UTC(),
			)
			newUser.FirstName = firstName
			newUser.LastName = lastName
			newUser.EmailVerified = true

			if err := s.userRepo.Save(ctx, newUser); err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}

			link := domain.NewOAuthIdentity(
				"oai_"+generateRandomHex(16),
				newUser.ID,
				domain.ProviderGoogle,
				claims.Subject,
				emailVO.String(),
			)
			if err := s.oauthRepo.SaveLink(ctx, link); err != nil {
				return nil, fmt.Errorf("failed to link oauth identity: %w", err)
			}

			user = newUser
		}
	}

	// 3. Confirm user is active
	if !user.IsActive() {
		return nil, domain.ErrAccountSuspended
	}

	// 4. Generate Access Token & Refresh Session
	principal := Principal{
		UserID:   user.ID,
		Email:    user.Email.String(),
		Role:     user.Role,
		TenantID: user.TenantID,
	}

	accessToken, err := s.tokenProvider.GenerateAccessToken(principal)
	if err != nil {
		return nil, fmt.Errorf("failed to issue access token: %w", err)
	}

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
			_ = err
		}
	}

	return &GoogleLoginResult{
		AccessToken:   accessToken,
		TokenType:     "Bearer",
		ExpiresIn:     int(s.accessTokenTTL.Seconds()),
		RefreshToken:  rawRefreshToken,
		User:          user,
		IsNewUser:     isNewUser,
		RequiresPhone: user.RequiresPhone(),
	}, nil
}

// ─────────────────────────────────────────────
// Private helpers
// ─────────────────────────────────────────────

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateSixDigitOTP() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	val := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 1_000_000
	return fmt.Sprintf("%06d", val), nil
}

func generateOpaqueToken() (raw string, hash string, err error) {
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
