package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

var _ ParcelLifecycleService = (*TransitionParcelStatusUseCase)(nil)

// TransitionParcelStatusUseCase orchestrates parcel lifecycle state transitions.
// It is the authoritative entrypoint for modifying parcel status in the SwiftDrop system.
// All transitions enforce domain aggregate invariants, optimistic concurrency control,
// status history audit logging, and transactional outbox event publication in one atomic transaction.
type TransitionParcelStatusUseCase struct {
	txManager         TransactionManager
	parcelRepo        ParcelRepository
	statusHistoryRepo StatusHistoryRepository
	outboxWriter      events.OutboxWriter
	idGen             IDGenerator
	clock             Clock
}

// NewTransitionParcelStatusUseCase constructs a new TransitionParcelStatusUseCase.
func NewTransitionParcelStatusUseCase(
	txManager TransactionManager,
	parcelRepo ParcelRepository,
	statusHistoryRepo StatusHistoryRepository,
	outboxWriter events.OutboxWriter,
	idGen IDGenerator,
	clock Clock,
) (*TransitionParcelStatusUseCase, error) {
	if txManager == nil {
		return nil, ErrNilTransactionManager
	}
	if parcelRepo == nil {
		return nil, ErrNilParcelRepository
	}
	if statusHistoryRepo == nil {
		return nil, ErrNilStatusHistoryRepository
	}
	if outboxWriter == nil {
		return nil, ErrNilOutboxWriter
	}
	if idGen == nil {
		idGen = NewUUIDv7Generator()
	}
	if clock == nil {
		clock = NewSystemClock()
	}

	return &TransitionParcelStatusUseCase{
		txManager:         txManager,
		parcelRepo:        parcelRepo,
		statusHistoryRepo: statusHistoryRepo,
		outboxWriter:      outboxWriter,
		idGen:             idGen,
		clock:             clock,
	}, nil
}

// Execute processes a parcel lifecycle status transition within an isolated, atomic database transaction.
func (uc *TransitionParcelStatusUseCase) Execute(ctx context.Context, cmd TransitionParcelStatusCommand) (TransitionParcelStatusResult, error) {
	var result TransitionParcelStatusResult
	err := uc.txManager.WithTx(ctx, func(tx *sql.Tx) error {
		var txErr error
		result, txErr = uc.ExecuteTx(ctx, tx, cmd)
		return txErr
	})
	if err != nil {
		return TransitionParcelStatusResult{}, err
	}
	return result, nil
}

// ExecuteTx executes a parcel lifecycle status transition within an existing database transaction.
// This enables other modules (such as Member 3 dispatch and delivery) to coordinate multi-entity
// mutations atomically alongside the parcel status update, status history insertion, and outbox event write.
func (uc *TransitionParcelStatusUseCase) ExecuteTx(ctx context.Context, tx *sql.Tx, cmd TransitionParcelStatusCommand) (TransitionParcelStatusResult, error) {
	if tx == nil {
		return TransitionParcelStatusResult{}, errors.New("transition parcel status: nil transaction provided")
	}

	// 1. Validate required command fields.
	parcelID := strings.TrimSpace(cmd.ParcelID)
	if parcelID == "" {
		return TransitionParcelStatusResult{}, domain.ErrInvalidParcelID
	}
	if err := cmd.TargetStatus.Validate(); err != nil {
		return TransitionParcelStatusResult{}, err
	}

	// 2. Load the parcel safely from persistence within the transaction.
	parcel, err := uc.parcelRepo.FindByID(ctx, tx, parcelID)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidParcelID) || errors.Is(err, sql.ErrNoRows) {
			return TransitionParcelStatusResult{}, fmt.Errorf("%w: %s", ErrParcelNotFound, parcelID)
		}
		return TransitionParcelStatusResult{}, fmt.Errorf("failed to load parcel %s: %w", parcelID, err)
	}

	// 3. Concurrency check: If caller provided an expected version, verify against loaded aggregate version.
	if cmd.ExpectedVersion != nil && parcel.Version != *cmd.ExpectedVersion {
		return TransitionParcelStatusResult{}, fmt.Errorf("%w: expected version %d, but parcel is at version %d",
			ErrConcurrencyConflict, *cmd.ExpectedVersion, parcel.Version)
	}

	expectedVersion := parcel.Version
	now := uc.clock.Now()

	// Resolve actor ID, actor type, role, and reason defaults if omitted.
	actorID, actorType, actorRole := resolveTransitionActor(cmd)

	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		switch cmd.TargetStatus {
		case domain.StatusAssigned:
			reason = "Courier assigned for pickup"
		case domain.StatusPickedUp:
			reason = "Parcel collected from sender"
		case domain.StatusInTransit:
			reason = "Parcel arrived at hub / vehicle in transit"
		case domain.StatusOutForDelivery:
			reason = "Dispatched for final-mile delivery"
		case domain.StatusDelivered:
			reason = "Delivery confirmed"
		case domain.StatusAttemptFailed:
			reason = "Delivery attempt failed"
		case domain.StatusReturned:
			reason = "Returned to sender"
		case domain.StatusCancelled:
			reason = "Parcel cancelled"
		}
	}

	// 4. Execute the domain transition on the aggregate root.
	// This evaluates lifecycle state machine legality, attempt counters, and terminal states.
	historyEntry, err := parcel.TransitionTo(cmd.TargetStatus, actorID, actorType, actorRole, reason, now)
	if err != nil {
		return TransitionParcelStatusResult{}, err
	}

	// 5. Generate status history unique ID (UUIDv7) and validate before any writes.
	historyID, err := uc.idGen.GenerateID()
	if err != nil {
		return TransitionParcelStatusResult{}, fmt.Errorf("failed to generate status history id: %w", err)
	}
	historyID = strings.TrimSpace(historyID)
	if !events.IsValidUUID(historyID) {
		return TransitionParcelStatusResult{}, fmt.Errorf("%w: invalid generated status history id %q", domain.ErrInvalidStatusHistory, historyID)
	}
	historyEntry.ID = historyID

	// 6. Persist the updated parcel aggregate with optimistic concurrency control.
	if err := uc.parcelRepo.UpdateStatus(ctx, tx, parcel, expectedVersion); err != nil {
		return TransitionParcelStatusResult{}, fmt.Errorf("failed to persist parcel update: %w", err)
	}

	// 7. Insert the corresponding status history audit entry.
	if err := uc.statusHistoryRepo.Save(ctx, tx, historyEntry); err != nil {
		return TransitionParcelStatusResult{}, fmt.Errorf("failed to persist status history entry: %w", err)
	}

	// 8. Write the appropriate event through the transactional outbox abstraction.
	eventType := EventTypeForStatus(cmd.TargetStatus)
	if eventType == "" {
		return TransitionParcelStatusResult{}, fmt.Errorf("unsupported event type for target status %s", cmd.TargetStatus)
	}

	var fromStatusStr string
	if historyEntry.FromStatus != nil {
		fromStatusStr = string(*historyEntry.FromStatus)
	}

	payload := ParcelStatusChangedPayload{
		ParcelID:        parcel.ID,
		TrackingNumber:  parcel.TrackingNumber.String(),
		PreviousStatus:  fromStatusStr,
		NewStatus:       string(historyEntry.ToStatus),
		StatusHistoryID: historyEntry.ID,
		ActorID:         historyEntry.ActorID,
		ActorRole:       historyEntry.ActorRole,
		Reason:          historyEntry.Reason,
		AttemptCount:    parcel.AttemptCount,
		OccurredAt:      historyEntry.OccurredAt,
	}

	corrID := strings.TrimSpace(cmd.CorrelationID)
	if corrID == "" {
		corrID = parcel.ID
	}

	envelope, err := events.NewEnvelope(
		eventType,
		"parcel",
		parcel.ID,
		parcel.Version,
		corrID,
		"core-api.parcels",
		payload,
	)
	if err != nil {
		return TransitionParcelStatusResult{}, fmt.Errorf("failed to build outbox envelope: %w", err)
	}

	if err := uc.outboxWriter.Write(ctx, tx, envelope); err != nil {
		return TransitionParcelStatusResult{}, fmt.Errorf("failed to write outbox event: %w", err)
	}

	return TransitionParcelStatusResult{
		Parcel:        parcel,
		StatusHistory: historyEntry,
	}, nil
}

func resolveTransitionActor(cmd TransitionParcelStatusCommand) (*string, domain.ActorType, string) {
	actorType := cmd.ActorType
	if actorType == "" {
		if cmd.ActorID == nil {
			actorType = domain.ActorTypeSystem
		} else {
			actorType = domain.ActorTypeUser
		}
	}

	actorRole := strings.TrimSpace(cmd.ActorRole)
	if actorRole == "" {
		switch actorType {
		case domain.ActorTypeSystem:
			actorRole = "SYSTEM"
		case domain.ActorTypeMerchantAPI:
			actorRole = "MERCHANT"
		default:
			switch cmd.TargetStatus {
			case domain.StatusAssigned, domain.StatusOutForDelivery:
				actorRole = "DISPATCHER"
			case domain.StatusPickedUp, domain.StatusDelivered, domain.StatusAttemptFailed:
				actorRole = "COURIER"
			case domain.StatusCancelled:
				actorRole = "CUSTOMER"
			default:
				actorRole = "SYSTEM"
			}
		}
	}

	return cmd.ActorID, actorType, actorRole
}
