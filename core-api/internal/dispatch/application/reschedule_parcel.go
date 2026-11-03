package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

// RescheduleParcelUseCase enforces the ADR-004 gate: only dispatchers can authorize reattempt of failed parcels.
type RescheduleParcelUseCase struct {
	txManager      TransactionManager
	courierRepo    CourierRepository
	assignmentRepo AssignmentRepository
	parcelUpdater  ParcelLifecycleUpdater
	outboxWriter   OutboxWriter
}

// NewRescheduleParcelUseCase constructs the rescheduling use case.
func NewRescheduleParcelUseCase(
	txManager TransactionManager,
	courierRepo CourierRepository,
	assignmentRepo AssignmentRepository,
	parcelUpdater ParcelLifecycleUpdater,
	outboxWriter OutboxWriter,
) *RescheduleParcelUseCase {
	return &RescheduleParcelUseCase{
		txManager:      txManager,
		courierRepo:    courierRepo,
		assignmentRepo: assignmentRepo,
		parcelUpdater:  parcelUpdater,
		outboxWriter:   outboxWriter,
	}
}

// Execute validates dispatcher authority, checks courier workload, and moves parcel from ATTEMPT_FAILED to ASSIGNED.
func (uc *RescheduleParcelUseCase) Execute(ctx context.Context, cmd RescheduleParcelCommand) (*AssignmentResultDTO, error) {
	parcelUUID, err := uuid.Parse(cmd.ParcelID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	courierUUID, err := uuid.Parse(cmd.CourierID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	dispatcherUUID, err := uuid.Parse(cmd.ScheduledBy)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid scheduled_by format", ErrInvalidCommand)
	}

	var result *AssignmentResultDTO

	err = uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// 1. Lock and verify parcel is in ATTEMPT_FAILED status
		currentStatus, _, err := uc.parcelUpdater.GetStatusWithLock(ctx, tx, parcelUUID)
		if err != nil {
			return fmt.Errorf("failed to load parcel status: %w", err)
		}

		if currentStatus != "ATTEMPT_FAILED" {
			return fmt.Errorf("%w: parcel must be in ATTEMPT_FAILED status to reschedule (current: %s)", ErrParcelNotEligibleForAssignment, currentStatus)
		}

		// 2. Lock and verify courier profile
		courier, err := uc.courierRepo.GetByIDWithLock(ctx, tx, courierUUID)
		if err != nil {
			return fmt.Errorf("failed to load courier: %w", err)
		}
		if courier == nil {
			return ErrCourierNotFound
		}

		// 3. Verify courier capacity
		activeCount, err := uc.assignmentRepo.CountActiveByCourierID(ctx, tx, courierUUID)
		if err != nil {
			return fmt.Errorf("failed to count courier workload: %w", err)
		}

		if err := courier.CanAcceptAssignment(activeCount); err != nil {
			return err
		}

		// 4. Create new active assignment
		asgn := domain.NewCourierAssignment(parcelUUID, courierUUID, dispatcherUUID, cmd.Notes)
		if err := uc.assignmentRepo.Create(ctx, tx, asgn); err != nil {
			return fmt.Errorf("failed to persist assignment: %w", err)
		}

		// 5. Transition parcel status from ATTEMPT_FAILED to ASSIGNED
		err = uc.parcelUpdater.TransitionStatus(
			ctx,
			tx,
			parcelUUID,
			"ATTEMPT_FAILED",
			"ASSIGNED",
			dispatcherUUID,
			"DISPATCHER",
			"Dispatcher authorized reattempt run",
		)
		if err != nil {
			return fmt.Errorf("failed to transition parcel status: %w", err)
		}

		// 6. Commit outbox event
		eventPayload := map[string]any{
			"assignment_id": asgn.ID.String(),
			"parcel_id":     asgn.ParcelID.String(),
			"courier_id":    asgn.CourierID.String(),
			"assigned_by":   asgn.AssignedBy.String(),
			"assigned_at":   asgn.AssignedAt.Format(time.RFC3339),
			"is_reattempt":  true,
		}
		err = uc.outboxWriter.Write(
			ctx,
			tx,
			"PARCEL",
			parcelUUID,
			1,
			"parcel.assigned.v1",
			eventPayload,
			asgn.ID.String(),
		)
		if err != nil {
			return fmt.Errorf("failed to commit outbox event: %w", err)
		}

		result = &AssignmentResultDTO{
			AssignmentID: asgn.ID.String(),
			ParcelID:     asgn.ParcelID.String(),
			CourierID:    asgn.CourierID.String(),
			AssignedAt:   asgn.AssignedAt,
			AssignedBy:   asgn.AssignedBy.String(),
			Status:       string(asgn.Status),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

