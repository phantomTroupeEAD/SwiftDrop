package application

import (
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// LoginCommand encapsulates the parameters for user authentication.
type LoginCommand struct {
	Email     string
	Password  string
	ClientIP  string
	UserAgent string
}

// LoginResult contains the authenticated access token, metadata, and user info.
type LoginResult struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int // in seconds, typically 900 (15 min)
	RefreshToken string
	User         *domain.User
}

// RefreshSessionCommand parameters for token rotation.
type RefreshSessionCommand struct {
	RawRefreshToken string
	ClientIP        string
	UserAgent       string
}

// RefreshResult contains the rotated access and refresh tokens.
type RefreshResult struct {
	AccessToken     string
	TokenType       string
	ExpiresIn       int
	NewRefreshToken string
	User            *domain.User
}

// LogoutCommand encapsulates parameters for session invalidation.
type LogoutCommand struct {
	RawRefreshToken string
}

// SendRegistrationOTPCommand encapsulates the email availability check and OTP dispatch.
type SendRegistrationOTPCommand struct {
	Email string
}

// SendRegistrationOTPResult contains metadata about the dispatched OTP.
type SendRegistrationOTPResult struct {
	Email           string
	ExpiresIn       int // seconds until OTP expires (600 = 10 minutes)
	CooldownSeconds int // seconds before the user may request another OTP (60)
}

// RegisterCustomerCommand contains the full payload required to create a new Customer account.
// The OTP must have been issued via SendRegistrationOTP for this email before calling Register.
type RegisterCustomerCommand struct {
	Email         string
	OTP           string
	FirstName     string
	LastName      string
	Phone         string
	Password      string
	TermsAccepted bool
	ClientIP      string
	UserAgent     string
}

// RegisterResult is returned on successful account creation.
type RegisterResult struct {
	User          *domain.User
	IsNewUser     bool
	RequiresPhone bool // true when the account was created via Google and phone is not yet set
}

// UpdateProfileCommand encapsulates fields a user may update on their own profile.
// Used after Google sign-in (Approach A progressive onboarding) to collect the phone number.
type UpdateProfileCommand struct {
	UserID        string
	Phone         string // E.164 format, e.g. +94771234567
	TermsAccepted bool
}

// GoogleLoginCommand contains the token and request metadata for Google OAuth sign-in / registration.
type GoogleLoginCommand struct {
	IDToken   string
	ClientIP  string
	UserAgent string
}

// GoogleLoginResult contains the session tokens and user onboarding state.
type GoogleLoginResult struct {
	AccessToken   string
	TokenType     string
	ExpiresIn     int
	RefreshToken  string
	User          *domain.User
	IsNewUser     bool
	RequiresPhone bool
}

