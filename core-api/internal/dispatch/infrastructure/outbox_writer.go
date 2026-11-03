package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// PostgresOutboxWriter implements application.OutboxWriter
// to commit versioned integration events to PostgreSQL within caller's transaction.
type PostgresOutboxWriter struct {
	db *sql.DB
}

// NewPostgresOutboxWriter constructs the outbox writer.
func NewPostgresOutboxWriter(db *sql.DB) *PostgresOutboxWriter {
	return &PostgresOutboxWriter{db: db}
}

// Write appends an event to the outbox_events table within the active transaction.
func (w *PostgresOutboxWriter) Write(
	ctx context.Context,
	tx *sql.Tx,
	aggregateType string,
	aggregateID uuid.UUID,
	aggregateVersion int,
	eventType string,
	payload any,
	correlationID string,
) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox event payload: %w", err)
	}

	query := `
		INSERT INTO outbox_events (
			id,
			producer,
			occurred_at,
			correlation_id,
			aggregate_type,
			aggregate_id,
			aggregate_version,
			event_type,
			schema_version,
			payload,
			status,
			attempt_count,
			created_at
		) VALUES (
			gen_random_uuid(),
			'core-api',
			NOW(),
			$1,
			$2,
			$3,
			$4,
			$5,
			'1.0.0',
			$6,
			'PENDING',
			0,
			NOW()
		);
	`

	if tx != nil {
		_, err = tx.ExecContext(ctx, query, correlationID, aggregateType, aggregateID, aggregateVersion, eventType, payloadJSON)
	} else {
		_, err = w.db.ExecContext(ctx, query, correlationID, aggregateType, aggregateID, aggregateVersion, eventType, payloadJSON)
	}

	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}
	return nil
}

