-- SwiftDrop: Migration 20261101000050 (Member 08 - Transactional Outbox Engine)
-- Executed inside core-api monolithic schema migrations (migrator wraps each file in transaction)

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    producer VARCHAR(64) DEFAULT 'core-api' NOT NULL,
    occurred_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    correlation_id VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version INTEGER DEFAULT 1 NOT NULL CHECK (aggregate_version >= 1),
    event_type VARCHAR(128) NOT NULL CHECK (event_type ~ '^[a-z_]+\.[a-z_]+\.v[0-9]+$'),
    schema_version VARCHAR(16) DEFAULT '1.0.0' NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(32) DEFAULT 'PENDING' NOT NULL 
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
    attempt_count INTEGER DEFAULT 0 NOT NULL CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    published_at TIMESTAMPTZ NULL
);

-- Partial index for high-throughput pollers skipping published events
-- Note: IMMUTABLE predicate only. Dynamic time filtering (next_attempt_at <= now()) is performed in query.
CREATE INDEX IF NOT EXISTS idx_outbox_pending 
ON outbox_events (created_at ASC) 
WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_outbox_aggregate 
ON outbox_events (aggregate_type, aggregate_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_outbox_correlation 
ON outbox_events (correlation_id);

CREATE INDEX IF NOT EXISTS idx_outbox_retention 
ON outbox_events (published_at ASC) 
WHERE status = 'PUBLISHED';

-- Guarded role grants (D6)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_relay') THEN
        GRANT SELECT, UPDATE ON outbox_events TO swiftdrop_relay;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_reconciler') THEN
        GRANT SELECT, UPDATE, DELETE ON outbox_events TO swiftdrop_reconciler;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        GRANT INSERT, SELECT ON outbox_events TO swiftdrop_app;
    END IF;
END $$;

