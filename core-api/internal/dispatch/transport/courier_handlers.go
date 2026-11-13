package transport

import (
	"net/http"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/dispatch/application"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// CourierShiftHandler handles courier shift state transitions.
type CourierShiftHandler struct {
	toggleShiftUC *application.ToggleShiftUseCase
}

// NewCourierShiftHandler constructs the shift handler.
func NewCourierShiftHandler(toggleShiftUC *application.ToggleShiftUseCase) *CourierShiftHandler {
	return &CourierShiftHandler{
		toggleShiftUC: toggleShiftUC,
	}
}

// ToggleShift handles POST /api/v1/courier/shift
func (h *CourierShiftHandler) ToggleShift(w http.ResponseWriter, r *http.Request) {
	var req ToggleShiftRequest
	if err := httpx.DecodeJSON(w, r, &req, 1024*16); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Malformed or invalid JSON body")
		return
	}

	if details := req.Validate(); len(details) > 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed for shift toggle", details...)
		return
	}

	actorID := identityTransport.GetUserID(r.Context())
	actorRole := string(identityTransport.GetUserRole(r.Context()))

	courierID := actorID
	// If an admin or dispatcher provides a specific courier_id in the body, allow administrative override
	if (actorRole == "ADMIN" || actorRole == "DISPATCHER") && strings.TrimSpace(req.CourierID) != "" {
		courierID = strings.TrimSpace(req.CourierID)
	}

	isSelfService := (actorRole == "COURIER")
	cmd := application.ToggleShiftCommand{
		CourierID:     courierID,
		TargetStatus:  domain.ShiftStatus(req.TargetStatus),
		IsSelfService: isSelfService,
	}

	courier, err := h.toggleShiftUC.Execute(r.Context(), cmd)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := ShiftStatusResponse{
		CourierID: courier.UserID.String(),
		NewStatus: courier.Status.String(),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}
