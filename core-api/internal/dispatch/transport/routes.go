package transport

import (
	"net/http"

	identityDomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// RegisterRoutes registers all dispatch and fleet HTTP REST routes onto the provided ServeMux.
func RegisterRoutes(
	mux *http.ServeMux,
	dispatchHandler *DispatchHandler,
	courierHandler *CourierShiftHandler,
	security *identityTransport.SecurityMiddleware,
) {
	dispatcherOrAdmin := identityTransport.RequireRole(identityDomain.RoleAdmin, identityDomain.RoleDispatcher)
	courierOrStaff := identityTransport.RequireRole(identityDomain.RoleCourier, identityDomain.RoleDispatcher, identityDomain.RoleAdmin)

	// Dispatcher & Fleet Management Routes
	mux.Handle(
		"POST /api/v1/dispatch/couriers",
		security.Authenticate(dispatcherOrAdmin(http.HandlerFunc(dispatchHandler.RegisterCourier))),
	)

	mux.Handle(
		"GET /api/v1/dispatch/couriers",
		security.Authenticate(dispatcherOrAdmin(http.HandlerFunc(dispatchHandler.ListCouriers))),
	)

	mux.Handle(
		"GET /api/v1/dispatch/board",
		security.Authenticate(dispatcherOrAdmin(http.HandlerFunc(dispatchHandler.GetDispatchBoard))),
	)

	mux.Handle(
		"POST /api/v1/dispatch/assignments",
		security.Authenticate(dispatcherOrAdmin(http.HandlerFunc(dispatchHandler.AssignCourier))),
	)

	mux.Handle(
		"POST /api/v1/dispatch/assignments/{id}/cancel",
		security.Authenticate(dispatcherOrAdmin(http.HandlerFunc(dispatchHandler.CancelAssignment))),
	)

	mux.Handle(
		"POST /api/v1/dispatch/parcels/{id}/reschedule",
		security.Authenticate(dispatcherOrAdmin(http.HandlerFunc(dispatchHandler.RescheduleParcel))),
	)

	// Courier Shift State Management
	mux.Handle(
		"POST /api/v1/courier/shift",
		security.Authenticate(courierOrStaff(http.HandlerFunc(courierHandler.ToggleShift))),
	)
}

