package application

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// TransactionManager abstracts atomic database transaction execution.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// ParcelLifecycleUpdater is implemented by Member 2 (internal/parcels)
// to enforce aggregate root state machine transitions without raw SQL updates.
type ParcelLifecycleUpdater interface {
	// TransitionStatus advances a parcel between legal statuses in the parcel aggregate root.
	TransitionStatus(
		ctx context.Context,
		tx *sql.Tx,
		parcelID uuid.UUID,
		fromStatus, toStatus string,
		actorID uuid.UUID,
		actorRole, note string,
	) error

	// GetCurrentStatus retrieves current parcel status and attempt count within the transaction.
	GetStatusWithLock(
		ctx context.Context,
		tx *sql.Tx,
		parcelID uuid.UUID,
	) (currentStatus string, attemptCount int, err error)
}

// OutboxWriter is implemented by Member 8 (internal/events)
// to commit versioned integration events to PostgreSQL within the caller's transaction.
type OutboxWriter interface {
	Write(
		ctx context.Context,
		tx *sql.Tx,
		aggregateType string,
		aggregateID uuid.UUID,
		aggregateVersion int,
		eventType string,
		payload any,
		correlationID string,
	) error
}

// UserRoleChecker validates whether a user exists and retrieves their role in the identity store.
type UserRoleChecker interface {
	GetUserRole(ctx context.Context, userID uuid.UUID) (role string, exists bool, err error)
}
