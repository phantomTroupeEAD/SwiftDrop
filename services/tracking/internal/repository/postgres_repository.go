package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/swiftdrop/tracking/internal/domain"
)

var _ TrackingRepository = (*PostgresTrackingRepository)(nil)

// PostgresTrackingRepository implements TrackingRepository against PostgreSQL
// with strictly read-only semantics.
type PostgresTrackingRepository struct {
	db           *sql.DB
	queryTimeout time.Duration
}

// NewPostgresTrackingRepository constructs a new PostgresTrackingRepository with the given *sql.DB and pool configuration.
func NewPostgresTrackingRepository(db *sql.DB, cfg PoolConfig) *PostgresTrackingRepository {
	if cfg.QueryTimeout <= 0 {
		cfg.QueryTimeout = 3 * time.Second
	}
	return &PostgresTrackingRepository{
		db:           db,
		queryTimeout: cfg.QueryTimeout,
	}
}

// OpenPostgresDB initializes and configures a bounded *sql.DB connection pool for PostgreSQL.
func OpenPostgresDB(databaseURL string, cfg PoolConfig) (*sql.DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("empty database url")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	if cfg.MaxOpenConns <= 0 {
		cfg.MaxOpenConns = 5
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 2
	}
	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		cfg.MaxIdleConns = cfg.MaxOpenConns
	}
	if cfg.ConnMaxLifetime <= 0 {
		cfg.ConnMaxLifetime = 15 * time.Minute
	}
	if cfg.ConnMaxIdleTime <= 0 {
		cfg.ConnMaxIdleTime = 5 * time.Minute
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return db, nil
}

// Ping verifies database connectivity within a bounded timeout.
func (r *PostgresTrackingRepository) Ping(ctx context.Context) error {
	if r.db == nil {
		return domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	if err := r.db.PingContext(timeoutCtx); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrDatabaseUnavailable, err)
	}
	return nil
}

// CheckReadiness verifies both PostgreSQL connectivity and schema readiness.
// In accordance with Task 5, it probes whether authoritative tracking tables exist
// and the configured role (swiftdrop_tracking_ro) has SELECT permissions,
// preventing the service from falsely advertising readiness.
func (r *PostgresTrackingRepository) CheckReadiness(ctx context.Context) error {
	if r.db == nil {
		return domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	// 1. Verify connection reachability
	if err := r.db.PingContext(timeoutCtx); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrDatabaseUnavailable, err)
	}

	// 2. Verify parcels table existence and SELECT privilege (LIMIT 0 prevents reading table data)
	const parcelsProbe = "SELECT 1 FROM parcels LIMIT 0;"
	if _, err := r.db.ExecContext(timeoutCtx, parcelsProbe); err != nil {
		return fmt.Errorf("readiness probe failed for parcels table: %w", err)
	}

	// 3. Verify parcel_status_history table existence and SELECT privilege
	const historyProbe = "SELECT 1 FROM parcel_status_history LIMIT 0;"
	if _, err := r.db.ExecContext(timeoutCtx, historyProbe); err != nil {
		return fmt.Errorf("readiness probe failed for parcel_status_history table: %w", err)
	}

	return nil
}

// Close gracefully terminates all database pool connections.
func (r *PostgresTrackingRepository) Close() error {
	if r.db == nil {
		return nil
	}
	return r.db.Close()
}

// FindParcelByTrackingNumber queries the authoritative parcels table for a single parcel
// and extracts the destination city directly from JSONB dropoff_address->>'city'.
func (r *PostgresTrackingRepository) FindParcelByTrackingNumber(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, error) {
	if r.db == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	const query = `
		SELECT
			id,
			tracking_number,
			status,
			sender_id,
			merchant_id,
			COALESCE(dropoff_address->>'city', '') AS destination_city,
			COALESCE(dropoff_address->>'postal_code', '') AS destination_postal_code,
			COALESCE(dropoff_address->>'zone_id', '') AS destination_zone_id,
			created_at,
			updated_at
		FROM parcels
		WHERE tracking_number = $1
		LIMIT 1;
	`

	var (
		id         string
		tnStr      string
		statusStr  string
		senderID   sql.NullString
		merchID    sql.NullString
		destCity   sql.NullString
		destPostal sql.NullString
		destZone   sql.NullString
		createdAt  time.Time
		updatedAt  time.Time
	)

	err := r.db.QueryRowContext(timeoutCtx, query, trackingNumber).Scan(
		&id,
		&tnStr,
		&statusStr,
		&senderID,
		&merchID,
		&destCity,
		&destPostal,
		&destZone,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTrackingNotFound
		}
		if errors.Is(timeoutCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: query parcel: %v", domain.ErrDatabaseUnavailable, err)
	}

	tn, err := domain.NewTrackingNumber(tnStr)
	if err != nil {
		return nil, fmt.Errorf("corrupt tracking number in db: %w", err)
	}

	var senderPtr *string
	if senderID.Valid && strings.TrimSpace(senderID.String) != "" {
		s := strings.TrimSpace(senderID.String)
		senderPtr = &s
	}

	var merchPtr *string
	if merchID.Valid && strings.TrimSpace(merchID.String) != "" {
		m := strings.TrimSpace(merchID.String)
		merchPtr = &m
	}

	var city, postalCode, zoneID string
	if destCity.Valid {
		city = destCity.String
	}
	if destPostal.Valid {
		postalCode = destPostal.String
	}
	if destZone.Valid {
		zoneID = destZone.String
	}

	return &domain.ParcelTrackingInfo{
		ID:                    id,
		TrackingNumber:        tn,
		Status:                domain.ParcelStatus(statusStr),
		SenderID:              senderPtr,
		MerchantID:            merchPtr,
		DestinationCity:       city,
		DestinationPostalCode: postalCode,
		DestinationZoneID:     zoneID,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}, nil
}

// FindStatusHistoryByParcelID queries the authoritative parcel_status_history table
// returning chronological milestones with deterministic ordering (occurred_at ASC, id ASC).
// Aligns strictly with Member 2's authoritative schema (occurred_at).
func (r *PostgresTrackingRepository) FindStatusHistoryByParcelID(ctx context.Context, parcelID string) ([]domain.MilestoneRecord, error) {
	if r.db == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	const query = `
		SELECT
			id,
			parcel_id,
			from_status,
			to_status,
			actor_id,
			actor_type,
			actor_role,
			reason,
			occurred_at
		FROM parcel_status_history
		WHERE parcel_id = $1
		ORDER BY occurred_at ASC, id ASC;
	`

	rows, err := r.db.QueryContext(timeoutCtx, query, parcelID)
	if err != nil {
		if errors.Is(timeoutCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: query parcel status history: %v", domain.ErrDatabaseUnavailable, err)
	}
	defer rows.Close()

	var records []domain.MilestoneRecord
	for rows.Next() {
		var (
			rec        domain.MilestoneRecord
			fromStatus sql.NullString
			actorID    sql.NullString
		)

		err := rows.Scan(
			&rec.ID,
			&rec.ParcelID,
			&fromStatus,
			&rec.ToStatus,
			&actorID,
			&rec.ActorType,
			&rec.ActorRole,
			&rec.Reason,
			&rec.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("scan parcel status history row: %w", err)
		}

		if fromStatus.Valid {
			statusVal := fromStatus.String
			rec.FromStatus = &statusVal
		}
		if actorID.Valid {
			rec.ActorID = actorID.String
		}

		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: iterate parcel status history: %v", domain.ErrDatabaseUnavailable, err)
	}

	return records, nil
}

// FindStatusHistoryByID queries the authoritative parcel_status_history table
// for a single status-history entry by its unique ID (UUIDv7).
// Correlates incoming RabbitMQ integration events (payload.status_history_id)
// directly with persisted status history entries.
func (r *PostgresTrackingRepository) FindStatusHistoryByID(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
	if r.db == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	cleanID := strings.TrimSpace(historyID)
	if cleanID == "" {
		return nil, errors.New("empty status history id")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	const query = `
		SELECT
			id,
			parcel_id,
			from_status,
			to_status,
			actor_id,
			actor_type,
			actor_role,
			reason,
			occurred_at
		FROM parcel_status_history
		WHERE id = $1
		LIMIT 1;
	`

	var (
		rec        domain.MilestoneRecord
		fromStatus sql.NullString
		actorID    sql.NullString
	)

	err := r.db.QueryRowContext(timeoutCtx, query, cleanID).Scan(
		&rec.ID,
		&rec.ParcelID,
		&fromStatus,
		&rec.ToStatus,
		&actorID,
		&rec.ActorType,
		&rec.ActorRole,
		&rec.Reason,
		&rec.Timestamp,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTrackingNotFound
		}
		if errors.Is(timeoutCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: query status history by id: %v", domain.ErrDatabaseUnavailable, err)
	}

	if fromStatus.Valid {
		statusVal := fromStatus.String
		rec.FromStatus = &statusVal
	}
	if actorID.Valid {
		rec.ActorID = actorID.String
	}

	return &rec, nil
}

// FindTrackingSummary retrieves both parcel details and chronological status history
// within a single consistent read-only transaction.
func (r *PostgresTrackingRepository) FindTrackingSummary(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
	if r.db == nil {
		return nil, nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(timeoutCtx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		if errors.Is(timeoutCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("%w: begin repeatable-read tx: %v", domain.ErrDatabaseUnavailable, err)
	}
	defer tx.Rollback()

	// 1. Query Parcel
	const parcelQuery = `
		SELECT
			id,
			tracking_number,
			status,
			sender_id,
			merchant_id,
			COALESCE(dropoff_address->>'city', '') AS destination_city,
			COALESCE(dropoff_address->>'postal_code', '') AS destination_postal_code,
			COALESCE(dropoff_address->>'zone_id', '') AS destination_zone_id,
			created_at,
			updated_at
		FROM parcels
		WHERE tracking_number = $1
		LIMIT 1;
	`

	var (
		id         string
		tnStr      string
		statusStr  string
		senderID   sql.NullString
		merchID    sql.NullString
		destCity   sql.NullString
		destPostal sql.NullString
		destZone   sql.NullString
		createdAt  time.Time
		updatedAt  time.Time
	)

	err = tx.QueryRowContext(timeoutCtx, parcelQuery, trackingNumber).Scan(
		&id,
		&tnStr,
		&statusStr,
		&senderID,
		&merchID,
		&destCity,
		&destPostal,
		&destZone,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, domain.ErrTrackingNotFound
		}
		if errors.Is(timeoutCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("%w: query parcel in tx: %v", domain.ErrDatabaseUnavailable, err)
	}

	tn, err := domain.NewTrackingNumber(tnStr)
	if err != nil {
		return nil, nil, fmt.Errorf("corrupt tracking number in db: %w", err)
	}

	var senderPtr *string
	if senderID.Valid && strings.TrimSpace(senderID.String) != "" {
		s := strings.TrimSpace(senderID.String)
		senderPtr = &s
	}

	var merchPtr *string
	if merchID.Valid && strings.TrimSpace(merchID.String) != "" {
		m := strings.TrimSpace(merchID.String)
		merchPtr = &m
	}

	var city, postalCode, zoneID string
	if destCity.Valid {
		city = destCity.String
	}
	if destPostal.Valid {
		postalCode = destPostal.String
	}
	if destZone.Valid {
		zoneID = destZone.String
	}

	parcel := &domain.ParcelTrackingInfo{
		ID:                    id,
		TrackingNumber:        tn,
		Status:                domain.ParcelStatus(statusStr),
		SenderID:              senderPtr,
		MerchantID:            merchPtr,
		DestinationCity:       city,
		DestinationPostalCode: postalCode,
		DestinationZoneID:     zoneID,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}

	// 2. Query History
	const historyQuery = `
		SELECT
			id,
			parcel_id,
			from_status,
			to_status,
			actor_id,
			actor_type,
			actor_role,
			reason,
			occurred_at
		FROM parcel_status_history
		WHERE parcel_id = $1
		ORDER BY occurred_at ASC, id ASC;
	`

	rows, err := tx.QueryContext(timeoutCtx, historyQuery, id)
	if err != nil {
		if errors.Is(timeoutCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("%w: query history in tx: %v", domain.ErrDatabaseUnavailable, err)
	}
	defer rows.Close()

	var records []domain.MilestoneRecord
	for rows.Next() {
		var (
			rec        domain.MilestoneRecord
			fromStatus sql.NullString
			actorID    sql.NullString
		)

		err := rows.Scan(
			&rec.ID,
			&rec.ParcelID,
			&fromStatus,
			&rec.ToStatus,
			&actorID,
			&rec.ActorType,
			&rec.ActorRole,
			&rec.Reason,
			&rec.Timestamp,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("scan history row: %w", err)
		}

		if fromStatus.Valid {
			statusVal := fromStatus.String
			rec.FromStatus = &statusVal
		}
		if actorID.Valid {
			rec.ActorID = actorID.String
		}

		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("%w: iterate history rows: %v", domain.ErrDatabaseUnavailable, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("%w: commit read-only tx: %v", domain.ErrDatabaseUnavailable, err)
	}

	return parcel, records, nil
}
