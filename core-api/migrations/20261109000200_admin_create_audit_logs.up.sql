-- Migration: 20261109000200_admin_create_audit_logs.up.sql
-- Owners: Member 7 and Member 1. Created once here; every module writes rows to it
-- in the same transaction as the change it records.

CREATE TABLE audit_logs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_role     VARCHAR(20) NOT NULL CHECK (actor_role IN ('ADMIN', 'DISPATCHER', 'COURIER', 'MERCHANT', 'CUSTOMER', 'SYSTEM')),
    action         VARCHAR(64) NOT NULL CHECK (action ~ '^[A-Z_]+$'),
    target_type    VARCHAR(30) NOT NULL,
    target_id      VARCHAR(128) NOT NULL,
    previous_state JSONB,
    new_state      JSONB,
    reason         TEXT,
    client_ip      INET,
    user_agent     VARCHAR(300),
    request_id     VARCHAR(128),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_created ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_logs_actor   ON audit_logs (actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_target  ON audit_logs (target_type, target_id, created_at DESC);
CREATE INDEX idx_audit_logs_action  ON audit_logs (action, created_at DESC);

-- Append-only (SRS SEC-015): no role, including the owner, may change or remove a row.
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
