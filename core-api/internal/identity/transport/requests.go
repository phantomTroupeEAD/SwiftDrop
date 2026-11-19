package transport

import "github.com/swiftdrop/core-api/internal/identity/domain"

// LoginRequest defines the JSON payload for user authentication.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshTokenRequest defines the optional JSON body fallback for token refresh.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

// SendOTPRequest defines the JSON payload for requesting a registration OTP.
type SendOTPRequest struct {
	Email string `json:"email"`
}

// RegisterRequest defines the full JSON payload for OTP-verified customer registration.
type RegisterRequest struct {
	Email         string `json:"email"`
	OTP           string `json:"otp"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Phone         string `json:"phone"`
	Password      string `json:"password"`
	TermsAccepted bool   `json:"terms_accepted"`
}

// UpdateProfileRequest defines the JSON payload for completing a Google-registered
// user's profile (Approach A progressive onboarding).
type UpdateProfileRequest struct {
	Phone         string `json:"phone"`
	TermsAccepted bool   `json:"terms_accepted"`
}

// UpdateUserRoleRequest defines the JSON payload for administrative role updates.
type UpdateUserRoleRequest struct {
	Role     domain.Role `json:"role"`
	TenantID *string     `json:"tenant_id,omitempty"`
}

// UpdateUserStatusRequest defines the JSON payload for administrative status updates (ACTIVE / SUSPENDED).
type UpdateUserStatusRequest struct {
	Status domain.UserStatus `json:"status"`
}

// GoogleLoginRequest defines the JSON payload for Sign-In with Google.
type GoogleLoginRequest struct {
	IDToken string `json:"id_token"`
}

// ForgotPasswordSendOTPRequest defines the JSON payload for requesting a password reset OTP.
type ForgotPasswordSendOTPRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest defines the JSON payload for completing an OTP-verified password reset.
type ResetPasswordRequest struct {
	Email       string `json:"email"`
	OTP         string `json:"otp"`
	NewPassword string `json:"new_password"`
}
