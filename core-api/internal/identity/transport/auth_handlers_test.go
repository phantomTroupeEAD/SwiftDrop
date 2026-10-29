package transport_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
	"github.com/swiftdrop/core-api/internal/identity/transport"
)

func setupTestRouter() (*http.ServeMux, application.TokenProvider, *application.AuthService) {
	hasher := infrastructure.NewStandardPasswordHasher()
	tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-at-least-32-chars-long")
	userRepo := infrastructure.NewMockUserRepository(hasher)
	tokenRepo := infrastructure.NewMockTokenRepository()

	authService := application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider)
	security := transport.NewSecurityMiddleware(tokenProvider)
	authHandler := transport.NewAuthHandler(authService)

	mux := http.NewServeMux()
	transport.RegisterRoutes(mux, authHandler, security)

	// Add an admin-only test route
	mux.Handle("GET /api/v1/admin/test", security.Authenticate(
		transport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"admin": true}`))
			}),
		),
	))

	return mux, tokenProvider, authService
}

func TestAuthHandler_Login(t *testing.T) {
	mux, _, _ := setupTestRouter()

	t.Run("Successful login returns 200 and access_token", func(t *testing.T) {
		payload := []byte(`{"email":"admin@swiftdrop.local","password":"AdminPassword123!"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data struct {
				AccessToken string            `json:"access_token"`
				TokenType   string            `json:"token_type"`
				ExpiresIn   int               `json:"expires_in"`
				User        transport.UserDTO `json:"user"`
			} `json:"data"`
		}

		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if res.Data.AccessToken == "" {
			t.Errorf("Expected access token, got empty string")
		}
		if res.Data.User.Role != domain.RoleAdmin {
			t.Errorf("Expected role ADMIN, got %s", res.Data.User.Role)
		}

		// Verify refresh token cookie is set
		cookies := rec.Result().Cookies()
		var foundCookie bool
		for _, c := range cookies {
			if c.Name == "swiftdrop_refresh_token" && c.HttpOnly {
				foundCookie = true
				break
			}
		}
		if !foundCookie {
			t.Errorf("Expected HttpOnly cookie 'swiftdrop_refresh_token'")
		}
	})

	t.Run("Invalid credentials returns 401", func(t *testing.T) {
		payload := []byte(`{"email":"admin@swiftdrop.local","password":"WrongPassword!"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("Empty fields return 400", func(t *testing.T) {
		payload := []byte(`{"email":"","password":""}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected status 400, got %d", rec.Code)
		}
	})
}

func TestSecurityMiddleware_RequireRole(t *testing.T) {
	mux, tokenProvider, _ := setupTestRouter()

	adminToken, _ := tokenProvider.GenerateAccessToken(application.Principal{
		UserID: "usr_admin",
		Email:  "admin@swiftdrop.local",
		Role:   domain.RoleAdmin,
	})

	customerToken, _ := tokenProvider.GenerateAccessToken(application.Principal{
		UserID: "usr_cust",
		Email:  "customer@swiftdrop.local",
		Role:   domain.RoleCustomer,
	})

	t.Run("Admin token allows access to Admin route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/test", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("Customer token is denied access (403 Forbidden) to Admin route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/test", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("Expected status 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("Missing token is denied access (401 Unauthorized)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/test", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status 401 Unauthorized, got %d", rec.Code)
		}
	})
}
