package relay

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// OutboxRepository defines persistence operations for outbox relay.
type OutboxRepository interface {
	ClaimBatch(ctx context.Context, batchSize int, leaseDuration time.Duration) ([]EventRecord, error)
	MarkPublished(ctx context.Context, id string) error
	RecordFailure(ctx context.Context, id string, errStr string, nextAttemptAt time.Time, finalFailure bool) error
	GetOutboxStats(ctx context.Context) (pendingCount int64, oldestAgeSeconds float64, err error)
}

// PostgresOutboxRepository implements OutboxRepository against PostgreSQL.
type PostgresOutboxRepository struct {
	db *sql.DB
}

// NewPostgresOutboxRepository creates a new PostgresOutboxRepository.
func NewPostgresOutboxRepository(db *sql.DB) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{db: db}
}

// ClaimBatch claims a batch of pending outbox events within a short transaction and sets visibility leases.
func (r *PostgresOutboxRepository) ClaimBatch(ctx context.Context, batchSize int, leaseDuration time.Duration) ([]EventRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const selectQuery = `
		SELECT
			id, event_type, aggregate_type, aggregate_id, aggregate_version,
			schema_version, occurred_at, correlation_id, producer, payload, attempt_count
		FROM outbox_events
		WHERE status = 'PENDING'
		  AND (next_attempt_at IS NULL OR next_attempt_at <= now())
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED;
	`

	rows, err := tx.QueryContext(ctx, selectQuery, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending outbox events: %w", err)
	}
	defer rows.Close()

	var records []EventRecord
	var ids []string

	for rows.Next() {
		var rec EventRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.EventType,
			&rec.AggregateType,
			&rec.AggregateID,
			&rec.AggregateVersion,
			&rec.SchemaVersion,
			&rec.OccurredAt,
			&rec.CorrelationID,
			&rec.Producer,
			&rec.Payload,
			&rec.AttemptCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan outbox record: %w", err)
		}
		rec.EventID = rec.ID
		records = append(records, rec)
		ids = append(ids, rec.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	if len(records) == 0 {
		return nil, tx.Commit()
	}

	leaseSeconds := int(leaseDuration.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}

	const updateLeaseQuery = `
		UPDATE outbox_events
		SET next_attempt_at = now() + ($1 * interval '1 second')
		WHERE id = ANY($2);
	`

	if _, err := tx.ExecContext(ctx, updateLeaseQuery, leaseSeconds, pq.Array(ids)); err != nil {
		return nil, fmt.Errorf("failed to set visibility lease on outbox events: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit claim transaction: %w", err)
	}

	return records, nil
}

// MarkPublished marks an outbox row as PUBLISHED.
func (r *PostgresOutboxRepository) MarkPublished(ctx context.Context, id string) error {
	const query = `
		UPDATE outbox_events
		SET status = 'PUBLISHED',
		    published_at = now(),
		    last_error = NULL
		WHERE id = $1;
	`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark outbox event %s published: %w", id, err)
	}
	return nil
}

// RecordFailure updates attempt count, last error, and status upon publishing error.
func (r *PostgresOutboxRepository) RecordFailure(ctx context.Context, id string, errStr string, nextAttemptAt time.Time, finalFailure bool) error {
	const query = `
		UPDATE outbox_events
		SET attempt_count = attempt_count + 1,
		    last_error = $1,
		    next_attempt_at = $2,
		    status = CASE WHEN $3::boolean THEN 'FAILED' ELSE 'PENDING' END
		WHERE id = $4;
	`
	_, err := r.db.ExecContext(ctx, query, errStr, nextAttemptAt, finalFailure, id)
	if err != nil {
		return fmt.Errorf("failed to record outbox failure for event %s: %w", id, err)
	}
	return nil
}

// GetOutboxStats retrieves count and oldest pending age of outbox events.
func (r *PostgresOutboxRepository) GetOutboxStats(ctx context.Context) (int64, float64, error) {
	const query = `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_count,
			COALESCE(EXTRACT(EPOCH FROM (now() - MIN(created_at) FILTER (WHERE status = 'PENDING'))), 0) AS oldest_pending_age_seconds
		FROM outbox_events;
	`
	var pendingCount int64
	var oldestAge float64
	err := r.db.QueryRowContext(ctx, query).Scan(&pendingCount, &oldestAge)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get outbox stats: %w", err)
	}
	return pendingCount, oldestAge, nil
}
