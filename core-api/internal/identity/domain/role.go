package domain

import "strings"

// Role represents an authoritative authorization role within SwiftDrop.
type Role string

const (
	RoleAdmin      Role = "ADMIN"
	RoleDispatcher Role = "DISPATCHER"
	RoleCourier    Role = "COURIER"
	RoleMerchant   Role = "MERCHANT"
	RoleCustomer   Role = "CUSTOMER"
)

// AllRoles provides the canonical list of valid roles.
var AllRoles = []Role{
	RoleAdmin,
	RoleDispatcher,
	RoleCourier,
	RoleMerchant,
	RoleCustomer,
}

// IsValidRole validates whether a given role matches an exact supported role constant.
func IsValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleDispatcher, RoleCourier, RoleMerchant, RoleCustomer:
		return true
	default:
		return false
	}
}

// ParseRole parses a string into a canonical Role (case-insensitive with trimming).
func ParseRole(s string) (Role, error) {
	candidate := Role(strings.ToUpper(strings.TrimSpace(s)))
	if !IsValidRole(candidate) {
		return "", ErrInvalidRole
	}
	return candidate, nil
}
