package transport

import (
	"time"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// UserDTO models user data exposed in authentication responses.
type UserDTO struct {
	ID          string      `json:"id"`
	Email       string      `json:"email"`
	DisplayName string      `json:"display_name,omitempty"`
	Role        domain.Role `json:"role"`
	TenantID    *string     `json:"tenant_id"`
}

// LoginDataDTO contains the token and user payload.
type LoginDataDTO struct {
	AccessToken string  `json:"access_token"`
	TokenType   string  `json:"token_type"`
	ExpiresIn   int     `json:"expires_in"`
	User        UserDTO `json:"user"`
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
