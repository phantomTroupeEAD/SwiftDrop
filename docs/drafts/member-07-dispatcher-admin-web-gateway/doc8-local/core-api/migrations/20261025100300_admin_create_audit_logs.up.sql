CREATE TABLE audit_logs (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id       uuid REFERENCES users(id) ON DELETE RESTRICT,
    actor_role     varchar(20)  NOT NULL CHECK (actor_role IN ('ADMIN','DISPATCHER','COURIER','MERCHANT','CUSTOMER','SYSTEM')),
    action         varchar(60)  NOT NULL CHECK (action ~ '^[A-Z_]+$'),
    target_type    varchar(30)  NOT NULL,
    target_id      varchar(100) NOT NULL,
    previous_state jsonb,
    new_state      jsonb,
    reason         text,
    client_ip      inet,
    user_agent     varchar(300),
    request_id     varchar(100),
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_created ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs (actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_target ON audit_logs (target_type, target_id, created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs (action, created_at DESC);
