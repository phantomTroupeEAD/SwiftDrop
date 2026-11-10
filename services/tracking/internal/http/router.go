package http

import (
	"log/slog"
	"net/http"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/health"
)

// NewRouter constructs and configures the HTTP request multiplexer with standard
// platform middleware and endpoints for Phase 1.
func NewRouter(cfg *config.Config, logger *slog.Logger, healthHandler *health.Handler) http.Handler {
	mux := http.NewServeMux()

	// Register Kubernetes & Infrastructure Health Probes
	mux.HandleFunc("GET /health/live", healthHandler.Live)
	mux.HandleFunc("GET /health/ready", healthHandler.Ready)

	// In Phase 1: Unimplemented tracking endpoints are NOT registered.
	// They will be introduced in subsequent phases (Phase 2: DB queries, Phase 3: Auth, Phase 4: SSE).

	// Pipeline: RequestID -> Recover -> AccessLog -> Mux
	return RequestID(
		Recover(logger)(
			AccessLog(logger)(mux),
		),
	)
}
