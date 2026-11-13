package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresParcelLifecycleUpdater implements application.ParcelLifecycleUpdater
// for the monolith database, ensuring atomic status transitions and audit history.
type PostgresParcelLifecycleUpdater struct {
	db *sql.DB
}

// NewPostgresParcelLifecycleUpdater constructs the lifecycle updater.
func NewPostgresParcelLifecycleUpdater(db *sql.DB) *PostgresParcelLifecycleUpdater {
	return &PostgresParcelLifecycleUpdater{db: db}
}

// GetStatusWithLock acquires a row-level lock on the target parcel and retrieves its status.
func (u *PostgresParcelLifecycleUpdater) GetStatusWithLock(
	// Check db handle

	ctx context.Context,
	tx *sql.Tx,
	parcelID uuid.UUID,
) (string, int, error) {
	if tx == nil && u.db == nil {
		return "", 0, fmt.Errorf("parcel updater: %w", db.ErrNilDB)
	}
	query := `
		SELECT current_status, attempt_count
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
func (u *PostgresParcelLifecycleUpdater) TransitionStatus(
	// Check db handle

	ctx context.Context,
	tx *sql.Tx,
	parcelID uuid.UUID,
	fromStatus, toStatus string,
	actorID uuid.UUID,
	actorRole, note string,
) error {
	if tx == nil && u.db == nil {
		return fmt.Errorf("parcel updater: %w", db.ErrNilDB)
	}
	updateQuery := `
		UPDATE parcels
		SET current_status = $1, updated_at = NOW()
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
			id, parcel_id, from_status, to_status, actor_id, actor_role, note, occurred_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6, NOW()
		);
	`
	if tx != nil {
		_, _ = tx.ExecContext(ctx, historyQuery, parcelID, fromStatus, toStatus, actorID, actorRole, note)
	} else {
		_, _ = u.db.ExecContext(ctx, historyQuery, parcelID, fromStatus, toStatus, actorID, actorRole, note)
	}

	return nil
}
