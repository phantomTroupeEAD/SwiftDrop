package domain_test

import (
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{"Standard email", "admin@swiftdrop.local", "admin@swiftdrop.local", false},
		{"Uppercase letters", "Admin@SwiftDrop.Local", "admin@swiftdrop.local", false},
		{"With whitespace", "  customer@swiftdrop.local  ", "customer@swiftdrop.local", false},
		{"Missing @", "customerswiftdrop.local", "", true},
		{"Missing domain", "customer@", "", true},
		{"Missing username", "@swiftdrop.local", "", true},
		{"Empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emailVO, err := domain.NewEmail(tt.input)
			if (err != nil) != tt.expectError {
				t.Fatalf("NewEmail(%q) error = %v; want error: %v", tt.input, err, tt.expectError)
			}
			if !tt.expectError && emailVO.String() != tt.expected {
				t.Errorf("NewEmail(%q) = %q; want %q", tt.input, emailVO.String(), tt.expected)
			}
		})
	}
}
