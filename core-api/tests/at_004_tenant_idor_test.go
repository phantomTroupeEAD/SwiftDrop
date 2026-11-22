package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityinfra "github.com/swiftdrop/core-api/internal/identity/infrastructure"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

func TestAcceptance_AT004_TenantIDOR(t *testing.T) {
	mux := http.NewServeMux()
	tokenProv := identityinfra.NewJWTTokenProvider(TestJWTSecret)
	security := identitytransport.NewSecurityMiddleware(tokenProv)

	// Simulated tenant-scoped order retrieval endpoint verifying tenant ownership
	mux.Handle("GET /api/v1/merchants/{tenant_id}/orders/{order_id}", security.Authenticate(
		identitytransport.RequireRole(identitydomain.RoleMerchant, identitydomain.RoleAdmin)(
			identitytransport.RequireTenant()(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					callerTenant := identitytransport.GetTenantID(r.Context())
					requestedTenant := r.PathValue("tenant_id")

					// INVARIANT: IDOR defense - caller's token tenant MUST match requested tenant resource!
					if callerTenant == nil || *callerTenant != requestedTenant {
						w.WriteHeader(http.StatusForbidden)
						_ = json.NewEncoder(w).Encode(map[string]string{
							"error":   "FORBIDDEN",
							"message": "cross-tenant access prohibited",
						})
						return
					}

					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"order_id":  r.PathValue("order_id"),
						"tenant_id": *callerTenant,
						"status":    "ACTIVE",
					})
				}),
			),
		),
	))

	t.Run("1. Unauthenticated Request is Barred with 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/merchants/mrc_01HMERCHANTTENANT00001/orders/ord-001", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("2. Customer Token without Tenant ID is Barred with 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/merchants/mrc_01HMERCHANTTENANT00001/orders/ord-001", nil)
		req.Header.Set("Authorization", "Bearer "+MockCustomerToken(t))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code, "customer role without tenant must be barred from merchant resources")
	})

	t.Run("3. Merchant Alpha Accessing Own Tenant Resource Succeeds (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/merchants/mrc_01HMERCHANTTENANT00001/orders/ord-001", nil)
		req.Header.Set("Authorization", "Bearer "+MockMerchantAlphaToken(t))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		require.NoError(t, err)
		assert.Equal(t, SeedMerchantAlphaTID, body["tenant_id"])
	})

	t.Run("4. IDOR Defense: Merchant Alpha Accessing Merchant Beta Resource is Barred (403 Forbidden)", func(t *testing.T) {
		// Merchant Alpha token attempts to access Merchant Beta's tenant path:
		req := httptest.NewRequest(http.MethodGet, "/api/v1/merchants/mrc_02HMERCHANTTENANT00002/orders/ord-002", nil)
		req.Header.Set("Authorization", "Bearer "+MockMerchantAlphaToken(t))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code, "Merchant Alpha must never be permitted to read Merchant Beta resources")
	})

	t.Run("5. IDOR Defense: Merchant Beta Accessing Merchant Alpha Resource is Barred (403 Forbidden)", func(t *testing.T) {
		// Reverse IDOR check: Merchant Beta token attempts to access Merchant Alpha's tenant path:
		req := httptest.NewRequest(http.MethodGet, "/api/v1/merchants/mrc_01HMERCHANTTENANT00001/orders/ord-001", nil)
		req.Header.Set("Authorization", "Bearer "+MockMerchantBetaToken(t))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code, "Merchant Beta must never be permitted to read Merchant Alpha resources")
	})
}
