package transport

import (
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// UserDTO models user data exposed in authentication and registration responses.
type UserDTO struct {
	ID            string            `json:"id"`
	Email         string            `json:"email"`
	FirstName     string            `json:"first_name,omitempty"`
	LastName      string            `json:"last_name,omitempty"`
	DisplayName   string            `json:"display_name,omitempty"`
	Phone         string            `json:"phone,omitempty"`
	Role          domain.Role       `json:"role"`
	TenantID      *string           `json:"tenant_id"`
	Status        domain.UserStatus `json:"status"`
	EmailVerified bool              `json:"email_verified"`
	RequiresPhone bool              `json:"requires_phone"`
	CreatedAt     string            `json:"created_at,omitempty"`
}

// toUserDTO converts a domain.User aggregate into a transport-safe DTO.
func toUserDTO(u *domain.User) UserDTO {
	return UserDTO{
		ID:            u.ID,
		Email:         u.Email.String(),
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		DisplayName:   u.DisplayName,
		Phone:         u.Phone,
		Role:          u.Role,
		TenantID:      u.TenantID,
		Status:        u.Status,
		EmailVerified: u.EmailVerified,
		RequiresPhone: u.RequiresPhone(),
		CreatedAt:     u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// LoginDataDTO contains the token and user payload returned on successful login.
type LoginDataDTO struct {
	AccessToken string  `json:"access_token"`
	TokenType   string  `json:"token_type"`
	ExpiresIn   int     `json:"expires_in"`
	User        UserDTO `json:"user"`
}

// GoogleLoginDataDTO contains the token, user payload, and onboarding flags returned on Google Sign-In.
type GoogleLoginDataDTO struct {
	AccessToken   string  `json:"access_token"`
	TokenType     string  `json:"token_type"`
	ExpiresIn     int     `json:"expires_in"`
	User          UserDTO `json:"user"`
	IsNewUser     bool    `json:"is_new_user"`
	RequiresPhone bool    `json:"requires_phone"`
}

// RegisterDataDTO is the response body returned after successful account creation.
type RegisterDataDTO struct {
	User      UserDTO `json:"user"`
	IsNewUser bool    `json:"is_new_user"`
}

// OTPDataDTO is returned after a registration OTP is dispatched.
type OTPDataDTO struct {
	Message         string `json:"message"`
	Email           string `json:"email"`
	ExpiresIn       int    `json:"expires_in"`
	CooldownSeconds int    `json:"cooldown_seconds"`
}

// SingleDataEnvelope wraps successful responses per SwiftDrop API conventions.
type SingleDataEnvelope struct {
	Data interface{} `json:"data"`
}

// ErrorDetail describes a specific validation problem.
type ErrorDetail struct {
	Field string `json:"field,omitempty"`
	Issue string `json:"issue"`
}

// ErrorEnvelopeBody defines the standard structured error envelope per api-conventions.md.
type ErrorEnvelopeBody struct {
	Code          string        `json:"code"`
	Message       string        `json:"message"`
	CorrelationID string        `json:"correlation_id,omitempty"`
	Timestamp     string        `json:"timestamp"`
	Details       []ErrorDetail `json:"details,omitempty"`
}

// ErrorEnvelope wraps an ErrorEnvelopeBody.
type ErrorEnvelope struct {
	Error ErrorEnvelopeBody `json:"error"`
}

// NewErrorEnvelope creates a structured error payload.
func NewErrorEnvelope(code, message string, details ...ErrorDetail) ErrorEnvelope {
	return ErrorEnvelope{
		Error: ErrorEnvelopeBody{
			Code:      code,
			Message:   message,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Details:   details,
		},
	}
}
