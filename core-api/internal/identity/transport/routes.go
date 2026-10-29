package transport

import (
	"net/http"
)

// RegisterRoutes registers all identity-related HTTP REST routes onto the provided router mux.
func RegisterRoutes(mux *http.ServeMux, authHandler *AuthHandler, security *SecurityMiddleware) {
	// Public REST endpoints
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)

	// Protected REST endpoints (require valid Bearer JWT)
	mux.Handle("GET /api/v1/auth/me", security.Authenticate(http.HandlerFunc(authHandler.GetMe)))
}
