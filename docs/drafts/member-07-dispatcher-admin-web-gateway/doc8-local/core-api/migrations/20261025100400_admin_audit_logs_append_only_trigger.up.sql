CREATE FUNCTION audit_logs_block_change() RETURNS trigger AS $$
BEGIN
    IF current_setting('swiftdrop.retention_job', true) = 'on' AND TG_OP = 'DELETE' THEN
        RETURN OLD;   -- only the retention job may delete, and only when it sets this flag
    END IF;
    RAISE EXCEPTION 'audit_logs is append-only';
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_audit_logs_append_only
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_block_change();
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        REVOKE UPDATE, DELETE ON audit_logs FROM swiftdrop_app;
    END IF;
END $$;
