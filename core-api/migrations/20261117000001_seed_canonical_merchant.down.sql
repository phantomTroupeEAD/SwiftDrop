-- Migration: 20261117000001_seed_canonical_merchant.down.sql
-- Description: Rollback canonical merchant seed and restore legacy string tenant ID.

DELETE FROM merchants WHERE id = '018f4a7c-1004-7000-9000-000000000004';

UPDATE users
SET tenant_id = 'mrc_01HMERCHANTTENANT00001'
WHERE id = '018f4a7c-1004-7000-8000-000000000004';
