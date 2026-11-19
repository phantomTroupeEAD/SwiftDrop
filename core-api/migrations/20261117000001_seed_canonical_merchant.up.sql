-- Migration: 20261117000001_seed_canonical_merchant.up.sql
-- Description: Align seeded merchant user with a canonical UUID tenant and seed approved merchant aggregate.

-- 1. Align seeded merchant user with a canonical UUID tenant
UPDATE users
SET tenant_id = '018f4a7c-1004-7000-9000-000000000004'
WHERE id = '018f4a7c-1004-7000-8000-000000000004'
   OR email = 'merchant@vendor.com';

-- 2. Seed canonical approved merchant aggregate for Member 4
INSERT INTO merchants (
    id,
    owner_user_id,
    business_name,
    status,
    contact_email,
    contact_phone,
    address,
    approved_by,
    approved_at,
    version,
    created_at,
    updated_at
) VALUES (
    '018f4a7c-1004-7000-9000-000000000004',
    '018f4a7c-1004-7000-8000-000000000004',
    'Colombo Tech Stores',
    'APPROVED',
    'merchant@vendor.com',
    '+94770000004',
    '{"line1": "42 Galle Road", "city": "Colombo", "postal_code": "00300", "country": "LK"}'::jsonb,
    '018f4a7c-1001-7000-8000-000000000001',
    NOW(),
    1,
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;
