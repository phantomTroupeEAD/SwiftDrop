package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

// CompleteDeliveryUseCase records successful doorstep handover along with verified proof-of-delivery.
type CompleteDeliveryUseCase struct {
	txManager        TransactionManager
	assignmentFinder ActiveAssignmentFinder
	deliveryRepo     DeliveryAttemptRepository
	parcelUpdater    ParcelLifecycleUpdater
	outboxWriter     OutboxWriter
}

// NewCompleteDeliveryUseCase constructs the delivery completion use case.
func NewCompleteDeliveryUseCase(
	txManager TransactionManager,
	assignmentFinder ActiveAssignmentFinder,
	deliveryRepo DeliveryAttemptRepository,
	parcelUpdater ParcelLifecycleUpdater,
	outboxWriter OutboxWriter,
) *CompleteDeliveryUseCase {
	return &CompleteDeliveryUseCase{
		txManager:        txManager,
		assignmentFinder: assignmentFinder,
		deliveryRepo:     deliveryRepo,
		parcelUpdater:    parcelUpdater,
		outboxWriter:     outboxWriter,
	}
}

// Execute performs doorstep delivery completion with strict courier IDOR and proof verification.
func (uc *CompleteDeliveryUseCase) Execute(ctx context.Context, cmd CompleteDeliveryCommand) (*DeliveryCompletionResultDTO, error) {
	parcelUUID, err := uuid.Parse(cmd.ParcelID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	courierUUID, err := uuid.Parse(cmd.CourierID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	var proof *domain.ProofOfDelivery
	switch domain.ProofType(cmd.ProofType) {
	case domain.ProofTypePhoto:
		proof, err = domain.NewPhotoProof(cmd.ProofURL, cmd.ReceivedByName)
	case domain.ProofTypeSignature:
		proof, err = domain.NewSignatureProof(cmd.SignatureData, cmd.ReceivedByName)
	case domain.ProofTypeRecipientConfirmation:
		proof, err = domain.NewConfirmationProof(cmd.ReceivedByName)
	default:
		return nil, domain.ErrMissingDeliveryProof
	}
	if err != nil {
		return nil, err
	}

	var coords *domain.GeoCoordinate
	if cmd.Latitude != nil && cmd.Longitude != nil {
		coords, err = domain.NewGeoCoordinate(*cmd.Latitude, *cmd.Longitude)
		if err != nil {
			return nil, err
		}
	}

	var result *DeliveryCompletionResultDTO

	err = uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// 1. Fetch active courier assignment and enforce courier IDOR isolation (AT-006)
		asgn, err := uc.assignmentFinder.GetActiveAssignment(ctx, tx, parcelUUID)
		if err != nil {
			return fmt.Errorf("failed to fetch active assignment: %w", err)
		}
		if asgn == nil {
			return ErrParcelNotAssigned
		}
		if asgn.CourierID != courierUUID {
			return domain.ErrNotAssignedCourier
		}

		// 2. Lock and verify parcel status is OUT_FOR_DELIVERY
		currentStatus, attemptCount, err := uc.parcelUpdater.GetStatusWithLock(ctx, tx, parcelUUID)
		if err != nil {
			return fmt.Errorf("failed to load parcel status: %w", err)
		}
		if currentStatus != "OUT_FOR_DELIVERY" {
			return fmt.Errorf("%w: parcel must be in OUT_FOR_DELIVERY status (current: %s)", ErrParcelNotOutForDelivery, currentStatus)
		}

		// 3. Create immutable delivery attempt entity with verified proof
		attempt, err := domain.NewSuccessfulAttempt(
			parcelUUID,
			courierUUID,
			attemptCount+1,
			proof,
			coords,
			cmd.Notes,
		)
		if err != nil {
			return err
		}

		// 4. Insert delivery attempt record in delivery_attempts
		if err := uc.deliveryRepo.InsertAttempt(ctx, tx, attempt); err != nil {
			return fmt.Errorf("failed to record delivery attempt: %w", err)
		}

		// 5. Complete courier assignment (freeing courier capacity)
		if err := uc.assignmentFinder.CompleteAssignment(ctx, tx, asgn.AssignmentID); err != nil {
			return fmt.Errorf("failed to complete courier assignment: %w", err)
		}

		// 6. Transition parcel aggregate root state machine to DELIVERED
		err = uc.parcelUpdater.TransitionStatus(
			ctx,
			tx,
			parcelUUID,
			"OUT_FOR_DELIVERY",
			"DELIVERED",
			courierUUID,
			"COURIER",
			fmt.Sprintf("Handed over to %s with verified %s proof", cmd.ReceivedByName, cmd.ProofType),
		)
		if err != nil {
			return fmt.Errorf("failed to transition parcel status: %w", err)
		}

		// 7. Commit parcel.delivered.v1 outbox event
		deliveredAt := attempt.AttemptedAt
		eventPayload := map[string]any{
			"delivery_id":      attempt.ID.String(),
			"parcel_id":        parcelUUID.String(),
			"courier_id":       courierUUID.String(),
			"proof_type":       string(proof.Type),
			"proof_url":        proof.AttachmentURL,
			"received_by_name": proof.ReceivedByName,
			"delivered_at":     deliveredAt.Format(time.RFC3339),
		}
		err = uc.outboxWriter.Write(
			ctx,
			tx,
			"PARCEL",
			parcelUUID,
			1,
			"parcel.delivered.v1",
			eventPayload,
			attempt.ID.String(),
		)
		if err != nil {
			return fmt.Errorf("failed to commit delivered outbox event: %w", err)
		}

		result = &DeliveryCompletionResultDTO{
			DeliveryID:    attempt.ID.String(),
			ParcelID:      parcelUUID.String(),
			Status:        "DELIVERED",
			DeliveredAt:   deliveredAt,
			ProofVerified: true,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

