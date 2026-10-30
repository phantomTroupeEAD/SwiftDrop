package domain

import (
	"regexp"
	"strings"
)

// Simple RFC 5322 compliant regex pattern for email format verification.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)

// Email is an immutable value object representing a normalized email address.
type Email string

// NewEmail constructs, normalizes (lowercase, trimmed), and validates an Email value object.
func NewEmail(raw string) (Email, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > 254 {
		return "", ErrInvalidEmail
	}

	lowered := strings.ToLower(trimmed)
	if !emailRegex.MatchString(lowered) {
		return "", ErrInvalidEmail
	}

	return Email(lowered), nil
}

// String returns the normalized email address string.
func (e Email) String() string {
	return string(e)
}
