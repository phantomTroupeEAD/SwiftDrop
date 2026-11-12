package idempotency

import (
	"context"
	"database/sql"
	"fmt"
)

// Store defines operations for consumer idempotency tracking.
type Store interface {
	Seen(ctx context.Context, consumerName, eventID string) (bool, error)
	MarkDone(ctx context.Context, tx *sql.Tx, consumerName, eventID string) error
}

// PostgresStore implements Store against PostgreSQL processed_events table.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// Seen checks if an event has already reached terminal processing for a consumer.
func (s *PostgresStore) Seen(ctx context.Context, consumerName, eventID string) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM processed_events
			WHERE consumer_name = $1 AND event_id = $2
		);
	`
	var exists bool
	if err := s.db.QueryRowContext(ctx, query, consumerName, eventID).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check idempotency for %s/%s: %w", consumerName, eventID, err)
	}
	return exists, nil
}

// MarkDone atomically marks an event as processed within an existing transaction.
func (s *PostgresStore) MarkDone(ctx context.Context, tx *sql.Tx, consumerName, eventID string) error {
	const query = `
		INSERT INTO processed_events (consumer_name, event_id, processed_at)
		VALUES ($1, $2, now())
		ON CONFLICT (consumer_name, event_id) DO NOTHING;
	`
	if tx != nil {
		_, err := tx.ExecContext(ctx, query, consumerName, eventID)
		if err != nil {
			return fmt.Errorf("failed to record processed_event in tx for %s/%s: %w", consumerName, eventID, err)
		}
		return nil
	}

	_, err := s.db.ExecContext(ctx, query, consumerName, eventID)
	if err != nil {
		return fmt.Errorf("failed to record processed_event for %s/%s: %w", consumerName, eventID, err)
	}
	return nil
}
