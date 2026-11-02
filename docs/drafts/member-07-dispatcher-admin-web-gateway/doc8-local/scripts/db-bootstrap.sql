-- Run ONCE per database by the database administrator (Member 9), before migrations.
-- Role names and rights are Member 9's agreements D6 and D7:
--   swiftdrop_migrator owns the schema and runs migrations (DDL);
--   swiftdrop_app is what core-api uses at run time (no DDL, DELETE only where granted).
-- Local demo passwords only. Real ones come from the Kubernetes Secrets
-- postgres-secrets (app) and postgres-migrator-secrets (migrator).
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_migrator') THEN
        CREATE ROLE swiftdrop_migrator LOGIN PASSWORD 'migrator_dev_only';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        CREATE ROLE swiftdrop_app LOGIN PASSWORD 'app_dev_only';
    END IF;
END $$;
GRANT CONNECT ON DATABASE swiftdrop_db TO swiftdrop_app;
GRANT ALL ON SCHEMA public TO swiftdrop_migrator;
GRANT USAGE ON SCHEMA public TO swiftdrop_app;
-- D7: every table the migrator creates later gets SELECT, INSERT, UPDATE for the app.
-- DELETE is granted per table inside the migration that needs it (000002 for prefixes).
-- 000011 takes UPDATE back on audit_logs.
ALTER DEFAULT PRIVILEGES FOR ROLE swiftdrop_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE ON TABLES TO swiftdrop_app;
ALTER DATABASE swiftdrop_db OWNER TO swiftdrop_migrator;
ALTER SCHEMA public OWNER TO swiftdrop_migrator;
