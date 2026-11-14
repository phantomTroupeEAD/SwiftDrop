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

// DispatchHandler handles HTTP requests for fleet management and parcel dispatch operations.
type DispatchHandler struct {
	registerCourierUC  *application.RegisterCourierUseCase
	listCouriersQ      *application.ListCouriersQueryHandler
	dispatchBoardQ     *application.GetDispatchBoardQueryHandler
	assignCourierUC    *application.AssignCourierUseCase
	cancelAssignmentUC *application.CancelAssignmentUseCase
	rescheduleParcelUC *application.RescheduleParcelUseCase
}

// NewDispatchHandler constructs a DispatchHandler instance with all required use cases.
func NewDispatchHandler(
	registerCourierUC *application.RegisterCourierUseCase,
	listCouriersQ *application.ListCouriersQueryHandler,
	dispatchBoardQ *application.GetDispatchBoardQueryHandler,
	assignCourierUC *application.AssignCourierUseCase,
	cancelAssignmentUC *application.CancelAssignmentUseCase,
	rescheduleParcelUC *application.RescheduleParcelUseCase,
) *DispatchHandler {
	return &DispatchHandler{
		registerCourierUC:  registerCourierUC,
		listCouriersQ:      listCouriersQ,
		dispatchBoardQ:     dispatchBoardQ,
		assignCourierUC:    assignCourierUC,
		cancelAssignmentUC: cancelAssignmentUC,
		rescheduleParcelUC: rescheduleParcelUC,
	}
}

// RegisterCourier handles POST /api/v1/dispatch/couriers
func (h *DispatchHandler) RegisterCourier(w http.ResponseWriter, r *http.Request) {
	var req RegisterCourierRequest
	if err := httpx.DecodeJSON(w, r, &req, 1024*64); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Malformed or invalid JSON body")
		return
	}

	if details := req.Validate(); len(details) > 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed for courier profile", details...)
		return
	}

	cmd := application.RegisterCourierCommand{
		UserID:        req.UserID,
		EmployeeCode:  req.EmployeeCode,
		VehicleType:   domain.VehicleType(req.VehicleType),
		MaxCapacity:   req.MaxCapacity,
		CurrentZoneID: req.CurrentZoneID,
	}

	courier, err := h.registerCourierUC.Execute(r.Context(), cmd)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := CourierProfileResponse{
		UserID:        courier.UserID.String(),
		EmployeeCode:  courier.EmployeeCode,
		VehicleType:   courier.Vehicle.String(),
		MaxCapacity:   courier.MaxCapacity,
		Status:        courier.Status.String(),
		CurrentZoneID: courier.CurrentZoneID,
		IsActive:      courier.IsActive,
		CreatedAt:     courier.CreatedAt,
	}

	_ = httpx.WriteData(w, http.StatusCreated, resp)
}

// ListCouriers handles GET /api/v1/dispatch/couriers?zone_id=...
func (h *DispatchHandler) ListCouriers(w http.ResponseWriter, r *http.Request) {
	zoneID := strings.TrimSpace(r.URL.Query().Get("zone_id"))

	loads, err := h.listCouriersQ.Execute(r.Context(), zoneID, nil, nil)
	if err != nil {
		MapError(w, r, err)
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, loads)
}

// GetDispatchBoard handles GET /api/v1/dispatch/board?zone_id=...
func (h *DispatchHandler) GetDispatchBoard(w http.ResponseWriter, r *http.Request) {
	zoneID := strings.TrimSpace(r.URL.Query().Get("zone_id"))

	board, err := h.dispatchBoardQ.Execute(r.Context(), zoneID)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := DispatchBoardResponse{
		ZoneID:            board.ZoneID,
		UnassignedParcels: board.UnassignedParcels,
		AssignedParcels:   board.AssignedParcels,
		CourierLoads:      board.ActiveCouriers,
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}

// AssignCourier handles POST /api/v1/dispatch/assignments
func (h *DispatchHandler) AssignCourier(w http.ResponseWriter, r *http.Request) {
	var req AssignParcelRequest
	if err := httpx.DecodeJSON(w, r, &req, 1024*64); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Malformed or invalid JSON body")
		return
	}

	if details := req.Validate(); len(details) > 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed for assignment request", details...)
		return
	}

	actorID := identityTransport.GetUserID(r.Context())

	cmd := application.AssignCourierCommand{
		ParcelID:   req.ParcelID,
		CourierID:  req.CourierID,
		AssignedBy: actorID,
		Notes:      req.Notes,
	}

	dto, err := h.assignCourierUC.Execute(r.Context(), cmd)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := AssignmentResponse{
		AssignmentID: dto.AssignmentID,
		ParcelID:     dto.ParcelID,
		CourierID:    dto.CourierID,
		AssignedBy:   dto.AssignedBy,
		Status:       dto.Status,
		AssignedAt:   dto.AssignedAt,
	}

	_ = httpx.WriteData(w, http.StatusCreated, resp)
}

// CancelAssignment handles POST /api/v1/dispatch/assignments/{id}/cancel
func (h *DispatchHandler) CancelAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID := r.PathValue("id")
	if strings.TrimSpace(assignmentID) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_ASSIGNMENT_ID", "Assignment ID is required in URL path")
		return
	}

	var req CancelAssignmentRequest
	if err := httpx.DecodeJSON(w, r, &req, 1024*64); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Malformed or invalid JSON body")
		return
	}

	if err := req.Validate(); err != nil {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
		return
	}

	actorID := identityTransport.GetUserID(r.Context())

	cmd := application.CancelAssignmentCommand{
		ParcelID:    assignmentID,
		CancelledBy: actorID,
		Reason:      req.Reason,
	}

	err := h.cancelAssignmentUC.Execute(r.Context(), cmd)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := CancelAssignmentResponse{
		AssignmentID: assignmentID,
		ParcelID:     assignmentID,
		Status:       "CANCELLED",
		EndedAt:      time.Now().UTC(),
		Reason:       req.Reason,
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}

// RescheduleParcel handles POST /api/v1/dispatch/parcels/{id}/reschedule
func (h *DispatchHandler) RescheduleParcel(w http.ResponseWriter, r *http.Request) {
	parcelID := r.PathValue("id")
	if strings.TrimSpace(parcelID) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_PARCEL_ID", "Parcel ID is required in URL path")
		return
	}

	var req RescheduleParcelRequest
	if err := httpx.DecodeJSON(w, r, &req, 1024*64); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Malformed or invalid JSON body")
		return
	}

	if details := req.Validate(); len(details) > 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed for reschedule request", details...)
		return
	}

	actorID := identityTransport.GetUserID(r.Context())

	cmd := application.RescheduleParcelCommand{
		ParcelID:    parcelID,
		CourierID:   req.CourierID,
		ScheduledBy: actorID,
		Notes:       req.Notes,
	}

	dto, err := h.rescheduleParcelUC.Execute(r.Context(), cmd)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := AssignmentResponse{
		AssignmentID: dto.AssignmentID,
		ParcelID:     dto.ParcelID,
		CourierID:    dto.CourierID,
		AssignedBy:   dto.AssignedBy,
		Status:       dto.Status,
		AssignedAt:   dto.AssignedAt,
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}
