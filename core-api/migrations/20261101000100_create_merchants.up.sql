-- Member 4 — Merchant Integration
-- DRAFT migration (golang-migrate). Final version number assigned at graduation (U-28).
-- Depends on: users (Member 1).

CREATE TABLE merchants (
    id              uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id   uuid         NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    business_name   varchar(150) NOT NULL,
    status          varchar(16)  NOT NULL DEFAULT 'PENDING',
    status_reason   varchar(500),
    contact_email   varchar(254) NOT NULL,
    contact_phone   varchar(16)  NOT NULL,
    address         jsonb        NOT NULL,
    approved_by     uuid         REFERENCES users (id) ON DELETE RESTRICT,
    approved_at     timestamptz,
    version         integer      NOT NULL DEFAULT 1,
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT ck_merchants_status
        CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'SUSPENDED', 'DISABLED')),
    CONSTRAINT ck_merchants_business_name
        CHECK (char_length(btrim(business_name)) BETWEEN 2 AND 150),
    CONSTRAINT ck_merchants_contact_phone
        CHECK (contact_phone ~ '^\+[1-9][0-9]{6,14}$'),
    CONSTRAINT ck_merchants_contact_email
        CHECK (position('@' IN contact_email) > 1),
    CONSTRAINT ck_merchants_address_shape
        CHECK (jsonb_typeof(address) = 'object'
               AND address ? 'line1' AND address ? 'city' AND address ? 'country'),
    CONSTRAINT ck_merchants_approval_fields
        CHECK (status NOT IN ('APPROVED', 'SUSPENDED', 'DISABLED')
               OR (approved_at IS NOT NULL AND approved_by IS NOT NULL)),
    CONSTRAINT ck_merchants_version
        CHECK (version >= 1)
);

-- BR-M-05: at most one active merchant per user.
CREATE UNIQUE INDEX uq_merchants_active_owner
    ON merchants (owner_user_id)
    WHERE status IN ('PENDING', 'APPROVED', 'SUSPENDED');

-- Owner -> merchant lookup (dashboard "my merchant", application history, U-15).
-- JWT callers carry tenant_id (ADR-003) and are status-checked by PK, not via this index.
CREATE INDEX idx_merchants_owner_created
    ON merchants (owner_user_id, created_at DESC);

-- Admin list filtered by status, newest first.
CREATE INDEX idx_merchants_status_created
    ON merchants (status, created_at DESC);

COMMENT ON TABLE merchants IS 'Owner: Member 4 (core-api/internal/merchants). Merchant aggregate.';
