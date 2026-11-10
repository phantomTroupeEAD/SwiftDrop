-- Migration: 20261110000001_create_pricing_tables.up.sql
-- Owner: Member 2 (Parcel Management & Pricing)
-- Dependencies: users (20261021000001), merchants (20261101000100)
-- Note: Foreign keys on origin_zone_id/destination_zone_id to delivery_zones(id) are deferred
--       until Member 7's delivery_zones migration is merged.

CREATE TABLE IF NOT EXISTS pricing_rules (
    id                  VARCHAR(64) NOT NULL CHECK (length(btrim(id)) > 0),
    version             INTEGER     NOT NULL DEFAULT 1 CHECK (version >= 1),
    origin_zone_id      VARCHAR(32) NOT NULL CHECK (origin_zone_id ~ '^ZONE_[A-Z0-9_]{2,27}$'),
    destination_zone_id VARCHAR(32) NOT NULL CHECK (destination_zone_id ~ '^ZONE_[A-Z0-9_]{2,27}$'),
    base_fee_cents      BIGINT      NOT NULL CHECK (base_fee_cents >= 0),
    per_kg_rate_cents   BIGINT      NOT NULL CHECK (per_kg_rate_cents >= 0),
    effective_from      TIMESTAMPTZ NOT NULL,
    effective_until     TIMESTAMPTZ NULL CHECK (effective_until IS NULL OR effective_until > effective_from),
    is_active           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP CHECK (updated_at >= created_at),

    PRIMARY KEY (id, version)
);

CREATE INDEX IF NOT EXISTS idx_pricing_rules_lookup
    ON pricing_rules (origin_zone_id, destination_zone_id, version DESC, effective_from DESC)
    WHERE is_active = TRUE;

CREATE TABLE IF NOT EXISTS pricing_quotes (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_id            VARCHAR(64)  NOT NULL,
    tariff_version       INTEGER      NOT NULL CHECK (tariff_version >= 1),
    requester_id         UUID         NULL REFERENCES users(id) ON DELETE RESTRICT,
    merchant_id          UUID         NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    chargeable_weight_kg NUMERIC(8,4) NOT NULL CHECK (chargeable_weight_kg > 0 AND chargeable_weight_kg <= 30.0000),
    service_level        VARCHAR(32)  NOT NULL CHECK (service_level IN ('STANDARD', 'EXPRESS', 'SAME_DAY')),
    fee_cents            BIGINT       NOT NULL CHECK (fee_cents >= 0),
    currency             VARCHAR(3)   NOT NULL DEFAULT 'LKR' CHECK (currency = 'LKR'),
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at           TIMESTAMPTZ  NOT NULL CHECK (expires_at > created_at),
    used_at              TIMESTAMPTZ  NULL CHECK (used_at IS NULL OR used_at >= created_at),

    CONSTRAINT chk_pricing_quotes_owner_present
        CHECK (requester_id IS NOT NULL OR merchant_id IS NOT NULL),
    CONSTRAINT fk_pricing_quotes_tariff
        FOREIGN KEY (tariff_id, tariff_version) REFERENCES pricing_rules(id, version) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_pricing_quotes_tariff
    ON pricing_quotes (tariff_id, tariff_version);

CREATE INDEX IF NOT EXISTS idx_pricing_quotes_requester
    ON pricing_quotes (requester_id, created_at DESC)
    WHERE requester_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pricing_quotes_merchant
    ON pricing_quotes (merchant_id, created_at DESC)
    WHERE merchant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pricing_quotes_unused_expiry
    ON pricing_quotes (id, expires_at)
    WHERE used_at IS NULL;

