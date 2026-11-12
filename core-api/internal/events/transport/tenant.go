package transport

import (
	"context"
	"net/http"
	"regexp"

	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type merchantIDContextKey struct{}

// GetMerchantID extracts the validated merchant UUID from request context.
func GetMerchantID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(merchantIDContextKey{}).(string)
	return val, ok
}

// TenantResolver enforces Decision D25:
// The Principal.TenantID must be a valid merchant UUID.
// If it is non-UUID (e.g. seeded mrc_01HMERCHANTTENANT00001), it halts with 403 TENANT_NOT_MERCHANT_UUID.
func TenantResolver() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := identitytransport.GetPrincipal(r.Context())
			if !ok || principal == nil || principal.TenantID == nil || *principal.TenantID == "" {
				httpx.WriteError(w, r, http.StatusForbidden, "MISSING_TENANT_ID", "tenant id required for merchant endpoints")
				return
			}

			tenantID := *principal.TenantID
			if !uuidRegex.MatchString(tenantID) {
				httpx.WriteError(w, r, http.StatusForbidden, "TENANT_NOT_MERCHANT_UUID", "tenant ID in JWT claim must be a valid merchant UUID")
				return
			}

			ctx := context.WithValue(r.Context(), merchantIDContextKey{}, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
