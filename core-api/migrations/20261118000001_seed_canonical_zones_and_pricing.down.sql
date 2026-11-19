-- Migration: 20261118000001_seed_canonical_zones_and_pricing.down.sql
-- Description: Rollback seeded canonical pricing rules, postal code prefixes, delivery zones, and secondary users.

DELETE FROM couriers WHERE user_id IN ('018f4a7c-1003-7000-8000-000000000003', '018f4a7c-1007-7000-8000-000000000007');
DELETE FROM user_roles WHERE user_id IN ('018f4a7c-1006-7000-8000-000000000006', '018f4a7c-1007-7000-8000-000000000007');
DELETE FROM users WHERE id IN ('018f4a7c-1006-7000-8000-000000000006', '018f4a7c-1007-7000-8000-000000000007');

DELETE FROM pricing_rules WHERE id LIKE 'rule_zone_%';

DELETE FROM zone_postal_codes WHERE prefix IN (
    '00100', '00200', '00300', '00400', '00500', '00600',
    '00700', '00800', '11500', '20000', '80000', '99999'
);

DELETE FROM delivery_zones WHERE id IN (
    'ZONE_COLOMBO_01',
    'ZONE_COLOMBO_02',
    'ZONE_COLOMBO_03',
    'ZONE_NEGOMBO_01',
    'ZONE_KANDY_01',
    'ZONE_GALLE_01',
    'ZONE_DEMO_DISABLED'
);
