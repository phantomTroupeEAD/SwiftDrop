package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

// PostgresCourierRepository implements application.CourierRepository using PostgreSQL.
type PostgresCourierRepository struct {
	db *sql.DB
}

// NewPostgresCourierRepository constructs the repository.
func NewPostgresCourierRepository(db *sql.DB) *PostgresCourierRepository {
	return &PostgresCourierRepository{db: db}
}

// GetByID loads a courier profile by user ID.
func (r *PostgresCourierRepository) GetByID(ctx context.Context, userID uuid.UUID) (*domain.Courier, error) {
	query := `
		SELECT user_id, employee_code, vehicle_type, max_capacity, status, current_zone_id, is_active, created_at, updated_at
		FROM couriers
		WHERE user_id = $1;
	`
	return r.scanRow(r.db.QueryRowContext(ctx, query, userID))
}

// GetByIDWithLock acquires an exclusive row lock on the courier row for concurrency safety.
func (r *PostgresCourierRepository) GetByIDWithLock(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*domain.Courier, error) {
	query := `
		SELECT user_id, employee_code, vehicle_type, max_capacity, status, current_zone_id, is_active, created_at, updated_at
		FROM couriers
		WHERE user_id = $1
		FOR UPDATE;
	`

	var row *sql.Row
	if tx != nil {
		row = tx.QueryRowContext(ctx, query, userID)
	} else {
		row = r.db.QueryRowContext(ctx, query, userID)
	}

	return r.scanRow(row)
}

// Save inserts or updates a courier profile.
func (r *PostgresCourierRepository) Save(ctx context.Context, tx *sql.Tx, c *domain.Courier) error {
	query := `
		INSERT INTO couriers (
			user_id, employee_code, vehicle_type, max_capacity, status, current_zone_id, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		ON CONFLICT (user_id) DO UPDATE SET
			employee_code = EXCLUDED.employee_code,
			vehicle_type = EXCLUDED.vehicle_type,
			max_capacity = EXCLUDED.max_capacity,
			status = EXCLUDED.status,
			current_zone_id = EXCLUDED.current_zone_id,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at;
	`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, c.UserID, c.EmployeeCode, string(c.Vehicle), c.MaxCapacity, string(c.Status), c.CurrentZoneID, c.IsActive, c.CreatedAt, c.UpdatedAt)
	} else {
		_, err = r.db.ExecContext(ctx, query, c.UserID, c.EmployeeCode, string(c.Vehicle), c.MaxCapacity, string(c.Status), c.CurrentZoneID, c.IsActive, c.CreatedAt, c.UpdatedAt)
	}

	if err != nil {
		return fmt.Errorf("failed to save courier: %w", err)
	}
	return nil
}

// List queries courier profiles with optional filters.
func (r *PostgresCourierRepository) List(
	ctx context.Context,
	zoneID string,
	status *domain.ShiftStatus,
	vehicleType *domain.VehicleType,
) ([]*domain.Courier, error) {
	query := `
		SELECT user_id, employee_code, vehicle_type, max_capacity, status, current_zone_id, is_active, created_at, updated_at
		FROM couriers
		WHERE ($1 = '' OR current_zone_id = $1)
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR vehicle_type = $3)
		ORDER BY created_at DESC;
	`

	statusStr := ""
	if status != nil {
		statusStr = string(*status)
	}

	vehicleStr := ""
	if vehicleType != nil {
		vehicleStr = string(*vehicleType)
	}

	rows, err := r.db.QueryContext(ctx, query, zoneID, statusStr, vehicleStr)
	if err != nil {
		return nil, fmt.Errorf("failed to list couriers: %w", err)
	}
	defer rows.Close()

	var list []*domain.Courier
	for rows.Next() {
		c, err := r.scanRows(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}

	return list, rows.Err()
}

func (r *PostgresCourierRepository) scanRow(row *sql.Row) (*domain.Courier, error) {
	var c domain.Courier
	var vehicleStr string
	var statusStr string
	var zoneID sql.NullString
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&c.UserID,
		&c.EmployeeCode,
		&vehicleStr,
		&c.MaxCapacity,
		&statusStr,
		&zoneID,
		&c.IsActive,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan courier row: %w", err)
	}

	c.Vehicle = domain.VehicleType(vehicleStr)
	c.Status = domain.ShiftStatus(statusStr)
	if zoneID.Valid {
		c.CurrentZoneID = zoneID.String
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt

	return &c, nil
}

func (r *PostgresCourierRepository) scanRows(rows *sql.Rows) (*domain.Courier, error) {
	var c domain.Courier
	var vehicleStr string
	var statusStr string
	var zoneID sql.NullString
	var createdAt, updatedAt time.Time

	err := rows.Scan(
		&c.UserID,
		&c.EmployeeCode,
		&vehicleStr,
		&c.MaxCapacity,
		&statusStr,
		&zoneID,
		&c.IsActive,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan courier row: %w", err)
	}

	c.Vehicle = domain.VehicleType(vehicleStr)
	c.Status = domain.ShiftStatus(statusStr)
	if zoneID.Valid {
		c.CurrentZoneID = zoneID.String
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt

	return &c, nil
}

