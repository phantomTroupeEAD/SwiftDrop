package domain

import "errors"

// Standard domain errors for the identity and access management bounded context.
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrEmailAlreadyExists  = errors.New("email is already registered")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrAccountSuspended    = errors.New("account is suspended")
	ErrAccountInactive     = errors.New("account is inactive")
	ErrTokenExpired        = errors.New("token has expired")
	ErrTokenRevoked        = errors.New("token has been revoked")
	ErrTokenReused         = errors.New("token reuse detected")
	ErrInvalidToken        = errors.New("invalid authentication token")
	ErrInvalidEmail        = errors.New("invalid email format")
	ErrWeakPassword        = errors.New("password does not meet security requirements")
	ErrInvalidRole         = errors.New("unrecognized or invalid user role")
	ErrUnauthorized        = errors.New("authentication credentials required")
	ErrForbidden           = errors.New("access denied for this role")
	ErrTenantRequired      = errors.New("merchant tenant context is required")
	ErrConcurrentConflict  = errors.New("concurrent update conflict")
)
