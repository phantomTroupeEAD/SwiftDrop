package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	admintransport "github.com/swiftdrop/core-api/internal/admin/transport"
	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

func TestRegisterAdminModule_Validation(t *testing.T) {
	sec := identitytransport.NewSecurityMiddleware(mockTokenProvider{})
	if err := registerAdminModule(http.NewServeMux(), nil, sec); err == nil || !strings.Contains(err.Error(), "PostgreSQL connection") {
		t.Fatalf("expected PostgreSQL error, got %v", err)
	}
	if err := registerAdminModule(http.NewServeMux(), &sql.DB{}, nil); err == nil || !strings.Contains(err.Error(), "SecurityMiddleware") {
		t.Fatalf("expected SecurityMiddleware error, got %v", err)
	}
	if err := registerAdminModule(http.NewServeMux(), &sql.DB{}, sec); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

// The adapter must keep the Auth contract with real identity tokens: no token 401,
// wrong role 403, admin passes with its principal in the context.
func TestIdentityAuthAdapter(t *testing.T) {
	prov := infrastructure.NewJWTTokenProvider("test-secret-key-that-is-at-least-32-bytes-long")
	auth := identityAuthAdapter{sec: identitytransport.NewSecurityMiddleware(prov)}

	var seen admintransport.Principal
	h := auth.Require(admintransport.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = auth.Principal(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	token := func(role identitydomain.Role) string {
		tok, err := prov.GenerateAccessToken(identityapp.Principal{UserID: "00000000-0000-4000-8000-000000000001", Role: role})
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	call := func(authz string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/zones", nil)
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := call(""); got != http.StatusUnauthorized {
		t.Fatalf("no token: got %d", got)
	}
	if got := call("Bearer not-a-jwt"); got != http.StatusUnauthorized {
		t.Fatalf("bad token: got %d", got)
	}
	if got := call("Bearer " + token(identitydomain.RoleDispatcher)); got != http.StatusForbidden {
		t.Fatalf("dispatcher: got %d", got)
	}
	if got := call("Bearer " + token(identitydomain.RoleAdmin)); got != http.StatusNoContent {
		t.Fatalf("admin: got %d", got)
	}
	if seen.Role != "ADMIN" || seen.UserID == "" {
		t.Fatalf("principal not passed through: %+v", seen)
	}
}
