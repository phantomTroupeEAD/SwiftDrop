package transport

import (
	"net/http"

	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// RegisterRoutes registers pricing HTTP endpoints on mux protected by Bearer JWT authentication
// and role authorization.
func RegisterRoutes(mux *http.ServeMux, h *Handler, sec *identitytransport.SecurityMiddleware) {
	if mux == nil || h == nil || sec == nil {
		return
	}

	mux.Handle("POST /api/v1/pricing/quotes", sec.Authenticate(
		identitytransport.RequireRole(
			identitydomain.RoleCustomer,
			identitydomain.RoleMerchant,
			identitydomain.RoleAdmin,
		)(http.HandlerFunc(h.CreateQuote)),
	))
}
