-- Migration: 20261112000100_admin_add_parcel_settings.down.sql
DELETE FROM system_settings WHERE key IN ('parcels.max_weight_kg', 'parcels.max_cod_cents');

ALTER TABLE system_settings DROP CONSTRAINT IF EXISTS system_settings_value_type_check;
ALTER TABLE system_settings ADD CONSTRAINT system_settings_value_type_check
    CHECK (value_type IN ('BOOLEAN', 'INTEGER', 'STRING', 'JSON'));
