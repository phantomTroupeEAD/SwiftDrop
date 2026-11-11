-- Migration: 20261109000100_admin_create_delivery_zones.up.sql
-- Owner: Member 7 (internal/admin). Needs users (20261021000001, Member 1).
-- Zones hold no money: fees, including any per-zone charge, are Member 2's pricing rules.

CREATE TABLE delivery_zones (
    id               VARCHAR(32)  PRIMARY KEY CHECK (id ~ '^ZONE_[A-Z0-9_]{2,27}$'),
    code             VARCHAR(32)  NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9_-]{2,32}$'),
    name             VARCHAR(120) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 120),
    description      TEXT,
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    boundary_polygon JSONB,
    version          INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_delivery_zones_enabled ON delivery_zones (enabled);

-- A prefix belongs to one zone only; overlapping prefixes ("001" and "00100") are
-- refused by the admin use case under an advisory lock.
CREATE TABLE zone_postal_codes (
    prefix     VARCHAR(10) PRIMARY KEY CHECK (prefix ~ '^[0-9]{3,5}$'),
    zone_id    VARCHAR(32) NOT NULL REFERENCES delivery_zones(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_zone_postal_codes_zone ON zone_postal_codes (zone_id);
CREATE INDEX idx_zone_postal_codes_prefix_pattern ON zone_postal_codes (prefix varchar_pattern_ops);

CREATE TABLE system_settings (
    key         VARCHAR(64) PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]{2,63}$'),
    value       JSONB NOT NULL,
    value_type  VARCHAR(16) NOT NULL CHECK (value_type IN ('BOOLEAN', 'INTEGER', 'STRING', 'JSON')),
    description TEXT,
    version     INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    updated_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Defaults for the keys the admin API allows (application/settings_rules.go).
-- The API only updates existing keys, so these rows must exist.
INSERT INTO system_settings (key, value, value_type, description) VALUES
    ('reports.max_range_days', '92', 'INTEGER', 'Longest report period in days'),
    ('ops.board_refresh_seconds', '30', 'INTEGER', 'Dispatch board polling interval'),
    ('ops.maintenance_banner', '""', 'STRING', 'Optional message shown on ops screens')
ON CONFLICT (key) DO NOTHING;

-- The only DELETE the admin module needs: a zone edit replaces its prefixes.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        GRANT DELETE ON zone_postal_codes TO swiftdrop_app;
    END IF;
END $$;
