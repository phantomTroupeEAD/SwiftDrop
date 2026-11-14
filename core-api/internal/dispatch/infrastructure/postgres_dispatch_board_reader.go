package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/dispatch/application"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresDispatchBoardReader implements application.DispatchBoardReader with optimized SQL joins.
type PostgresDispatchBoardReader struct {
	db *sql.DB
}

// NewPostgresDispatchBoardReader constructs the reader.
func NewPostgresDispatchBoardReader(db *sql.DB) *PostgresDispatchBoardReader {
	return &PostgresDispatchBoardReader{db: db}
}

// GetUnassignedParcels queries eligible unassigned parcels for a target delivery zone.
func (r *PostgresDispatchBoardReader) GetUnassignedParcels(ctx context.Context, _ string) ([]application.UnassignedParcelDTO, error) {
	if r.db == nil {
		return nil, fmt.Errorf("failed to get unassigned parcels: %w", db.ErrNilDB)
	}
	// Query parcels in CREATED or ATTEMPT_FAILED status
	query := `
		SELECT 
			p.id,
			p.tracking_number,
			p.actual_weight_kg AS weight_kg,
			COALESCE(p.dropoff_address->>'city', '') AS recipient_city,
			p.status,
			p.created_at
		FROM parcels p
		WHERE p.status IN ('CREATED', 'ATTEMPT_FAILED')
		ORDER BY p.created_at ASC
		LIMIT 100;
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unassigned parcels: %w", err)
	}
	defer rows.Close()

	var list []application.UnassignedParcelDTO
	for rows.Next() {
		var item application.UnassignedParcelDTO
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.TrackingNumber, &item.WeightKg, &item.RecipientCity, &item.Status, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan unassigned parcel: %w", err)
		}
		item.CreatedAt = createdAt
		list = append(list, item)
	}

	return list, rows.Err()
}

// GetCourierLoads queries couriers and their active parcel workloads for a zone.
func (r *PostgresDispatchBoardReader) GetCourierLoads(ctx context.Context, zoneID string) ([]application.CourierLoadDTO, error) {
	if r.db == nil {
		return nil, fmt.Errorf("failed to get courier loads: %w", db.ErrNilDB)
	}
	query := `
		SELECT 
			c.user_id,
			COALESCE(u.display_name, c.employee_code) AS courier_name,
			c.employee_code,
			COALESCE(c.current_zone_id, '') AS current_zone_id,
			COUNT(ca.id) FILTER (WHERE ca.status = 'ACTIVE') AS active_count,
			c.max_capacity,
			c.status,
			c.vehicle_type
		FROM couriers c
		LEFT JOIN users u ON u.id = c.user_id
		LEFT JOIN courier_assignments ca ON ca.courier_id = c.user_id
		WHERE ($1 = '' OR c.current_zone_id = $1)
		  AND c.is_active = TRUE
		GROUP BY c.user_id, u.display_name, c.employee_code, c.current_zone_id, c.max_capacity, c.status, c.vehicle_type
		ORDER BY active_count ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to query courier workloads: %w", err)
	}
	defer rows.Close()

	var list []application.CourierLoadDTO
	for rows.Next() {
		var item application.CourierLoadDTO
		var statusStr string
		var vehicleStr string

		err := rows.Scan(
			&item.CourierID,
			&item.Name,
			&item.EmployeeCode,
			&item.CurrentZoneID,
			&item.ActiveAssignments,
			&item.MaxCapacity,
			&statusStr,
			&vehicleStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan courier load: %w", err)
		}

		item.Status = domain.ShiftStatus(statusStr)
		item.VehicleType = domain.VehicleType(vehicleStr)
		list = append(list, item)
	}

	return list, rows.Err()
}

func (r *PostgresDispatchBoardReader) GetAssignedParcels(ctx context.Context, zoneID string) ([]application.AssignedParcelDTO, error) {
	if r.db == nil {
		return nil, db.ErrNilDB
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.tracking_number, p.actual_weight_kg, COALESCE(p.dropoff_address->>'city', ''),
		       p.status, p.created_at, a.courier_id
		FROM courier_assignments a JOIN parcels p ON p.id = a.parcel_id
		WHERE a.status = 'ACTIVE' AND ($1 = '' OR p.destination_zone_id = $1)
		ORDER BY a.assigned_at DESC, a.id DESC LIMIT 100
	`, zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to query assigned parcels: %w", err)
	}
	defer rows.Close()
	var parcels []application.AssignedParcelDTO
	for rows.Next() {
		var p application.AssignedParcelDTO
		if err := rows.Scan(&p.ID, &p.TrackingNumber, &p.WeightKg, &p.RecipientCity, &p.Status, &p.CreatedAt, &p.CourierID); err != nil {
			return nil, err
		}
		parcels = append(parcels, p)
	}
	return parcels, rows.Err()
}
