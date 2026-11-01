// Package adminsec holds the admin module's request guards, safe error mapping,
// settings allowlist and audit redaction (Doc 7 starter code, Member 7).
package adminsec

import "context"

// Roles from ADR-003 section 3.2.
const (
	RoleAdmin      = "ADMIN"
	RoleDispatcher = "DISPATCHER"
	RoleCourier    = "COURIER"
	RoleMerchant   = "MERCHANT"
	RoleCustomer   = "CUSTOMER"
)

// Principal is who is calling. It is filled ONLY by the shared auth middleware
// (Member 1) from a verified token, never from request headers or body.
type Principal struct {
	UserID   string
	Role     string
	TenantID string
}

type ctxKey struct{}

// WithPrincipal is used by the auth middleware and by tests.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// PrincipalFrom returns the caller, or false when the request is not authenticated.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	if !ok || p.UserID == "" || p.Role == "" {
		return Principal{}, false
	}
	return p, true
}
