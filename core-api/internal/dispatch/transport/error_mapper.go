package transport

import (
	"errors"
	"net/http"

	"github.com/swiftdrop/core-api/internal/dispatch/application"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// MapError translates dispatch domain/application errors into canonical HTTP status codes and payloads.
func MapError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, db.ErrNilDB):
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database connection is not available")
	case errors.Is(err, domain.ErrCourierOverloaded):
		httpx.WriteError(w, r, http.StatusConflict, "COURIER_CAPACITY_EXCEEDED", err.Error())
	case errors.Is(err, domain.ErrCourierNotOnShift):
		httpx.WriteError(w, r, http.StatusConflict, "COURIER_NOT_ON_SHIFT", err.Error())
	case errors.Is(err, domain.ErrCourierSuspended):
		httpx.WriteError(w, r, http.StatusForbidden, "COURIER_SUSPENDED", err.Error())
	case errors.Is(err, domain.ErrInvalidShiftTransition):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_SHIFT_TRANSITION", err.Error())
	case errors.Is(err, domain.ErrInvalidZoneID):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_ZONE_ID", err.Error())
	case errors.Is(err, domain.ErrInvalidVehicleType):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_VEHICLE_TYPE", err.Error())
	case errors.Is(err, domain.ErrAssignmentAlreadyClosed):
		httpx.WriteError(w, r, http.StatusConflict, "ASSIGNMENT_ALREADY_CLOSED", err.Error())

	case errors.Is(err, application.ErrCourierNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "COURIER_NOT_FOUND", err.Error())
	case errors.Is(err, application.ErrUserNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
	case errors.Is(err, application.ErrUserNotCourier):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "USER_NOT_A_COURIER", err.Error())
	case errors.Is(err, application.ErrCourierProfileExists):
		httpx.WriteError(w, r, http.StatusConflict, "COURIER_PROFILE_EXISTS", err.Error())
	case errors.Is(err, application.ErrAssignmentNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", err.Error())
	case errors.Is(err, application.ErrParcelNotAssigned):
		httpx.WriteError(w, r, http.StatusNotFound, "PARCEL_NOT_ASSIGNED", err.Error())
	case errors.Is(err, application.ErrDuplicateActiveAssignment):
		httpx.WriteError(w, r, http.StatusConflict, "PARCEL_ALREADY_ASSIGNED", err.Error())
	case errors.Is(err, application.ErrInvalidParcelStatusForAssignment), errors.Is(err, application.ErrParcelNotEligibleForAssignment):
		httpx.WriteError(w, r, http.StatusConflict, "INVALID_PARCEL_STATE", err.Error())
	case errors.Is(err, application.ErrMaxAttemptsExceeded):
		httpx.WriteError(w, r, http.StatusConflict, "MAX_ATTEMPTS_EXCEEDED", err.Error())
	case errors.Is(err, application.ErrInvalidCommand):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_COMMAND", err.Error())
	case errors.Is(err, application.ErrUnauthorized):
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error())

	default:
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected dispatch processing error occurred")
	}
}
