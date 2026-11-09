package main

import (
	"database/sql"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"testing"

	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
)

type mockTokenProvider struct{}

func (mockTokenProvider) GenerateAccessToken(principal identityapp.Principal) (string, error) {
	return "mock.token", nil
}

func (mockTokenProvider) ValidateAccessToken(tokenString string) (*identityapp.Principal, error) {
	return &identityapp.Principal{UserID: "usr_123"}, nil
}

func TestRegisterMerchantsModule_Validation(t *testing.T) {
	origEnv := os.Getenv("APP_ENV")
	origPepper := os.Getenv("MERCHANT_API_KEY_PEPPER")
	origKey := os.Getenv("WEBHOOK_SECRET_ENCRYPTION_KEY")
	defer func() {
		_ = os.Setenv("APP_ENV", origEnv)
		_ = os.Setenv("MERCHANT_API_KEY_PEPPER", origPepper)
		_ = os.Setenv("WEBHOOK_SECRET_ENCRYPTION_KEY", origKey)
	}()

	mux := http.NewServeMux()
	tokenProv := mockTokenProvider{}
	db := &sql.DB{} // dummy pointer to satisfy non-nil check

	t.Run("Missing DB fails cleanly", func(t *testing.T) {
		err := registerMerchantsModule(mux, nil, tokenProv)
		if err == nil || !strings.Contains(err.Error(), "PostgreSQL connection") {
			t.Fatalf("expected PostgreSQL error, got %v", err)
		}
	})

	t.Run("Production requires MERCHANT_API_KEY_PEPPER", func(t *testing.T) {
		_ = os.Setenv("APP_ENV", "production")
		_ = os.Unsetenv("MERCHANT_API_KEY_PEPPER")
		err := registerMerchantsModule(mux, db, tokenProv)
		if err == nil || !strings.Contains(err.Error(), "MERCHANT_API_KEY_PEPPER is missing") {
			t.Fatalf("expected missing pepper error in production, got %v", err)
		}
	})

	t.Run("Pepper shorter than 32 bytes is rejected", func(t *testing.T) {
		_ = os.Setenv("APP_ENV", "production")
		_ = os.Setenv("MERCHANT_API_KEY_PEPPER", "too_short_pepper")
		err := registerMerchantsModule(mux, db, tokenProv)
		if err == nil || !strings.Contains(err.Error(), "must be at least 32 bytes") {
			t.Fatalf("expected short pepper error, got %v", err)
		}
	})

	t.Run("Production requires WEBHOOK_SECRET_ENCRYPTION_KEY", func(t *testing.T) {
		_ = os.Setenv("APP_ENV", "production")
		_ = os.Setenv("MERCHANT_API_KEY_PEPPER", "valid_pepper_with_at_least_32_characters_length!")
		_ = os.Unsetenv("WEBHOOK_SECRET_ENCRYPTION_KEY")
		err := registerMerchantsModule(mux, db, tokenProv)
		if err == nil || !strings.Contains(err.Error(), "WEBHOOK_SECRET_ENCRYPTION_KEY is missing") {
			t.Fatalf("expected missing encryption key error in production, got %v", err)
		}
	})

	t.Run("Invalid base64 encryption key is rejected", func(t *testing.T) {
		_ = os.Setenv("APP_ENV", "production")
		_ = os.Setenv("MERCHANT_API_KEY_PEPPER", "valid_pepper_with_at_least_32_characters_length!")
		_ = os.Setenv("WEBHOOK_SECRET_ENCRYPTION_KEY", "not-valid-base64!@@")
		err := registerMerchantsModule(mux, db, tokenProv)
		if err == nil || !strings.Contains(err.Error(), "must be a valid base64-encoded string") {
			t.Fatalf("expected invalid base64 error, got %v", err)
		}
	})

	t.Run("Encryption key of length != 32 bytes is rejected", func(t *testing.T) {
		_ = os.Setenv("APP_ENV", "production")
		_ = os.Setenv("MERCHANT_API_KEY_PEPPER", "valid_pepper_with_at_least_32_characters_length!")
		// 16 bytes encoded in base64
		_ = os.Setenv("WEBHOOK_SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("1234567890123456")))
		err := registerMerchantsModule(mux, db, tokenProv)
		if err == nil || !strings.Contains(err.Error(), "must be exactly 32 bytes") {
			t.Fatalf("expected 32-byte requirement error, got %v", err)
		}
	})

	t.Run("Valid production configuration succeeds", func(t *testing.T) {
		_ = os.Setenv("APP_ENV", "production")
		_ = os.Setenv("MERCHANT_API_KEY_PEPPER", "valid_pepper_with_at_least_32_characters_length!")
		valid32 := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
		_ = os.Setenv("WEBHOOK_SECRET_ENCRYPTION_KEY", valid32)

		err := registerMerchantsModule(http.NewServeMux(), db, tokenProv)
		if err != nil {
			t.Fatalf("expected clean success, got error: %v", err)
		}
	})
}
