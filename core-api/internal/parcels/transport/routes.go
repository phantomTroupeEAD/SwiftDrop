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

	bookingRoles := identitytransport.RequireRole(
		identitydomain.RoleCustomer,
		identitydomain.RoleMerchant,
		identitydomain.RoleAdmin,
	)

	viewRoles := identitytransport.RequireRole(
		identitydomain.RoleCustomer,
		identitydomain.RoleMerchant,
		identitydomain.RoleAdmin,
		identitydomain.RoleDispatcher,
	)

	mux.Handle("POST /api/v1/parcels", sec.Authenticate(
		bookingRoles(http.HandlerFunc(h.BookParcel)),
	))

	mux.Handle("GET /api/v1/parcels", sec.Authenticate(
		viewRoles(http.HandlerFunc(h.ListParcels)),
	))

	mux.Handle("GET /api/v1/parcels/{id}", sec.Authenticate(
		viewRoles(http.HandlerFunc(h.GetParcel)),
	))

	mux.Handle("GET /api/v1/parcels/{id}/history", sec.Authenticate(
		viewRoles(http.HandlerFunc(h.GetParcelHistory)),
	))

	mux.Handle("GET /api/v1/parcels/{id}/receipt", sec.Authenticate(
		viewRoles(http.HandlerFunc(h.GetParcelReceipt)),
	))
}
