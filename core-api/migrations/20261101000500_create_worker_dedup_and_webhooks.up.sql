-- ==============================================================================
-- Migration: 20261101000500_create_worker_dedup_and_webhooks.up.sql
-- Module: Event Worker & Messaging Support Context (Member 08)
-- Tables: processed_events, notification_deliveries, webhook_deliveries, webhook_delivery_attempts
-- Rules:
--   - No BEGIN / COMMIT blocks (migrator handles transactions atomically).
--   - Role grants are conditional (guarded against undefined roles in dev/test).
-- ==============================================================================

-- 1. Consumer Idempotency Ledger (Authoritative deduplication)
CREATE TABLE IF NOT EXISTS processed_events (
    consumer_name VARCHAR(64) NOT NULL,
    event_id UUID NOT NULL,
    processed_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (consumer_name, event_id)
);

CREATE INDEX IF NOT EXISTS idx_processed_cleanup 
ON processed_events (processed_at ASC);

-- 2. Notification Deliveries Log (Real SMTP Email & SMS Audit)
CREATE TABLE IF NOT EXISTS notification_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version INTEGER NOT NULL CHECK (aggregate_version >= 1),
    payload JSONB NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    channel VARCHAR(32) DEFAULT 'EMAIL' NOT NULL 
        CHECK (channel IN ('EMAIL', 'SMS')),
    template_name VARCHAR(64) DEFAULT 'milestone_notice' NOT NULL,
    status VARCHAR(32) DEFAULT 'PENDING' NOT NULL 
        CHECK (status IN ('PENDING', 'IN_PROGRESS', 'SUCCESS', 'FAILED')),
    attempt_count INTEGER DEFAULT 0 NOT NULL CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    provider_message_id VARCHAR(255) NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    delivered_at TIMESTAMPTZ NULL,
    CONSTRAINT uq_notification_delivery UNIQUE (event_id, channel, recipient)
);

CREATE INDEX IF NOT EXISTS idx_notif_recipient 
ON notification_deliveries (recipient, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notif_retry 
ON notification_deliveries (next_attempt_at ASC) 
WHERE status IN ('PENDING', 'IN_PROGRESS');

-- 3. Webhook Deliveries State & Parent Record
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID NOT NULL,
    event_id UUID NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version INTEGER NOT NULL CHECK (aggregate_version >= 1),
    occurred_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(32) DEFAULT 'PENDING' NOT NULL 
        CHECK (status IN ('PENDING', 'IN_PROGRESS', 'SUCCESS', 'FAILED')),
    attempt_count INTEGER DEFAULT 0 NOT NULL CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    response_status_code INTEGER NULL,
    response_body_snippet VARCHAR(500) NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    delivered_at TIMESTAMPTZ NULL,
    CONSTRAINT uq_webhook_delivery UNIQUE (endpoint_id, event_id)
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoint 
ON webhook_deliveries (endpoint_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_webhook_retry_queue 
ON webhook_deliveries (next_attempt_at ASC) 
WHERE status IN ('PENDING', 'IN_PROGRESS');

-- Conditional Foreign Key to webhook_endpoints if Member 04's table exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'webhook_endpoints') THEN
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.table_constraints 
            WHERE constraint_name = 'fk_webhook_deliveries_endpoint'
        ) THEN
            ALTER TABLE webhook_deliveries 
            ADD CONSTRAINT fk_webhook_deliveries_endpoint 
            FOREIGN KEY (endpoint_id) REFERENCES webhook_endpoints(id) ON DELETE RESTRICT;
        END IF;
    END IF;
END $$;

-- 4. Append-Only Webhook Delivery Attempts History (SRS FR-MER-011)
CREATE TABLE IF NOT EXISTS webhook_delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id UUID NOT NULL REFERENCES webhook_deliveries(id) ON DELETE CASCADE,
    attempt_no INTEGER NOT NULL CHECK (attempt_no >= 1),
    started_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    response_status_code INTEGER NULL,
    response_body_snippet VARCHAR(500) NULL,
    error_class VARCHAR(64) NULL,
    error_message TEXT NULL,
    next_attempt_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_webhook_attempts_delivery 
ON webhook_delivery_attempts (delivery_id, attempt_no ASC);

-- 5. Guarded Role Grants
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_worker') THEN
        GRANT SELECT, INSERT, UPDATE ON processed_events TO swiftdrop_worker;
        GRANT SELECT, INSERT, UPDATE ON notification_deliveries TO swiftdrop_worker;
        GRANT SELECT, INSERT, UPDATE ON webhook_deliveries TO swiftdrop_worker;
        GRANT SELECT, INSERT ON webhook_delivery_attempts TO swiftdrop_worker;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        GRANT SELECT ON notification_deliveries TO swiftdrop_app;
        GRANT SELECT ON webhook_deliveries TO swiftdrop_app;
        GRANT SELECT ON webhook_delivery_attempts TO swiftdrop_app;
        GRANT UPDATE (status, attempt_count, next_attempt_at) ON webhook_deliveries TO swiftdrop_app;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_reconciler') THEN
        GRANT SELECT, UPDATE, DELETE ON processed_events TO swiftdrop_reconciler;
        GRANT SELECT, UPDATE, DELETE ON notification_deliveries TO swiftdrop_reconciler;
        GRANT SELECT, UPDATE, DELETE ON webhook_deliveries TO swiftdrop_reconciler;
        GRANT SELECT, DELETE ON webhook_delivery_attempts TO swiftdrop_reconciler;
    END IF;
END $$;

