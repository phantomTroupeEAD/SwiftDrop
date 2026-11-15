package transport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
	"github.com/swiftdrop/core-api/internal/identity/transport"
)

type testOIDCClient struct{}

func (c *testOIDCClient) VerifyIDToken(_ context.Context, idToken string) (*application.OIDCClaims, error) {
	if idToken == "mock_invalid_token" || idToken == "" {
		return nil, domain.ErrInvalidToken
	}
	email := idToken
	for _, prefix := range []string{"mock_", "dev_", "google_id_token_"} {
		if len(email) > len(prefix) && email[:len(prefix)] == prefix {
			email = email[len(prefix):]
		}
	}
	return &application.OIDCClaims{
		Subject:       "google_sub_" + email,
		Email:         email,
		EmailVerified: true,
		Name:          "Test Google User",
	}, nil
}

func setupTestRouter() (*http.ServeMux, application.TokenProvider, *application.AuthService) {
	hasher := infrastructure.NewStandardPasswordHasher()
	tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-at-least-32-chars-long")
	userRepo := infrastructure.NewMockUserRepository(hasher)
	tokenRepo := infrastructure.NewMockTokenRepository()
	otpRepo := infrastructure.NewMemoryOTPRepository()
	emailSender := infrastructure.NewDevEmailSender()
	oauthRepo := infrastructure.NewMockOAuthRepository()
	oidcClient := &testOIDCClient{}

	authService := application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider).
		WithOTP(otpRepo, emailSender).
		WithOAuth(oauthRepo, oidcClient)
	security := transport.NewSecurityMiddleware(tokenProvider)
	authHandler := transport.NewAuthHandler(authService)

	mux := http.NewServeMux()
	transport.RegisterRoutes(mux, authHandler, security)

	// Add an admin-only test route
	mux.Handle("GET /api/v1/admin/test", security.Authenticate(
		transport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"admin": true}`))
			}),
		),
	))

	return mux, tokenProvider, authService
}

// â”€â”€â”€ Send OTP Tests â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestSendRegistrationOTP(t *testing.T) {
	mux, _, _ := setupTestRouter()

	t.Run("200 OK for new available email", func(t *testing.T) {
		body := []byte(`{"email":"brand-new@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/send-otp", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d â€” body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("409 Conflict for already-registered email", func(t *testing.T) {
		body := []byte(`{"email":"admin@swiftdrop.local"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/send-otp", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("Expected 409, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request for invalid email format", func(t *testing.T) {
		body := []byte(`{"email":"not-an-email"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/send-otp", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request when email field is empty", func(t *testing.T) {
		body := []byte(`{"email":""}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/send-otp", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400, got %d", rec.Code)
		}
	})

	t.Run("429 Too Many Requests on rapid resend", func(t *testing.T) {
		email := `{"email":"cooldown-transport@example.com"}`
		// First send
		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/send-otp", bytes.NewBufferString(email))
		req1.Header.Set("Content-Type", "application/json")
		rec1 := httptest.NewRecorder()
		mux.ServeHTTP(rec1, req1)

		// Rapid resend
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/send-otp", bytes.NewBufferString(email))
		req2.Header.Set("Content-Type", "application/json")
		rec2 := httptest.NewRecorder()
		mux.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusTooManyRequests {
			t.Fatalf("Expected 429, got %d", rec2.Code)
		}
	})
}

// â”€â”€â”€ Full Registration Tests â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestRegister(t *testing.T) {
	t.Run("400 Bad Request when terms_accepted is false", func(t *testing.T) {
		mux, _, _ := setupTestRouter()
		payload := `{"email":"a@b.com","otp":"123456","first_name":"A","last_name":"B","phone":"+94771234567","password":"SecurePassword123!","terms_accepted":false}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400, got %d â€” body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("400 Bad Request for missing required fields", func(t *testing.T) {
		mux, _, _ := setupTestRouter()
		payload := `{"email":"a@b.com","terms_accepted":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request for incorrect OTP", func(t *testing.T) {
		mux, _, _ := setupTestRouter()

		// First send OTP for new email
		otpBody := `{"email":"otp-test@example.com"}`
		otpReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/send-otp", bytes.NewBufferString(otpBody))
		otpReq.Header.Set("Content-Type", "application/json")
		httptest.NewRecorder() // discard

		payload := `{"email":"otp-test@example.com","otp":"000000","first_name":"Test","last_name":"User","phone":"+94771234567","password":"SecurePassword123!","terms_accepted":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400 for bad OTP, got %d â€” body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("201 Created on successful registration with correct OTP", func(t *testing.T) {
		// Build a fresh service with injectable OTP repo
		hasher := infrastructure.NewStandardPasswordHasher()
		tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-at-least-32-chars-long")
		userRepo := infrastructure.NewMockUserRepository(hasher)
		tokenRepo := infrastructure.NewMockTokenRepository()
		otpRepo := infrastructure.NewMemoryOTPRepository()
		emailSender := infrastructure.NewDevEmailSender()
		authService := application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider).
			WithOTP(otpRepo, emailSender)
		security := transport.NewSecurityMiddleware(tokenProvider)
		authHandler := transport.NewAuthHandler(authService)
		mux := http.NewServeMux()
		transport.RegisterRoutes(mux, authHandler, security)

		email := "register-201@example.com"
		ctx := context.Background()

		// Plant known OTP directly into the OTP store
		knownCode := "654321"
		_ = otpRepo.SaveOTP(ctx, email, knownCode)

		payload, _ := json.Marshal(map[string]interface{}{
			"email":          email,
			"otp":            knownCode,
			"first_name":     "Alice",
			"last_name":      "Johnson",
			"phone":          "+94771234567",
			"password":       "SecurePassword123!",
			"terms_accepted": true,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("Expected 201 Created, got %d â€” body: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data struct {
				User struct {
					Email         string      `json:"email"`
					Role          domain.Role `json:"role"`
					EmailVerified bool        `json:"email_verified"`
					RequiresPhone bool        `json:"requires_phone"`
				} `json:"user"`
				IsNewUser bool `json:"is_new_user"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if res.Data.User.Email != email {
			t.Errorf("Email mismatch: got %s", res.Data.User.Email)
		}
		if res.Data.User.Role != domain.RoleCustomer {
			t.Errorf("Expected CUSTOMER role, got %s", res.Data.User.Role)
		}
		if !res.Data.User.EmailVerified {
			t.Error("Expected email_verified: true")
		}
		if res.Data.User.RequiresPhone {
			t.Error("Expected requires_phone: false")
		}
		if !res.Data.IsNewUser {
			t.Error("Expected is_new_user: true")
		}
	})
}

// â”€â”€â”€ Existing Login & Role Tests â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

func TestAuthHandler_Refresh_and_Logout(t *testing.T) {
	mux, _, _ := setupTestRouter()

	// Step 1: Login to get refresh token cookie
	loginPayload := []byte(`{"email":"customer@swiftdrop.local","password":"CustomerPassword123!"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	mux.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Login failed: %d", loginRec.Code)
	}

	cookies := loginRec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "swiftdrop_refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("Expected swiftdrop_refresh_token cookie")
	}

	t.Run("POST /auth/refresh returns 200 and sets rotated cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.AddCookie(refreshCookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data struct {
				AccessToken string `json:"access_token"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("Failed to decode json: %v", err)
		}
		if res.Data.AccessToken == "" {
			t.Error("Expected new access token")
		}

		// Check rotated cookie
		rotatedCookies := rec.Result().Cookies()
		var foundRotated bool
		for _, c := range rotatedCookies {
			if c.Name == "swiftdrop_refresh_token" && c.Value != refreshCookie.Value {
				foundRotated = true
				break
			}
		}
		if !foundRotated {
			t.Error("Expected rotated refresh token in cookie")
		}
	})

	t.Run("POST /auth/logout revokes session and clears cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.AddCookie(refreshCookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rec.Code)
		}

		var cleared bool
		for _, c := range rec.Result().Cookies() {
			if c.Name == "swiftdrop_refresh_token" && c.MaxAge == -1 {
				cleared = true
				break
			}
		}
		if !cleared {
			t.Error("Expected cookie to be cleared (MaxAge: -1)")
		}
	})
}

func TestGoogleLogin(t *testing.T) {
	mux, _, _ := setupTestRouter()

	t.Run("POST /auth/google creates new user with 201 Created", func(t *testing.T) {
		payload := `{"id_token": "mock_google_id_token_bob@gmail.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("Expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data struct {
				AccessToken   string `json:"access_token"`
				IsNewUser     bool   `json:"is_new_user"`
				RequiresPhone bool   `json:"requires_phone"`
				User          struct {
					Email string `json:"email"`
					Role  string `json:"role"`
				} `json:"user"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if res.Data.AccessToken == "" {
			t.Error("Expected access token in response")
		}
		if !res.Data.IsNewUser {
			t.Error("Expected IsNewUser to be true")
		}
		if !res.Data.RequiresPhone {
			t.Error("Expected RequiresPhone to be true")
		}
		if res.Data.User.Role != "CUSTOMER" {
			t.Errorf("Expected role CUSTOMER, got %s", res.Data.User.Role)
		}

		// Verify refresh cookie set
		var foundCookie bool
		for _, c := range rec.Result().Cookies() {
			if c.Name == "swiftdrop_refresh_token" && c.Value != "" && c.HttpOnly {
				foundCookie = true
				break
			}
		}
		if !foundCookie {
			t.Error("Expected HttpOnly swiftdrop_refresh_token cookie")
		}
	})

	t.Run("POST /auth/google returns 200 OK for existing linked user", func(t *testing.T) {
		payload := `{"id_token": "mock_google_id_token_bob@gmail.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Data struct {
				IsNewUser bool `json:"is_new_user"`
			} `json:"data"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.Data.IsNewUser {
			t.Error("Expected IsNewUser to be false on second login")
		}
	})

	t.Run("POST /auth/google returns 401 Unauthorized on invalid token", func(t *testing.T) {
		payload := `{"id_token": "mock_invalid_token"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("Expected 401 Unauthorized, got %d", rec.Code)
		}
	})
}

func TestAuthHandler_ForgotPassword(t *testing.T) {
	hasher := infrastructure.NewStandardPasswordHasher()
	tokenProvider := infrastructure.NewJWTTokenProvider("test-secret-at-least-32-chars-long")
	userRepo := infrastructure.NewMockUserRepository(hasher)
	tokenRepo := infrastructure.NewMockTokenRepository()
	otpRepo := infrastructure.NewMemoryOTPRepository()
	emailSender := infrastructure.NewDevEmailSender()
	authService := application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider).
		WithOTP(otpRepo, emailSender)
	security := transport.NewSecurityMiddleware(tokenProvider)
	authHandler := transport.NewAuthHandler(authService)
	mux := http.NewServeMux()
	transport.RegisterRoutes(mux, authHandler, security)

	t.Run("POST /forgot-password/send-otp returns 200 OK and enforces cooldown", func(t *testing.T) {
		payload := []byte(`{"email":"customer@swiftdrop.local"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password/send-otp", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		// Second immediate request hits 429 cooldown
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password/send-otp", bytes.NewBuffer(payload))
		req2.Header.Set("Content-Type", "application/json")
		rec2 := httptest.NewRecorder()
		mux.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusTooManyRequests {
			t.Fatalf("Expected 429 Too Many Requests, got %d: %s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("POST /forgot-password/reset returns 200 OK on valid OTP and updates password", func(t *testing.T) {
		_ = otpRepo.SaveOTP(context.Background(), "reset:customer@swiftdrop.local", "778899")

		payload := []byte(`{"email":"customer@swiftdrop.local","otp":"778899","new_password":"ResetPassword123!"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password/reset", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify login works with new password
		loginPayload := []byte(`{"email":"customer@swiftdrop.local","password":"ResetPassword123!"}`)
		loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRec := httptest.NewRecorder()
		mux.ServeHTTP(loginRec, loginReq)

		if loginRec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK on login with new password, got %d: %s", loginRec.Code, loginRec.Body.String())
		}
	})

	t.Run("POST /forgot-password/reset returns 400 on invalid OTP", func(t *testing.T) {
		_ = otpRepo.SaveOTP(context.Background(), "reset:customer@swiftdrop.local", "123456")

		payload := []byte(`{"email":"customer@swiftdrop.local","otp":"000000","new_password":"ResetPassword123!"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password/reset", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
