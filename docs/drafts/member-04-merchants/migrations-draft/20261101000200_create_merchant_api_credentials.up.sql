-- Member 4 — Merchant Integration
-- DRAFT migration. Depends on: merchants, users.

CREATE TABLE merchant_api_credentials (
    id              uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id     uuid         NOT NULL REFERENCES merchants (id) ON DELETE RESTRICT,
    key_prefix      char(8)      NOT NULL,
    key_hash        char(64)     NOT NULL,
    pepper_version  smallint     NOT NULL DEFAULT 1,
    label           varchar(60)  NOT NULL,
    scopes          text[]       NOT NULL DEFAULT ARRAY['parcels:create', 'parcels:read'],
    created_by      uuid         NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at      timestamptz  NOT NULL DEFAULT now(),
    expires_at      timestamptz,
    revoked_at      timestamptz,
    last_used_at    timestamptz,

    CONSTRAINT uq_credentials_key_prefix UNIQUE (key_prefix),
    CONSTRAINT ck_credentials_key_prefix CHECK (key_prefix ~ '^[0-9A-Za-z]{8}$'),
    CONSTRAINT ck_credentials_key_hash   CHECK (key_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_credentials_label      CHECK (char_length(btrim(label)) BETWEEN 1 AND 60),
    CONSTRAINT ck_credentials_scopes
        CHECK (cardinality(scopes) >= 1
               AND scopes <@ ARRAY['parcels:create', 'parcels:read']::text[]),  -- ADR-003 names; webhooks:manage not grantable (Doc 7 C-3)
    CONSTRAINT ck_credentials_expiry     CHECK (expires_at IS NULL OR expires_at > created_at),
    CONSTRAINT ck_credentials_revoked    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

-- BR-K-05: label unique among non-revoked keys of a merchant.
CREATE UNIQUE INDEX uq_credentials_merchant_label_active
    ON merchant_api_credentials (merchant_id, label)
    WHERE revoked_at IS NULL;

-- Key list for dashboard and active-key count (BR-K-09).
CREATE INDEX idx_credentials_merchant_created
    ON merchant_api_credentials (merchant_id, created_at DESC);

COMMENT ON COLUMN merchant_api_credentials.key_prefix IS
  'The 8-char public prefix from the key format sd_live_<prefix>_<secret> (ADR-003 §6.1).';
COMMENT ON TABLE merchant_api_credentials IS
  'Owner: Member 4. Stores ONLY prefix + HMAC-SHA256 hash of merchant API keys. No other module may read this table.';
COMMENT ON COLUMN merchant_api_credentials.key_hash IS
  'hex(HMAC-SHA256(pepper[pepper_version], full_key)). Plaintext key is never stored.';
