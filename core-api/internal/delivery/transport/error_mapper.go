package transport

import (
	"errors"
	"net/http"

	"github.com/swiftdrop/core-api/internal/delivery/application"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// MapError translates delivery domain/application errors into canonical HTTP status codes and payloads.
func MapError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	switch {
	// Courier IDOR violation (AT-006)
	case errors.Is(err, domain.ErrNotAssignedCourier):
		httpx.WriteError(w, r, http.StatusForbidden, "COURIER_NOT_ASSIGNED", err.Error())

	// Proof of Delivery validation errors
	case errors.Is(err, domain.ErrMissingDeliveryProof):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "DELIVERY_PROOF_REQUIRED", err.Error())
	case errors.Is(err, domain.ErrInvalidProofRecipient):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_RECIPIENT_NAME", err.Error())
	case errors.Is(err, domain.ErrInvalidSignatureData):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_SIGNATURE_DATA", err.Error())

	// Failure Reason validation errors (ADR-004)
	case errors.Is(err, domain.ErrInvalidFailureReason):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_FAILURE_REASON", err.Error())
	case errors.Is(err, domain.ErrDetailedNoteRequired):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "DETAILED_NOTE_REQUIRED", err.Error())
	case errors.Is(err, domain.ErrInvalidGeoCoordinate):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_COORDINATES", err.Error())
	case errors.Is(err, domain.ErrAttemptCeilingExceeded):
		httpx.WriteError(w, r, http.StatusConflict, "ATTEMPT_CEILING_EXCEEDED", err.Error())

	// Application state & assignment errors
	case errors.Is(err, application.ErrParcelNotAssigned):
		httpx.WriteError(w, r, http.StatusNotFound, "PARCEL_NOT_ASSIGNED", err.Error())
	case errors.Is(err, application.ErrParcelNotOutForDelivery):
		httpx.WriteError(w, r, http.StatusConflict, "PARCEL_NOT_OUT_FOR_DELIVERY", err.Error())
	case errors.Is(err, application.ErrInvalidCommand):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_COMMAND", err.Error())

	// Photo upload errors
	case errors.Is(err, application.ErrEmptyPhotoUpload):
		httpx.WriteError(w, r, http.StatusBadRequest, "EMPTY_PHOTO_UPLOAD", err.Error())
	case errors.Is(err, application.ErrInvalidPhotoFormat):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_PHOTO_FORMAT", err.Error())
	case errors.Is(err, application.ErrPhotoTooLarge):
		httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, "PHOTO_TOO_LARGE", err.Error())

	default:
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected delivery processing error occurred")
	}
}

