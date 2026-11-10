package http

import (
	"log/slog"
	"net/http"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/health"
)

// NewRouter constructs and configures the HTTP request multiplexer with standard
// platform middleware and endpoints. Supports optional handlers (*PublicTrackingHandler, *SSEHandler).
// All public endpoints are strictly isolated under the /api/v1/tracking/public/ namespace,
// preserving the /api/v1/tracking/ namespace for authenticated tracking.
func NewRouter(
	cfg *config.Config,
	logger *slog.Logger,
	healthHandler *health.Handler,
	handlers ...any,
) http.Handler {
	mux := http.NewServeMux()

	// Register Kubernetes & Infrastructure Health Probes
	mux.HandleFunc("GET /health/live", healthHandler.Live)
	mux.HandleFunc("GET /health/ready", healthHandler.Ready)

	// Scan optional handlers
	var (
		trackingHandler *PublicTrackingHandler
		sseHandler      *SSEHandler
	)

	for _, h := range handlers {
		switch v := h.(type) {
		case *PublicTrackingHandler:
			trackingHandler = v
		case *SSEHandler:
			sseHandler = v
		}
	}

	// Phase 2: Public Tracking REST endpoint
	if trackingHandler != nil {
		mux.HandleFunc("GET /api/v1/tracking/public/{tracking_number}", trackingHandler.GetPublicTracking)
	}

	// Phase 4: Canonical Server-Sent Events (SSE) Public Streaming Endpoint
	if sseHandler != nil {
		mux.HandleFunc("GET /api/v1/tracking/public/{tracking_number}/stream", sseHandler.StreamTracking)
	}

	// Pipeline: RequestID -> Recover -> AccessLog -> Mux
	return RequestID(
		Recover(logger)(
			AccessLog(logger)(mux),
		),
	)
}
