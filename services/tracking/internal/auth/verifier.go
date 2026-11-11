package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrMissingToken indicates that no token was provided in the request.
	ErrMissingToken = errors.New("missing access token")

	// ErrInvalidToken indicates a malformed token or signature verification failure.
	ErrInvalidToken = errors.New("invalid access token")

	// ErrTokenExpired indicates that the token's expiration timestamp has passed.
	ErrTokenExpired = errors.New("access token has expired")

	// ErrUnsupportedAlgorithm indicates that the token was signed with an unexpected algorithm.
	ErrUnsupportedAlgorithm = errors.New("unsupported signing algorithm")

	// ErrInvalidIssuer indicates an issuer mismatch against the authoritative issuer.
	ErrInvalidIssuer = errors.New("invalid token issuer")

	// ErrInvalidClaims indicates that required claims (sub, role) are missing or invalid.
	ErrInvalidClaims = errors.New("invalid token claims")
)

// JWTClaims mirrors Member 1's authoritative JWT claims structure (ADR-003).
type JWTClaims struct {
	Issuer    string  `json:"iss,omitempty"`
	Subject   string  `json:"sub"`
	Role      Role    `json:"role"`
	TenantID  *string `json:"tenant_id,omitempty"`
	Email     string  `json:"email,omitempty"`
	IssuedAt  int64   `json:"iat,omitempty"`
	ExpiresAt int64   `json:"exp"`
	JWTID     string  `json:"jti,omitempty"`
}

// TokenVerifier defines the interface for validating Bearer tokens and returning Principals.
type TokenVerifier interface {
	VerifyToken(tokenString string) (*Principal, error)
}

// JWTVerifier implements TokenVerifier using HMAC-SHA256 (HS256) aligned with Member 1.
type JWTVerifier struct {
	secretKey      []byte
	expectedIssuer string
	nowFunc        func() time.Time
}

// NewJWTVerifier constructs a configured JWTVerifier.
// Aligned with Member 1: uses HMAC-SHA256 with the shared secret key.
func NewJWTVerifier(secret string, expectedIssuer string) *JWTVerifier {
	if strings.TrimSpace(secret) == "" {
		panic("auth: JWT secret cannot be empty")
	}
	return &JWTVerifier{
		secretKey:      []byte(secret),
		expectedIssuer: expectedIssuer,
		nowFunc:        time.Now,
	}
}

// SetNowFunc overrides the internal clock (primarily for deterministic unit testing).
func (v *JWTVerifier) SetNowFunc(fn func() time.Time) {
	if fn != nil {
		v.nowFunc = fn
	}
}

// VerifyToken parses, verifies the cryptographic signature, and checks the expiration of a JWT access token.
func (v *JWTVerifier) VerifyToken(tokenString string) (*Principal, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, ErrMissingToken
	}

	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// 1. Decode and validate header
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrInvalidToken
	}

	// Strictly enforce exact HS256 algorithm per Member 1 implementation
	if header.Alg != "HS256" {
		return nil, fmt.Errorf("%w: expected HS256, got %s", ErrUnsupportedAlgorithm, header.Alg)
	}

	// 2. Cryptographic signature verification using HMAC-SHA256
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, v.secretKey)
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, ErrInvalidToken
	}

	// 3. Decode and parse claims payload
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	// 4. Verify expiration timestamp
	now := v.nowFunc().UTC().Unix()
	if claims.ExpiresAt <= 0 || claims.ExpiresAt < now {
		return nil, ErrTokenExpired
	}

	// Verify issued-at timestamp is not in the future (allowing 300s clock skew)
	if claims.IssuedAt > 0 && claims.IssuedAt > now+300 {
		return nil, fmt.Errorf("%w: token issued in the future", ErrInvalidToken)
	}

	// 5. Verify issuer (if configured, must match strictly)
	if v.expectedIssuer != "" && claims.Issuer != v.expectedIssuer {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrInvalidIssuer, v.expectedIssuer, claims.Issuer)
	}

	// 6. Verify required claims
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, fmt.Errorf("%w: subject (sub) cannot be empty", ErrInvalidClaims)
	}

	if !IsValidRole(claims.Role) {
		return nil, fmt.Errorf("%w: unsupported role %q", ErrInvalidClaims, claims.Role)
	}

	return &Principal{
		UserID:   claims.Subject,
		Email:    claims.Email,
		Role:     claims.Role,
		TenantID: claims.TenantID,
	}, nil
}
