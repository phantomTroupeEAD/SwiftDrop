package domain

import (
	"time"
)

// UserStatus represents account lifecycle states.
type UserStatus string

const (
	StatusActive    UserStatus = "ACTIVE"
	StatusSuspended UserStatus = "SUSPENDED"
)

// User represents the authoritative Identity Aggregate Root in the system.
type User struct {
	ID           string
	Email        Email
	PasswordHash string
	DisplayName  string
	Phone        string
	Role         Role
	TenantID     *string
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser constructs a new User entity.
func NewUser(
	id string,
	email Email,
	passwordHash string,
	displayName string,
	phone string,
	role Role,
	tenantID *string,
	status UserStatus,
	createdAt time.Time,
	updatedAt time.Time,
) *User {
	if status == "" {
		status = StatusActive
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	return &User{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		Phone:        phone,
		Role:         role,
		TenantID:     tenantID,
		Status:       status,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

// IsActive checks if the user account is allowed to log in and access protected workflows.
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// Suspend transitions the account into SUSPENDED status.
func (u *User) Suspend() {
	u.Status = StatusSuspended
	u.UpdatedAt = time.Now().UTC()
}

// Reactivate restores the account to ACTIVE status.
func (u *User) Reactivate() {
	u.Status = StatusActive
	u.UpdatedAt = time.Now().UTC()
}

// PromoteRole updates the user's role.
func (u *User) PromoteRole(newRole Role) error {
	if !IsValidRole(newRole) {
		return ErrInvalidRole
	}
	u.Role = newRole
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// ChangePassword updates the user's hashed password.
func (u *User) ChangePassword(newHash string) {
	u.PasswordHash = newHash
	u.UpdatedAt = time.Now().UTC()
}
