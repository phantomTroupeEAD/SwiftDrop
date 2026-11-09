package events

import (
	"context"
	"database/sql"
)

// OutboxWriter enables application use cases to persist integration events atomically.
// In accordance with Architecture Rule §4.3 and Decision D8, domain use cases must pass
// their active database transaction (*sql.Tx) to enlist outbox writes within the same atomic commit.
type OutboxWriter interface {
	// Write persists one or more event envelopes into outbox_events.
	// The provided tx MUST be the active transaction committing the domain aggregate change.
	Write(ctx context.Context, tx *sql.Tx, envelopes ...AnyEnvelope) error
}
