-- Migration: 20261118000001_seed_canonical_zones_and_pricing.up.sql
-- Description: Seed canonical Sri Lankan delivery zones, postal code prefixes, active pricing rules, and secondary actors for E2E/live environments.

-- 1. Seed canonical delivery zones
INSERT INTO delivery_zones (id, code, name, description, enabled, version, created_at, updated_at) VALUES
    ('ZONE_COLOMBO_01', 'CMB-01', 'Colombo Central', 'Central business and commercial district', TRUE, 1, NOW(), NOW()),
    ('ZONE_COLOMBO_02', 'CMB-02', 'Colombo South', 'Southern residential and commercial district', TRUE, 1, NOW(), NOW()),
    ('ZONE_COLOMBO_03', 'CMB-03', 'Colombo East', 'Eastern district and administrative suburbs', TRUE, 1, NOW(), NOW()),
    ('ZONE_NEGOMBO_01', 'NEG-01', 'Negombo', 'Negombo coastal and airport trade hub', TRUE, 1, NOW(), NOW()),
    ('ZONE_KANDY_01', 'KDY-01', 'Kandy', 'Central highlands and Kandy metropolitan area', TRUE, 1, NOW(), NOW()),
    ('ZONE_GALLE_01', 'GAL-01', 'Galle', 'Southern province commercial and coastal hub', TRUE, 1, NOW(), NOW()),
    ('ZONE_DEMO_DISABLED', 'DEMO-OFF', 'Demo Disabled Zone', 'Disabled zone reserved for operational error testing', FALSE, 1, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    enabled = EXCLUDED.enabled,
    updated_at = NOW();

-- 2. Seed postal code prefixes for canonical delivery zones
INSERT INTO zone_postal_codes (prefix, zone_id, created_at) VALUES
    ('00100', 'ZONE_COLOMBO_01', NOW()),
    ('00200', 'ZONE_COLOMBO_01', NOW()),
    ('00300', 'ZONE_COLOMBO_01', NOW()),
    ('00400', 'ZONE_COLOMBO_02', NOW()),
    ('00500', 'ZONE_COLOMBO_02', NOW()),
    ('00600', 'ZONE_COLOMBO_02', NOW()),
    ('00700', 'ZONE_COLOMBO_03', NOW()),
    ('00800', 'ZONE_COLOMBO_03', NOW()),
    ('11500', 'ZONE_NEGOMBO_01', NOW()),
    ('20000', 'ZONE_KANDY_01', NOW()),
    ('80000', 'ZONE_GALLE_01', NOW()),
    ('99999', 'ZONE_DEMO_DISABLED', NOW())
ON CONFLICT (prefix) DO UPDATE SET
    zone_id = EXCLUDED.zone_id;

-- 3. Seed active pricing rules across all enabled canonical delivery zones
INSERT INTO pricing_rules (
    id,
    version,
    origin_zone_id,
    destination_zone_id,
    base_fee_cents,
    per_kg_rate_cents,
    effective_from,
    effective_until,
    is_active,
    created_at,
    updated_at
)
SELECT
    'rule_' || lower(oz.id) || '_' || lower(dz.id),
    1,
    oz.id,
    dz.id,
    CASE WHEN oz.id = dz.id THEN 35000 ELSE 45000 END,
    CASE WHEN oz.id = dz.id THEN 5000 ELSE 7500 END,
    NOW() - INTERVAL '1 year',
    NULL,
    TRUE,
    NOW(),
    NOW()
FROM delivery_zones oz
CROSS JOIN delivery_zones dz
WHERE oz.enabled = TRUE AND dz.enabled = TRUE
ON CONFLICT (id, version) DO UPDATE SET
    base_fee_cents = EXCLUDED.base_fee_cents,
    per_kg_rate_cents = EXCLUDED.per_kg_rate_cents,
    effective_from = EXCLUDED.effective_from,
    is_active = TRUE,
    updated_at = NOW();

-- 4. Seed secondary mock customer for IDOR & cross-user testing (password: CustomerPassword123!)
INSERT INTO users (id, email, password_hash, display_name, phone, role, tenant_id, status) VALUES
    ('018f4a7c-1006-7000-8000-000000000006', 'applicant@swiftdrop.local', '$sd$sha256$c271cc11a0beb226074d3a744477494d$dea45ec3435d22e4c1795402c22455766bddfba3ed1c73a786772ccbf9ff3c48', 'Applicant Customer', '+94770000006', 'CUSTOMER', NULL, 'ACTIVE')
ON CONFLICT (email) DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('018f4a7c-1006-7000-8000-000000000006', 'CUSTOMER')
ON CONFLICT (user_id, role_id) DO NOTHING;

-- 5. Seed secondary courier user & courier fleet profiles (password: CourierPassword123!)
INSERT INTO users (id, email, password_hash, display_name, phone, role, tenant_id, status) VALUES
    ('018f4a7c-1007-7000-8000-000000000007', 'courier2@swiftdrop.local', '$sd$sha256$6f19636f1cf0e6f3a9f047c0d003f736$98498e2cfe28af2307d3a02dc7e71a205e193f10c4a365a04277354baaf9f3f6', 'Sunil Silva (Courier 2)', '+94770000007', 'COURIER', NULL, 'ACTIVE')
ON CONFLICT (email) DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('018f4a7c-1007-7000-8000-000000000007', 'COURIER')
ON CONFLICT (user_id, role_id) DO NOTHING;

INSERT INTO couriers (user_id, employee_code, vehicle_type, max_capacity, status, current_zone_id, is_active, created_at, updated_at) VALUES
    ('018f4a7c-1003-7000-8000-000000000003', 'CR-0001', 'MOTORCYCLE', 25, 'ON_SHIFT', 'ZONE_COLOMBO_01', TRUE, NOW(), NOW()),
    ('018f4a7c-1007-7000-8000-000000000007', 'CR-0002', 'MOTORCYCLE', 25, 'ON_SHIFT', 'ZONE_COLOMBO_01', TRUE, NOW(), NOW())
ON CONFLICT (user_id) DO NOTHING;
