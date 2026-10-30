package transport

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// AuthHandler handles HTTP requests for the authentication lifecycle.
type AuthHandler struct {
	authService *application.AuthService
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(authService *application.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Email and password are required")
		return
	}

	cmd := application.LoginCommand{
		Email:     req.Email,
		Password:  req.Password,
		ClientIP:  r.RemoteAddr,
		UserAgent: r.UserAgent(),
	}

	result, err := h.authService.Login(r.Context(), cmd)
	if err != nil {
		switch err {
		case domain.ErrInvalidCredentials:
			writeJSONError(w, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "Invalid email or password")
		case domain.ErrAccountSuspended:
			writeJSONError(w, http.StatusForbidden, "AUTH_ACCOUNT_SUSPENDED", "Account has been suspended")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Authentication failed due to internal error")
		}
		return
	}

	// Set HttpOnly refresh token cookie (ADR-003)
	http.SetCookie(w, &http.Cookie{
		Name:     "swiftdrop_refresh_token",
		Value:    result.RefreshToken,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   false, // Set to true in TLS/production
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 3600, // 7 days
	})

	response := SingleDataEnvelope{
		Data: LoginDataDTO{
			AccessToken: result.AccessToken,
			TokenType:   result.TokenType,
			ExpiresIn:   result.ExpiresIn,
			User: UserDTO{
				ID:          result.User.ID,
				Email:       result.User.Email.String(),
				DisplayName: result.User.DisplayName,
				Role:        result.User.Role,
				TenantID:    result.User.TenantID,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// GetMe handles GET /api/v1/auth/me (Protected by AuthMiddleware)
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	principal, ok := GetPrincipal(r.Context())
	if !ok || principal == nil {
		writeJSONError(w, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Unauthorized")
		return
	}

	response := SingleDataEnvelope{
		Data: map[string]interface{}{
			"id":        principal.UserID,
			"email":     principal.Email,
			"role":      principal.Role,
			"tenant_id": principal.TenantID,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
