package transport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
	"github.com/swiftdrop/core-api/internal/identity/transport"
)

type stubTokenProvider struct {
	validateFn func(string) (*application.Principal, error)
}

func (s *stubTokenProvider) GenerateAccessToken(_ application.Principal) (string, error) {
	return "stub-token", nil
}

func (s *stubTokenProvider) ValidateAccessToken(tok string) (*application.Principal, error) {
	return s.validateFn(tok)
}

func TestSecurityMiddleware_Authenticate(t *testing.T) {
	jwtProvider := infrastructure.NewJWTTokenProvider("test-secret-for-middleware-tests-32ch")
	mw := transport.NewSecurityMiddleware(jwtProvider)

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := transport.GetPrincipal(r.Context())
		if !ok || p == nil {
			t.Fatalf("expected Principal in context")
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"user_id": p.UserID})
	})

	t.Run("missing Authorization header returns 401 AUTH_UNAUTHORIZED", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		mw.Authenticate(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("malformed Authorization header returns 401 AUTH_MALFORMED_HEADER", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rec := httptest.NewRecorder()
		mw.Authenticate(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("expired token returns 401 AUTH_TOKEN_EXPIRED", func(t *testing.T) {
		expiredMW := transport.NewSecurityMiddleware(&stubTokenProvider{
			validateFn: func(string) (*application.Principal, error) {
				return nil, domain.ErrTokenExpired
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer expired.jwt.token")
		rec := httptest.NewRecorder()
		expiredMW.Authenticate(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
		var env transport.ErrorEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		if env.Error.Code != "AUTH_TOKEN_EXPIRED" {
			t.Errorf("expected AUTH_TOKEN_EXPIRED, got %q", env.Error.Code)
		}
	})

	t.Run("invalid token returns 401 AUTH_INVALID_TOKEN", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")
		rec := httptest.NewRecorder()
		mw.Authenticate(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid Bearer token injects Principal and calls next", func(t *testing.T) {
		tok, err := jwtProvider.GenerateAccessToken(application.Principal{
			UserID: "usr_123",
			Email:  "user@swiftdrop.local",
			Role:   domain.RoleCourier,
		})
		if err != nil {
			t.Fatalf("unexpected token gen error: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		mw.Authenticate(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestRequireRole_And_RequireTenant(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("RequireRole rejects missing principal with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		rec := httptest.NewRecorder()
		transport.RequireRole(domain.RoleAdmin)(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("RequireRole rejects disallowed role with 403", func(t *testing.T) {
		ctx := transport.WithPrincipal(context.Background(), &application.Principal{
			UserID: "usr_cust",
			Role:   domain.RoleCustomer,
		})
		req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		transport.RequireRole(domain.RoleAdmin, domain.RoleDispatcher)(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("RequireRole allows matching role", func(t *testing.T) {
		ctx := transport.WithPrincipal(context.Background(), &application.Principal{
			UserID: "usr_disp",
			Role:   domain.RoleDispatcher,
		})
		req := httptest.NewRequest(http.MethodGet, "/dispatch", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		transport.RequireRole(domain.RoleAdmin, domain.RoleDispatcher)(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("RequireTenant rejects missing or empty tenant with 403", func(t *testing.T) {
		empty := ""
		cases := []*application.Principal{
			nil,
			{UserID: "u1", Role: domain.RoleMerchant, TenantID: nil},
			{UserID: "u2", Role: domain.RoleMerchant, TenantID: &empty},
		}
		for _, p := range cases {
			ctx := context.Background()
			if p != nil {
				ctx = transport.WithPrincipal(ctx, p)
			}
			req := httptest.NewRequest(http.MethodGet, "/merchant", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			transport.RequireTenant()(okHandler).ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 for principal %+v, got %d", p, rec.Code)
			}
		}
	})

	t.Run("RequireTenant allows valid tenant", func(t *testing.T) {
		tenant := "mrc_01"
		ctx := transport.WithPrincipal(context.Background(), &application.Principal{
			UserID:   "u3",
			Role:     domain.RoleMerchant,
			TenantID: &tenant,
		})
		req := httptest.NewRequest(http.MethodGet, "/merchant", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		transport.RequireTenant()(okHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestContextHelpers(t *testing.T) {
	emptyCtx := context.Background()
	if transport.GetUserID(emptyCtx) != "" {
		t.Errorf("expected empty UserID on empty context")
	}
	if transport.GetUserRole(emptyCtx) != "" {
		t.Errorf("expected empty Role on empty context")
	}
	if transport.GetTenantID(emptyCtx) != nil {
		t.Errorf("expected nil TenantID on empty context")
	}

	tenant := "mrc_99"
	ctx := transport.WithPrincipal(emptyCtx, &application.Principal{
		UserID:   "usr_99",
		Email:    "m@vendor.com",
		Role:     domain.RoleMerchant,
		TenantID: &tenant,
	})
	if transport.GetUserID(ctx) != "usr_99" {
		t.Errorf("expected usr_99, got %q", transport.GetUserID(ctx))
	}
	if transport.GetUserRole(ctx) != domain.RoleMerchant {
		t.Errorf("expected MERCHANT, got %q", transport.GetUserRole(ctx))
	}
	if gotTenant := transport.GetTenantID(ctx); gotTenant == nil || *gotTenant != "mrc_99" {
		t.Errorf("expected mrc_99, got %v", gotTenant)
	}
}
