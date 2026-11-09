package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/merchants/application"
	"github.com/swiftdrop/core-api/internal/merchants/domain"
	"github.com/swiftdrop/core-api/internal/merchants/infrastructure"
	"github.com/swiftdrop/core-api/internal/merchants/transport"
	"github.com/swiftdrop/core-api/internal/platform/config"
)

// registerMerchantsModule wires Member 04 Merchant Integration operations into the router.
// Returns an error if dependencies or cryptographic requirements fail validation.
func registerMerchantsModule(mux *http.ServeMux, db *sql.DB, tokenProv identityapp.TokenProvider) error {
	if mux == nil {
		return errors.New("merchants: http.ServeMux is required")
	}
	if db == nil {
		return errors.New("merchants: PostgreSQL connection (*sql.DB) is required")
	}
	if tokenProv == nil {
		return errors.New("merchants: identity TokenProvider is required")
	}

	// 1. API Key Pepper Configuration
	pepperStr := os.Getenv("MERCHANT_API_KEY_PEPPER")
	if pepperStr == "" {
		if config.IsProduction() {
			return errors.New("merchants: MERCHANT_API_KEY_PEPPER is missing (required in production)")
		}
		// Local development only fallback with >= 32 bytes entropy
		pepperStr = "dev_local_testing_pepper_at_least_32_bytes_long!"
	}
	pepper := []byte(pepperStr)
	if len(pepper) < 32 {
		return errors.New("merchants: MERCHANT_API_KEY_PEPPER must be at least 32 bytes")
	}

	// 2. Webhook Secret Encryption Key (Base64-encoded 32-byte AES-256 key)
	encKeyB64 := os.Getenv("WEBHOOK_SECRET_ENCRYPTION_KEY")
	var encKey []byte
	if encKeyB64 == "" {
		if config.IsProduction() {
			return errors.New("merchants: WEBHOOK_SECRET_ENCRYPTION_KEY is missing (required in production)")
		}
		// 32-byte deterministic key for local development
		encKey = []byte("01234567890123456789012345678901")
	} else {
		decoded, err := base64.StdEncoding.DecodeString(encKeyB64)
		if err != nil {
			return fmt.Errorf("merchants: WEBHOOK_SECRET_ENCRYPTION_KEY must be a valid base64-encoded string: %w", err)
		}
		if len(decoded) != 32 {
			return fmt.Errorf("merchants: WEBHOOK_SECRET_ENCRYPTION_KEY decoded length is %d bytes, must be exactly 32 bytes (AES-256)", len(decoded))
		}
		encKey = decoded
	}

	// 3. Environment-aware API key prefix
	keyEnvPrefix := domain.EnvPrefixLive
	if !config.IsProduction() {
		if env := os.Getenv("MERCHANT_KEY_ENV_PREFIX"); env != "" {
			keyEnvPrefix = env
		} else if config.IsLocal() || config.AppEnv() == "test" {
			keyEnvPrefix = domain.EnvPrefixTest
		}
	}

	// 4. Application Service construction
	svc := application.NewService(application.Deps{
		Tx:        &sqlTxRunner{db: db},
		Merchants: infrastructure.MerchantRepo{},
		Creds:     infrastructure.CredentialRepo{},
		Hooks:     infrastructure.WebhookRepo{},
		Audit:     infrastructure.AuditWriter{},
		Roles:     infrastructure.RoleAssigner{},
		KeyGen:    infrastructure.KeyGenerator{},
		Hasher:    infrastructure.Hasher{Peppers: map[int][]byte{1: pepper}, Current: 1},
		Cipher:    infrastructure.Cipher{Key: encKey, Version: 1},
		Resolver:  infrastructure.Resolver{},
		Booking:   infrastructure.NewBookingStub(),
		Observer:  noopObserver{},
		Now:       time.Now,
	}, application.Config{
		KeyEnvPrefix:  keyEnvPrefix,
		MaxActiveKeys: 5,
		MaxWebhooks:   10,
		AllowHTTP:     !config.IsProduction(),
	})

	h := &transport.Handler{
		Svc:      svc,
		Verifier: &jwtVerifierAdapter{prov: tokenProv},
		Log:      slog.Default(),
	}
	h.Register(mux)
	return nil
}

// sqlTxRunner wraps *sql.DB to satisfy application.TxRunner.
type sqlTxRunner struct {
	db *sql.DB
}

func (r *sqlTxRunner) WithinTx(ctx context.Context, fn func(context.Context, application.Querier) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *sqlTxRunner) Reader() application.Querier {
	return r.db
}

// jwtVerifierAdapter adapts Member 1's TokenProvider to transport.TokenVerifier.
type jwtVerifierAdapter struct {
	prov identityapp.TokenProvider
}

func (a *jwtVerifierAdapter) Verify(token string) (transport.Claims, error) {
	principal, err := a.prov.ValidateAccessToken(token)
	if err != nil {
		return transport.Claims{}, err
	}
	tenantID := ""
	if principal.TenantID != nil {
		tenantID = *principal.TenantID
	}
	return transport.Claims{
		Sub:      principal.UserID,
		Role:     string(principal.Role),
		TenantID: tenantID,
	}, nil
}

// noopObserver satisfies application.Observer.
type noopObserver struct{}

func (noopObserver) AuthResult(_ string)    {}
func (noopObserver) SSRFBlocked()           {}
func (noopObserver) StatusChanged(_ string) {}
