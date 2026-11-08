-- SwiftDrop Database Role Bootstrap (Agreement D6 & D7)
-- Authority: docs/drafts/member-09-devops-k8s/MEMBER_09_ALIGNMENT_AND_SPECIFICATION_GUIDE.md §5.1
-- Run once by postgres superuser on swiftdrop_db before initial migration.

DO $$
BEGIN
    -- 1. Database Schema Migrator (Owns DDL and schema migrations)
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'swiftdrop_migrator') THEN
        CREATE ROLE swiftdrop_migrator LOGIN PASSWORD 'swiftdrop_migrator_dev';
    END IF;

    -- 2. Core API Application Monolith
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        CREATE ROLE swiftdrop_app LOGIN PASSWORD 'swiftdrop_app_dev';
    END IF;

    -- 3. Tracking API (Read-Only Projection Service)
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'swiftdrop_tracking_ro') THEN
        CREATE ROLE swiftdrop_tracking_ro LOGIN PASSWORD 'swiftdrop_tracking_dev';
    END IF;

    -- 4. Transactional Outbox Relay Daemon
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'swiftdrop_relay') THEN
        CREATE ROLE swiftdrop_relay LOGIN PASSWORD 'swiftdrop_relay_dev';
    END IF;

    -- 5. Asynchronous Event Worker
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'swiftdrop_worker') THEN
        CREATE ROLE swiftdrop_worker LOGIN PASSWORD 'swiftdrop_worker_dev';
    END IF;

    -- 6. Outbox Reconciliation CronJob
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'swiftdrop_reconciler') THEN
        CREATE ROLE swiftdrop_reconciler LOGIN PASSWORD 'swiftdrop_reconciler_dev';
    END IF;
END $$;

-- Transfer public schema ownership to swiftdrop_migrator
ALTER SCHEMA public OWNER TO swiftdrop_migrator;

-- Grant schema usage to application roles
GRANT USAGE ON SCHEMA public
  TO swiftdrop_app, swiftdrop_tracking_ro, swiftdrop_relay, swiftdrop_worker, swiftdrop_reconciler;

-- Set default table & sequence privileges for swiftdrop_app on future migrator-created tables
ALTER DEFAULT PRIVILEGES FOR ROLE swiftdrop_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE ON TABLES TO swiftdrop_app;

ALTER DEFAULT PRIVILEGES FOR ROLE swiftdrop_migrator IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO swiftdrop_app;

-- Per-role execution timeouts & read-only enforcement (Agreement D9)
ALTER ROLE swiftdrop_app         SET statement_timeout = '5s';
ALTER ROLE swiftdrop_app         SET lock_timeout = '2s';
ALTER ROLE swiftdrop_tracking_ro SET statement_timeout = '3s';
ALTER ROLE swiftdrop_tracking_ro SET default_transaction_read_only = on;

