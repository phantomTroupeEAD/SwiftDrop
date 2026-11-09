package events

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PostgresOutboxWriter persists event envelopes into the outbox_events table using an active PostgreSQL transaction.
type PostgresOutboxWriter struct{}

// NewPostgresOutboxWriter constructs a new PostgresOutboxWriter.
func NewPostgresOutboxWriter() *PostgresOutboxWriter {
	return &PostgresOutboxWriter{}
}

// Ensure PostgresOutboxWriter implements OutboxWriter.
var _ OutboxWriter = (*PostgresOutboxWriter)(nil)

// Write validates all envelopes and appends them to outbox_events in a single multi-row INSERT statement.
// The provided tx must be the active PostgreSQL transaction committing the domain aggregate change.
func (w *PostgresOutboxWriter) Write(ctx context.Context, tx *sql.Tx, envelopes ...AnyEnvelope) error {
	if tx == nil {
		return ErrMissingTransaction
	}

	if len(envelopes) == 0 {
		return nil
	}

	// 1. Validate all envelopes upfront before building SQL
	for _, env := range envelopes {
		if err := ValidateEnvelope(env); err != nil {
			return err
		}
	}

	// 2. Build multi-row parameterized query
	// Columns: id, producer, occurred_at, correlation_id, aggregate_type, aggregate_id, aggregate_version, event_type, schema_version, payload, status
	const colsPerRow = 11
	var (
		valuePlaceholders []string
		args              []any
	)

	for i, env := range envelopes {
		payloadBytes, err := env.SerializePayload()
		if err != nil {
			return fmt.Errorf("failed to serialize payload for envelope %s: %w", env.GetEventID(), err)
		}

		offset := i * colsPerRow
		placeholder := fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			offset+1, offset+2, offset+3, offset+4, offset+5, offset+6,
			offset+7, offset+8, offset+9, offset+10, offset+11,
		)
		valuePlaceholders = append(valuePlaceholders, placeholder)

		schemaVer := env.GetSchemaVersion()
		if schemaVer == "" {
			schemaVer = "1.0.0"
		}

		args = append(args,
			env.GetEventID(),
			env.GetProducer(),
			env.GetOccurredAt().UTC(),
			env.GetCorrelationID(),
			strings.ToLower(env.GetAggregateType()),
			env.GetAggregateID(),
			env.GetAggregateVersion(),
			env.GetEventType(),
			schemaVer,
			payloadBytes,
			"PENDING",
		)
	}

	//nolint:gosec // G201: dynamic value placeholders for batch insert with parameterized args
	query := fmt.Sprintf(`
		INSERT INTO outbox_events (
			id, producer, occurred_at, correlation_id,
			aggregate_type, aggregate_id, aggregate_version,
			event_type, schema_version, payload, status
		) VALUES %s;
	`, strings.Join(valuePlaceholders, ", "))

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to insert outbox events: %w", err)
	}

	return nil
}
