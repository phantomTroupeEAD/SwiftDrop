package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/application"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresAssignmentFinder implements application.ActiveAssignmentFinder
// for checking parcel-to-courier assignment and closing assignments upon completion.
type PostgresAssignmentFinder struct {
	db *sql.DB
}

// NewPostgresAssignmentFinder constructs a PostgresAssignmentFinder instance.
func NewPostgresAssignmentFinder(db *sql.DB) *PostgresAssignmentFinder {
	return &PostgresAssignmentFinder{db: db}
}

// GetActiveAssignment returns the currently active assignment for a parcel, or nil if none is active.
func (f *PostgresAssignmentFinder) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*application.ActiveAssignmentDTO, error) {
	if tx == nil && f.db == nil {
		return nil, fmt.Errorf("assignment finder: %w", db.ErrNilDB)
	}
	query := `
		SELECT id, parcel_id, courier_id, status
		FROM courier_assignments
		WHERE parcel_id = $1 AND status = 'ACTIVE'
		LIMIT 1;
	`

	var row *sql.Row
	if tx != nil {
		row = tx.QueryRowContext(ctx, query, parcelID)
	} else {
		row = f.db.QueryRowContext(ctx, query, parcelID)
	}

	var dto application.ActiveAssignmentDTO
	err := row.Scan(&dto.AssignmentID, &dto.ParcelID, &dto.CourierID, &dto.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query active assignment for parcel %s: %w", parcelID, err)
	}

	return &dto, nil
}

// CompleteAssignment transitions the specified assignment to COMPLETED with an ended_at timestamp.
func (f *PostgresAssignmentFinder) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	query := `
		UPDATE courier_assignments
		SET status = 'COMPLETED', ended_at = NOW()
		WHERE id = $1;
	`

	var err error
	var res sql.Result
	if tx != nil {
		res, err = tx.ExecContext(ctx, query, assignmentID)
	} else {
		res, err = f.db.ExecContext(ctx, query, assignmentID)
	}

	if err != nil {
		return fmt.Errorf("failed to complete assignment %s: %w", assignmentID, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected for completed assignment: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("assignment %s not found for completion", assignmentID)
	}

	return nil
}
