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
	parcelsinfra "github.com/swiftdrop/core-api/internal/parcels/infrastructure"
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
		// Delivery numbers come from the parcels module's own read model, so the admin
		// module still never runs SQL on the parcels tables.
		Reports:      parcelDeliveryReports{reader: parcelsinfra.NewDeliveryStatsReader(db)},
		ReportSource: deliveryReportSource,
	}
	h.Register(mux)
	return nil
}

// deliveryReportSource is the report's "source" text when the numbers are real.
const deliveryReportSource = "live: parcel status history"

// parcelDeliveryReports adapts the parcels module's DeliveryStatsReader to the admin
// module's ReportReader port, so neither module imports the other.
type parcelDeliveryReports struct {
	reader interface {
		DailyDeliveryCounts(ctx context.Context, from, to time.Time) ([]parcelsinfra.DailyDeliveryCount, error)
	}
}

func (p parcelDeliveryReports) Deliveries(ctx context.Context, from, to time.Time) ([]application.DeliveryDay, error) {
	rows, err := p.reader.DailyDeliveryCounts(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]application.DeliveryDay, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.DeliveryDay{Date: r.Date, Delivered: r.Delivered, Failed: r.Failed, Returned: r.Returned})
	}
	return out, nil
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
