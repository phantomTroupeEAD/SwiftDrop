package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/swiftdrop/tracking/internal/domain"
	_ "github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/service"
)

// PublicTrackingHandler handles public parcel tracking HTTP lookups.
type PublicTrackingHandler struct {
	svc    *service.PublicTrackingService
	logger *slog.Logger
}

// NewPublicTrackingHandler constructs a new PublicTrackingHandler.
func NewPublicTrackingHandler(svc *service.PublicTrackingService, logger *slog.Logger) *PublicTrackingHandler {
	return &PublicTrackingHandler{
		svc:    svc,
		logger: logger,
	}
}

// GetPublicTracking handles GET /api/v1/tracking/public/{tracking_number}.
//
// @Summary      Get public parcel tracking details
// @Description  Retrieves current status, destination city, and sanitized chronological milestone timeline. Strictly redacts all PII per SEC-009.
// @Tags         tracking
// @Accept       json
// @Produce      json
// @Param        tracking_number  path      string  true  "Tracking Number (SD-YYYYMMDD-XXXXXX)"
// @Success      200              {object}  SingleDataEnvelope[dto.PublicTrackingResponse]
// @Failure      400              {object}  ErrorEnvelope  "Invalid tracking number format"
// @Failure      404              {object}  ErrorEnvelope  "Shipment not found"
// @Failure      503              {object}  ErrorEnvelope  "Database service unavailable"
// @Router       /api/v1/tracking/public/{tracking_number} [get]
func (h *PublicTrackingHandler) GetPublicTracking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Check early context cancellation
	if ctx.Err() != nil {
		return
	}

	// 2. Extract path parameter using Go 1.22+ ServeMux PathValue
	trackingNumber := r.PathValue("tracking_number")
	if trackingNumber == "" {
		// Fallback for sub-path routing or test frameworks
		trackingNumber = extractTrackingNumberFromPath(r.URL.Path)
	}

	trackingNumber = strings.TrimSpace(trackingNumber)
	if trackingNumber == "" {
		WriteError(w, r, http.StatusBadRequest, "INVALID_TRACKING_NUMBER",
			"Tracking number path parameter is required.",
			ErrorDetail{Field: "tracking_number", Issue: "missing tracking number parameter"},
		)
		return
	}

	if h.svc == nil {
		h.logger.Error("PublicTrackingService is nil (unconfigured database dependency)")
		WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
			"Tracking data temporarily unavailable.",
		)
		return
	}

	// 3. Delegate to application service
	result, err := h.svc.GetPublicTracking(ctx, trackingNumber)
	if err != nil {
		if errors.Is(ctx.Err(), http.ErrHandlerTimeout) || errors.Is(err, http.ErrHandlerTimeout) {
			return
		}
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return
		}

		if errors.Is(err, domain.ErrInvalidTrackingNumber) {
			WriteError(w, r, http.StatusBadRequest, "INVALID_TRACKING_NUMBER",
				"Tracking number does not conform to the required format (SD-YYYYMMDD-XXXXXX).",
				ErrorDetail{Field: "tracking_number", Issue: err.Error()},
			)
			return
		}

		if errors.Is(err, domain.ErrTrackingNotFound) {
			WriteError(w, r, http.StatusNotFound, "TRACKING_NOT_FOUND",
				fmt.Sprintf("Shipment not found for tracking number %s", trackingNumber),
			)
			return
		}

		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			h.logger.Error("Database unavailable during tracking lookup", "tracking_number", trackingNumber, "error", err)
			WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
				"Tracking data temporarily unavailable.",
			)
			return
		}

		h.logger.Error("Unexpected error during tracking lookup", "tracking_number", trackingNumber, "error", err)
		WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
			"Tracking data temporarily unavailable.",
		)
		return
	}

	// 4. Return canonical JSON envelope {"data": ...}
	_ = WriteData(w, http.StatusOK, result)
}

func extractTrackingNumberFromPath(path string) string {
	const prefix = "/api/v1/tracking/public/"
	if strings.HasPrefix(path, prefix) {
		trimmed := strings.TrimPrefix(path, prefix)
		// Strip query parameters or trailing slashes if present
		if idx := strings.Index(trimmed, "/"); idx != -1 {
			trimmed = trimmed[:idx]
		}
		if idx := strings.Index(trimmed, "?"); idx != -1 {
			trimmed = trimmed[:idx]
		}
		return trimmed
	}
	return ""
}
