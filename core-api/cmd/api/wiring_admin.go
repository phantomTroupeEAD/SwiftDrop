package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/swiftdrop/core-api/internal/admin/application"
	"github.com/swiftdrop/core-api/internal/admin/infrastructure"
	admintransport "github.com/swiftdrop/core-api/internal/admin/transport"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// adminLockTimeout bounds how long an admin write waits for the zone prefix lock
// or a row lock before it answers 503 TRY_AGAIN.
const adminLockTimeout = 2 * time.Second

// registerAdminModule wires Member 07 admin operations (zones, postal codes,
// settings, audit log) into the router. It needs PostgreSQL: there is no mock store.
func registerAdminModule(mux *http.ServeMux, db *sql.DB, sec *identitytransport.SecurityMiddleware) error {
	if mux == nil {
		return errors.New("admin: http.ServeMux is required")
	}
	if db == nil {
		return errors.New("admin: PostgreSQL connection (*sql.DB) is required")
	}
	if sec == nil {
		return errors.New("admin: identity SecurityMiddleware is required")
	}
	h := &admintransport.Handlers{
		Svc:  &application.Service{Store: &infrastructure.Store{DB: db, LockTimeout: adminLockTimeout}},
		Auth: identityAuthAdapter{sec: sec},
		Log:  slog.Default(),
	}
	h.Register(mux)
	return nil
}

// identityAuthAdapter adapts Member 1's middleware to admin's Auth port, so the
// admin module does not import identity.
type identityAuthAdapter struct {
	sec *identitytransport.SecurityMiddleware
}

func (a identityAuthAdapter) Require(roles ...string) func(http.Handler) http.Handler {
	allowed := make([]identitydomain.Role, 0, len(roles))
	for _, r := range roles {
		allowed = append(allowed, identitydomain.Role(r))
	}
	byRole := identitytransport.RequireRole(allowed...)
	return func(next http.Handler) http.Handler {
		return a.sec.Authenticate(byRole(next))
	}
}

func (identityAuthAdapter) Principal(ctx context.Context) (admintransport.Principal, bool) {
	p, ok := identitytransport.GetPrincipal(ctx)
	if !ok || p == nil {
		return admintransport.Principal{}, false
	}
	return admintransport.Principal{UserID: p.UserID, Role: string(p.Role)}, true
}
