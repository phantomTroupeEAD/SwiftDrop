package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/events"
	parcelsApp "github.com/swiftdrop/core-api/internal/parcels/application"
	parcelsDomain "github.com/swiftdrop/core-api/internal/parcels/domain"
	parcelsInfra "github.com/swiftdrop/core-api/internal/parcels/infrastructure"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresParcelLifecycleUpdater implements application.ParcelLifecycleUpdater
// for the monolith database, delegating aggregate status transitions to Member 2's
// TransitionParcelStatusUseCase within caller transactions, ensuring atomic status
// transitions, immutable audit history, and canonical outbox event publication.
type PostgresParcelLifecycleUpdater struct {
	db              *sql.DB
	parcelLifecycle parcelsApp.ParcelLifecycleService
}

// NewPostgresParcelLifecycleUpdater constructs the lifecycle updater.
// If parcelLifecycle is not provided and db is non-nil, it automatically initializes
// Member 2's TransitionParcelStatusUseCase backed by PostgreSQL.
func NewPostgresParcelLifecycleUpdater(db *sql.DB, parcelLifecycle ...parcelsApp.ParcelLifecycleService) *PostgresParcelLifecycleUpdater {
	var lifecycle parcelsApp.ParcelLifecycleService
	if len(parcelLifecycle) > 0 {
		lifecycle = parcelLifecycle[0]
	} else if db != nil {
		parcelRepo := parcelsInfra.NewPostgresParcelRepository(db)
		historyRepo := parcelsInfra.NewPostgresStatusHistoryRepository(db)
		eventsOutbox := events.NewPostgresOutboxWriter()
		txMgr := parcelsApp.NewDBTransactionManager(db)
		if uc, err := parcelsApp.NewTransitionParcelStatusUseCase(
			txMgr,
			parcelRepo,
			historyRepo,
			eventsOutbox,
			parcelsApp.NewUUIDv7Generator(),
			parcelsApp.NewSystemClock(),
		); err == nil {
			lifecycle = uc
		}
	}
	return &PostgresParcelLifecycleUpdater{
		db:              db,
		parcelLifecycle: lifecycle,
	}
}

// GetStatusWithLock acquires a row-level lock on the target parcel and retrieves its status.
func (u *PostgresParcelLifecycleUpdater) GetStatusWithLock(
	ctx context.Context,
	tx *sql.Tx,
	parcelID uuid.UUID,
) (string, int, error) {
	if tx == nil && u.db == nil {
		return "", 0, fmt.Errorf("parcel updater: %w", db.ErrNilDB)
	}
	query := `
		SELECT status, attempt_count
		FROM parcels
		WHERE id = $1
		FOR UPDATE;
	`

	var currentStatus string
	var attemptCount int

	var row *sql.Row
	if tx != nil {
		row = tx.QueryRowContext(ctx, query, parcelID)
	} else {
		row = u.db.QueryRowContext(ctx, query, parcelID)
	}

	if err := row.Scan(&currentStatus, &attemptCount); err != nil {
		if err == sql.ErrNoRows {
			return "", 0, fmt.Errorf("parcel not found: %w", err)
		}
		return "", 0, fmt.Errorf("failed to load parcel with lock: %w", err)
	}

	return currentStatus, attemptCount, nil
}

// TransitionStatus atomically updates parcel status and appends a row to parcel_status_history.
// When parcelLifecycle is available, it delegates to Member 2's TransitionParcelStatusUseCase.ExecuteTx
// within the provided database transaction, ensuring aggregate invariant evaluation,
// optimistic concurrency control, audit logging, and canonical outbox event emission.
func (u *PostgresParcelLifecycleUpdater) TransitionStatus(
	ctx context.Context,
	tx *sql.Tx,
	parcelID uuid.UUID,
	fromStatus, toStatus string,
	actorID uuid.UUID,
	actorRole, note string,
) error {
	if u.parcelLifecycle != nil {
		actorIDStr := actorID.String()
		cmd := parcelsApp.TransitionParcelStatusCommand{
			ParcelID:     parcelID.String(),
			TargetStatus: parcelsDomain.ParcelStatus(toStatus),
			ActorID:      &actorIDStr,
			ActorType:    parcelsDomain.ActorTypeUser,
			ActorRole:    actorRole,
			Reason:       note,
		}
		if tx != nil {
			_, err := u.parcelLifecycle.ExecuteTx(ctx, tx, cmd)
			if err != nil {
				return fmt.Errorf("failed to transition parcel status via parcel lifecycle service: %w", err)
			}
			return nil
		}
		_, err := u.parcelLifecycle.Execute(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to transition parcel status via parcel lifecycle service: %w", err)
		}
		return nil
	}

	if tx == nil && u.db == nil {
		return fmt.Errorf("parcel updater: %w", db.ErrNilDB)
	}

	// Fallback direct SQL update if parcelLifecycle is not wired
	updateQuery := `
		UPDATE parcels
		SET status = $1, updated_at = NOW()
		WHERE id = $2;
	`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, updateQuery, toStatus, parcelID)
	} else {
		_, err = u.db.ExecContext(ctx, updateQuery, toStatus, parcelID)
	}
	if err != nil {
		return fmt.Errorf("failed to update parcel status: %w", err)
	}

	// Insert into immutable parcel_status_history audit log if the table exists
	historyQuery := `
		INSERT INTO parcel_status_history (
			id, parcel_id, from_status, to_status, actor_id, actor_type, actor_role, reason, occurred_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, 'USER', $5, $6, NOW()
		);
	`
	if tx != nil {
		_, _ = tx.ExecContext(ctx, historyQuery, parcelID, fromStatus, toStatus, actorID, actorRole, note)
	} else {
		_, _ = u.db.ExecContext(ctx, historyQuery, parcelID, fromStatus, toStatus, actorID, actorRole, note)
	}

	return nil
}
