package http

import (
	"log/slog"
	"net/http"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/health"
)

// NewRouter constructs and configures the HTTP request multiplexer with standard
// platform middleware and endpoints.
func NewRouter(
	cfg *config.Config,
	logger *slog.Logger,
	healthHandler *health.Handler,
	trackingHandlers ...*PublicTrackingHandler,
) http.Handler {
	mux := http.NewServeMux()

	// Register Kubernetes & Infrastructure Health Probes
	mux.HandleFunc("GET /health/live", healthHandler.Live)
	mux.HandleFunc("GET /health/ready", healthHandler.Ready)

	// Phase 2: Public Tracking REST endpoint
	if len(trackingHandlers) > 0 && trackingHandlers[0] != nil {
		mux.HandleFunc("GET /api/v1/tracking/public/{tracking_number}", trackingHandlers[0].GetPublicTracking)
	}

	// Pipeline: RequestID -> Recover -> AccessLog -> Mux
	return RequestID(
		Recover(logger)(
			AccessLog(logger)(mux),
		),
	)
}
