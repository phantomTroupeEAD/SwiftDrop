package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	// Set local environment
	origEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", origEnv)

	t.Run("GET /health/live returns 200 OK", func(t *testing.T) {
		container := BuildAppContainer()
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		rec := httptest.NewRecorder()

		container.Mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var body map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if body["status"] != "ok" {
			t.Errorf("expected status 'ok', got %q", body["status"])
		}
	})

	t.Run("GET /health/ready in local fallback returns 200 mock", func(t *testing.T) {
		_ = os.Setenv("APP_ENV", "local")
		// Point to unreachable DB to ensure fallback
		_ = os.Setenv("DATABASE_URL", "postgres://none:none@127.0.0.1:54399/none?sslmode=disable")

		container := BuildAppContainer()
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		rec := httptest.NewRecorder()

		container.Mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK in local fallback, got %d", rec.Code)
		}

		var body map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&body)
		if body["database"] != "mock" && body["database"] != "connected" {
			t.Errorf("expected mock or connected, got %v", body["database"])
		}
	})

	t.Run("GET /api/v1/health returns healthy", func(t *testing.T) {
		container := BuildAppContainer()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rec := httptest.NewRecorder()

		container.Mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var body map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&body)
		if body["status"] != "healthy" {
			t.Errorf("expected healthy, got %v", body["status"])
		}
	})
}
