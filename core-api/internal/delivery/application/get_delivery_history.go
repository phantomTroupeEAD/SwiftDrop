package application

import (
	"context"
	"fmt"
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

// UploadProofPhotoUseCase handles storing binary doorstep photos in object storage with assigned courier verification.
type UploadProofPhotoUseCase struct {
	storageClient    StorageClient
	assignmentFinder ActiveAssignmentFinder
}

// NewUploadProofPhotoUseCase constructs the upload use case.
func NewUploadProofPhotoUseCase(storageClient StorageClient, assignmentFinder ActiveAssignmentFinder) *UploadProofPhotoUseCase {
	return &UploadProofPhotoUseCase{
		storageClient:    storageClient,
		assignmentFinder: assignmentFinder,
	}
}

// Execute uploads the photo data and returns the permanent storage key after validating courier assignment (AT-006).
func (uc *UploadProofPhotoUseCase) Execute(ctx context.Context, cmd UploadProofPhotoCommand) (*UploadProofPhotoResultDTO, error) {
	parcelUUID, err := uuid.Parse(cmd.ParcelID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	courierUUID, err := uuid.Parse(cmd.CourierID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	// 1. Verify active courier assignment (AT-006 IDOR check)
	if uc.assignmentFinder != nil {
		asgn, err := uc.assignmentFinder.GetActiveAssignment(ctx, nil, parcelUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch assignment: %w", err)
		}
		if asgn == nil {
			return nil, ErrParcelNotAssigned
		}
		if asgn.CourierID != courierUUID {
			return nil, domain.ErrNotAssignedCourier
		}
	}

	contentType := cmd.ContentType
	if contentType == "" {
		contentType = "image/jpeg"
	}

	now := time.Now().UTC()
	key := fmt.Sprintf("proofs/%s_%d.jpg", parcelUUID.String(), now.Unix())
	storageKey, err := uc.storageClient.UploadProofPhoto(ctx, key, cmd.FileReader, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to upload proof photo: %w", err)
	}

	photoURL, err := uc.storageClient.GeneratePresignedURL(ctx, storageKey, 15*time.Minute)
	if err != nil || photoURL == "" {
		photoURL = storageKey
	}

	return &UploadProofPhotoResultDTO{
		PhotoURL:    photoURL,
		StorageKey:  storageKey,
		ContentType: contentType,
		UploadedAt:  now,
	}, nil
}
