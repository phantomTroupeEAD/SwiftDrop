package transport

import (
	"net/http"

	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// RegisterRoutes registers parcel HTTP endpoints on mux protected by Bearer JWT authentication
// and role authorization.
func RegisterRoutes(mux *http.ServeMux, h *Handler, sec *identitytransport.SecurityMiddleware) {
	if mux == nil || h == nil || sec == nil {
		return
	}

	parcelRoles := identitytransport.RequireRole(
		identitydomain.RoleCustomer,
		identitydomain.RoleMerchant,
		identitydomain.RoleAdmin,
	)

	mux.Handle("POST /api/v1/parcels", sec.Authenticate(
		parcelRoles(http.HandlerFunc(h.BookParcel)),
	))

	mux.Handle("GET /api/v1/parcels/{id}", sec.Authenticate(
		parcelRoles(http.HandlerFunc(h.GetParcel)),
	))

	mux.Handle("GET /api/v1/parcels/{id}/history", sec.Authenticate(
		parcelRoles(http.HandlerFunc(h.GetParcelHistory)),
	))
}
