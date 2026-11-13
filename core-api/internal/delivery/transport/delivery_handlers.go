package transport

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/swiftdrop/core-api/internal/delivery/application"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// DeliveryHandler handles doorstep delivery attempts, proof uploads, and completion operations.
type DeliveryHandler struct {
	uploadPhotoUC         *application.UploadProofPhotoUseCase
	completeDeliveryUC    *application.CompleteDeliveryUseCase
	recordFailedAttemptUC *application.RecordFailedAttemptUseCase
	getDeliveryHistoryQ   *application.GetDeliveryHistoryQueryHandler
}

// NewDeliveryHandler constructs a DeliveryHandler instance.
func NewDeliveryHandler(
	uploadPhotoUC *application.UploadProofPhotoUseCase,
	completeDeliveryUC *application.CompleteDeliveryUseCase,
	recordFailedAttemptUC *application.RecordFailedAttemptUseCase,
	getDeliveryHistoryQ *application.GetDeliveryHistoryQueryHandler,
) *DeliveryHandler {
	return &DeliveryHandler{
		uploadPhotoUC:         uploadPhotoUC,
		completeDeliveryUC:    completeDeliveryUC,
		recordFailedAttemptUC: recordFailedAttemptUC,
		getDeliveryHistoryQ:   getDeliveryHistoryQ,
	}
}

// UploadProofPhoto handles POST /api/v1/deliveries/{id}/proof-photo
func (h *DeliveryHandler) UploadProofPhoto(w http.ResponseWriter, r *http.Request) {
	parcelID := r.PathValue("id")
	if strings.TrimSpace(parcelID) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_PARCEL_ID", "Parcel ID is required in URL path")
		return
	}

	// Limit request size to 6MB for multipart file upload
	r.Body = http.MaxBytesReader(w, r.Body, 6*1024*1024)
	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_MULTIPART_FORM", fmt.Sprintf("Failed to parse form: %v", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MISSING_FILE", "A multipart file named 'file' is required")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	actorID := identityTransport.GetUserID(r.Context())

	cmd := application.UploadProofPhotoCommand{
		ParcelID:    parcelID,
		CourierID:   actorID,
		FileName:    header.Filename,
		ContentType: contentType,
		FileReader:  file,
		FileSize:    header.Size,
	}

	dto, err := h.uploadPhotoUC.Execute(r.Context(), cmd)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := PhotoUploadResponse{
		PhotoURL:    dto.PhotoURL,
		StorageKey:  dto.StorageKey,
		ContentType: dto.ContentType,
		UploadedAt:  dto.UploadedAt,
	}

	_ = httpx.WriteData(w, http.StatusCreated, resp)
}

// CompleteDelivery handles POST /api/v1/deliveries/{id}/complete
func (h *DeliveryHandler) CompleteDelivery(w http.ResponseWriter, r *http.Request) {
	parcelID := r.PathValue("id")
	if strings.TrimSpace(parcelID) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_PARCEL_ID", "Parcel ID is required in URL path")
		return
	}

	var req CompleteDeliveryRequest
	if err := httpx.DecodeJSON(w, r, &req, 1024*64); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Malformed or invalid JSON body")
		return
	}

	if details := req.Validate(); len(details) > 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed for delivery completion", details...)
		return
	}

	actorID := identityTransport.GetUserID(r.Context())

	cmd := application.CompleteDeliveryCommand{
		ParcelID:       parcelID,
		CourierID:      actorID,
		ProofType:      req.ProofType,
		ProofURL:       req.ProofURL,
		SignatureData:  req.SignatureData,
		ReceivedByName: req.ReceivedByName,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		Notes:          req.Notes,
	}

	dto, err := h.completeDeliveryUC.Execute(r.Context(), cmd)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := DeliveryCompletionResponse{
		AttemptID:   dto.DeliveryID,
		ParcelID:    dto.ParcelID,
		Status:      dto.Status,
		Outcome:     "SUCCESS",
		CompletedAt: dto.DeliveredAt,
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}

// RecordFailedAttempt handles POST /api/v1/deliveries/{id}/failed
func (h *DeliveryHandler) RecordFailedAttempt(w http.ResponseWriter, r *http.Request) {
	parcelID := r.PathValue("id")
	if strings.TrimSpace(parcelID) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_PARCEL_ID", "Parcel ID is required in URL path")
		return
	}

	var req RecordFailedAttemptRequest
	if err := httpx.DecodeJSON(w, r, &req, 1024*32); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Malformed or invalid JSON body")
		return
	}

	if details := req.Validate(); len(details) > 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Validation failed for failed attempt record", details...)
		return
	}

	actorID := identityTransport.GetUserID(r.Context())

	cmd := application.RecordFailedAttemptCommand{
		ParcelID:   parcelID,
		CourierID:  actorID,
		ReasonCode: req.FailureReasonCode,
		Notes:      req.Notes,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
	}

	dto, err := h.recordFailedAttemptUC.Execute(r.Context(), cmd)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := FailedAttemptResponse{
		AttemptID:         dto.AttemptID,
		ParcelID:          dto.ParcelID,
		AttemptNumber:     dto.AttemptNumber,
		MaxAttempts:       dto.MaxAttempts,
		Outcome:           dto.Outcome,
		FailureReasonCode: dto.FailureReasonCode,
		ParcelStatus:      dto.ParcelStatus,
		IsTerminal:        dto.IsTerminal,
		AttemptedAt:       dto.AttemptedAt,
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}

// GetDeliveryHistory handles GET /api/v1/deliveries/parcels/{id}
func (h *DeliveryHandler) GetDeliveryHistory(w http.ResponseWriter, r *http.Request) {
	parcelID := r.PathValue("id")
	if strings.TrimSpace(parcelID) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_PARCEL_ID", "Parcel ID is required in URL path")
		return
	}

	history, err := h.getDeliveryHistoryQ.Execute(r.Context(), parcelID)
	if err != nil {
		MapError(w, r, err)
		return
	}

	resp := DeliveryHistoryResponse{
		ParcelID:      history.ParcelID,
		CurrentStatus: history.CurrentStatus,
		AttemptCount:  history.AttemptCount,
		Attempts:      history.Attempts,
	}

	_ = httpx.WriteData(w, http.StatusOK, resp)
}
