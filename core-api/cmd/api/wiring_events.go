package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"

	"github.com/swiftdrop/core-api/internal/events/application"
	"github.com/swiftdrop/core-api/internal/events/infrastructure"
	"github.com/swiftdrop/core-api/internal/events/swagger"
	eventstransport "github.com/swiftdrop/core-api/internal/events/transport"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/config"
	"github.com/swiftdrop/core-api/internal/platform/middleware"
)

// wrapGlobalMiddleware wraps the root HTTP handler with standard platform middlewares:
// RequestID tracing (X-Request-ID propagation to correlation_id), panic recovery with standard
// 500 JSON envelope, privacy-safe access logging, and request body size limiting.
func wrapGlobalMiddleware(handler http.Handler) http.Handler {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Pipeline: RequestID -> Recover -> AccessLog -> BodyLimit -> Mux
	return middleware.RequestID(
		middleware.Recover(logger)(
			middleware.AccessLog(logger)(
				middleware.BodyLimit(1 << 20)(handler), // 1MB default body limit
			),
		),
	)
}

// registerEventsModules wires Member 08 events admin/merchant routes and Swagger UI.
func registerEventsModules(mux *http.ServeMux, db *sql.DB, sec *identitytransport.SecurityMiddleware) {
	// 1. Register OpenAPI Specification in Swagger registry
	_ = swagger.Register("events", eventstransport.OpenAPISpec)

	// 2. Enable Swagger UI if configured or local
	appEnv := config.AppEnv()
	swaggerEnabled := config.Bool("SWAGGER_ENABLED", appEnv == "local")
	if swaggerEnabled {
		swagger.RegisterSwaggerRoutes(mux)
	}

	// 3. Register Events Operations & Merchant Delivery Routes
	if db != nil {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		eventsRepo := infrastructure.NewPostgresEventsRepository(db)
		eventsService := application.NewEventsService(eventsRepo, logger)
		eventsHandler := eventstransport.NewHandler(eventsService)
		eventstransport.RegisterRoutes(mux, eventsHandler, sec)
	}
}
