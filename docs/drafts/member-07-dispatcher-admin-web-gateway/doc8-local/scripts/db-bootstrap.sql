-- Run ONCE per database by the database administrator (Member 9), before migrations.
-- Two roles: the owner runs migrations; the app role is what core-api uses at run time.
-- Local demo passwords only. Real ones come from Kubernetes Secrets.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_owner') THEN
        CREATE ROLE swiftdrop_owner LOGIN PASSWORD 'owner_dev_only';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        CREATE ROLE swiftdrop_app LOGIN PASSWORD 'app_dev_only';
    END IF;
END $$;
GRANT CONNECT ON DATABASE swiftdrop_db TO swiftdrop_app;
GRANT ALL ON SCHEMA public TO swiftdrop_owner;
GRANT USAGE ON SCHEMA public TO swiftdrop_app;
-- Every table the owner creates later is usable by the app role...
ALTER DEFAULT PRIVILEGES FOR ROLE swiftdrop_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO swiftdrop_app;
-- ...except that the audit migration takes UPDATE and DELETE back on audit_logs.
-- The owner role owns the database and schema, so migrations can create tables.
ALTER DATABASE swiftdrop_db OWNER TO swiftdrop_owner;
ALTER SCHEMA public OWNER TO swiftdrop_owner;
