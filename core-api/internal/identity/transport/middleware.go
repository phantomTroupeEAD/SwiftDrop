package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

type contextKey string

const principalContextKey contextKey = "swiftdrop_principal"

// SecurityMiddleware bundles authentication and authorization middleware handlers.
type SecurityMiddleware struct {
	tokenProvider application.TokenProvider
}

// NewSecurityMiddleware constructs a security middleware handler.
func NewSecurityMiddleware(tokenProvider application.TokenProvider) *SecurityMiddleware {
	return &SecurityMiddleware{
		tokenProvider: tokenProvider,
	}
}

// Authenticate verifies the incoming Bearer JWT and injects the Principal into request context.
func (m *SecurityMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSONError(w, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Missing Authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeJSONError(w, http.StatusUnauthorized, "AUTH_MALFORMED_HEADER", "Authorization header must be 'Bearer <token>'")
			return
		}

		rawToken := strings.TrimSpace(parts[1])
		principal, err := m.tokenProvider.ValidateAccessToken(rawToken)
		if err != nil {
			if err == domain.ErrTokenExpired {
				writeJSONError(w, http.StatusUnauthorized, "AUTH_TOKEN_EXPIRED", "Access token has expired")
				return
			}
			writeJSONError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid access token")
			return
		}

		// Inject principal into request context
		ctx := context.WithValue(r.Context(), principalContextKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole ensures that the authenticated principal holds at least one of the specified roles.
func RequireRole(allowedRoles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := GetPrincipal(r.Context())
			if !ok || principal == nil {
				writeJSONError(w, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Authentication required")
				return
			}

			for _, role := range allowedRoles {
				if principal.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeJSONError(w, http.StatusForbidden, "AUTH_FORBIDDEN", "Insufficient role permissions for this resource")
		})
	}
}

// RequireTenant ensures that the authenticated actor has an associated tenant ID (e.g. for merchants).
func RequireTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := GetPrincipal(r.Context())
			if !ok || principal == nil || principal.TenantID == nil || *principal.TenantID == "" {
				writeJSONError(w, http.StatusForbidden, "AUTH_TENANT_REQUIRED", "Tenant isolation context required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Context Helpers

// GetPrincipal extracts the authenticated Principal from the context.
func GetPrincipal(ctx context.Context) (*application.Principal, bool) {
	p, ok := ctx.Value(principalContextKey).(*application.Principal)
	return p, ok
}

// GetUserID returns the authenticated subject user ID, or empty string.
func GetUserID(ctx context.Context) string {
	if p, ok := GetPrincipal(ctx); ok && p != nil {
		return p.UserID
	}
	return ""
}

// GetUserRole returns the authenticated subject role.
func GetUserRole(ctx context.Context) domain.Role {
	if p, ok := GetPrincipal(ctx); ok && p != nil {
		return p.Role
	}
	return ""
}

// GetTenantID returns the authenticated merchant tenant ID, or nil.
func GetTenantID(ctx context.Context) *string {
	if p, ok := GetPrincipal(ctx); ok && p != nil {
		return p.TenantID
	}
	return nil
}

func writeJSONError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(NewErrorEnvelope(code, message))
}
