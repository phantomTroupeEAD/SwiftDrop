package application_test

import (
	"context"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
)

func TestUserService_AssignRole(t *testing.T) {
	ctx := context.Background()
	hasher := infrastructure.NewStandardPasswordHasher()
	userRepo := infrastructure.NewMockUserRepository(hasher)
	auditWriter := infrastructure.NewMockAuditWriter()
	svc := application.NewUserService(userRepo, auditWriter)

	t.Run("AssignRole successfully promotes user to MERCHANT with tenant_id", func(t *testing.T) {
		// customer@swiftdrop.local is preloaded
		customer, err := userRepo.FindByEmail(ctx, "customer@swiftdrop.local")
		if err != nil {
			t.Fatalf("Failed to find customer: %v", err)
		}

		merchantTenant := "mrc_test_tenant_123"
		err = svc.AssignRole(ctx, customer.ID, domain.RoleMerchant, &merchantTenant)
		if err != nil {
			t.Fatalf("AssignRole failed: %v", err)
		}

		// Reload user and verify
		updated, err := userRepo.FindByID(ctx, customer.ID)
		if err != nil {
			t.Fatalf("Failed to reload user: %v", err)
		}
		if updated.Role != domain.RoleMerchant {
			t.Errorf("Expected role MERCHANT, got %s", updated.Role)
		}
		if updated.TenantID == nil || *updated.TenantID != merchantTenant {
			t.Errorf("Expected tenant %s, got %v", merchantTenant, updated.TenantID)
		}

		// Verify audit log entry was recorded
		if len(auditWriter.Entries) == 0 {
			t.Error("Expected audit log entry to be written")
		} else {
			last := auditWriter.Entries[len(auditWriter.Entries)-1]
			if last.Action != "USER_ROLE_ASSIGNED" || last.EntityID != customer.ID {
				t.Errorf("Unexpected audit log entry: %+v", last)
			}
		}
	})

	t.Run("UpdateUserRole fails when caller is not Admin", func(t *testing.T) {
		customerActor := application.Principal{
			UserID: "usr_cust",
			Email:  "customer@swiftdrop.local",
			Role:   domain.RoleCustomer,
		}
		err := svc.UpdateUserRole(ctx, customerActor, "usr_target", domain.RoleCourier, nil)
		if err != domain.ErrForbidden {
			t.Errorf("Expected ErrForbidden, got %v", err)
		}
	})

	t.Run("UpdateUserRole succeeds when caller is Admin", func(t *testing.T) {
		adminActor := application.Principal{
			UserID: "usr_admin",
			Email:  "admin@swiftdrop.local",
			Role:   domain.RoleAdmin,
		}
		targetUser, _ := userRepo.FindByEmail(ctx, "courier@swiftdrop.local")

		err := svc.UpdateUserRole(ctx, adminActor, targetUser.ID, domain.RoleDispatcher, nil)
		if err != nil {
			t.Fatalf("UpdateUserRole failed: %v", err)
		}

		reloaded, _ := userRepo.FindByID(ctx, targetUser.ID)
		if reloaded.Role != domain.RoleDispatcher {
			t.Errorf("Expected role DISPATCHER, got %s", reloaded.Role)
		}
	})
}
