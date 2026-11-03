package application

import (
	"errors"
	"time"
)

var (
	ErrParcelNotAssigned       = errors.New("parcel is not in ASSIGNED status")
	ErrParcelNotOutForDelivery = errors.New("parcel is not in OUT_FOR_DELIVERY status")
	ErrEmptyParcelList         = errors.New("parcel IDs list cannot be empty")
	ErrInvalidCommand          = errors.New("invalid delivery command parameters")
	ErrParcelNotFound          = errors.New("parcel not found or has no delivery records")
)

// StartRunCommand carries input parameters to confirm package pickup and begin the run.
type StartRunCommand struct {
	ParcelID     string `json:"parcel_id"`
	CourierID    string `json:"courier_id"`
	DepotHubCode string `json:"depot_hub_code"`
	Notes        string `json:"notes"`
}

// StartBatchRunCommand enables batch collection and run initiation for multiple assigned parcels.
type StartBatchRunCommand struct {
	ParcelIDs    []string `json:"parcel_ids"`
	CourierID    string   `json:"courier_id"`
	DepotHubCode string   `json:"depot_hub_code"`
}

// BatchRunResultDTO reports the outcome of a batch start-run action.
type BatchRunResultDTO struct {
	TotalStarted int      `json:"total_started"`
	StartedAt    time.Time `json:"started_at"`
	ParcelIDs    []string `json:"parcel_ids"`
}

// CompleteDeliveryCommand carries proof and handover information for a successful delivery.
type CompleteDeliveryCommand struct {
	ParcelID       string   `json:"parcel_id"`
	CourierID      string   `json:"courier_id"`
	ProofType      string   `json:"proof_type"`
	ProofURL       string   `json:"proof_url"`
	SignatureData  string   `json:"signature_data"`
	ReceivedByName string   `json:"received_by_name"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	Notes          string   `json:"notes"`
}

// DeliveryCompletionResultDTO confirms successful delivery completion.
type DeliveryCompletionResultDTO struct {
	DeliveryID    string    `json:"delivery_id"`
	ParcelID      string    `json:"parcel_id"`
	Status        string    `json:"status"`
	DeliveredAt   time.Time `json:"delivered_at"`
	ProofVerified bool      `json:"proof_verified"`
}

// RecordFailedAttemptCommand carries failure classification and operational notes for an unsuccessful attempt.
type RecordFailedAttemptCommand struct {
	ParcelID   string   `json:"parcel_id"`
	CourierID  string   `json:"courier_id"`
	ReasonCode string   `json:"reason_code"`
	Latitude   *float64 `json:"latitude"`
	Longitude  *float64 `json:"longitude"`
	Notes      string   `json:"notes"`
}

// FailedAttemptResultDTO reports the recorded failure attempt outcome and next state machine step.
type FailedAttemptResultDTO struct {
	AttemptNumber int       `json:"attempt_number"`
	MaxAttempts   int       `json:"max_attempts"`
	Status        string    `json:"status"`
	NextAction    string    `json:"next_action"`
	RecordedAt    time.Time `json:"recorded_at"`
}

// AttemptRecordDTO represents an item in a parcel's historical delivery attempt timeline.
type AttemptRecordDTO struct {
	AttemptID          string    `json:"attempt_id"`
	AttemptNumber      int       `json:"attempt_number"`
	Outcome            string    `json:"outcome"`
	ReasonCode         *string   `json:"reason_code,omitempty"`
	ProofType          *string   `json:"proof_type,omitempty"`
	ProofAttachmentURL *string   `json:"proof_attachment_url,omitempty"`
	ReceivedByName     *string   `json:"received_by_name,omitempty"`
	Notes              string    `json:"notes"`
	CourierID          string    `json:"courier_id"`
	AttemptedAt        time.Time `json:"attempted_at"`
}

// DeliveryHistoryDTO returns the consolidated attempt history for a parcel.
type DeliveryHistoryDTO struct {
	ParcelID      string             `json:"parcel_id"`
	CurrentStatus string             `json:"current_status"`
	AttemptCount  int                `json:"attempt_count"`
	Attempts      []AttemptRecordDTO `json:"attempts"`
}

