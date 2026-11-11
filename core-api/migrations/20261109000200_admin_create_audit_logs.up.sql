-- Migration: 20261109000200_admin_create_audit_logs.up.sql
-- Owners: Member 7 and Member 1. Created once here; every module writes rows to it
-- in the same transaction as the change it records.

CREATE TABLE IF NOT EXISTS audit_logs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id       VARCHAR(100),
    actor_role     VARCHAR(50) NOT NULL,
    action         VARCHAR(100) NOT NULL,
    target_type    VARCHAR(100),
    target_id      VARCHAR(128),
    previous_state JSONB,
    new_state      JSONB,
    reason         TEXT,
    client_ip      INET,
    user_agent     VARCHAR(300),
    request_id     VARCHAR(128),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Extend table if already created by 20261109000001_create_audit_logs.up.sql
ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS target_type VARCHAR(100),
    ADD COLUMN IF NOT EXISTS target_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS previous_state JSONB,
    ADD COLUMN IF NOT EXISTS new_state JSONB,
    ADD COLUMN IF NOT EXISTS reason TEXT,
    ADD COLUMN IF NOT EXISTS client_ip INET,
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(128);

-- Ensure actor_id, entity_type, entity_id are nullable if created by 20261109000001
DO $$
BEGIN
    ALTER TABLE audit_logs ALTER COLUMN actor_id DROP NOT NULL;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'audit_logs' AND column_name = 'entity_type') THEN
        ALTER TABLE audit_logs ALTER COLUMN entity_type DROP NOT NULL;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'audit_logs' AND column_name = 'entity_id') THEN
        ALTER TABLE audit_logs ALTER COLUMN entity_id DROP NOT NULL;
    END IF;
END $$;

-- Enforce role check constraint
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_audit_logs_actor_role') THEN
        ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_actor_role
            CHECK (actor_role IN ('ADMIN', 'DISPATCHER', 'COURIER', 'MERCHANT', 'CUSTOMER', 'SYSTEM'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor   ON audit_logs (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target  ON audit_logs (target_type, target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action  ON audit_logs (action, created_at DESC);

-- Append-only (SRS SEC-015): no role, including the owner, may change or remove a row.
CREATE OR REPLACE FUNCTION prevent_audit_log_modification() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only (SRS SEC-015)';
END $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_audit_logs_immutable ON audit_logs;
CREATE TRIGGER trg_audit_logs_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        REVOKE UPDATE, DELETE ON audit_logs FROM swiftdrop_app;
    END IF;
END $$;
