package main

import (
	"database/sql"
	"errors"
	"net/http"

	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/notifications/application"
	"github.com/swiftdrop/core-api/internal/notifications/infrastructure"
	"github.com/swiftdrop/core-api/internal/notifications/transport"
)

// registerNotificationsModule wires in-app notifications operations into the router.
func registerNotificationsModule(mux *http.ServeMux, db *sql.DB, sec *identitytransport.SecurityMiddleware) error {
	if mux == nil {
		return errors.New("notifications: http.ServeMux is required")
	}
	if db == nil {
		return errors.New("notifications: PostgreSQL connection (*sql.DB) is required")
	}
	if sec == nil {
		return errors.New("notifications: security middleware is required")
	}

	repo := infrastructure.NewPostgresRepository(db)
	service := application.NewService(repo)
	handler := transport.NewHandler(service)

	transport.RegisterRoutes(mux, handler, sec)
	return nil
}
