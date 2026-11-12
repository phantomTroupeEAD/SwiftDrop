package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/testutil"
)

const testSecret = "test-secret-key-at-least-32-bytes-long!!"
const testIssuer = "https://api.swiftdrop.io"

func TestJWTVerifier_VerifyToken(t *testing.T) {
	fixedNow := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	verifier := auth.NewJWTVerifier(testSecret, testIssuer)
	verifier.SetNowFunc(func() time.Time { return fixedNow })

	t.Run("Valid customer token passes verification", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			Email:     "customer@example.com",
			IssuedAt:  fixedNow.Add(-5 * time.Minute).Unix(),
			ExpiresAt: fixedNow.Add(10 * time.Minute).Unix(),
			JWTID:     "jwt_test_1",
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		principal, err := verifier.VerifyToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if principal.UserID != "usr_customer_123" {
			t.Errorf("expected UserID usr_customer_123, got %s", principal.UserID)
		}
		if principal.Role != auth.RoleCustomer {
			t.Errorf("expected Role CUSTOMER, got %s", principal.Role)
		}
		if principal.TenantID != nil {
			t.Errorf("expected nil TenantID for customer, got %v", *principal.TenantID)
		}
	})

	t.Run("Valid merchant token passes verification with TenantID", func(t *testing.T) {
		tenantID := "mrc_merchant_456"
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_merchant_owner",
			Role:      auth.RoleMerchant,
			TenantID:  &tenantID,
			Email:     "merchant@example.com",
			IssuedAt:  fixedNow.Add(-5 * time.Minute).Unix(),
			ExpiresAt: fixedNow.Add(10 * time.Minute).Unix(),
			JWTID:     "jwt_test_2",
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		principal, err := verifier.VerifyToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if principal.UserID != "usr_merchant_owner" {
			t.Errorf("expected UserID usr_merchant_owner, got %s", principal.UserID)
		}
		if principal.Role != auth.RoleMerchant {
			t.Errorf("expected Role MERCHANT, got %s", principal.Role)
		}
		if principal.TenantID == nil || *principal.TenantID != "mrc_merchant_456" {
			t.Errorf("expected TenantID mrc_merchant_456, got %v", principal.TenantID)
		}
	})

	t.Run("Expired token is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			IssuedAt:  fixedNow.Add(-30 * time.Minute).Unix(),
			ExpiresAt: fixedNow.Add(-5 * time.Minute).Unix(), // expired 5 mins ago
			JWTID:     "jwt_test_expired",
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for expired token, got nil")
		}
		if !errors.Is(err, auth.ErrTokenExpired) {
			t.Errorf("expected ErrTokenExpired, got %v", err)
		}
	})

	t.Run("Invalid signature is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken("wrong-secret-key-that-does-not-match-at-all", auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			IssuedAt:  fixedNow.Unix(),
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
			JWTID:     "jwt_test_wrong_sig",
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for invalid signature, got nil")
		}
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Unsupported algorithm 'none' is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		}, "none")
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for 'none' algorithm, got nil")
		}
		if !errors.Is(err, auth.ErrUnsupportedAlgorithm) {
			t.Errorf("expected ErrUnsupportedAlgorithm, got %v", err)
		}
	})

	t.Run("Unsupported algorithm 'RS256' is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		}, "RS256")
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for 'RS256' algorithm, got nil")
		}
		if !errors.Is(err, auth.ErrUnsupportedAlgorithm) {
			t.Errorf("expected ErrUnsupportedAlgorithm, got %v", err)
		}
	})

	t.Run("Algorithm case sensitivity: 'hs256' is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		}, "hs256")
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for lowercase 'hs256' algorithm, got nil")
		}
		if !errors.Is(err, auth.ErrUnsupportedAlgorithm) {
			t.Errorf("expected ErrUnsupportedAlgorithm, got %v", err)
		}
	})

	t.Run("Incorrect issuer is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    "https://evil.untrusted-issuer.com",
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for issuer mismatch, got nil")
		}
		if !errors.Is(err, auth.ErrInvalidIssuer) {
			t.Errorf("expected ErrInvalidIssuer, got %v", err)
		}
	})

	t.Run("Missing issuer claim is rejected when issuer is configured", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    "",
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for missing issuer, got nil")
		}
		if !errors.Is(err, auth.ErrInvalidIssuer) {
			t.Errorf("expected ErrInvalidIssuer, got %v", err)
		}
	})

	t.Run("Token issued in the future is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_customer_123",
			Role:      auth.RoleCustomer,
			IssuedAt:  fixedNow.Add(10 * time.Minute).Unix(), // 600s in the future (> 300s clock skew)
			ExpiresAt: fixedNow.Add(20 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for future-issued token, got nil")
		}
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Missing subject claim is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for empty subject, got nil")
		}
		if !errors.Is(err, auth.ErrInvalidClaims) {
			t.Errorf("expected ErrInvalidClaims, got %v", err)
		}
	})

	t.Run("Unsupported or invalid role is rejected", func(t *testing.T) {
		token, err := testutil.GenerateTestToken(testSecret, auth.JWTClaims{
			Issuer:    testIssuer,
			Subject:   "usr_hacker",
			Role:      auth.Role("SUPERUSER"),
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = verifier.VerifyToken(token)
		if err == nil {
			t.Fatal("expected error for invalid role, got nil")
		}
		if !errors.Is(err, auth.ErrInvalidClaims) {
			t.Errorf("expected ErrInvalidClaims, got %v", err)
		}
	})

	t.Run("Empty or malformed token string is rejected", func(t *testing.T) {
		malformedCases := []string{
			"",
			"   ",
			"not-a-jwt",
			"header.payload",
			"header.payload.sig.extra",
			"invalid-base64.invalid-base64.invalid-base64",
		}
		for _, tc := range malformedCases {
			_, err := verifier.VerifyToken(tc)
			if err == nil {
				t.Errorf("expected error for token %q, got nil", tc)
			}
		}
	})

	t.Run("NewJWTVerifier panics with empty secret", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Error("expected panic for empty JWT secret, but none occurred")
			}
		}()
		_ = auth.NewJWTVerifier("", testIssuer)
	})
}
