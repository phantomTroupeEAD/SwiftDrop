package infrastructure

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

func TestJWTTokenProvider(t *testing.T) {
	provider := NewJWTTokenProvider("test-secret-key-at-least-32-chars-long")

	t.Run("generates and validates token for customer", func(t *testing.T) {
		principal := application.Principal{
			UserID: "usr_cust_1",
			Email:  "customer@swiftdrop.local",
			Role:   domain.RoleCustomer,
		}

		token, err := provider.GenerateAccessToken(principal)
		if err != nil {
			t.Fatalf("GenerateAccessToken failed: %v", err)
		}

		got, err := provider.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("ValidateAccessToken failed: %v", err)
		}

		if got.UserID != principal.UserID || got.Email != principal.Email || got.Role != principal.Role {
			t.Errorf("unexpected principal: %+v", got)
		}
		if got.TenantID != nil {
			t.Errorf("expected nil TenantID, got %v", *got.TenantID)
		}
	})

	t.Run("preserves tenant_id claim for merchant", func(t *testing.T) {
		tenant := "mrc_01ABC"
		principal := application.Principal{
			UserID:   "usr_merch_1",
			Email:    "merchant@vendor.com",
			Role:     domain.RoleMerchant,
			TenantID: &tenant,
		}

		token, err := provider.GenerateAccessToken(principal)
		if err != nil {
			t.Fatalf("GenerateAccessToken failed: %v", err)
		}

		got, err := provider.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("ValidateAccessToken failed: %v", err)
		}

		if got.TenantID == nil || *got.TenantID != tenant {
			t.Errorf("expected TenantID %q, got %v", tenant, got.TenantID)
		}
	})

	t.Run("rejects malformed and tampered tokens", func(t *testing.T) {
		validToken, err := provider.GenerateAccessToken(application.Principal{
			UserID: "usr_1",
			Email:  "a@b.com",
			Role:   domain.RoleAdmin,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		parts := strings.Split(validToken, ".")
		tamperedPayload := parts[0] + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"hacker","role":"ADMIN"}`)) + "." + parts[2]
		otherProvider := NewJWTTokenProvider("different-secret-key-at-least-32-chars")

		cases := []struct {
			name  string
			token string
		}{
			{"empty", ""},
			{"two parts", "abc.def"},
			{"invalid signature base64", parts[0] + "." + parts[1] + ".!!!"},
			{"tampered payload", tamperedPayload},
			{"signed by another secret", func() string {
				tok, _ := otherProvider.GenerateAccessToken(application.Principal{UserID: "usr_1", Email: "a@b.com", Role: domain.RoleAdmin})
				return tok
			}()},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := provider.ValidateAccessToken(tc.token); err == nil {
					t.Errorf("expected error for %s, got nil", tc.name)
				}
			})
		}
	})

	t.Run("rejects expired token with ErrTokenExpired", func(t *testing.T) {
		expiredProvider := &JWTTokenProvider{
			secretKey: []byte("test-secret-key-at-least-32-chars-long"),
			issuer:    "https://api.swiftdrop.io",
			ttl:       -5 * time.Minute,
		}
		tok, err := expiredProvider.GenerateAccessToken(application.Principal{
			UserID: "usr_exp",
			Email:  "exp@swiftdrop.local",
			Role:   domain.RoleCustomer,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := provider.ValidateAccessToken(tok); err != domain.ErrTokenExpired {
			t.Errorf("expected ErrTokenExpired, got %v", err)
		}
	})

	t.Run("rejects unsupported role in claims", func(t *testing.T) {
		claims := JWTClaims{
			Issuer:    provider.issuer,
			Subject:   "usr_bad_role",
			Role:      domain.Role("GHOST"),
			Email:     "ghost@swiftdrop.local",
			IssuedAt:  time.Now().UTC().Unix(),
			ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Unix(),
		}
		headerB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
		claimsBytes, _ := json.Marshal(claims)
		claimsB64 := base64.RawURLEncoding.EncodeToString(claimsBytes)
		signingInput := headerB64 + "." + claimsB64
		sigB64 := base64.RawURLEncoding.EncodeToString(provider.sign([]byte(signingInput)))

		if _, err := provider.ValidateAccessToken(signingInput + "." + sigB64); err == nil {
			t.Errorf("expected error for unsupported role, got nil")
		}
	})

	t.Run("empty secret falls back to development default", func(t *testing.T) {
		p := NewJWTTokenProvider("")
		tok, err := p.GenerateAccessToken(application.Principal{UserID: "u1", Email: "u1@test.com", Role: domain.RoleCourier})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := p.ValidateAccessToken(tok); err != nil {
			t.Fatalf("expected fallback secret to validate own token: %v", err)
		}
	})
}
