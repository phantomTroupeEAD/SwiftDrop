package transport

import (
	"net/http"

	identityDomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

// RegisterRoutes registers all doorstep delivery and manifest HTTP REST routes onto the provided ServeMux.
func RegisterRoutes(
	mux *http.ServeMux,
	deliveryHandler *DeliveryHandler,
	manifestHandler *ManifestHandler,
	security *identityTransport.SecurityMiddleware,
) {
	courierOnly := identityTransport.RequireRole(identityDomain.RoleCourier)
	courierOrStaff := identityTransport.RequireRole(identityDomain.RoleCourier, identityDomain.RoleDispatcher, identityDomain.RoleAdmin)

	// Courier Manifest & Delivery Runs
	mux.Handle(
		"GET /api/v1/courier/parcels",
		security.Authenticate(courierOnly(http.HandlerFunc(manifestHandler.GetManifest))),
	)

	mux.Handle(
		"POST /api/v1/courier/parcels/{id}/start-run",
		security.Authenticate(courierOnly(http.HandlerFunc(manifestHandler.StartRun))),
	)

	mux.Handle(
		"POST /api/v1/courier/runs/start-batch",
		security.Authenticate(courierOnly(http.HandlerFunc(manifestHandler.StartBatchRun))),
	)

	// Doorstep Delivery Execution & Proof Attachment
	mux.Handle(
		"POST /api/v1/deliveries/{id}/proof-photo",
		security.Authenticate(courierOnly(http.HandlerFunc(deliveryHandler.UploadProofPhoto))),
	)

	mux.Handle(
		"POST /api/v1/deliveries/{id}/complete",
		security.Authenticate(courierOnly(http.HandlerFunc(deliveryHandler.CompleteDelivery))),
	)

	mux.Handle(
		"POST /api/v1/deliveries/{id}/failed",
		security.Authenticate(courierOnly(http.HandlerFunc(deliveryHandler.RecordFailedAttempt))),
	)

	// Historical Attempt Lookup (Accessible to courier, dispatcher, and admin)
	mux.Handle(
		"GET /api/v1/deliveries/parcels/{id}",
		security.Authenticate(courierOrStaff(http.HandlerFunc(deliveryHandler.GetDeliveryHistory))),
	)
}

