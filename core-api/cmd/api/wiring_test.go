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
	origDBURL, hasDBURL := os.LookupEnv("DATABASE_URL")
	defer func() {
		_ = os.Setenv("APP_ENV", origEnv)
		if hasDBURL {
			_ = os.Setenv("DATABASE_URL", origDBURL)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()

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
		subEnv := os.Getenv("APP_ENV")
		subDBURL, subHasDB := os.LookupEnv("DATABASE_URL")
		defer func() {
			_ = os.Setenv("APP_ENV", subEnv)
			if subHasDB {
				_ = os.Setenv("DATABASE_URL", subDBURL)
			} else {
				_ = os.Unsetenv("DATABASE_URL")
			}
		}()

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

	t.Run("GET /swagger/events/ returns 404 when SWAGGER_ENABLED=false with APP_ENV=production", func(t *testing.T) {
		prevAppEnv := os.Getenv("APP_ENV")
		_ = os.Setenv("APP_ENV", "production")
		_ = os.Setenv("SWAGGER_ENABLED", "false")
		defer func() {
			_ = os.Setenv("APP_ENV", prevAppEnv)
			_ = os.Unsetenv("SWAGGER_ENABLED")
		}()

		// Test the Swagger gate directly: a full BuildAppContainer in production now (correctly)
		// refuses to start without a reachable database and a real JWT_SECRET.
		mux := http.NewServeMux()
		registerEventsModules(mux, nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/swagger/events/", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})
}
