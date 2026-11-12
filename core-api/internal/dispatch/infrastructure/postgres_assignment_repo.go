package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresAssignmentRepository implements application.AssignmentRepository using PostgreSQL.
type PostgresAssignmentRepository struct {
	db *sql.DB
}

// NewPostgresAssignmentRepository constructs the assignment repository.
func NewPostgresAssignmentRepository(db *sql.DB) *PostgresAssignmentRepository {
	return &PostgresAssignmentRepository{db: db}
}

// Create inserts a new courier assignment.
// Partial unique index idx_assignments_parcel_single_active rejects concurrent active assignments for the same parcel.
func (r *PostgresAssignmentRepository) Create(ctx context.Context, tx *sql.Tx, asgn *domain.CourierAssignment) error {
	if tx == nil && r.db == nil {
		return fmt.Errorf("failed to create courier assignment: %w", db.ErrNilDB)
	}
	query := `
		INSERT INTO courier_assignments (
			id, parcel_id, courier_id, assigned_by, status, assigned_at, ended_at, notes
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		);
	`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, asgn.ID, asgn.ParcelID, asgn.CourierID, asgn.AssignedBy, string(asgn.Status), asgn.AssignedAt, asgn.EndedAt, asgn.Notes)
	} else {
		_, err = r.db.ExecContext(ctx, query, asgn.ID, asgn.ParcelID, asgn.CourierID, asgn.AssignedBy, string(asgn.Status), asgn.AssignedAt, asgn.EndedAt, asgn.Notes)
	}

	if err != nil {
		return fmt.Errorf("failed to create courier assignment: %w", err)
	}
	return nil
}

// GetActiveByParcelID loads the currently active assignment for a parcel with an exclusive lock.
func (r *PostgresAssignmentRepository) GetActiveByParcelID(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*domain.CourierAssignment, error) {
	if tx == nil && r.db == nil {
		return nil, fmt.Errorf("failed to get active assignment: %w", db.ErrNilDB)
	}
	query := `
		SELECT id, parcel_id, courier_id, assigned_by, status, assigned_at, ended_at, notes
		FROM courier_assignments
		WHERE parcel_id = $1 AND status = 'ACTIVE'
		FOR UPDATE;
	`

	var row *sql.Row
	if tx != nil {
		row = tx.QueryRowContext(ctx, query, parcelID)
	} else {
		row = r.db.QueryRowContext(ctx, query, parcelID)
	}

	return r.scanRow(row)
}

// GetActiveByParcelIDReadOnly loads the currently active assignment without taking a row lock.
func (r *PostgresAssignmentRepository) GetActiveByParcelIDReadOnly(ctx context.Context, parcelID uuid.UUID) (*domain.CourierAssignment, error) {
	if r.db == nil {
		return nil, fmt.Errorf("failed to get active assignment: %w", db.ErrNilDB)
	}
	query := `
		SELECT id, parcel_id, courier_id, assigned_by, status, assigned_at, ended_at, notes
		FROM courier_assignments
		WHERE parcel_id = $1 AND status = 'ACTIVE';
	`
	return r.scanRow(r.db.QueryRowContext(ctx, query, parcelID))
}

// CountActiveByCourierID counts active assignments for a courier to enforce max capacity limits.
func (r *PostgresAssignmentRepository) CountActiveByCourierID(ctx context.Context, tx *sql.Tx, courierID uuid.UUID) (int, error) {
	if tx == nil && r.db == nil {
		return 0, fmt.Errorf("failed to count active courier assignments: %w", db.ErrNilDB)
	}
	query := `
		SELECT COUNT(*)
		FROM courier_assignments
		WHERE courier_id = $1 AND status = 'ACTIVE';
	`

	var count int
	var err error
	if tx != nil {
		err = tx.QueryRowContext(ctx, query, courierID).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx, query, courierID).Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to count active courier assignments: %w", err)
	}
	return count, nil
}

// Update persists status, ended_at, and notes changes on an existing assignment.
func (r *PostgresAssignmentRepository) Update(ctx context.Context, tx *sql.Tx, asgn *domain.CourierAssignment) error {
	if tx == nil && r.db == nil {
		return fmt.Errorf("failed to update courier assignment: %w", db.ErrNilDB)
	}
	query := `
		UPDATE courier_assignments
		SET status = $1, ended_at = $2, notes = $3
		WHERE id = $4;
	`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, string(asgn.Status), asgn.EndedAt, asgn.Notes, asgn.ID)
	} else {
		_, err = r.db.ExecContext(ctx, query, string(asgn.Status), asgn.EndedAt, asgn.Notes, asgn.ID)
	}

	if err != nil {
		return fmt.Errorf("failed to update assignment: %w", err)
	}
	return nil
}

func (r *PostgresAssignmentRepository) scanRow(row *sql.Row) (*domain.CourierAssignment, error) {
	var a domain.CourierAssignment
	var statusStr string
	var endedAt sql.NullTime
	var notes sql.NullString

	err := row.Scan(
		&a.ID,
		&a.ParcelID,
		&a.CourierID,
		&a.AssignedBy,
		&statusStr,
		&a.AssignedAt,
		&endedAt,
		&notes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan assignment row: %w", err)
	}

	a.Status = domain.AssignmentStatus(statusStr)
	if endedAt.Valid {
		t := endedAt.Time
		a.EndedAt = &t
	}
	if notes.Valid {
		a.Notes = notes.String
	}

	return &a, nil
}
