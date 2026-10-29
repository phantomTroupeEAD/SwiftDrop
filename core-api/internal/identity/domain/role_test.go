package domain_test

import (
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		name  string
		role  domain.Role
		valid bool
	}{
		{"Admin role", domain.RoleAdmin, true},
		{"Dispatcher role", domain.RoleDispatcher, true},
		{"Courier role", domain.RoleCourier, true},
		{"Merchant role", domain.RoleMerchant, true},
		{"Customer role", domain.RoleCustomer, true},
		{"Lowercase admin is valid when parsed", "admin", false}, // strict check on raw Role
		{"Unknown role", "SUPERUSER", false},
		{"Empty role", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.IsValidRole(tt.role)
			if got != tt.valid {
				t.Errorf("IsValidRole(%q) = %v; want %v", tt.role, got, tt.valid)
			}
		})
	}
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		input       string
		expected    domain.Role
		expectError bool
	}{
		{"admin", domain.RoleAdmin, false},
		{"ADMIN", domain.RoleAdmin, false},
		{" Courier ", domain.RoleCourier, false},
		{"dispatcher", domain.RoleDispatcher, false},
		{"merchant", domain.RoleMerchant, false},
		{"customer", domain.RoleCustomer, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r, err := domain.ParseRole(tt.input)
			if (err != nil) != tt.expectError {
				t.Fatalf("ParseRole(%q) error = %v; want error: %v", tt.input, err, tt.expectError)
			}
			if !tt.expectError && r != tt.expected {
				t.Errorf("ParseRole(%q) = %v; want %v", tt.input, r, tt.expected)
			}
		})
	}
}
