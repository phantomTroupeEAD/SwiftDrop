package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "postgres://none:none@127.0.0.1:54399/none?sslmode=disable")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func TestRegisterNotificationsModule_Validation(t *testing.T) {
	mux := http.NewServeMux()
	sec := identitytransport.NewSecurityMiddleware(mockTokenProvider{})
	db := newTestDB(t)
	defer db.Close()

	t.Run("nil mux fails", func(t *testing.T) {
		err := registerNotificationsModule(nil, db, sec)
		if err == nil || !strings.Contains(err.Error(), "http.ServeMux is required") {
			t.Fatalf("expected mux error, got %v", err)
		}
	})

	t.Run("nil db fails", func(t *testing.T) {
		err := registerNotificationsModule(mux, nil, sec)
		if err == nil || !strings.Contains(err.Error(), "PostgreSQL connection") {
			t.Fatalf("expected db error, got %v", err)
		}
	})

	t.Run("nil sec fails", func(t *testing.T) {
		err := registerNotificationsModule(mux, db, nil)
		if err == nil || !strings.Contains(err.Error(), "security middleware is required") {
			t.Fatalf("expected security error, got %v", err)
		}
	})

	t.Run("valid arguments succeed", func(t *testing.T) {
		m := http.NewServeMux()
		if err := registerNotificationsModule(m, db, sec); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})
}

func TestNotificationsWiring_RouteRegistrationAndSecurity(t *testing.T) {
	mux := http.NewServeMux()
	sec := identitytransport.NewSecurityMiddleware(mockTokenProvider{})
	db := newTestDB(t)
	defer db.Close()

	if err := registerNotificationsModule(mux, db, sec); err != nil {
		t.Fatalf("failed to register notifications module: %v", err)
	}

	expectedRoutes := []struct {
		method  string
		path    string
		pattern string
	}{
		{method: http.MethodGet, path: "/api/v1/notifications", pattern: "GET /api/v1/notifications"},
		{method: http.MethodGet, path: "/api/v1/notifications/unread-count", pattern: "GET /api/v1/notifications/unread-count"},
		{method: http.MethodPatch, path: "/api/v1/notifications/00000000-0000-0000-0000-000000000001/read", pattern: "PATCH /api/v1/notifications/{id}/read"},
		{method: http.MethodPatch, path: "/api/v1/notifications/read-all", pattern: "PATCH /api/v1/notifications/read-all"},
	}

	for _, tc := range expectedRoutes {
		t.Run(tc.pattern, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			_, matchedPattern := mux.Handler(req)
			if matchedPattern != tc.pattern {
				t.Errorf("expected pattern %q, got %q", tc.pattern, matchedPattern)
			}

			// 1. Unauthenticated request must be intercepted by SecurityMiddleware and return 401 Unauthorized
			unauthRec := httptest.NewRecorder()
			mux.ServeHTTP(unauthRec, req)
			if unauthRec.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401 Unauthorized without Bearer token, got %d", unauthRec.Code)
			}

			// 2. Authenticated request with Bearer token passes SecurityMiddleware
			authReq := httptest.NewRequest(tc.method, tc.path, nil)
			authReq.Header.Set("Authorization", "Bearer mock.token")
			authRec := httptest.NewRecorder()
			mux.ServeHTTP(authRec, authReq)
			if authRec.Code == http.StatusUnauthorized {
				t.Errorf("expected SecurityMiddleware to authenticate valid Bearer token, but got 401")
			}
		})
	}
}
