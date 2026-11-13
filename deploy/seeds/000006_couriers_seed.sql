-- ============================================================================
-- SwiftDrop Seed Data: 000006_couriers_seed.sql
-- Author: Member 03 (Dispatch, Couriers & Proof of Delivery)
-- Dependencies: 20261021000002_seed_mock_users.up.sql, delivery_zones
-- Standards: Agreement D12 (Migrations DDL only; seeds separated in deploy/seeds)
-- ============================================================================

-- 1. Ensure primary delivery zone exists
INSERT INTO delivery_zones (id, name, code, enabled)
VALUES 
    ('ZONE_COLOMBO_01', 'Colombo Metro & Suburbs', 'ZONE_COLOMBO_01', TRUE),
    ('ZONE_KANDY_01', 'Kandy Central District', 'ZONE_KANDY_01', TRUE)
ON CONFLICT (id) DO NOTHING;

-- 2. Seed Operational Courier Profiles linked to Member 1 seeded users
-- Primary Courier from 20261021000002_seed_mock_users: '018f4a7c-1003-7000-8000-000000000003' (Kamal Perera)
INSERT INTO couriers (user_id, employee_code, vehicle_type, max_capacity, status, current_zone_id, is_active)
VALUES 
    ('018f4a7c-1003-7000-8000-000000000003', 'CR-0012', 'MOTORCYCLE', 25, 'ON_SHIFT', 'ZONE_COLOMBO_01', TRUE)
ON CONFLICT (user_id) DO UPDATE SET
    employee_code = EXCLUDED.employee_code,
    vehicle_type = EXCLUDED.vehicle_type,
    max_capacity = EXCLUDED.max_capacity,
    status = EXCLUDED.status,
    current_zone_id = EXCLUDED.current_zone_id,
    is_active = EXCLUDED.is_active;

