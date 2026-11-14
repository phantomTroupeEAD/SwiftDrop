package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	adminapp "github.com/swiftdrop/core-api/internal/admin/application"
	admininfra "github.com/swiftdrop/core-api/internal/admin/infrastructure"
	admintransport "github.com/swiftdrop/core-api/internal/admin/transport"
	"github.com/swiftdrop/core-api/internal/events"
	eventstransport "github.com/swiftdrop/core-api/internal/events/transport"
	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	merchantsapp "github.com/swiftdrop/core-api/internal/merchants/application"
	merchantsinfra "github.com/swiftdrop/core-api/internal/merchants/infrastructure"
	merchantstransport "github.com/swiftdrop/core-api/internal/merchants/transport"
	parcelsapp "github.com/swiftdrop/core-api/internal/parcels/application"
	parcelsinfra "github.com/swiftdrop/core-api/internal/parcels/infrastructure"
	parcelstransport "github.com/swiftdrop/core-api/internal/parcels/transport"
	pricingapp "github.com/swiftdrop/core-api/internal/pricing/application"
	pricinginfra "github.com/swiftdrop/core-api/internal/pricing/infrastructure"
	pricingtransport "github.com/swiftdrop/core-api/internal/pricing/transport"
)

type swaggerDocument struct {
	Paths map[string]map[string]struct {
		Summary   string                 `json:"summary"`
		Security  []map[string][]string  `json:"security"`
		Responses map[string]interface{} `json:"responses"`
	} `json:"paths"`
}

var pathParamRegex = regexp.MustCompile(`\{[a-zA-Z0-9_-]+\}`)

type testFixedClock struct {
	now time.Time
}

func (c testFixedClock) Now() time.Time {
	return c.now
}

type fakeMerchantReader struct{}

func (f *fakeMerchantReader) FindByID(_ context.Context, merchantID string) (pricingapp.MerchantProfile, error) {
	return pricingapp.MerchantProfile{ID: merchantID, Status: "ACTIVE"}, nil
}

type merchantVerifierAdapter struct {
	prov *identityinfra.JWTTokenProvider
}

func (v *merchantVerifierAdapter) Verify(tok string) (merchantstransport.Claims, error) {
	principal, err := v.prov.ValidateAccessToken(tok)
	if err != nil {
		return merchantstransport.Claims{}, err
	}
	var tenantID string
	if principal.TenantID != nil {
		tenantID = *principal.TenantID
	}
	return merchantstransport.Claims{
		Sub:      principal.UserID,
		Role:     string(principal.Role),
		TenantID: tenantID,
	}, nil
}

// contractAdminAuth mirrors cmd/api/wiring_admin.go's identityAuthAdapter (that type is
// in package main, so tests cannot import it). It keeps the admin 401/403 contract.
type contractAdminAuth struct {
	sec *identitytransport.SecurityMiddleware
}

func (a contractAdminAuth) Require(roles ...string) func(http.Handler) http.Handler {
	allowed := make([]identitydomain.Role, 0, len(roles))
	for _, r := range roles {
		allowed = append(allowed, identitydomain.Role(r))
	}
	byRole := identitytransport.RequireRole(allowed...)
	return func(next http.Handler) http.Handler { return a.sec.Authenticate(byRole(next)) }
}

func (contractAdminAuth) Principal(ctx context.Context) (admintransport.Principal, bool) {
	p, ok := identitytransport.GetPrincipal(ctx)
	if !ok || p == nil {
		return admintransport.Principal{}, false
	}
	return admintransport.Principal{UserID: p.UserID, Role: string(p.Role)}, true
}

func buildContractTestRouter(db *sql.DB) (http.Handler, *identityinfra.JWTTokenProvider) {
	mux := http.NewServeMux()
	tokenProv := identityinfra.NewJWTTokenProvider("test-contract-smoke-secret-key-32chars")
	security := identitytransport.NewSecurityMiddleware(tokenProv)

	// 1. Identity Routes
	hasher := identityinfra.NewStandardPasswordHasher()
	var userRepo identityapp.UserRepository
	var tokenRepo identityapp.TokenRepository
	if db != nil {
		userRepo = identityinfra.NewPostgresUserRepository(db)
		tokenRepo = identityinfra.NewPostgresTokenRepository(db)
	} else {
		userRepo = identityinfra.NewMockUserRepository(hasher)
		tokenRepo = identityinfra.NewMockTokenRepository()
	}
	authSvc := identityapp.NewAuthService(userRepo, tokenRepo, hasher, tokenProv)
	authHandler := identitytransport.NewAuthHandler(authSvc)
	identitytransport.RegisterRoutes(mux, authHandler, security)

	// 2. Events Routes
	eventsHandler := eventstransport.NewHandler(nil)
	eventstransport.RegisterRoutes(mux, eventsHandler, security)

	// 3. Merchants Routes
	merchantHandler := &merchantstransport.Handler{
		Svc:      merchantsapp.NewService(merchantsapp.Deps{}, merchantsapp.Config{}),
		Verifier: &merchantVerifierAdapter{prov: tokenProv},
		Log:      slog.Default(),
	}
	merchantHandler.Register(mux)

	// 4. Pricing Routes
	now := time.Now().UTC()
	var tariffRepo pricingapp.TariffRepository
	var quoteRepo pricingapp.QuoteRepository
	if db != nil {
		tariffRepo = pricinginfra.NewPostgresTariffRepository(db)
		quoteRepo = pricinginfra.NewPostgresQuoteRepository(db)
	}
	quoteUC := pricingapp.NewGenerateQuoteUseCase(
		tariffRepo,
		quoteRepo,
		testFixedClock{now: now},
		pricinginfra.NewUUIDv7QuoteIDGenerator(),
	)
	pricingHandler, err := pricingtransport.NewHandler(quoteUC, &fakeMerchantReader{})
	if err != nil {
		panic(err)
	}
	pricingtransport.RegisterRoutes(mux, pricingHandler, security)

	// 5. Admin Operations (Member 07: zones, settings, audit log, delivery report).
	// Needs PostgreSQL, like the real wiring: offline runs skip these routes (404).
	if db != nil {
		(&admintransport.Handlers{
			Svc:          &adminapp.Service{Store: &admininfra.Store{DB: db, LockTimeout: 2 * time.Second}},
			Auth:         contractAdminAuth{sec: security},
			Log:          slog.Default(),
			Reports:      adminapp.NoDeliveryData{},
			ReportSource: adminapp.NoDeliveryDataSource,
		}).Register(mux)
	}

	// 6. Parcel Operations (Member 02: parcels booking, tracking, status history)
	// Needs PostgreSQL, like the real wiring: offline runs skip these routes (404).
	if db != nil {
		txManager := parcelsapp.NewDBTransactionManager(db)
		quoteConsumer := parcelsinfra.NewPostgresQuoteConsumer(db)
		parcelRepo := parcelsinfra.NewPostgresParcelRepository(db)
		statusHistoryRepo := parcelsinfra.NewPostgresStatusHistoryRepository(db)
		idempotencyRepo := parcelsinfra.NewPostgresIdempotencyRepository(db)
		outboxWriter := events.NewPostgresOutboxWriter()
		trackingGen := parcelsapp.NewCryptoTrackingNumberGenerator()
		idGen := parcelsapp.NewUUIDv7Generator()
		clock := parcelsapp.NewSystemClock()
		merchantReader := parcelsinfra.NewMerchantRepoAdapter(merchantsinfra.MerchantRepo{}, db)

		bookUC, err := parcelsapp.NewBookParcelUseCase(
			txManager,
			quoteConsumer,
			parcelRepo,
			statusHistoryRepo,
			outboxWriter,
			trackingGen,
			idGen,
			clock,
		)
		if err != nil {
			panic(err)
		}
		bookUC = bookUC.WithIdempotency(idempotencyRepo, parcelsapp.DefaultIdempotencyTTL)

		getParcelUC, err := parcelsapp.NewGetParcelUseCase(parcelRepo, merchantReader)
		if err != nil {
			panic(err)
		}

		getHistoryUC, err := parcelsapp.NewGetParcelHistoryUseCase(parcelRepo, statusHistoryRepo, merchantReader)
		if err != nil {
			panic(err)
		}

		parcelHandler, err := parcelstransport.NewHandler(bookUC, getParcelUC, getHistoryUC, merchantReader)
		if err != nil {
			panic(err)
		}

		parcelstransport.RegisterRoutes(mux, parcelHandler, security)
	}

	// 4. Standalone & Health Endpoints
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"degraded"}`))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Sample Role Protected Dashboards
	mux.Handle("GET /api/v1/admin/dashboard", security.Authenticate(
		identitytransport.RequireRole(identitydomain.RoleAdmin)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"admin_ok"}`))
			}),
		),
	))
	mux.Handle("GET /api/v1/courier/dashboard", security.Authenticate(
		identitytransport.RequireRole(identitydomain.RoleCourier, identitydomain.RoleAdmin)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"courier_ok"}`))
			}),
		),
	))
	mux.Handle("GET /api/v1/merchant/portal", security.Authenticate(
		identitytransport.RequireRole(identitydomain.RoleMerchant, identitydomain.RoleAdmin)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"merchant_ok"}`))
			}),
		),
	))

	return mux, tokenProv
}

// TestOpenAPIContractSmoke iterates through every endpoint documented in swagger.json
// and ensures:
// 1. The route is mounted in the HTTP multiplexer (does not answer 404).
// 2. Protected routes answer 401 Unauthorized when requested without a token.
// 3. Admin routes answer 403 Forbidden when requested by unauthorized roles.
func TestOpenAPIContractSmoke(t *testing.T) {
	// Parse swagger.json
	swaggerCandidates := []string{
		filepath.Join("..", "docs", "swagger.json"),
		filepath.Join("docs", "swagger.json"),
	}
	var data []byte
	for _, p := range swaggerCandidates {
		b, err := os.ReadFile(p)
		if err == nil {
			data = b
			break
		}
	}
	if len(data) == 0 {
		t.Fatalf("could not find swagger.json in %v", swaggerCandidates)
	}

	var doc swaggerDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("failed to unmarshal swagger.json: %v", err)
	}

	var db *sql.DB
	if os.Getenv("INTEGRATION") == "1" {
		testDB, cleanup := SetupTestDB(t)
		defer cleanup()
		db = testDB
	}

	router, tokenProv := buildContractTestRouter(db)

	customerToken, err := tokenProv.GenerateAccessToken(identityapp.Principal{
		UserID: "cust-smoke-001",
		Role:   identitydomain.RoleCustomer,
	})
	if err != nil {
		t.Fatalf("failed to generate customer token: %v", err)
	}

	adminToken, err := tokenProv.GenerateAccessToken(identityapp.Principal{
		UserID: "admin-smoke-001",
		Role:   identitydomain.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	for routePath, methods := range doc.Paths {
		for method, operation := range methods {
			httpMethod := strings.ToUpper(method)
			testPath := pathParamRegex.ReplaceAllString(routePath, "018f4a7c-1001-7000-8000-000000000001")

			testName := fmt.Sprintf("%s %s", httpMethod, routePath)
			t.Run(testName, func(t *testing.T) {
				isProtected := len(operation.Security) > 0 ||
					strings.HasPrefix(routePath, "/api/v1/admin/") ||
					strings.HasPrefix(routePath, "/api/v1/merchant/") ||
					strings.HasPrefix(routePath, "/api/v1/merchants/me") ||
					strings.HasPrefix(routePath, "/api/v1/courier/") ||
					routePath == "/api/v1/auth/me"

				// Request without credentials
				req := httptest.NewRequest(httpMethod, testPath, bytes.NewBufferString("{}"))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				// Route must not be missing if it's in our core router
				if rec.Code == http.StatusNotFound {
					if os.Getenv("INTEGRATION") != "1" {
						t.Skipf("skipping route %s %s in offline mode (requires database)", httpMethod, routePath)
						return
					}
					t.Fatalf("route %s %s is documented in swagger.json but returned 404 Not Found (route not mounted)", httpMethod, routePath)
				}

				if isProtected {
					if rec.Code != http.StatusUnauthorized {
						t.Errorf("expected 401 Unauthorized on protected route %s %s, got %d", httpMethod, routePath, rec.Code)
					}
				} else {
					if rec.Code >= 500 {
						t.Errorf("expected non-500 on public route %s %s, got %d", httpMethod, routePath, rec.Code)
					}
				}
			})
		}
	}

	// Verify RBAC guards
	t.Run("RBAC: Customer on Admin Dashboard returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("RBAC: Admin on Admin Dashboard returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	})
}
