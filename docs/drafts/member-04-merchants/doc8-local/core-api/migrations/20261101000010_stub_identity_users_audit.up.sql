-- STAND-IN tables owned by Member 1 (users, roles) and Members 1/7 (audit_logs).
-- Exist only so the merchants slice runs alone. Replaced by the owners' real migrations.

CREATE TABLE IF NOT EXISTS users (
    id         uuid        PRIMARY KEY,
    email      text        NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_roles_stub (
    user_id    uuid        NOT NULL REFERENCES users (id),
    role       varchar(16) NOT NULL,
    tenant_id  uuid,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role)
);

-- Columns follow docs/architecture/database-schema.md audit_logs.
CREATE TABLE IF NOT EXISTS audit_logs (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id       uuid,
    actor_role     varchar(16),
    action         varchar(64) NOT NULL,
    target_type    varchar(16) NOT NULL,
    target_id      uuid,
    previous_state jsonb,
    new_state      jsonb,
    client_ip      inet,
    user_agent     varchar(200),
    request_id     varchar(64),
    created_at     timestamptz NOT NULL DEFAULT now()
);

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_worker') THEN
    CREATE ROLE swiftdrop_worker NOLOGIN;   -- Member 9 creates this in real environments
  END IF;
END $$;
