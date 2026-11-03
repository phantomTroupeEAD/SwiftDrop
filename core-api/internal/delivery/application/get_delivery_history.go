package application

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

// GetDeliveryHistoryQueryHandler retrieves chronological attempt records and generates 15-minute presigned URLs for photos.
type GetDeliveryHistoryQueryHandler struct {
	deliveryRepo  DeliveryAttemptRepository
	parcelUpdater ParcelLifecycleUpdater
	storageClient StorageClient
}

// NewGetDeliveryHistoryQueryHandler constructs the history query handler.
func NewGetDeliveryHistoryQueryHandler(
	deliveryRepo DeliveryAttemptRepository,
	parcelUpdater ParcelLifecycleUpdater,
	storageClient StorageClient,
) *GetDeliveryHistoryQueryHandler {
	return &GetDeliveryHistoryQueryHandler{
		deliveryRepo:  deliveryRepo,
		parcelUpdater: parcelUpdater,
		storageClient: storageClient,
	}
}

// Execute retrieves all delivery attempts for a given parcel.
func (h *GetDeliveryHistoryQueryHandler) Execute(ctx context.Context, parcelID string) (*DeliveryHistoryDTO, error) {
	parcelUUID, err := uuid.Parse(parcelID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	attempts, err := h.deliveryRepo.ListAttemptsByParcelID(ctx, parcelUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch attempts: %w", err)
	}
	if len(attempts) == 0 {
		return nil, ErrParcelNotFound
	}

	currentStatus, _, _ := h.parcelUpdater.GetStatusWithLock(ctx, nil, parcelUUID)

	var dtos []AttemptRecordDTO
	for _, a := range attempts {
		item := AttemptRecordDTO{
			AttemptID:     a.ID.String(),
			AttemptNumber: a.AttemptNumber,
			Outcome:       string(a.Outcome),
			Notes:         a.Notes,
			CourierID:     a.CourierID.String(),
			AttemptedAt:   a.AttemptedAt,
		}

		if a.FailureReason != nil {
			reasonStr := string(*a.FailureReason)
			item.ReasonCode = &reasonStr
		}

		if a.Proof != nil {
			proofTypeStr := string(a.Proof.Type)
			item.ProofType = &proofTypeStr

			if a.Proof.ReceivedByName != "" {
				nameStr := a.Proof.ReceivedByName
				item.ReceivedByName = &nameStr
			}

			// Generate short-lived presigned URL if photo proof is present
			if a.Proof.Type == domain.ProofTypePhoto && a.Proof.AttachmentURL != "" {
				presignedURL, err := h.storageClient.GeneratePresignedURL(ctx, a.Proof.AttachmentURL, 15*time.Minute)
				if err == nil {
					item.ProofAttachmentURL = &presignedURL
				} else {
					// Fallback to relative storage path if presigner offline
					pathStr := a.Proof.AttachmentURL
					item.ProofAttachmentURL = &pathStr
				}
			}
		}

		dtos = append(dtos, item)
	}

	return &DeliveryHistoryDTO{
		ParcelID:      parcelUUID.String(),
		CurrentStatus: currentStatus,
		AttemptCount:  len(dtos),
		Attempts:      dtos,
	}, nil
}

// UploadProofPhotoUseCase handles storing binary doorstep photos in object storage.
type UploadProofPhotoUseCase struct {
	storageClient StorageClient
}

// NewUploadProofPhotoUseCase constructs the upload use case.
func NewUploadProofPhotoUseCase(storageClient StorageClient) *UploadProofPhotoUseCase {
	return &UploadProofPhotoUseCase{storageClient: storageClient}
}

// Execute uploads the photo data and returns the permanent storage key.
func (uc *UploadProofPhotoUseCase) Execute(ctx context.Context, parcelID string, data io.Reader, contentType string) (string, error) {
	parcelUUID, err := uuid.Parse(parcelID)
	if err != nil {
		return "", fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	key := fmt.Sprintf("proofs/%s_%d.jpg", parcelUUID.String(), time.Now().UTC().Unix())
	storageURL, err := uc.storageClient.UploadProofPhoto(ctx, key, data, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload proof photo: %w", err)
	}

	return storageURL, nil
}

