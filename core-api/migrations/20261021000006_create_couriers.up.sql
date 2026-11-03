-- ============================================================================
-- SwiftDrop Core API: Migration 20261021000006 - Courier Operational Profiles
-- Author: Member 03 (Dispatch, Couriers & Proof of Delivery)
-- Canonical Sequence: 000006
-- Dependencies: 20261021000001_create_identity_tables, delivery_zones
-- Standards: AGENTS.md §7.2, database-schema.md
-- ============================================================================

-- Ensure delivery_zones table exists for foreign key reference
CREATE TABLE IF NOT EXISTS delivery_zones (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(40) UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    boundary_polygon JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Operational Courier Fleet Profiles Table
CREATE TABLE IF NOT EXISTS couriers (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    employee_code VARCHAR(20) NOT NULL UNIQUE,
    vehicle_type VARCHAR(20) NOT NULL,
    max_capacity INTEGER NOT NULL DEFAULT 25,
    status VARCHAR(20) NOT NULL DEFAULT 'OFF_SHIFT',
    current_zone_id VARCHAR(32) REFERENCES delivery_zones(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Invariant: Permitted vehicle classifications
    CONSTRAINT chk_couriers_vehicle_type CHECK (
        vehicle_type IN ('MOTORCYCLE', 'VAN', 'TRUCK')
    ),
    -- Invariant: Sensible capacity boundary (1 to 200 active parcels)
    CONSTRAINT chk_couriers_max_capacity CHECK (
        max_capacity BETWEEN 1 AND 200
    ),
    -- Invariant: Valid shift lifecycle statuses
    CONSTRAINT chk_couriers_status CHECK (
        status IN ('OFF_SHIFT', 'ON_SHIFT', 'ON_BREAK', 'SUSPENDED')
    )
);

COMMENT ON TABLE couriers IS 'Operational fleet profiles linked 1:1 with authenticated users';
COMMENT ON COLUMN couriers.user_id IS 'Foreign key to users(id), role must be COURIER';
COMMENT ON COLUMN couriers.employee_code IS 'Unique company employee code (e.g., CR-0012)';
COMMENT ON COLUMN couriers.vehicle_type IS 'Fleet vehicle classification: MOTORCYCLE, VAN, or TRUCK';
COMMENT ON COLUMN couriers.max_capacity IS 'Maximum concurrent active parcels allowed for this courier';
COMMENT ON COLUMN couriers.status IS 'Shift state: OFF_SHIFT, ON_SHIFT, ON_BREAK, SUSPENDED';
COMMENT ON COLUMN couriers.current_zone_id IS 'Assigned delivery zone referencing delivery_zones(id)';

-- Performance Indexes
-- 1. Fast lookup for dispatch board filtering by zone and active shift
CREATE INDEX IF NOT EXISTS idx_couriers_zone_status 
ON couriers (current_zone_id, status) 
WHERE is_active = TRUE;

-- 2. Fast lookup by employee code
CREATE INDEX IF NOT EXISTS idx_couriers_employee_code 
ON couriers (employee_code);

-- Guarded role grants (Agreement D6)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON couriers TO swiftdrop_app;
    END IF;
END $$;

