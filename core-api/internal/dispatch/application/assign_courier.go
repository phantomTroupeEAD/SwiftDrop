package application

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

// AssignCourierUseCase orchestrates race-free parcel assignment with capacity and status invariant enforcement.
type AssignCourierUseCase struct {
	txManager      TransactionManager
	courierRepo    CourierRepository
	assignmentRepo AssignmentRepository
	parcelUpdater  ParcelLifecycleUpdater
	outboxWriter   OutboxWriter
}

// NewAssignCourierUseCase constructs the assignment command handler.
func NewAssignCourierUseCase(
	txManager TransactionManager,
	courierRepo CourierRepository,
	assignmentRepo AssignmentRepository,
	parcelUpdater ParcelLifecycleUpdater,
	outboxWriter OutboxWriter,
) *AssignCourierUseCase {
	return &AssignCourierUseCase{
		txManager:      txManager,
		courierRepo:    courierRepo,
		assignmentRepo: assignmentRepo,
		parcelUpdater:  parcelUpdater,
		outboxWriter:   outboxWriter,
	}
}

// Execute performs concurrency-safe courier assignment inside a single atomic PostgreSQL transaction (AT-013).
func (uc *AssignCourierUseCase) Execute(ctx context.Context, cmd AssignCourierCommand) (*AssignmentResultDTO, error) {
	parcelUUID, err := uuid.Parse(cmd.ParcelID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid parcel_id format", ErrInvalidCommand)
	}

	courierUUID, err := uuid.Parse(cmd.CourierID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	dispatcherUUID, err := uuid.Parse(cmd.AssignedBy)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid assigned_by format", ErrInvalidCommand)
	}

	var result *AssignmentResultDTO

	err = uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// 1. Acquire exclusive lock on target parcel and inspect status
		currentStatus, _, err := uc.parcelUpdater.GetStatusWithLock(ctx, tx, parcelUUID)
		if err != nil {
			return fmt.Errorf("failed to load parcel status: %w", err)
		}

		// Invariant: Parcel must be in CREATED or ATTEMPT_FAILED status
		if currentStatus != "CREATED" && currentStatus != "ATTEMPT_FAILED" {
			return ErrParcelNotEligibleForAssignment
		}

		// 2. Acquire exclusive lock on courier profile
		courier, err := uc.courierRepo.GetByIDWithLock(ctx, tx, courierUUID)
		if err != nil {
			return fmt.Errorf("failed to load courier: %w", err)
		}
		if courier == nil {
			return ErrCourierNotFound
		}

		// 3. Count current active assignments for this courier with lock held
		activeCount, err := uc.assignmentRepo.CountActiveByCourierID(ctx, tx, courierUUID)
		if err != nil {
			return fmt.Errorf("failed to count courier active workload: %w", err)
		}

		// 4. Evaluate domain capacity & shift invariants
		if err := courier.CanAcceptAssignment(activeCount); err != nil {
			return err
		}

		// 5. Create new active assignment aggregate
		asgn := domain.NewCourierAssignment(parcelUUID, courierUUID, dispatcherUUID, cmd.Notes)
		if err := uc.assignmentRepo.Create(ctx, tx, asgn); err != nil {
			return fmt.Errorf("failed to persist courier assignment: %w", err)
		}

		// 6. Transition parcel aggregate root state machine (internal/parcels)
		// Member 2's TransitionParcelStatusUseCase.ExecuteTx handles the parcel state machine,
		// history insertion, concurrency validation, and publishes the canonical parcel.assigned.v1
		// outbox event within this atomic transaction. Dispatch avoids creating a duplicate event.
		err = uc.parcelUpdater.TransitionStatus(
			ctx,
			tx,
			parcelUUID,
			currentStatus,
			"ASSIGNED",
			dispatcherUUID,
			"DISPATCHER",
			fmt.Sprintf("Assigned to courier %s", courier.EmployeeCode),
		)
		if err != nil {
			return fmt.Errorf("failed to transition parcel status: %w", err)
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
