CREATE TABLE zone_postal_codes (
    prefix     varchar(5) PRIMARY KEY CHECK (prefix ~ '^[0-9]{3,5}$'),
    zone_id    uuid NOT NULL REFERENCES delivery_zones(id) ON DELETE RESTRICT,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_zone_postal_codes_zone ON zone_postal_codes (zone_id);
CREATE INDEX idx_zone_postal_codes_prefix_pattern ON zone_postal_codes (prefix varchar_pattern_ops);
