package transport

import (
	"net/http"
	"strings"

	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/delivery/application"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// ManifestHandler handles courier delivery manifests and delivery run commencement.
type ManifestHandler struct {
	manifestQ  *application.GetCourierManifestQueryHandler
	startRunUC *application.StartDeliveryRunUseCase
}

// NewManifestHandler constructs a ManifestHandler instance.
func NewManifestHandler(
	manifestQ *application.GetCourierManifestQueryHandler,
	startRunUC *application.StartDeliveryRunUseCase,
) *ManifestHandler {
	return &ManifestHandler{
		manifestQ:  manifestQ,
		startRunUC: startRunUC,
	}
}

// GetManifest handles GET /api/v1/courier/parcels
func (h *ManifestHandler) GetManifest(w http.ResponseWriter, r *http.Request) {
	courierID := identityTransport.GetUserID(r.Context())
	if strings.TrimSpace(courierID) == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required to access courier manifest")
		return
	}

	manifest, err := h.manifestQ.Execute(r.Context(), courierID)
	if err != nil {
		MapError(w, r, err)
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, manifest)
}

// StartRun handles POST /api/v1/courier/parcels/{id}/start-run
func (h *ManifestHandler) StartRun(w http.ResponseWriter, r *http.Request) {
	parcelID := r.PathValue("id")
	if strings.TrimSpace(parcelID) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_PARCEL_ID", "Parcel ID is required in URL path")
		return
	}

	courierID := identityTransport.GetUserID(r.Context())

	dto, err := h.startRunUC.ExecuteSingle(r.Context(), parcelID, courierID)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := StartRunResponse{
		ParcelID:  dto.ParcelID,
		Status:    dto.Status,
		StartedAt: dto.StartedAt,
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}

// StartBatchRun handles POST /api/v1/courier/runs/start-batch
func (h *ManifestHandler) StartBatchRun(w http.ResponseWriter, r *http.Request) {
	var req StartBatchRunRequest
	if err := httpx.DecodeJSON(w, r, &req, 1024*64); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Malformed or invalid JSON body")
		return
	}

	if details := req.Validate(); len(details) > 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed for batch run", details...)
		return
	}

	courierID := identityTransport.GetUserID(r.Context())

	dto, err := h.startRunUC.ExecuteBatch(r.Context(), courierID, req.ParcelIDs)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := BatchStartRunResponse{
		StartedParcels: dto.StartedParcels,
		FailedParcels:  dto.FailedParcels,
		TotalStarted:   dto.TotalStarted,
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}

