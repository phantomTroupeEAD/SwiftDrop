package transport

import (
	"net/http"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// RegisterRoutes registers all identity-related HTTP REST routes onto the provided router mux.
func RegisterRoutes(mux *http.ServeMux, authHandler *AuthHandler, security *SecurityMiddleware) {
	// ── Public Registration Endpoints (Verify-First OTP) ─────────────────────
	// Step 1: Check availability & send 6-digit OTP
	mux.HandleFunc("POST /api/v1/auth/register/send-otp", authHandler.SendRegistrationOTP)
	// Step 2: Verify OTP + complete registration
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)

	// ── Public Auth & Session Endpoints ──────────────────────────────────────
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/google", authHandler.GoogleLogin)
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)

	// ── Protected Auth Endpoints (Bearer JWT required) ────────────────────────
	mux.Handle("GET /api/v1/auth/me", security.Authenticate(http.HandlerFunc(authHandler.GetMe)))
	// PUT /me: used for Google Approach A progressive onboarding (phone + terms)
	mux.Handle("PUT /api/v1/auth/me", security.Authenticate(http.HandlerFunc(authHandler.UpdateMe)))
}

// RegisterAdminRoutes registers administrative governance and role management endpoints onto the router mux.
// All endpoints are strictly guarded with Bearer JWT authentication and RoleAdmin authorization.
func RegisterAdminRoutes(mux *http.ServeMux, adminHandler *AdminHandler, security *SecurityMiddleware) {
	adminGuard := func(handler http.HandlerFunc) http.Handler {
		return security.Authenticate(RequireRole(domain.RoleAdmin)(handler))
	}

	// ── Admin Governance Endpoints (RoleAdmin Only) ──────────────────────────
	// List users with role filtering & pagination
	mux.Handle("GET /api/v1/admin/users", adminGuard(adminHandler.ListUsers))
	// Promote or reassign user role & merchant tenant ID
	mux.Handle("PUT /api/v1/admin/users/{id}/role", adminGuard(adminHandler.UpdateRole))
	// Suspend or reactivate user account
	mux.Handle("PUT /api/v1/admin/users/{id}/status", adminGuard(adminHandler.UpdateStatus))
}
