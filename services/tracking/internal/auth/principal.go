package auth

import "strings"

// Role represents an authoritative authorization role within SwiftDrop (aligned with Member 1).
type Role string

const (
	RoleAdmin      Role = "ADMIN"
	RoleDispatcher Role = "DISPATCHER"
	RoleCourier    Role = "COURIER"
	RoleMerchant   Role = "MERCHANT"
	RoleCustomer   Role = "CUSTOMER"
)

// AllRoles lists the canonical supported roles.
var AllRoles = []Role{
	RoleAdmin,
	RoleDispatcher,
	RoleCourier,
	RoleMerchant,
	RoleCustomer,
}

// IsValidRole validates whether a role matches one of the canonical supported roles.
func IsValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleDispatcher, RoleCourier, RoleMerchant, RoleCustomer:
		return true
	default:
		return false
	}
}

// ParseRole parses and normalizes a role string.
func ParseRole(s string) (Role, bool) {
	candidate := Role(strings.ToUpper(strings.TrimSpace(s)))
	if !IsValidRole(candidate) {
		return "", false
	}
	return candidate, true
}

// Principal represents the authenticated security context of an actor extracted from a valid JWT.
type Principal struct {
	UserID   string  `json:"user_id"`
	Email    string  `json:"email"`
	Role     Role    `json:"role"`
	TenantID *string `json:"tenant_id,omitempty"`
}
