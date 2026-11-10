package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/health"
)

func setupTestRouter() (http.Handler, *health.Handler) {
	cfg := &config.Config{
		HTTPAddr:            ":8081",
		AppEnv:              "local",
		ShutdownTimeout:     15 * time.Second,
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        30 * time.Second,
		IdleTimeout:         60 * time.Second,
		LogLevel:            "info",
		SSEHeartbeatSeconds: 15,
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	healthHandler := health.NewHandler()
	router := NewRouter(cfg, logger, healthHandler)
	return router, healthHandler
}

func TestRouterHealthEndpoints(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("GET /health/live returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}
		if rr.Header().Get("X-Request-ID") == "" {
			t.Error("expected X-Request-ID header in response")
		}

		var body map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		if body["status"] != "ok" {
			t.Errorf("expected status 'ok', got %q", body["status"])
		}
	})

	t.Run("GET /health/ready returns 200 OK with empty checks in Phase 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}

		var body struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		if body.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", body.Status)
		}
		if len(body.Checks) != 0 {
			t.Errorf("expected 0 checks for Phase 1 baseline, got %d", len(body.Checks))
		}
	})
}

func TestRouterMethodBehavior(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("POST /health/live is rejected with 405 Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/health/live", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed, got %d", rr.Code)
		}
	})

	t.Run("POST /health/ready is rejected with 405 Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/health/ready", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed, got %d", rr.Code)
		}
	})
}

func TestRouterUnimplementedEndpoints(t *testing.T) {
	router, _ := setupTestRouter()

	// In Phase 1, business tracking endpoints should NOT be exposed yet
	unimplementedPaths := []string{
		"/api/v1/tracking/public/SD-20261021-04921",
		"/api/v1/tracking/SD-20261021-04921",
		"/api/v1/tracking/SD-20261021-04921/stream",
		"/unknown/endpoint",
	}

	for _, path := range unimplementedPaths {
		t.Run("GET "+path+" returns 404 Not Found", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Errorf("expected 404 Not Found for unimplemented %s, got %d", path, rr.Code)
			}
		})
	}
}
