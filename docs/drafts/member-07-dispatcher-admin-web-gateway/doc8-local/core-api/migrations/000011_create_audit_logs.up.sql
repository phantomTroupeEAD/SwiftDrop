-- 000011_create_audit_logs.up.sql   (owners: Member 7 and Member 1, Doc 5 section 6.4)
-- Created ONCE here (Member 9 D4); every module writes rows with the shared AuditWriter.
CREATE TABLE audit_logs (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id       uuid REFERENCES users(id) ON DELETE SET NULL,
    actor_role     varchar(20) NOT NULL CHECK (actor_role IN ('ADMIN','DISPATCHER','COURIER','MERCHANT','CUSTOMER','SYSTEM')),
    action         varchar(64) NOT NULL CHECK (action ~ '^[A-Z_]+$'),
    target_type    varchar(30) NOT NULL,
    target_id      varchar(128) NOT NULL,          -- U-64 (schema says uuid)
    previous_state jsonb,
    new_state      jsonb,
    reason         text,                           -- U-64 (not in schema yet)
    client_ip      inet,
    user_agent     varchar(300),
    request_id     varchar(100),
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_created ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_logs_actor   ON audit_logs (actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_target  ON audit_logs (target_type, target_id, created_at DESC);
CREATE INDEX idx_audit_logs_action  ON audit_logs (action, created_at DESC);

CREATE FUNCTION prevent_audit_log_modification() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only (SRS SEC-015)';
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_audit_logs_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        REVOKE UPDATE, DELETE ON audit_logs FROM swiftdrop_app;
    END IF;
END $$;
