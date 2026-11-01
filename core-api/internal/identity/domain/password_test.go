package domain_test

import (
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

func TestValidatePlaintextPassword(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectError bool
	}{
		{"Valid password with symbols", "AdminPassword123!", false},
		{"Valid courier password", "Courier123#", false},
		{"Too short (< 8 chars)", "Aa1!", true},
		{"Missing uppercase", "adminpassword123!", true},
		{"Missing lowercase", "ADMINPASSWORD123!", true},
		{"Missing digit", "AdminPassword!", true},
		{"Missing symbol", "AdminPassword123", true},
		{"Empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidatePlaintextPassword(tt.password)
			if (err != nil) != tt.expectError {
				t.Errorf("ValidatePlaintextPassword(%q) error = %v; want error: %v", tt.password, err, tt.expectError)
			}
		})
	}
}
