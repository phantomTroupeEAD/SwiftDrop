package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CancelAssignmentUseCase terminates an active assignment before departure, restoring driver capacity.
type CancelAssignmentUseCase struct {
	txManager      TransactionManager
	assignmentRepo AssignmentRepository
	parcelUpdater  ParcelLifecycleUpdater
	outboxWriter   OutboxWriter
}

// NewCancelAssignmentUseCase constructs the assignment cancellation use case.
func NewCancelAssignmentUseCase(
	txManager TransactionManager,
	assignmentRepo AssignmentRepository,
	parcelUpdater ParcelLifecycleUpdater,
	outboxWriter OutboxWriter,
) *CancelAssignmentUseCase {
	return &CancelAssignmentUseCase{
		txManager:      txManager,
		assignmentRepo: assignmentRepo,
		parcelUpdater:  parcelUpdater,
		outboxWriter:   outboxWriter,
	}
}

// Execute terminates the active assignment and reverts parcel status back to CREATED within a single transaction.
func (uc *CancelAssignmentUseCase) Execute(ctx context.Context, cmd CancelAssignmentCommand) error {
	parcelUUID, err := uuid.Parse(cmd.ParcelID)
	if err != nil {
		return fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	dispatcherUUID, err := uuid.Parse(cmd.CancelledBy)
	if err != nil {
		return fmt.Errorf("%w: invalid cancelled_by format", ErrInvalidCommand)
	}

	return uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// 1. Fetch active assignment for this parcel with lock
		asgn, err := uc.assignmentRepo.GetActiveByParcelID(ctx, tx, parcelUUID)
		if err != nil {
			return fmt.Errorf("failed to fetch active assignment: %w", err)
		}
		if asgn == nil {
			return ErrAssignmentNotFound
		}

		// 2. Transition assignment aggregate to CANCELLED
		if err := asgn.Cancel(cmd.Reason); err != nil {
			return err
		}

		// 3. Persist updated assignment record
		if err := uc.assignmentRepo.Update(ctx, tx, asgn); err != nil {
			return fmt.Errorf("failed to update assignment: %w", err)
		}

		// 4. Revert parcel status from ASSIGNED back to CREATED
		err = uc.parcelUpdater.TransitionStatus(
			ctx,
			tx,
			parcelUUID,
			"ASSIGNED",
			"CREATED",
			dispatcherUUID,
			"DISPATCHER",
			fmt.Sprintf("Assignment cancelled: %s", cmd.Reason),
		)
		if err != nil {
			return fmt.Errorf("failed to revert parcel status: %w", err)
		}

		// 5. Emit courier.unassigned.v1 outbox event
		eventPayload := map[string]any{
			"assignment_id": asgn.ID.String(),
			"parcel_id":     asgn.ParcelID.String(),
			"courier_id":    asgn.CourierID.String(),
			"cancelled_by":  dispatcherUUID.String(),
			"reason":        cmd.Reason,
			"unassigned_at": time.Now().UTC().Format(time.RFC3339),
		}
		err = uc.outboxWriter.Write(
			ctx,
			tx,
			"PARCEL",
			parcelUUID,
			1,
			"courier.unassigned.v1",
			eventPayload,
			asgn.ID.String(),
		)
		if err != nil {
			return fmt.Errorf("failed to commit unassigned outbox event: %w", err)
		}

		return nil
	})
}

