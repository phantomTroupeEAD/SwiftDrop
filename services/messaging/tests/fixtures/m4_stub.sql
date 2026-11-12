-- ==============================================================================
-- Fixture: m4_stub.sql
-- Purpose: Provides stub tables and view for Member 04 (Merchants) for local
--          and integration testing before Member 04 migrations merge.
-- ==============================================================================

CREATE TABLE IF NOT EXISTS merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_name VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'APPROVED',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    url VARCHAR(2048) NOT NULL,
    secret_ciphertext BYTEA NOT NULL,
    secret_key_version SMALLINT NOT NULL DEFAULT 1,
    secret_last4 CHAR(4) NOT NULL DEFAULT 'abcd',
    subscribed_event_types TEXT[] NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL
);

-- Canonical view consumed by Member 08 Webhook Dispatcher
CREATE OR REPLACE VIEW merchant_webhook_targets AS
SELECT
    e.id AS endpoint_id,
    e.merchant_id,
    e.url,
    e.secret_ciphertext,
    e.secret_key_version,
    e.subscribed_event_types,
    e.updated_at
FROM webhook_endpoints e
JOIN merchants m ON m.id = e.merchant_id
WHERE e.enabled = true
  AND e.deleted_at IS NULL
  AND m.status = 'APPROVED';

