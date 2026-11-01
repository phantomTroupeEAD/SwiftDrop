package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	eventsapp "github.com/swiftdrop/core-api/internal/events/application"
	eventsinfra "github.com/swiftdrop/core-api/internal/events/infrastructure"
	eventstransport "github.com/swiftdrop/core-api/internal/events/transport"
	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

func buildTestRouter(db *sql.DB) http.Handler {
	hasher := identityinfra.NewStandardPasswordHasher()
	tokenProv := identityinfra.NewJWTTokenProvider("test-integration-secret-key-32chars")
	userRepo := identityinfra.NewPostgresUserRepository(db)
	tokenRepo := identityinfra.NewMockTokenRepository()

	authService := identityapp.NewAuthService(userRepo, tokenRepo, hasher, tokenProv)
	security := identitytransport.NewSecurityMiddleware(tokenProv)
	authHandler := identitytransport.NewAuthHandler(authService)

	eventsRepo := eventsinfra.NewPostgresEventsRepository(db)
	eventsSvc := eventsapp.NewEventsService(eventsRepo, nil)
	eventsHandler := eventstransport.NewHandler(eventsSvc)

	mux := http.NewServeMux()

	identitytransport.RegisterRoutes(mux, authHandler, security)
	eventstransport.RegisterRoutes(mux, eventsHandler, security)

	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":   "degraded",
				"database": "down",
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": "connected",
		})
	})

	return mux
}

func TestCrossModuleSmoke(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	defer cleanup()

	handler := buildTestRouter(db)

	// 1. Health Ready Check
	t.Run("Health Ready is connected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from /health/ready, got %d", rec.Code)
		}

		var body map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&body)
		if body["database"] != "connected" {
			t.Errorf("expected database connected, got %v", body["database"])
		}
	})

	// 2. Authenticate as Seeded Admin
	var accessToken string
	t.Run("Admin login succeeds with seeded credentials", func(t *testing.T) {
		loginPayload := []byte(`{"email":"admin@swiftdrop.local","password":"AdminPassword123!"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from login, got %d: %s", rec.Code, rec.Body.String())
		}

		var loginResp struct {
			Data struct {
				AccessToken string `json:"access_token"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&loginResp); err != nil {
			t.Fatalf("failed to decode login response: %v", err)
		}
		if loginResp.Data.AccessToken == "" {
			t.Fatalf("expected non-empty access_token")
		}
		accessToken = loginResp.Data.AccessToken
	})

	// 3. Call Admin Events Outbox Endpoint
	t.Run("Admin can inspect events outbox", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events/outbox?limit=10", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from outbox endpoint, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode outbox response: %v", err)
		}
	})
}
