-- Migration: 20261110000003_create_idempotency_keys.up.sql
-- Owner: Member 2 (Parcel Management & Pricing)
-- Description: Stores caller-scoped request idempotency records with 24-hour expiration.
-- Note: Role grants are coordinated separately with Member 9 (Database Coordinator).

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key                 VARCHAR(255) NOT NULL CHECK (length(btrim(key)) > 0),
    caller_scope        VARCHAR(64)  NOT NULL CHECK (caller_scope ~ '^(USER|MERCHANT):[^[:space:]]+$'),
    operation           VARCHAR(64)  NOT NULL CHECK (length(btrim(operation)) > 0),
    request_hash        CHAR(64)     NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    response_status     INTEGER      NOT NULL CHECK (response_status BETWEEN 100 AND 599),
    response_body       JSONB        NOT NULL,
    created_resource_id UUID         NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at          TIMESTAMPTZ  NOT NULL CHECK (expires_at > created_at),

    PRIMARY KEY (key, caller_scope)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at
    ON idempotency_keys (expires_at ASC);

