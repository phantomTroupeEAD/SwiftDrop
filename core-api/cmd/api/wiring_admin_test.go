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

// The delivery report is the one admin route that also admits DISPATCHER (R-16). The real
// identity adapter must honour a two-role list: both pass, every other role is 403.
func TestIdentityAuthAdapterAdmitsAdminAndDispatcherOnly(t *testing.T) {
	prov := infrastructure.NewJWTTokenProvider("test-secret-key-that-is-at-least-32-bytes-long")
	auth := identityAuthAdapter{sec: identitytransport.NewSecurityMiddleware(prov)}
	h := auth.Require(admintransport.RoleAdmin, admintransport.RoleDispatcher)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	want := map[identitydomain.Role]int{
		identitydomain.RoleAdmin: http.StatusNoContent, identitydomain.RoleDispatcher: http.StatusNoContent,
		identitydomain.RoleCourier: http.StatusForbidden, identitydomain.RoleMerchant: http.StatusForbidden, identitydomain.RoleCustomer: http.StatusForbidden,
	}
	for role, status := range want {
		tok, err := prov.GenerateAccessToken(identityapp.Principal{UserID: "00000000-0000-4000-8000-000000000003", Role: role})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reports/deliveries", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Errorf("%s: got %d, want %d", role, rec.Code, status)
		}
	}
}

// A-11 to A-13 (GET /api/v1/admin/merchants, GET .../{id}, POST .../{id}/{action}) are
// served by Member 4's merchants module. The admin module must not mount the same
// patterns: Go's router panics on a duplicate, and Core API would not start. This test
// mounts both modules on one router, then checks the three routes exist (not 404) and
// refuse a missing token (401) and a non-ADMIN token (403) before touching the database.
func TestMerchantApprovalRoutesAreServedByMember4AndGuarded(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	prov := infrastructure.NewJWTTokenProvider("test-secret-key-that-is-at-least-32-bytes-long")
	sec := identitytransport.NewSecurityMiddleware(prov)
	mux := http.NewServeMux()
	if err := registerMerchantsModule(mux, &sql.DB{}, prov); err != nil {
		t.Fatalf("merchants: %v", err)
	}
	if err := registerAdminModule(mux, &sql.DB{}, sec); err != nil { // would panic on a duplicate pattern
		t.Fatalf("admin: %v", err)
	}

	const id = "11111111-1111-4111-8111-111111111111"
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/merchants", ""},
		{http.MethodGet, "/api/v1/admin/merchants/" + id, ""},
		{http.MethodPost, "/api/v1/admin/merchants/" + id + "/approve", `{"expected_version":1}`},
		{http.MethodPost, "/api/v1/admin/merchants/" + id + "/reject", `{"expected_version":1,"reason":"incomplete documents"}`},
	}
	token := func(role identitydomain.Role) string {
		tok, err := prov.GenerateAccessToken(identityapp.Principal{UserID: "00000000-0000-4000-8000-000000000002", Role: role})
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	call := func(method, path, body, authz string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, rt := range routes {
		if got := call(rt.method, rt.path, rt.body, ""); got != http.StatusUnauthorized {
			t.Errorf("%s %s without a token: got %d, want 401 (404 would mean the route is missing)", rt.method, rt.path, got)
		}
		for _, role := range []identitydomain.Role{identitydomain.RoleDispatcher, identitydomain.RoleCourier, identitydomain.RoleMerchant, identitydomain.RoleCustomer} {
			if got := call(rt.method, rt.path, rt.body, "Bearer "+token(role)); got != http.StatusForbidden {
				t.Errorf("%s %s as %s: got %d, want 403", rt.method, rt.path, role, got)
			}
		}
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
