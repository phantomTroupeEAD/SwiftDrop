package infrastructure

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// JWTClaims represents the authoritative payload of a SwiftDrop Access Token (ADR-003).
type JWTClaims struct {
	Issuer    string      `json:"iss"`
	Subject   string      `json:"sub"`
	Role      domain.Role `json:"role"`
	TenantID  *string     `json:"tenant_id,omitempty"`
	Email     string      `json:"email"`
	IssuedAt  int64       `json:"iat"`
	ExpiresAt int64       `json:"exp"`
	JWTID     string      `json:"jti"`
}

// JWTTokenProvider implements application.TokenProvider using HMAC-SHA256.
type JWTTokenProvider struct {
	secretKey []byte
	issuer    string
	ttl       time.Duration
}

// NewJWTTokenProvider constructs a token provider with secret and configuration.
func NewJWTTokenProvider(secret string) *JWTTokenProvider {
	if secret == "" {
		secret = "swiftdrop-development-secret-key-change-in-production-32chars"
	}
	return &JWTTokenProvider{
		secretKey: []byte(secret),
		issuer:    "https://api.swiftdrop.io",
		ttl:       15 * time.Minute,
	}
}

// GenerateAccessToken signs a new JWT access token with required claims.
func (p *JWTTokenProvider) GenerateAccessToken(principal application.Principal) (string, error) {
	now := time.Now().UTC()
	claims := JWTClaims{
		Issuer:    p.issuer,
		Subject:   principal.UserID,
		Role:      principal.Role,
		TenantID:  principal.TenantID,
		Email:     principal.Email,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(p.ttl).Unix(),
		JWTID:     "jwt_" + randomHex(12),
	}

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	signature := p.sign([]byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + sigB64, nil
}

// ValidateAccessToken parses, verifies signature, and checks expiry on a JWT string.
func (p *JWTTokenProvider) ValidateAccessToken(tokenString string) (*application.Principal, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, domain.ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := p.sign([]byte(signingInput))
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, domain.ErrInvalidToken
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, domain.ErrInvalidToken
	}

	now := time.Now().UTC().Unix()
	if claims.ExpiresAt < now {
		return nil, domain.ErrTokenExpired
	}

	if !domain.IsValidRole(claims.Role) {
		return nil, errors.New("unsupported role in token")
	}

	return &application.Principal{
		UserID:   claims.Subject,
		Email:    claims.Email,
		Role:     claims.Role,
		TenantID: claims.TenantID,
	}, nil
}

func (p *JWTTokenProvider) sign(data []byte) []byte {
	mac := hmac.New(sha256.New, p.secretKey)
	mac.Write(data)
	return mac.Sum(nil)
}

func randomHex(bytes int) string {
	b := make([]byte, bytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
