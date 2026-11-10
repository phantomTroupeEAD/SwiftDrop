package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// CheckFunc is a function that returns an error if a dependency is unhealthy.
type CheckFunc func(ctx context.Context) error

// Handler encapsulates health probe logic for liveness and readiness.
type Handler struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

// NewHandler initializes a new health probe handler.
func NewHandler() *Handler {
	return &Handler{
		checks: make(map[string]CheckFunc),
	}
}

// AddCheck registers a named dependency readiness check.
func (h *Handler) AddCheck(name string, check CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

type liveResponse struct {
	Status string `json:"status"`
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Live handles Kubernetes / container liveness probes.
// It reports whether the HTTP server process is running and able to handle requests.
func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(liveResponse{
		Status: "ok",
	})
}

// Ready handles Kubernetes / container readiness probes.
// It truthfully evaluates all registered dependency health checks.
// In Phase 1, no external database or broker checks are registered, so it confirms
// that the service foundation is ready to receive traffic without falsely claiming
// that unintegrated dependencies exist.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	checksCopy := make(map[string]CheckFunc, len(h.checks))
	for k, v := range h.checks {
		checksCopy[k] = v
	}
	h.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	results := make(map[string]string, len(checksCopy))
	allHealthy := true

	for name, check := range checksCopy {
		if err := check(ctx); err != nil {
			allHealthy = false
			results[name] = err.Error()
		} else {
			results[name] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if !allHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(readyResponse{
			Status: "degraded",
			Checks: results,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(readyResponse{
		Status: "ok",
		Checks: results,
	})
}
