-- Migration: 20261110000002_create_parcel_tables.up.sql
-- Owner: Member 2 (Parcel Management & Pricing)
-- Dependencies: users (20261021000001), merchants (20261101000100), pricing_rules & pricing_quotes (20261110000001)
-- Note: Foreign keys on origin_zone_id/destination_zone_id to delivery_zones(id) are deferred
--       until Member 7's delivery_zones migration is merged.

CREATE TABLE IF NOT EXISTS parcels (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tracking_number      VARCHAR(32)  NOT NULL UNIQUE
        CHECK (tracking_number ~ '^SD-[0-9]{8}-[0-9A-HJ-KM-NP-TV-Z]{6}$'),
    sender_id            UUID         NULL REFERENCES users(id) ON DELETE RESTRICT,
    merchant_id          UUID         NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    recipient_name       VARCHAR(150) NOT NULL CHECK (length(btrim(recipient_name)) > 0),
    recipient_phone      VARCHAR(32)  NOT NULL CHECK (length(btrim(recipient_phone)) > 0),
    quote_id             UUID         NOT NULL UNIQUE REFERENCES pricing_quotes(id) ON DELETE RESTRICT,
    pricing_rule_id      VARCHAR(64)  NOT NULL,
    pricing_rule_version INTEGER      NOT NULL CHECK (pricing_rule_version >= 1),
    delivery_fee_cents   BIGINT       NOT NULL CHECK (delivery_fee_cents >= 0),
    currency             VARCHAR(3)   NOT NULL DEFAULT 'LKR' CHECK (currency = 'LKR'),
    origin_zone_id       VARCHAR(32)  NOT NULL CHECK (origin_zone_id ~ '^ZONE_[A-Z0-9_]{2,27}$'),
    destination_zone_id  VARCHAR(32)  NOT NULL CHECK (destination_zone_id ~ '^ZONE_[A-Z0-9_]{2,27}$'),
    service_level        VARCHAR(32)  NOT NULL CHECK (service_level IN ('STANDARD', 'EXPRESS', 'SAME_DAY')),
    actual_weight_kg     NUMERIC(8,4) NOT NULL CHECK (actual_weight_kg > 0 AND actual_weight_kg <= 30.0000),
    volumetric_weight_kg NUMERIC(8,4) NOT NULL CHECK (volumetric_weight_kg > 0 AND volumetric_weight_kg <= 30.0000),
    chargeable_weight_kg NUMERIC(8,4) NOT NULL CHECK (
        chargeable_weight_kg > 0
        AND chargeable_weight_kg <= 30.0000
        AND chargeable_weight_kg >= actual_weight_kg
        AND chargeable_weight_kg >= volumetric_weight_kg
    ),
    length_cm            NUMERIC(8,2) NOT NULL CHECK (length_cm > 0),
    width_cm             NUMERIC(8,2) NOT NULL CHECK (width_cm > 0),
    height_cm            NUMERIC(8,2) NOT NULL CHECK (height_cm > 0),
    pickup_address       JSONB        NOT NULL CHECK (
        jsonb_typeof(pickup_address) = 'object'
        AND pickup_address ? 'contact_name'
        AND pickup_address ? 'contact_phone'
        AND pickup_address ? 'address_line1'
        AND pickup_address ? 'city'
        AND pickup_address ? 'postal_code'
        AND pickup_address ? 'zone_id'
    ),
    dropoff_address      JSONB        NOT NULL CHECK (
        jsonb_typeof(dropoff_address) = 'object'
        AND dropoff_address ? 'contact_name'
        AND dropoff_address ? 'contact_phone'
        AND dropoff_address ? 'address_line1'
        AND dropoff_address ? 'city'
        AND dropoff_address ? 'postal_code'
        AND dropoff_address ? 'zone_id'
    ),
    status               VARCHAR(32)  NOT NULL DEFAULT 'CREATED' CHECK (status IN (
        'CREATED',
        'ASSIGNED',
        'PICKED_UP',
        'IN_TRANSIT',
        'OUT_FOR_DELIVERY',
        'DELIVERED',
        'ATTEMPT_FAILED',
        'RETURNED',
        'CANCELLED'
    )),
    attempt_count        INTEGER      NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 3),
    version              INTEGER      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP CHECK (updated_at >= created_at),

    CONSTRAINT chk_parcels_owner_present
        CHECK (sender_id IS NOT NULL OR merchant_id IS NOT NULL),
    CONSTRAINT fk_parcels_pricing_rule
        FOREIGN KEY (pricing_rule_id, pricing_rule_version) REFERENCES pricing_rules(id, version) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_parcels_sender_created
    ON parcels (sender_id, created_at DESC)
    WHERE sender_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_parcels_merchant_created
    ON parcels (merchant_id, created_at DESC)
    WHERE merchant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_parcels_status_created
    ON parcels (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_parcels_zones_created
    ON parcels (origin_zone_id, destination_zone_id, created_at DESC);

CREATE TABLE IF NOT EXISTS parcel_status_history (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id   UUID        NOT NULL REFERENCES parcels(id) ON DELETE RESTRICT,
    from_status VARCHAR(32) NULL CHECK (from_status IS NULL OR from_status IN (
        'CREATED',
        'ASSIGNED',
        'PICKED_UP',
        'IN_TRANSIT',
        'OUT_FOR_DELIVERY',
        'DELIVERED',
        'ATTEMPT_FAILED',
        'RETURNED',
        'CANCELLED'
    )),
    to_status   VARCHAR(32) NOT NULL CHECK (to_status IN (
        'CREATED',
        'ASSIGNED',
        'PICKED_UP',
        'IN_TRANSIT',
        'OUT_FOR_DELIVERY',
        'DELIVERED',
        'ATTEMPT_FAILED',
        'RETURNED',
        'CANCELLED'
    )),
    actor_id    VARCHAR(64) NULL,
    actor_type  VARCHAR(20) NOT NULL CHECK (actor_type IN ('USER', 'SYSTEM', 'MERCHANT_API')),
    actor_role  VARCHAR(32) NOT NULL,
    reason      TEXT        NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_parcel_status_history_initial
        CHECK (from_status IS NOT NULL OR to_status = 'CREATED'),
    CONSTRAINT chk_parcel_status_history_actor_consistency
        CHECK (
            (actor_type = 'SYSTEM' AND actor_id IS NULL)
            OR (
                actor_type IN ('USER', 'MERCHANT_API')
                AND actor_id IS NOT NULL
                AND length(btrim(actor_id)) > 0
                AND upper(actor_id) <> 'SYSTEM'
            )
        )
);

CREATE INDEX IF NOT EXISTS idx_parcel_status_history_parcel_occurred
    ON parcel_status_history (parcel_id, occurred_at ASC, id ASC);

CREATE OR REPLACE FUNCTION prevent_parcel_status_history_modification() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'parcel_status_history is strictly append-only';
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_parcel_status_history_immutable ON parcel_status_history;
CREATE TRIGGER trg_parcel_status_history_immutable
    BEFORE UPDATE OR DELETE ON parcel_status_history
    FOR EACH ROW EXECUTE FUNCTION prevent_parcel_status_history_modification();

