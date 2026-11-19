package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// AuthHandler handles HTTP requests for the authentication and registration lifecycle.
type AuthHandler struct {
	authService *application.AuthService
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(authService *application.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// ─────────────────────────────────────────────────────
// Registration Handlers
// ─────────────────────────────────────────────────────

// SendRegistrationOTP handles POST /api/v1/auth/register/send-otp
func (h *AuthHandler) SendRegistrationOTP(w http.ResponseWriter, r *http.Request) {
	var req SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", "email is required")
		return
	}

	result, err := h.authService.SendRegistrationOTP(r.Context(), application.SendRegistrationOTPCommand{
		Email: req.Email,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			writeJSONError(w, http.StatusBadRequest, "INVALID_EMAIL", "Email format is invalid")
		case errors.Is(err, domain.ErrEmailAlreadyExists):
			writeJSONError(w, http.StatusConflict, "AUTH_EMAIL_ALREADY_EXISTS", "An account with this email already exists")
		case errors.Is(err, domain.ErrOTPCooldownActive):
			writeJSONError(w, http.StatusTooManyRequests, "OTP_COOLDOWN_ACTIVE", "Please wait 60 seconds before requesting another code")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to send verification code")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: OTPDataDTO{
			Message:         "Verification code sent to your email",
			Email:           result.Email,
			ExpiresIn:       result.ExpiresIn,
			CooldownSeconds: result.CooldownSeconds,
		},
	})
}

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.OTP) == "" ||
		strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" ||
		strings.TrimSpace(req.Phone) == "" || strings.TrimSpace(req.Password) == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", "email, otp, first_name, last_name, phone, and password are all required")
		return
	}

	if !req.TermsAccepted {
		writeJSONError(w, http.StatusBadRequest, "TERMS_REQUIRED", "You must accept the terms and conditions to register")
		return
	}

	result, err := h.authService.Register(r.Context(), application.RegisterCustomerCommand{
		Email:         req.Email,
		OTP:           req.OTP,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		Phone:         req.Phone,
		Password:      req.Password,
		TermsAccepted: req.TermsAccepted,
		ClientIP:      r.RemoteAddr,
		UserAgent:     r.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			writeJSONError(w, http.StatusBadRequest, "INVALID_EMAIL", "Email format is invalid")
		case errors.Is(err, domain.ErrWeakPassword):
			writeJSONError(w, http.StatusBadRequest, "WEAK_PASSWORD",
				"Password must be at least 8 characters with uppercase, lowercase, digit, and symbol")
		case errors.Is(err, domain.ErrInvalidPhone):
			writeJSONError(w, http.StatusBadRequest, "INVALID_PHONE", "Phone must be in E.164 format, e.g. +94771234567")
		case errors.Is(err, domain.ErrOTPInvalid):
			writeJSONError(w, http.StatusBadRequest, "AUTH_OTP_INVALID", "Verification code is incorrect")
		case errors.Is(err, domain.ErrOTPExpired):
			writeJSONError(w, http.StatusBadRequest, "AUTH_OTP_EXPIRED", "Verification code has expired, please request a new one")
		case errors.Is(err, domain.ErrOTPMaxAttempts):
			writeJSONError(w, http.StatusBadRequest, "AUTH_OTP_MAX_ATTEMPTS", "Too many invalid attempts, please request a new code")
		case errors.Is(err, domain.ErrTermsNotAccepted):
			writeJSONError(w, http.StatusBadRequest, "TERMS_REQUIRED", "You must accept the terms and conditions to register")
		case errors.Is(err, domain.ErrEmailAlreadyExists):
			writeJSONError(w, http.StatusConflict, "AUTH_EMAIL_ALREADY_EXISTS", "An account with this email already exists")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Registration failed due to an internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: RegisterDataDTO{
			User:      toUserDTO(result.User),
			IsNewUser: result.IsNewUser,
		},
	})
}

// ─────────────────────────────────────────────────────
// Login & Session Rotation Handlers
// ─────────────────────────────────────────────────────

// Login godoc
// @Summary      Authenticate user and issue JWT
// @Description  Validates credentials and returns access token and HttpOnly refresh token cookie.
// @Tags         identity
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "User credentials"
// @Success      200 {object} SingleDataEnvelope
// @Failure      400 {object} ErrorEnvelope "Malformed JSON or validation error"
// @Failure      401 {object} ErrorEnvelope "Invalid email or password"
// @Failure      403 {object} ErrorEnvelope "Account has been suspended"
// @Failure      500 {object} ErrorEnvelope "Authentication failed due to internal error"
// @Router       /api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
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
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			writeJSONError(w, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "Invalid email or password")
		case errors.Is(err, domain.ErrAccountSuspended):
			writeJSONError(w, http.StatusForbidden, "AUTH_ACCOUNT_SUSPENDED", "Account has been suspended")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Authentication failed due to internal error")
		}
		return
	}

	setRefreshTokenCookie(w, result.RefreshToken)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: LoginDataDTO{
			AccessToken: result.AccessToken,
			TokenType:   result.TokenType,
			ExpiresIn:   result.ExpiresIn,
			User:        toUserDTO(result.User),
		},
	})
}

// GoogleLogin handles POST /api/v1/auth/google
func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	var req GoogleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if strings.TrimSpace(req.IDToken) == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", "id_token is required")
		return
	}

	cmd := application.GoogleLoginCommand{
		IDToken:   req.IDToken,
		ClientIP:  r.RemoteAddr,
		UserAgent: r.UserAgent(),
	}

	result, err := h.authService.LoginWithGoogle(r.Context(), cmd)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidToken):
			writeJSONError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid Google ID token")
		case errors.Is(err, domain.ErrInvalidEmail):
			writeJSONError(w, http.StatusBadRequest, "INVALID_EMAIL", "Email provided by Google is invalid")
		case errors.Is(err, domain.ErrAccountSuspended):
			writeJSONError(w, http.StatusForbidden, "AUTH_ACCOUNT_SUSPENDED", "Account has been suspended")
		case errors.Is(err, domain.ErrInvalidCredentials):
			writeJSONError(w, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "Linked account could not be found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Google authentication failed")
		}
		return
	}

	setRefreshTokenCookie(w, result.RefreshToken)

	status := http.StatusOK
	if result.IsNewUser {
		status = http.StatusCreated
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: GoogleLoginDataDTO{
			AccessToken:   result.AccessToken,
			TokenType:     result.TokenType,
			ExpiresIn:     result.ExpiresIn,
			User:          toUserDTO(result.User),
			IsNewUser:     result.IsNewUser,
			RequiresPhone: result.RequiresPhone,
		},
	})
}

// Refresh handles POST /api/v1/auth/refresh (ADR-003: Single-Use Token Rotation)
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var rawToken string

	// 1. Read from HttpOnly cookie first
	if cookie, err := r.Cookie("swiftdrop_refresh_token"); err == nil && cookie.Value != "" {
		rawToken = cookie.Value
	}

	// 2. Fallback to JSON body
	if rawToken == "" && r.Body != nil {
		var req RefreshTokenRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		rawToken = req.RefreshToken
	}

	if strings.TrimSpace(rawToken) == "" {
		writeJSONError(w, http.StatusUnauthorized, "AUTH_REFRESH_TOKEN_REQUIRED", "Refresh token cookie or payload is required")
		return
	}

	cmd := application.RefreshSessionCommand{
		RawRefreshToken: rawToken,
		ClientIP:        r.RemoteAddr,
		UserAgent:       r.UserAgent(),
	}

	result, err := h.authService.RefreshSession(r.Context(), cmd)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTokenReused):
			// Security alert: Replay attack detected! Invalidate cookie immediately.
			clearRefreshTokenCookie(w)
			writeJSONError(w, http.StatusUnauthorized, "AUTH_TOKEN_REUSED", "Token reuse detected; all sessions in this family have been revoked")
		case errors.Is(err, domain.ErrTokenRevoked):
			clearRefreshTokenCookie(w)
			writeJSONError(w, http.StatusUnauthorized, "AUTH_TOKEN_REVOKED", "Refresh token has been revoked")
		case errors.Is(err, domain.ErrTokenExpired):
			clearRefreshTokenCookie(w)
			writeJSONError(w, http.StatusUnauthorized, "AUTH_TOKEN_EXPIRED", "Refresh token has expired; please log in again")
		case errors.Is(err, domain.ErrAccountSuspended):
			writeJSONError(w, http.StatusForbidden, "AUTH_ACCOUNT_SUSPENDED", "Account has been suspended")
		default:
			clearRefreshTokenCookie(w)
			writeJSONError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid refresh token")
		}
		return
	}

	// Set rotated refresh token cookie
	setRefreshTokenCookie(w, result.NewRefreshToken)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: LoginDataDTO{
			AccessToken: result.AccessToken,
			TokenType:   result.TokenType,
			ExpiresIn:   result.ExpiresIn,
			User:        toUserDTO(result.User),
		},
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var rawToken string
	if cookie, err := r.Cookie("swiftdrop_refresh_token"); err == nil && cookie.Value != "" {
		rawToken = cookie.Value
	}
	if rawToken == "" && r.Body != nil {
		var req RefreshTokenRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		rawToken = req.RefreshToken
	}

	_ = h.authService.Logout(r.Context(), application.LogoutCommand{
		RawRefreshToken: rawToken,
	})

	clearRefreshTokenCookie(w)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: map[string]string{
			"message": "Successfully logged out",
		},
	})
}

// ─────────────────────────────────────────────────────
// Profile Handlers
// ─────────────────────────────────────────────────────

// GetMe godoc
// @Summary      Get authenticated user profile
// @Description  Returns user profile and tenant details for the authenticated bearer token.
// @Tags         identity
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} SingleDataEnvelope
// @Failure      401 {object} ErrorEnvelope "Unauthorized"
// @Router       /api/v1/auth/me [get]
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := GetPrincipal(r.Context())
	if !ok || principal == nil {
		writeJSONError(w, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Unauthorized")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: map[string]interface{}{
			"id":        principal.UserID,
			"email":     principal.Email,
			"role":      principal.Role,
			"tenant_id": principal.TenantID,
		},
	})
}

// UpdateMe handles PUT /api/v1/auth/me (Protected by AuthMiddleware)
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := GetPrincipal(r.Context())
	if !ok || principal == nil {
		writeJSONError(w, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Unauthorized")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if strings.TrimSpace(req.Phone) == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", "phone is required")
		return
	}

	user, err := h.authService.UpdateProfile(r.Context(), application.UpdateProfileCommand{
		UserID:        principal.UserID,
		Phone:         req.Phone,
		TermsAccepted: req.TermsAccepted,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			writeJSONError(w, http.StatusNotFound, "USER_NOT_FOUND", "User account not found")
		case errors.Is(err, domain.ErrInvalidPhone):
			writeJSONError(w, http.StatusBadRequest, "INVALID_PHONE", "Phone must be in E.164 format, e.g. +94771234567")
		case errors.Is(err, domain.ErrTermsNotAccepted):
			writeJSONError(w, http.StatusBadRequest, "TERMS_REQUIRED", "You must accept the terms and conditions")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Profile update failed")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{Data: toUserDTO(user)})
}

// ─────────────────────────────────────────────────────
// Password Recovery Handlers
// ─────────────────────────────────────────────────────

// ForgotPasswordSendOTP handles POST /api/v1/auth/forgot-password/send-otp
func (h *AuthHandler) ForgotPasswordSendOTP(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordSendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", "email is required")
		return
	}

	result, err := h.authService.SendPasswordResetOTP(r.Context(), application.SendPasswordResetOTPCommand{
		Email: req.Email,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			writeJSONError(w, http.StatusBadRequest, "INVALID_EMAIL", "Email format is invalid")
		case errors.Is(err, domain.ErrOTPCooldownActive):
			writeJSONError(w, http.StatusTooManyRequests, "OTP_COOLDOWN_ACTIVE", "Please wait 60 seconds before requesting another code")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to process password reset request")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: OTPDataDTO{
			Message:         "If an account with that email exists, a password reset code has been sent",
			Email:           result.Email,
			ExpiresIn:       result.ExpiresIn,
			CooldownSeconds: result.CooldownSeconds,
		},
	})
}

// ResetPassword handles POST /api/v1/auth/forgot-password/reset
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", "Malformed JSON body")
		return
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.OTP) == "" || strings.TrimSpace(req.NewPassword) == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", "email, otp, and new_password are all required")
		return
	}

	err := h.authService.ResetPassword(r.Context(), application.ResetPasswordCommand{
		Email:       req.Email,
		OTP:         req.OTP,
		NewPassword: req.NewPassword,
		ClientIP:    r.RemoteAddr,
		UserAgent:   r.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			writeJSONError(w, http.StatusBadRequest, "INVALID_EMAIL", "Email format is invalid")
		case errors.Is(err, domain.ErrWeakPassword):
			writeJSONError(w, http.StatusBadRequest, "WEAK_PASSWORD",
				"Password must be at least 8 characters with uppercase, lowercase, digit, and symbol")
		case errors.Is(err, domain.ErrOTPInvalid):
			writeJSONError(w, http.StatusBadRequest, "AUTH_OTP_INVALID", "Verification code is incorrect")
		case errors.Is(err, domain.ErrOTPExpired):
			writeJSONError(w, http.StatusBadRequest, "AUTH_OTP_EXPIRED", "Verification code has expired, please request a new one")
		case errors.Is(err, domain.ErrOTPMaxAttempts):
			writeJSONError(w, http.StatusBadRequest, "AUTH_OTP_MAX_ATTEMPTS", "Too many invalid attempts, please request a new code")
		case errors.Is(err, domain.ErrAccountSuspended):
			writeJSONError(w, http.StatusForbidden, "ACCOUNT_SUSPENDED", "This account is suspended and cannot reset its password")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reset password")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SingleDataEnvelope{
		Data: MessageDataDTO{
			Message: "Password has been reset successfully. You can now sign in with your new password.",
		},
	})
}

// ─────────────────────────────────────────────────────
// Cookie Helpers
// ─────────────────────────────────────────────────────

func setRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "swiftdrop_refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   false, // set to true in TLS/production
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 3600, // 7 days
	})
}

func clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "swiftdrop_refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
