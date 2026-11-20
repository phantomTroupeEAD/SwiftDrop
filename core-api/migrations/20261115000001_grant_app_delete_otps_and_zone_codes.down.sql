-- Migration: 20261115000001_grant_app_delete_otps_and_zone_codes.down.sql
-- Description: Remove the DELETE grants added by the up migration.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        REVOKE DELETE ON email_otps, zone_postal_codes FROM swiftdrop_app;
    END IF;
END $$;
