package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

// RecordFailedAttemptUseCase manages unsuccessful doorstep delivery attempts under ADR-004 semantics.
type RecordFailedAttemptUseCase struct {
	txManager        TransactionManager
	assignmentFinder ActiveAssignmentFinder
	deliveryRepo     DeliveryAttemptRepository
	parcelUpdater    ParcelLifecycleUpdater
	outboxWriter     OutboxWriter
}

// NewRecordFailedAttemptUseCase constructs the failure recording use case.
func NewRecordFailedAttemptUseCase(
	txManager TransactionManager,
	assignmentFinder ActiveAssignmentFinder,
	deliveryRepo DeliveryAttemptRepository,
	parcelUpdater ParcelLifecycleUpdater,
	outboxWriter OutboxWriter,
) *RecordFailedAttemptUseCase {
	return &RecordFailedAttemptUseCase{
		txManager:        txManager,
		assignmentFinder: assignmentFinder,
		deliveryRepo:     deliveryRepo,
		parcelUpdater:    parcelUpdater,
		outboxWriter:     outboxWriter,
	}
}

// Execute logs an immutable attempt record and enforces the ADR-004 3-attempt ceiling.
func (uc *RecordFailedAttemptUseCase) Execute(ctx context.Context, cmd RecordFailedAttemptCommand) (*FailedAttemptResultDTO, error) {
	parcelUUID, err := uuid.Parse(cmd.ParcelID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	courierUUID, err := uuid.Parse(cmd.CourierID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	reasonCode := domain.FailureReasonCode(cmd.ReasonCode)
	if err := reasonCode.Validate(cmd.Notes); err != nil {
		return nil, err
	}

	var coords *domain.GeoCoordinate
	if cmd.Latitude != nil && cmd.Longitude != nil {
		coords, err = domain.NewGeoCoordinate(*cmd.Latitude, *cmd.Longitude)
		if err != nil {
			return nil, err
		}
	}

	var result *FailedAttemptResultDTO

	err = uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// 1. Fetch active assignment & enforce IDOR check (AT-006)
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

		// 2. Lock and verify parcel status
		currentStatus, previousAttempts, err := uc.parcelUpdater.GetStatusWithLock(ctx, tx, parcelUUID)
		if err != nil {
			return fmt.Errorf("failed to load parcel status: %w", err)
		}
		if currentStatus != "OUT_FOR_DELIVERY" {
			return fmt.Errorf("%w: parcel must be in OUT_FOR_DELIVERY status (current: %s)", ErrParcelNotOutForDelivery, currentStatus)
		}

		newAttemptNumber := previousAttempts + 1
		attempt, err := domain.NewFailedAttempt(
			parcelUUID,
			courierUUID,
			newAttemptNumber,
			reasonCode,
			coords,
			cmd.Notes,
		)
		if err != nil {
			return err
		}

		// 3. Insert immutable attempt record
		if err := uc.deliveryRepo.InsertAttempt(ctx, tx, attempt); err != nil {
			return fmt.Errorf("failed to insert delivery attempt: %w", err)
		}

		// 4. ADR-004 Decision: Evaluate terminal status vs recoverable attempt failure
		if attempt.IsTerminal() {
			// Complete courier assignment (free courier workload)
			if err := uc.assignmentFinder.CompleteAssignment(ctx, tx, asgn.AssignmentID); err != nil {
				return fmt.Errorf("failed to close assignment: %w", err)
			}

			// Transition parcel aggregate root to terminal RETURNED
			err = uc.parcelUpdater.TransitionStatus(
				ctx,
				tx,
				parcelUUID,
				"OUT_FOR_DELIVERY",
				"RETURNED",
				courierUUID,
				"COURIER",
				fmt.Sprintf("Terminal return triggered: attempt %d/3 or refusal (%s)", newAttemptNumber, reasonCode),
			)
			if err != nil {
				return fmt.Errorf("failed to transition parcel status: %w", err)
			}

			// Emit parcel.returned.v1 outbox event
			eventPayload := map[string]any{
				"parcel_id":         parcelUUID.String(),
				"courier_id":        courierUUID.String(),
				"total_attempts":    newAttemptNumber,
				"final_reason_code": string(reasonCode),
				"returned_at":       attempt.AttemptedAt.Format(time.RFC3339),
			}
			err = uc.outboxWriter.Write(
				ctx,
				tx,
				"PARCEL",
				parcelUUID,
				1,
				"parcel.returned.v1",
				eventPayload,
				attempt.ID.String(),
			)
			if err != nil {
				return fmt.Errorf("failed to commit returned outbox event: %w", err)
			}

			result = &FailedAttemptResultDTO{
				AttemptID:         attempt.ID.String(),
				ParcelID:          parcelUUID.String(),
				AttemptNumber:     newAttemptNumber,
				MaxAttempts:       3,
				Outcome:           "FAILED",
				FailureReasonCode: string(reasonCode),
				ParcelStatus:      "RETURNED",
				Status:            "RETURNED",
				IsTerminal:        true,
				NextAction:        "RETURN_TO_HUB_OR_SENDER",
				AttemptedAt:       attempt.AttemptedAt,
				RecordedAt:        attempt.AttemptedAt,
			}
		} else {
			// Transient recoverable failure: Parcel pauses in ATTEMPT_FAILED pending dispatcher review
			err = uc.parcelUpdater.TransitionStatus(
				ctx,
				tx,
				parcelUUID,
				"OUT_FOR_DELIVERY",
				"ATTEMPT_FAILED",
				courierUUID,
				"COURIER",
				fmt.Sprintf("Delivery attempt %d failed: %s", newAttemptNumber, reasonCode),
			)
			if err != nil {
				return fmt.Errorf("failed to transition parcel status: %w", err)
			}

			// Emit parcel.delivery_failed.v1 outbox event
			eventPayload := map[string]any{
				"parcel_id":      parcelUUID.String(),
				"courier_id":     courierUUID.String(),
				"attempt_number": newAttemptNumber,
				"max_attempts":   3,
				"reason_code":    string(reasonCode),
				"next_action":    "AWAITING_DISPATCHER_REVIEW",
				"attempted_at":   attempt.AttemptedAt.Format(time.RFC3339),
			}
			err = uc.outboxWriter.Write(
				ctx,
				tx,
				"PARCEL",
				parcelUUID,
				1,
				"parcel.delivery_failed.v1",
				eventPayload,
				attempt.ID.String(),
			)
			if err != nil {
				return fmt.Errorf("failed to commit delivery_failed outbox event: %w", err)
			}

			result = &FailedAttemptResultDTO{
				AttemptID:         attempt.ID.String(),
				ParcelID:          parcelUUID.String(),
				AttemptNumber:     newAttemptNumber,
				MaxAttempts:       3,
				Outcome:           "FAILED",
				FailureReasonCode: string(reasonCode),
				ParcelStatus:      "ATTEMPT_FAILED",
				Status:            "ATTEMPT_FAILED",
				IsTerminal:        false,
				NextAction:        "AWAITING_DISPATCHER_REVIEW",
				AttemptedAt:       attempt.AttemptedAt,
				RecordedAt:        attempt.AttemptedAt,
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
