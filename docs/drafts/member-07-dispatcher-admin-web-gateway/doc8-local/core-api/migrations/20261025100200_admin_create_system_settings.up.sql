CREATE TABLE system_settings (
    key         varchar(100) PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]{2,99}$'),
    value       jsonb        NOT NULL,
    value_type  varchar(10)  NOT NULL CHECK (value_type IN ('BOOLEAN','INTEGER','STRING')),
    description text         NOT NULL,
    version     integer      NOT NULL DEFAULT 1,
    updated_by  uuid REFERENCES users(id),
    updated_at  timestamptz  NOT NULL DEFAULT now()
);
