-- Migration: 20261112000100_admin_add_parcel_settings.up.sql
-- Owner: Member 7 (internal/admin). Adds two allowlisted settings and a DECIMAL
-- value type (kilograms are not whole numbers). parcels.max_cod_cents is LKR minor units.

ALTER TABLE system_settings DROP CONSTRAINT IF EXISTS system_settings_value_type_check;
ALTER TABLE system_settings ADD CONSTRAINT system_settings_value_type_check
    CHECK (value_type IN ('BOOLEAN', 'INTEGER', 'DECIMAL', 'STRING', 'JSON'));

INSERT INTO system_settings (key, value, value_type, description) VALUES
    ('parcels.max_weight_kg', '25', 'DECIMAL', 'Heaviest parcel accepted, in kilograms (0.5 to 30)'),
    ('parcels.max_cod_cents', '3500000', 'INTEGER', 'Highest cash-on-delivery amount, in LKR cents (0 to 10000000)')
ON CONFLICT (key) DO NOTHING;
