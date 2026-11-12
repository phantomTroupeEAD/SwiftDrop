package transport

import (
	"strings"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// StartBatchRunRequest represents the JSON payload to mark a batch of assigned parcels as OUT_FOR_DELIVERY.
type StartBatchRunRequest struct {
	ParcelIDs []string `json:"parcel_ids"`
}

// Validate checks batch parcel ID invariants.
func (r *StartBatchRunRequest) Validate() []httpx.ErrorDetail {
	var details []httpx.ErrorDetail
	if len(r.ParcelIDs) == 0 {
		details = append(details, httpx.ErrorDetail{Field: "parcel_ids", Issue: "Must provide at least one parcel ID"})
		return details
	}
	if len(r.ParcelIDs) > 100 {
		details = append(details, httpx.ErrorDetail{Field: "parcel_ids", Issue: "Batch limit exceeded (maximum 100 parcels)"})
		return details
	}

	for i, id := range r.ParcelIDs {
		if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
			details = append(details, httpx.ErrorDetail{Field: "parcel_ids", Issue: "Invalid parcel UUID at index"})
			break
		}
		_ = i
	}
	return details
}

// CompleteDeliveryRequest represents the JSON payload to record a successful doorstep delivery.
type CompleteDeliveryRequest struct {
	ProofType      string   `json:"proof_type"`
	ProofURL       string   `json:"proof_url,omitempty"`
	SignatureData  string   `json:"signature_data,omitempty"`
	ReceivedByName string   `json:"received_by_name"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

// Validate checks delivery completion fields.
func (r *CompleteDeliveryRequest) Validate() []httpx.ErrorDetail {
	var details []httpx.ErrorDetail
	if strings.TrimSpace(r.ProofType) == "" {
		details = append(details, httpx.ErrorDetail{Field: "proof_type", Issue: "Proof type is required (PHOTO, SIGNATURE, RECIPIENT_CONFIRMATION)"})
	}
	if strings.TrimSpace(r.ReceivedByName) == "" {
		details = append(details, httpx.ErrorDetail{Field: "received_by_name", Issue: "Recipient name is required"})
	}
	return details
}

// RecordFailedAttemptRequest represents the JSON payload to log an unsuccessful delivery attempt.
type RecordFailedAttemptRequest struct {
	FailureReasonCode string   `json:"failure_reason_code"`
	Notes             string   `json:"notes"`
	Latitude          *float64 `json:"latitude,omitempty"`
	Longitude         *float64 `json:"longitude,omitempty"`
}

// Validate checks failure attempt fields.
func (r *RecordFailedAttemptRequest) Validate() []httpx.ErrorDetail {
	var details []httpx.ErrorDetail
	if strings.TrimSpace(r.FailureReasonCode) == "" {
		details = append(details, httpx.ErrorDetail{Field: "failure_reason_code", Issue: "Failure reason code is required"})
	}
	return details
}
