package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

// StartDeliveryRunUseCase confirms package collection at the depot and initiates the final-mile run.
type StartDeliveryRunUseCase struct {
	txManager        TransactionManager
	assignmentFinder ActiveAssignmentFinder
	parcelUpdater    ParcelLifecycleUpdater
	outboxWriter     OutboxWriter
}

// NewStartDeliveryRunUseCase constructs the run initiation use case.
func NewStartDeliveryRunUseCase(
	txManager TransactionManager,
	assignmentFinder ActiveAssignmentFinder,
	parcelUpdater ParcelLifecycleUpdater,
	outboxWriter OutboxWriter,
) *StartDeliveryRunUseCase {
	return &StartDeliveryRunUseCase{
		txManager:        txManager,
		assignmentFinder: assignmentFinder,
		parcelUpdater:    parcelUpdater,
		outboxWriter:     outboxWriter,
	}
}

// ExecuteSingle confirms pickup and transitions a single parcel from ASSIGNED to OUT_FOR_DELIVERY.
func (uc *StartDeliveryRunUseCase) ExecuteSingle(ctx context.Context, cmd StartRunCommand) error {
	parcelUUID, err := uuid.Parse(cmd.ParcelID)
	if err != nil {
		return fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	courierUUID, err := uuid.Parse(cmd.CourierID)
	if err != nil {
		return fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	return uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// 1. Verify active courier assignment (Courier IDOR guard)
		asgn, err := uc.assignmentFinder.GetActiveAssignment(ctx, tx, parcelUUID)
		if err != nil {
			return fmt.Errorf("failed to fetch assignment: %w", err)
		}
		if asgn == nil {
			return ErrParcelNotAssigned
		}
		if asgn.CourierID != courierUUID {
			return domain.ErrNotAssignedCourier
		}

		// 2. Lock and verify parcel status
		status, _, err := uc.parcelUpdater.GetStatusWithLock(ctx, tx, parcelUUID)
		if err != nil {
			return fmt.Errorf("failed to load parcel status: %w", err)
		}
		if status != "ASSIGNED" {
			return fmt.Errorf("%w: current status is %s", ErrParcelNotAssigned, status)
		}

		// 3. Transition parcel status to OUT_FOR_DELIVERY
		note := "Courier confirmed pickup from depot; delivery run started"
		if cmd.Notes != "" {
			note = cmd.Notes
		}
		err = uc.parcelUpdater.TransitionStatus(
			ctx,
			tx,
			parcelUUID,
			"ASSIGNED",
			"OUT_FOR_DELIVERY",
			courierUUID,
			"COURIER",
			note,
		)
		if err != nil {
			return fmt.Errorf("failed to transition parcel status: %w", err)
		}

		// 4. Commit parcel.out_for_delivery.v1 outbox event
		eventPayload := map[string]any{
			"parcel_id":      parcelUUID.String(),
			"courier_id":     courierUUID.String(),
			"depot_hub_code": cmd.DepotHubCode,
			"started_at":     time.Now().UTC().Format(time.RFC3339),
		}
		return uc.outboxWriter.Write(
			ctx,
			tx,
			"PARCEL",
			parcelUUID,
			1,
			"parcel.out_for_delivery.v1",
			eventPayload,
			parcelUUID.String(),
		)
	})
}

// ExecuteBatch enables couriers to start their morning route for all assigned packages in a single transaction.
func (uc *StartDeliveryRunUseCase) ExecuteBatch(ctx context.Context, cmd StartBatchRunCommand) (*BatchRunResultDTO, error) {
	if len(cmd.ParcelIDs) == 0 {
		return nil, ErrEmptyParcelList
	}

	courierUUID, err := uuid.Parse(cmd.CourierID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	var startedIDs []string
	now := time.Now().UTC()

	err = uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		for _, rawID := range cmd.ParcelIDs {
			pID, err := uuid.Parse(rawID)
			if err != nil {
				return fmt.Errorf("%w: invalid parcel_id %s", ErrInvalidCommand, rawID)
			}

			// Verify assignment & IDOR
			asgn, err := uc.assignmentFinder.GetActiveAssignment(ctx, tx, pID)
			if err != nil {
				return err
			}
			if asgn == nil {
				return fmt.Errorf("parcel %s is not actively assigned", rawID)
			}
			if asgn.CourierID != courierUUID {
				return domain.ErrNotAssignedCourier
			}

			// Verify status
			status, _, err := uc.parcelUpdater.GetStatusWithLock(ctx, tx, pID)
			if err != nil {
				return err
			}
			if status != "ASSIGNED" {
				return fmt.Errorf("parcel %s is not in ASSIGNED status (current: %s)", rawID, status)
			}

			// Transition status
			err = uc.parcelUpdater.TransitionStatus(
				ctx,
				tx,
				pID,
				"ASSIGNED",
				"OUT_FOR_DELIVERY",
				courierUUID,
				"COURIER",
				"Batch run start from depot",
			)
			if err != nil {
				return err
			}

			// Commit outbox event
			eventPayload := map[string]any{
				"parcel_id":      pID.String(),
				"courier_id":     courierUUID.String(),
				"depot_hub_code": cmd.DepotHubCode,
				"started_at":     now.Format(time.RFC3339),
			}
			err = uc.outboxWriter.Write(
				ctx,
				tx,
				"PARCEL",
				pID,
				1,
				"parcel.out_for_delivery.v1",
				eventPayload,
				pID.String(),
			)
			if err != nil {
				return err
			}

			startedIDs = append(startedIDs, pID.String())
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &BatchRunResultDTO{
		TotalStarted: len(startedIDs),
		StartedAt:    now,
		ParcelIDs:    startedIDs,
	}, nil
}
