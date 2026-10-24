-- Member 4 — Merchant Integration
-- DRAFT migration. Depends on: merchants, users.
-- Member 8's webhook_deliveries references webhook_endpoints(id) and must be created AFTER this.

CREATE TABLE webhook_endpoints (
    id                      uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id             uuid          NOT NULL REFERENCES merchants (id) ON DELETE RESTRICT,
    url                     varchar(2048) NOT NULL,
    secret_ciphertext       bytea         NOT NULL,
    secret_key_version      smallint      NOT NULL,
    secret_last4            char(4)       NOT NULL,
    subscribed_event_types  text[]        NOT NULL,
    enabled                 boolean       NOT NULL DEFAULT true,
    created_by              uuid          NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at              timestamptz   NOT NULL DEFAULT now(),
    updated_at              timestamptz   NOT NULL DEFAULT now(),
    deleted_at              timestamptz,

    CONSTRAINT ck_webhooks_url_scheme  CHECK (url ~ '^https?://'),
    CONSTRAINT ck_webhooks_events      CHECK (cardinality(subscribed_event_types) >= 1),
    CONSTRAINT ck_webhooks_secret_len  CHECK (octet_length(secret_ciphertext) >= 28),
    CONSTRAINT ck_webhooks_deleted_disabled CHECK (deleted_at IS NULL OR enabled = false)
);

-- BR-W-08: one live endpoint per (merchant, url). Soft-deleted rows do not block re-registration.
CREATE UNIQUE INDEX uq_webhooks_merchant_url_live
    ON webhook_endpoints (merchant_id, url)
    WHERE deleted_at IS NULL;

-- Dashboard list + endpoint count (BR-W-09).
CREATE INDEX idx_webhooks_merchant_live
    ON webhook_endpoints (merchant_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- Event Worker lookup through merchant_webhook_targets view.
CREATE INDEX idx_webhooks_merchant_enabled
    ON webhook_endpoints (merchant_id)
    WHERE enabled = true AND deleted_at IS NULL;

COMMENT ON TABLE webhook_endpoints IS
  'Owner: Member 4. Webhook configuration. Member 8 reads ONLY via merchant_webhook_targets view.';
COMMENT ON COLUMN webhook_endpoints.secret_ciphertext IS
  'AES-256-GCM: nonce(12) || ciphertext || tag(16), key = WEBHOOK_SECRET_ENCRYPTION_KEY[secret_key_version].';
