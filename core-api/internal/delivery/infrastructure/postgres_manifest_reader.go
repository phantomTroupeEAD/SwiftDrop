package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/application"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresManifestReader implements application.CourierManifestReader
// with data minimization for couriers on active delivery runs.
type PostgresManifestReader struct {
	db *sql.DB
}

// NewPostgresManifestReader constructs a PostgresManifestReader instance.
func NewPostgresManifestReader(db *sql.DB) *PostgresManifestReader {
	return &PostgresManifestReader{db: db}
}

// GetActiveManifest retrieves the list of active delivery tasks for a specific courier.
func (r *PostgresManifestReader) GetActiveManifest(ctx context.Context, courierID uuid.UUID) ([]application.CourierManifestItemDTO, error) {
	if r.db == nil {
		return nil, fmt.Errorf("manifest reader: %w", db.ErrNilDB)
	}
	query := `
		SELECT 
			ca.parcel_id::text,
			p.tracking_number,
			p.status,
			COALESCE(p.dropoff_address->>'zone_id', '') AS destination_zone,
			COALESCE(p.dropoff_address->>'contact_name', '') AS recipient_name,
			COALESCE(p.dropoff_address->>'contact_phone', '') AS recipient_phone,
			COALESCE(p.dropoff_address->>'address_line1', '') AS delivery_street,
			COALESCE(p.dropoff_address->>'city', '') AS delivery_city,
			COALESCE(p.actual_weight_kg, 1.0) AS weight_kg,
			COALESCE(p.dropoff_address->>'handling_instructions', '') AS handling_instructions,
			p.attempt_count,
			ca.assigned_at
		FROM courier_assignments ca
		JOIN parcels p ON p.id = ca.parcel_id

		WHERE ca.courier_id = $1 AND ca.status = 'ACTIVE'
		ORDER BY ca.assigned_at ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, courierID)
	if err != nil {
		return nil, fmt.Errorf("failed to query courier manifest: %w", err)
	}
	defer rows.Close()

	var manifest []application.CourierManifestItemDTO
	for rows.Next() {
		var item application.CourierManifestItemDTO
		err := rows.Scan(
			&item.ParcelID,
			&item.TrackingNumber,
			&item.CurrentStatus,
			&item.DestinationZone,
			&item.RecipientName,
			&item.RecipientPhone,
			&item.DeliveryStreet,
			&item.DeliveryCity,
			&item.WeightKg,
			&item.HandlingInstructions,
			&item.AttemptCount,
			&item.AssignedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan manifest row for courier %s: %w", courierID, err)
		}
		manifest = append(manifest, item)
	}

	return manifest, rows.Err()
}
