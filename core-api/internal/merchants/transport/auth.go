package transport

// TokenVerifier is satisfied by the jwtVerifierAdapter in cmd/api/wiring_merchants.go,
// which adapts Member 1's identity.application.TokenProvider to this interface.
type TokenVerifier interface {
	Verify(token string) (Claims, error)
}

// Claims holds the JWT payload fields that the merchant handlers need.
type Claims struct {
	Sub      string // user ID (subject)
	Role     string // e.g. "ADMIN", "MERCHANT", "COURIER"
	TenantID string // merchant_id when role is MERCHANT; empty otherwise
}
