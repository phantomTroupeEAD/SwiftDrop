-- 000002_create_delivery_zones.up.sql   (owner: Member 7, Doc 5 section 6.4)
-- Needs 000001 (users, Member 1). Must run before 000004 (pricing) and 000006 (couriers).
CREATE TABLE delivery_zones (
    id               varchar(32)  PRIMARY KEY CHECK (id ~ '^ZONE_[A-Z0-9_]{2,27}$'),
    code             varchar(32)  NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9_-]{2,32}$'),
    name             varchar(120) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 120),
    description      text,
    enabled          boolean NOT NULL DEFAULT true,
    boundary_polygon jsonb,
    version          integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_delivery_zones_enabled ON delivery_zones (enabled);

CREATE TABLE zone_postal_codes (
    prefix     varchar(10) PRIMARY KEY CHECK (prefix ~ '^[0-9]{3,5}$'),
    zone_id    varchar(32) NOT NULL REFERENCES delivery_zones(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_zone_postal_codes_zone ON zone_postal_codes (zone_id);
CREATE INDEX idx_zone_postal_codes_prefix_pattern ON zone_postal_codes (prefix varchar_pattern_ops);

CREATE TABLE system_settings (
    key         varchar(64) PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]{2,63}$'),
    value       jsonb NOT NULL,
    value_type  varchar(16) NOT NULL CHECK (value_type IN ('BOOLEAN','INTEGER','STRING','JSON')),
    description text,
    version     integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    updated_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);
-- The only DELETE my module needs (Member 9 D7): a zone edit removes prefixes.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        GRANT DELETE ON zone_postal_codes TO swiftdrop_app;
    END IF;
END $$;
