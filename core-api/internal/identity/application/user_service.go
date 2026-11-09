package application

import (
	"context"
	"fmt"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// UserService manages user profile administration and role assignments.
type UserService struct {
	userRepo    UserRepository
	auditWriter AuditWriter
}

// NewUserService constructs a configured UserService.
func NewUserService(userRepo UserRepository, auditWriter AuditWriter) *UserService {
	return &UserService{
		userRepo:    userRepo,
		auditWriter: auditWriter,
	}
}

// AssignRole directly assigns a new role and optional tenant to a user.
// Implements the RoleAssigner port interface used by Member 4 (Merchant Integration).
func (s *UserService) AssignRole(ctx context.Context, userID string, newRole domain.Role, tenantID *string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return domain.ErrUserNotFound
	}

	if err := user.PromoteRole(newRole); err != nil {
		return err
	}

	if tenantID != nil {
		user.TenantID = tenantID
	}

	if err := s.userRepo.Save(ctx, user); err != nil {
		return fmt.Errorf("failed to save updated role: %w", err)
	}

	// Record audit trail if audit writer is available
	if s.auditWriter != nil {
		meta := map[string]interface{}{
			"new_role": string(newRole),
		}
		if tenantID != nil {
			meta["tenant_id"] = *tenantID
		}
		_ = s.auditWriter.WriteAuditLog(ctx, AuditEntry{
			ActorID:    "system_or_admin",
			ActorRole:  "SYSTEM",
			Action:     "USER_ROLE_ASSIGNED",
			EntityType: "USER",
			EntityID:   userID,
			Metadata:   meta,
		})
	}

	return nil
}

// UpdateUserRole handles administrative role change requests with actor security verification.
func (s *UserService) UpdateUserRole(ctx context.Context, actor Principal, targetUserID string, newRole domain.Role, tenantID *string) error {
	if actor.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	user, err := s.userRepo.FindByID(ctx, targetUserID)
	if err != nil {
		return domain.ErrUserNotFound
	}

	oldRole := user.Role
	if err := user.PromoteRole(newRole); err != nil {
		return err
	}

	if tenantID != nil {
		user.TenantID = tenantID
	}

	if err := s.userRepo.Save(ctx, user); err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}

	if s.auditWriter != nil {
		meta := map[string]interface{}{
			"previous_role": string(oldRole),
			"new_role":      string(newRole),
		}
		if tenantID != nil {
			meta["tenant_id"] = *tenantID
		}
		_ = s.auditWriter.WriteAuditLog(ctx, AuditEntry{
			ActorID:    actor.UserID,
			ActorRole:  string(actor.Role),
			Action:     "USER_ROLE_UPDATED",
			EntityType: "USER",
			EntityID:   targetUserID,
			Metadata:   meta,
		})
	}

	return nil
}

// GetProfile retrieves a user aggregate by ID.
func (s *UserService) GetProfile(ctx context.Context, userID string) (*domain.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

// ListUsers retrieves paginated user records with optional role filtering (Admin only).
func (s *UserService) ListUsers(ctx context.Context, role *domain.Role, limit int, offset int) ([]*domain.User, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.userRepo.ListUsers(ctx, role, limit, offset)
}

// UpdateUserStatus changes a user's account status (ACTIVE / SUSPENDED) with audit logging.
func (s *UserService) UpdateUserStatus(ctx context.Context, actor Principal, targetUserID string, newStatus domain.UserStatus) error {
	if actor.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	user, err := s.userRepo.FindByID(ctx, targetUserID)
	if err != nil {
		return domain.ErrUserNotFound
	}

	oldStatus := user.Status
	if newStatus == domain.StatusSuspended {
		user.Suspend()
	} else if newStatus == domain.StatusActive {
		user.Reactivate()
	} else {
		return fmt.Errorf("invalid user status: %s", newStatus)
	}

	if err := s.userRepo.Save(ctx, user); err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}

	if s.auditWriter != nil {
		_ = s.auditWriter.WriteAuditLog(ctx, AuditEntry{
			ActorID:    actor.UserID,
			ActorRole:  string(actor.Role),
			Action:     "USER_STATUS_UPDATED",
			EntityType: "USER",
			EntityID:   targetUserID,
			Metadata: map[string]interface{}{
				"previous_status": string(oldStatus),
				"new_status":      string(newStatus),
			},
		})
	}

	return nil
}
