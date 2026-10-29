package domain

import (
	"unicode"
)

// ValidatePlaintextPassword verifies that a password complies with SwiftDrop security requirements:
// Minimum 8 characters, at least 1 uppercase letter, 1 lowercase letter, 1 digit, and 1 special symbol.
func ValidatePlaintextPassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	if len(password) > 128 {
		return ErrWeakPassword
	}

	var (
		hasUpper  bool
		hasLower  bool
		hasDigit  bool
		hasSymbol bool
	)

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}

	if !(hasUpper && hasLower && hasDigit && hasSymbol) {
		return ErrWeakPassword
	}

	return nil
}
