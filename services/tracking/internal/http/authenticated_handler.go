package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/domain"
	_ "github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/service"
)

// AuthenticatedTrackingHandler handles private authenticated parcel tracking HTTP lookups.
type AuthenticatedTrackingHandler struct {
	svc      *service.AuthenticatedTrackingService
	verifier auth.TokenVerifier
	logger   *slog.Logger
}

// NewAuthenticatedTrackingHandler constructs an AuthenticatedTrackingHandler.
func NewAuthenticatedTrackingHandler(
	svc *service.AuthenticatedTrackingService,
	verifier auth.TokenVerifier,
	logger *slog.Logger,
) *AuthenticatedTrackingHandler {
	return &AuthenticatedTrackingHandler{
		svc:      svc,
		verifier: verifier,
		logger:   logger,
	}
}

// GetAuthenticatedTracking handles GET /api/v1/tracking/{tracking_number}.
//
// @Summary      Get authenticated parcel tracking history
// @Description  Retrieves full parcel lifecycle and status history for authorized parcel owners (customer or merchant) or operations staff. Strictly enforces server-side database ownership checks.
// @Tags         tracking
// @Accept       json
// @Produce      json
// @Param        tracking_number  path      string  true  "Tracking Number (SD-YYYYMMDD-XXXXXX)"
// @Security     BearerAuth
// @Success      200              {object}  SingleDataEnvelope[dto.AuthenticatedTrackingResponse]
// @Failure      400              {object}  ErrorEnvelope  "Invalid tracking number format"
// @Failure      401              {object}  ErrorEnvelope  "Missing, expired, or invalid Bearer access token"
// @Failure      403              {object}  ErrorEnvelope  "Access denied - caller does not own this parcel"
// @Failure      404              {object}  ErrorEnvelope  "Shipment not found"
// @Failure      503              {object}  ErrorEnvelope  "Database service unavailable"
// @Router       /api/v1/tracking/{tracking_number} [get]
func (h *AuthenticatedTrackingHandler) GetAuthenticatedTracking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Check early context cancellation
	if ctx.Err() != nil {
		return
	}

	// 2. Extract Bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Authorization header required.")
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Authorization header must be 'Bearer <token>'.")
		return
	}

	rawToken := strings.TrimSpace(parts[1])

	if h.verifier == nil {
		h.logger.Error("TokenVerifier is unconfigured in AuthenticatedTrackingHandler")
		WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Authentication service unavailable.")
		return
	}

	// 3. Cryptographically verify the token and extract Principal
	principal, err := h.verifier.VerifyToken(rawToken)
	if err != nil {
		if errors.Is(err, auth.ErrTokenExpired) {
			WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Access token has expired.")
			return
		}
		WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Invalid or malformed access token.")
		return
	}

	// 4. Extract path parameter using Go 1.22+ ServeMux PathValue
	trackingNumber := r.PathValue("tracking_number")
	if trackingNumber == "" {
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
		h.logger.Error("AuthenticatedTrackingService is nil (unconfigured database dependency)")
		WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
			"Tracking data temporarily unavailable.",
		)
		return
	}

	// 5. Delegate to application service with server-side ownership authorization
	result, err := h.svc.GetAuthenticatedTracking(ctx, trackingNumber, principal)
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

		if errors.Is(err, domain.ErrAccessDenied) {
			WriteError(w, r, http.StatusForbidden, "ACCESS_DENIED",
				fmt.Sprintf("Access denied for tracking number %s", trackingNumber),
			)
			return
		}

		if errors.Is(err, domain.ErrUnauthenticated) {
			WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED",
				"Authentication required.",
			)
			return
		}

		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			h.logger.Error("Tracking repository unavailable during authenticated lookup",
				"tracking_number", trackingNumber,
				"error", err,
			)
			WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
				"Tracking data temporarily unavailable.",
			)
			return
		}

		h.logger.Error("Unhandled error during authenticated tracking lookup",
			"tracking_number", trackingNumber,
			"error", err,
		)
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR",
			"Internal server error.",
		)
		return
	}

	// 6. Return standard SingleDataEnvelope response
	_ = WriteData(w, http.StatusOK, result)
}
