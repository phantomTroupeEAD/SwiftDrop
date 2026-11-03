-- ============================================================================
-- SwiftDrop Core API: Migration 20261021000007 - Dispatch Assignments & Delivery Attempts
-- Author: Member 03 (Dispatch, Couriers & Proof of Delivery)
-- Canonical Sequence: 000007
-- Dependencies: 20261021000001_create_identity_tables, 20261021000006_create_couriers, parcels
-- Standards: AGENTS.md §7.2, ADR-004, database-schema.md
-- ============================================================================

-- Ensure parcels table exists for foreign key reference
CREATE TABLE IF NOT EXISTS parcels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tracking_number VARCHAR(32) NOT NULL UNIQUE,
    current_status VARCHAR(32) NOT NULL DEFAULT 'CREATED',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ----------------------------------------------------------------------------
-- 1. Table: courier_assignments
-- Description: Historical audit and active assignments of parcels to couriers
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS courier_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id UUID NOT NULL REFERENCES parcels(id) ON DELETE RESTRICT,
    courier_id UUID NOT NULL REFERENCES couriers(user_id) ON DELETE RESTRICT,
    assigned_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ NULL,
    notes TEXT NULL,

    -- Invariant: Valid assignment lifecycle states
    CONSTRAINT chk_assignments_status CHECK (
        status IN ('ACTIVE', 'COMPLETED', 'REASSIGNED', 'CANCELLED')
    ),
    -- Invariant: Active assignments have no ended_at; terminal assignments MUST have ended_at
    CONSTRAINT chk_assignments_ended_at CHECK (
        (status = 'ACTIVE' AND ended_at IS NULL) OR
        (status IN ('COMPLETED', 'REASSIGNED', 'CANCELLED') AND ended_at IS NOT NULL)
    )
);

COMMENT ON TABLE courier_assignments IS 'Historical audit and active assignment links between parcels and couriers';
COMMENT ON COLUMN courier_assignments.status IS 'Lifecycle: ACTIVE, COMPLETED, REASSIGNED, CANCELLED';
COMMENT ON COLUMN courier_assignments.assigned_by IS 'Dispatcher user ID who authorized the assignment';

-- Indexes for courier_assignments
-- 1. Critical Concurrency Guard (AT-013): Exactly ONE active assignment per parcel
CREATE UNIQUE INDEX IF NOT EXISTS idx_assignments_parcel_single_active 
ON courier_assignments (parcel_id) 
WHERE status = 'ACTIVE';

-- 2. Fast courier active workload aggregation (active_workload <= max_capacity check)
CREATE INDEX IF NOT EXISTS idx_assignments_courier_active 
ON courier_assignments (courier_id) 
WHERE status = 'ACTIVE';

-- 3. Chronological lookup of courier assignment history
CREATE INDEX IF NOT EXISTS idx_assignments_courier_history 
ON courier_assignments (courier_id, assigned_at DESC);


-- ----------------------------------------------------------------------------
-- 2. Table: delivery_attempts
-- Description: Immutable doorstep attempt logs, failure reasons & POD references
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id UUID NOT NULL REFERENCES parcels(id) ON DELETE RESTRICT,
    courier_id UUID NOT NULL REFERENCES couriers(user_id) ON DELETE RESTRICT,
    attempt_number INTEGER NOT NULL,
    outcome VARCHAR(20) NOT NULL,
    failure_reason_code VARCHAR(50) NULL,
    proof_type VARCHAR(30) NULL,
    proof_attachment_url TEXT NULL,
    proof_signature_data TEXT NULL,
    received_by_name VARCHAR(150) NULL,
    latitude DECIMAL(10, 8) NULL,
    longitude DECIMAL(11, 8) NULL,
    notes TEXT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Invariant: ADR-004 maximum 3 delivery attempts ceiling
    CONSTRAINT chk_delivery_attempts_number CHECK (
        attempt_number BETWEEN 1 AND 3
    ),
    -- Invariant: Outcome must be SUCCESS or FAILED
    CONSTRAINT chk_delivery_attempts_outcome CHECK (
        outcome IN ('SUCCESS', 'FAILED')
    ),
    -- Invariant: ADR-004 approved standard reason codes
    CONSTRAINT chk_delivery_attempts_reason CHECK (
        failure_reason_code IS NULL OR failure_reason_code IN (
            'RECIPIENT_UNAVAILABLE',
            'CUSTOMER_REQUESTED_RESCHEDULE',
            'ACCESS_RESTRICTED',
            'INVALID_ADDRESS',
            'REFUSED_BY_RECIPIENT',
            'DAMAGED_IN_TRANSIT',
            'OTHER'
        )
    ),
    -- Invariant: Supported proof-of-delivery types
    CONSTRAINT chk_delivery_attempts_proof_type CHECK (
        proof_type IS NULL OR proof_type IN ('PHOTO', 'SIGNATURE', 'RECIPIENT_CONFIRMATION')
    ),
    -- Invariant: Outcome consistency (SUCCESS requires proof; FAILED requires reason code)
    CONSTRAINT chk_delivery_attempts_outcome_consistency CHECK (
        (outcome = 'SUCCESS' AND proof_type IS NOT NULL) OR
        (outcome = 'FAILED' AND failure_reason_code IS NOT NULL)
    ),
    -- Invariant: Signature proof MUST capture recipient name (length >= 2)
    CONSTRAINT chk_delivery_attempts_signature_recipient CHECK (
        proof_type != 'SIGNATURE' OR (received_by_name IS NOT NULL AND length(trim(received_by_name)) >= 2)
    )
);

COMMENT ON TABLE delivery_attempts IS 'Immutable doorstep delivery attempts, proof references, and ADR-004 failure logs';
COMMENT ON COLUMN delivery_attempts.attempt_number IS 'Attempt sequence (1, 2, or 3)';
COMMENT ON COLUMN delivery_attempts.outcome IS 'Outcome: SUCCESS or FAILED';
COMMENT ON COLUMN delivery_attempts.proof_attachment_url IS 'Private storage path in Cloudflare R2 / S3';
COMMENT ON COLUMN delivery_attempts.proof_signature_data IS 'SVG or Base64 vector stroke data';

-- Indexes for delivery_attempts
-- 1. Sequential attempt history retrieval per parcel
CREATE INDEX IF NOT EXISTS idx_delivery_attempts_parcel_seq 
ON delivery_attempts (parcel_id, attempt_number ASC);

-- 2. Courier performance audit index
CREATE INDEX IF NOT EXISTS idx_delivery_attempts_courier_audit 
ON delivery_attempts (courier_id, attempted_at DESC);

-- Guarded role grants (Agreement D6)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON courier_assignments TO swiftdrop_app;
        GRANT SELECT, INSERT, UPDATE, DELETE ON delivery_attempts TO swiftdrop_app;
    END IF;
END $$;

