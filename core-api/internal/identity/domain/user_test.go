package domain

import (
	"testing"
	"time"
)

func TestNewUser(t *testing.T) {
	email, err := NewEmail("alice@swiftdrop.local")
	if err != nil {
		t.Fatalf("unexpected email error: %v", err)
	}

	t.Run("defaults status to ACTIVE and sets timestamps when zero", func(t *testing.T) {
		tenantID := "mrc_123"
		u := NewUser("usr_1", email, "hash123", "Alice", "+94771234567", RoleMerchant, &tenantID, "", time.Time{}, time.Time{})

		if u.Status != StatusActive {
			t.Errorf("expected status %q, got %q", StatusActive, u.Status)
		}
		if !u.IsActive() {
			t.Errorf("expected IsActive() to be true")
		}
		if u.CreatedAt.IsZero() || u.UpdatedAt.IsZero() {
			t.Errorf("expected non-zero CreatedAt and UpdatedAt")
		}
		if u.TenantID == nil || *u.TenantID != "mrc_123" {
			t.Errorf("expected TenantID mrc_123, got %v", u.TenantID)
		}
	})

	t.Run("preserves explicit status and timestamps", func(t *testing.T) {
		created := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
		updated := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
		u := NewUser("usr_2", email, "hash", "Bob", "", RoleCustomer, nil, StatusSuspended, created, updated)

		if u.Status != StatusSuspended {
			t.Errorf("expected status %q, got %q", StatusSuspended, u.Status)
		}
		if u.IsActive() {
			t.Errorf("expected IsActive() to be false for suspended user")
		}
		if !u.CreatedAt.Equal(created) || !u.UpdatedAt.Equal(updated) {
			t.Errorf("expected preserved timestamps")
		}
	})
}

func TestUser_LifecycleMethods(t *testing.T) {
	email, _ := NewEmail("bob@swiftdrop.local")
	u := NewUser("usr_3", email, "old_hash", "Bob", "", RoleCustomer, nil, StatusActive, time.Time{}, time.Time{})

	t.Run("Suspend and Reactivate", func(t *testing.T) {
		u.Suspend()
		if u.Status != StatusSuspended || u.IsActive() {
			t.Errorf("expected user to be suspended")
		}

		u.Reactivate()
		if u.Status != StatusActive || !u.IsActive() {
			t.Errorf("expected user to be reactivated")
		}
	})

	t.Run("PromoteRole valid and invalid", func(t *testing.T) {
		if err := u.PromoteRole(RoleCourier); err != nil {
			t.Fatalf("expected valid role promotion to succeed, got %v", err)
		}
		if u.Role != RoleCourier {
			t.Errorf("expected role %q, got %q", RoleCourier, u.Role)
		}

		if err := u.PromoteRole(Role("SUPERUSER")); err != ErrInvalidRole {
			t.Errorf("expected ErrInvalidRole, got %v", err)
		}
		if u.Role != RoleCourier {
			t.Errorf("expected role to remain %q after failed promotion, got %q", RoleCourier, u.Role)
		}
	})

	t.Run("ChangePassword updates hash", func(t *testing.T) {
		u.ChangePassword("new_hash_abc")
		if u.PasswordHash != "new_hash_abc" {
			t.Errorf("expected updated hash, got %q", u.PasswordHash)
		}
	})

	t.Run("RequiresPhone reflects empty phone", func(t *testing.T) {
		if !u.RequiresPhone() {
			t.Errorf("expected RequiresPhone() true when phone is empty")
		}
		u.Phone = "+94770001111"
		if u.RequiresPhone() {
			t.Errorf("expected RequiresPhone() false when phone is set")
		}
	})
}
