package transport

import (
	"time"

	"github.com/swiftdrop/core-api/internal/delivery/application"
)

// PhotoUploadResponse describes the uploaded proof photo in object storage.
type PhotoUploadResponse struct {
	PhotoURL    string    `json:"photo_url"`
	StorageKey  string    `json:"storage_key"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// DeliveryCompletionResponse describes the successful delivery handover.
type DeliveryCompletionResponse struct {
	AttemptID   string    `json:"attempt_id"`
	ParcelID    string    `json:"parcel_id"`
	Status      string    `json:"status"`
	Outcome     string    `json:"outcome"`
	CompletedAt time.Time `json:"completed_at"`
}

// FailedAttemptResponse describes the recorded failed attempt outcome.
type FailedAttemptResponse struct {
	AttemptID         string    `json:"attempt_id"`
	ParcelID          string    `json:"parcel_id"`
	AttemptNumber     int       `json:"attempt_number"`
	MaxAttempts       int       `json:"max_attempts"`
	Outcome           string    `json:"outcome"`
	FailureReasonCode string    `json:"failure_reason_code"`
	ParcelStatus      string    `json:"parcel_status"`
	IsTerminal        bool      `json:"is_terminal"`
	AttemptedAt       time.Time `json:"attempted_at"`
}

// StartRunResponse describes the individual parcel out-for-delivery update.
type StartRunResponse struct {
	ParcelID  string    `json:"parcel_id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

// BatchStartRunResponse describes the result of starting a batch delivery run.
type BatchStartRunResponse struct {
	StartedParcels []string `json:"started_parcels"`
	FailedParcels  []string `json:"failed_parcels"`
	TotalStarted   int      `json:"total_started"`
}

// DeliveryHistoryResponse wraps the chronological attempt logs and current status.
type DeliveryHistoryResponse struct {
	ParcelID      string                         `json:"parcel_id"`
	CurrentStatus string                         `json:"current_status"`
	AttemptCount  int                            `json:"attempt_count"`
	Attempts      []application.AttemptRecordDTO `json:"attempts"`
}
