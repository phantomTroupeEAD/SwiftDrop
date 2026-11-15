-- Migration: 20261115000001_grant_app_delete_otps_and_zone_codes.up.sql
-- Owner: Member 9 (DB coordinator)
-- Description: The application role swiftdrop_app gets SELECT, INSERT, UPDATE on tables by default
-- (deploy/kubernetes/postgres/bootstrap-job.yaml), not DELETE. Two repositories delete rows by design:
--   - identity: email_otps (a used, expired or exhausted OTP is removed)
--   - admin:    zone_postal_codes (a zone's postal codes are replaced on update)
-- Grant DELETE on exactly these two tables. The role only exists where the bootstrap Job ran
-- (Kubernetes); local and CI databases connect as the owner and skip this.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        GRANT DELETE ON email_otps, zone_postal_codes TO swiftdrop_app;
    END IF;
END $$;
