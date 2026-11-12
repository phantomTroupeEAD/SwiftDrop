package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/health"
	"github.com/swiftdrop/tracking/internal/service"
	"github.com/swiftdrop/tracking/internal/testutil"
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

func TestRouterEndpointCoexistence(t *testing.T) {
	cfg := &config.Config{
		HTTPAddr:            ":8081",
		AppEnv:              "local",
		ShutdownTimeout:     15 * time.Second,
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        30 * time.Second,
		IdleTimeout:         60 * time.Second,
		LogLevel:            "info",
		SSEHeartbeatSeconds: 15,
		JWTSecret:           testJWTSecret,
		JWTIssuer:           testJWTIssuer,
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	healthHandler := health.NewHandler()

	validTN := "SD-20261110-A1B2C3"
	parsedTN, _ := domain.NewTrackingNumber(validTN)
	fixedNow := time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC)

	repo := &mockHTTPRepo{
		findSummaryFn: func(_ context.Context, tn string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			parcel := &domain.ParcelTrackingInfo{
				ID:                    "parcel-1",
				TrackingNumber:        parsedTN,
				Status:                domain.StatusDelivered,
				SenderID:              strPtr("usr_customer_valid"),
				DestinationCity:       "Colombo",
				DestinationPostalCode: "00300",
				DestinationZoneID:     "ZONE_COLOMBO_03",
				CreatedAt:             fixedNow,
			}
			return parcel, nil, nil
		},
	}

	publicSvc := service.NewPublicTrackingService(repo)
	publicHandler := NewPublicTrackingHandler(publicSvc, logger)

	sseHandler := NewSSEHandler(publicSvc, nil, 15*time.Second, logger)

	jwtVerifier := auth.NewJWTVerifier(testJWTSecret, testJWTIssuer)
	jwtVerifier.SetNowFunc(func() time.Time { return fixedNow })
	authSvc := service.NewAuthenticatedTrackingService(repo)
	authHandler := NewAuthenticatedTrackingHandler(authSvc, jwtVerifier, logger)

	router := NewRouter(cfg, logger, healthHandler, publicHandler, sseHandler, authHandler)

	t.Run("Public REST endpoint is unaffected and succeeds without Auth header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/"+validTN, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		var resp SingleDataEnvelope[dto.PublicTrackingResponse]
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}
		if resp.Data.TrackingNumber != validTN {
			t.Errorf("expected %s, got %s", validTN, resp.Data.TrackingNumber)
		}
	})

	t.Run("Authenticated REST endpoint requires Bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for unauthenticated request, got %d", rr.Code)
		}
	})

	t.Run("Authenticated REST endpoint succeeds with valid Bearer token", func(t *testing.T) {
		tok, _ := testutil.GenerateTestToken(testJWTSecret, auth.JWTClaims{
			Issuer:    testJWTIssuer,
			Subject:   "usr_customer_valid",
			Role:      auth.RoleCustomer,
			ExpiresAt: fixedNow.Add(15 * time.Minute).Unix(),
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/"+validTN, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Public SSE stream endpoint is unaffected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/"+validTN+"/stream", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Without live event broker (nil eventSource in Phase 4), returns 503 DEPENDENCY_UNAVAILABLE
		// crucially without requiring authentication!
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 for unconfigured broker, got %d", rr.Code)
		}
		var errEnv ErrorEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&errEnv)
		if errEnv.Error.Code != "DEPENDENCY_UNAVAILABLE" {
			t.Errorf("expected DEPENDENCY_UNAVAILABLE, got %s", errEnv.Error.Code)
		}
	})
}
