package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

func TestRegisterPricingModule_Validation(t *testing.T) {
	mux := http.NewServeMux()
	sec := identitytransport.NewSecurityMiddleware(mockTokenProvider{})
	db := &sql.DB{}

	t.Run("nil mux fails", func(t *testing.T) {
		err := registerPricingModule(nil, db, sec)
		if err == nil || !strings.Contains(err.Error(), "http.ServeMux is required") {
			t.Fatalf("expected mux error, got %v", err)
		}
	})

	t.Run("nil db fails", func(t *testing.T) {
		err := registerPricingModule(mux, nil, sec)
		if err == nil || !strings.Contains(err.Error(), "PostgreSQL connection") {
			t.Fatalf("expected db error, got %v", err)
		}
	})

	t.Run("nil sec fails", func(t *testing.T) {
		err := registerPricingModule(mux, db, nil)
		if err == nil || !strings.Contains(err.Error(), "security middleware is required") {
			t.Fatalf("expected security error, got %v", err)
		}
	})
}

func TestRegisterParcelsModule_Validation(t *testing.T) {
	mux := http.NewServeMux()
	sec := identitytransport.NewSecurityMiddleware(mockTokenProvider{})
	db := &sql.DB{}

	t.Run("nil mux fails", func(t *testing.T) {
		err := registerParcelsModule(nil, db, sec)
		if err == nil || !strings.Contains(err.Error(), "http.ServeMux is required") {
			t.Fatalf("expected mux error, got %v", err)
		}
	})

	t.Run("nil db fails", func(t *testing.T) {
		err := registerParcelsModule(mux, nil, sec)
		if err == nil || !strings.Contains(err.Error(), "PostgreSQL connection") {
			t.Fatalf("expected db error, got %v", err)
		}
	})

	t.Run("nil sec fails", func(t *testing.T) {
		err := registerParcelsModule(mux, db, nil)
		if err == nil || !strings.Contains(err.Error(), "security middleware is required") {
			t.Fatalf("expected security error, got %v", err)
		}
	})
}

func TestMember02Wiring_RouteRegistrationAndSecurity(t *testing.T) {
	mux := http.NewServeMux()
	sec := identitytransport.NewSecurityMiddleware(mockTokenProvider{})
	db := &sql.DB{}

	if err := registerPricingModule(mux, db, sec); err != nil {
		t.Fatalf("failed to register pricing module: %v", err)
	}

	if err := registerParcelsModule(mux, db, sec); err != nil {
		t.Fatalf("failed to register parcels module: %v", err)
	}

	expectedRoutes := []struct {
		method  string
		path    string
		pattern string
	}{
		{method: http.MethodPost, path: "/api/v1/pricing/quotes", pattern: "POST /api/v1/pricing/quotes"},
		{method: http.MethodPost, path: "/api/v1/parcels", pattern: "POST /api/v1/parcels"},
		{method: http.MethodGet, path: "/api/v1/parcels/pcl_01HXYZ1234567890", pattern: "GET /api/v1/parcels/{id}"},
		{method: http.MethodGet, path: "/api/v1/parcels/pcl_01HXYZ1234567890/history", pattern: "GET /api/v1/parcels/{id}/history"},
	}

	for _, tc := range expectedRoutes {
		t.Run(tc.pattern, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			_, matchedPattern := mux.Handler(req)
			if matchedPattern != tc.pattern {
				t.Errorf("expected pattern %q, got %q", tc.pattern, matchedPattern)
			}

			// Unauthenticated request should be intercepted by SecurityMiddleware and return 401
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401 Unauthorized without Bearer token, got %d", rec.Code)
			}
		})
	}
}

func TestMember02Wiring_BuildAppContainer_NilDB(t *testing.T) {
	origEnv := os.Getenv("APP_ENV")
	origDBURL := os.Getenv("DATABASE_URL")
	defer func() {
		_ = os.Setenv("APP_ENV", origEnv)
		_ = os.Setenv("DATABASE_URL", origDBURL)
	}()

	_ = os.Setenv("APP_ENV", "local")
	_ = os.Setenv("DATABASE_URL", "postgres://none:none@127.0.0.1:54399/none?sslmode=disable")

	container := BuildAppContainer()
	if container == nil {
		t.Fatal("expected non-nil AppContainer")
	}
	if container.Mux == nil {
		t.Fatal("expected non-nil Mux")
	}
	if container.DB != nil {
		t.Errorf("expected nil DB in offline test, got %v", container.DB)
	}
}

func TestMember02Wiring_Integration(t *testing.T) {
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("skipping integration test; set INTEGRATION=1 to run")
	}

	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = os.Getenv("DATABASE_URL")
	}
	if testDBURL == "" {
		testDBURL = "postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable"
	}

	db, err := sql.Open("postgres", testDBURL)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	mux := http.NewServeMux()
	sec := identitytransport.NewSecurityMiddleware(mockTokenProvider{})

	if err := registerPricingModule(mux, db, sec); err != nil {
		t.Fatalf("pricing registration failed with live DB: %v", err)
	}

	if err := registerParcelsModule(mux, db, sec); err != nil {
		t.Fatalf("parcels registration failed with live DB: %v", err)
	}
}
