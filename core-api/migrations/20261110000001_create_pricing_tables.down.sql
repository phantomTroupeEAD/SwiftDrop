-- Migration: 20261110000001_create_pricing_tables.down.sql
-- Reverses 20261110000001_create_pricing_tables.up.sql.
-- Note: Fails if parcels (20261110000002) still references pricing_quotes or pricing_rules; roll back parcels first.

DROP TABLE IF EXISTS pricing_quotes;
DROP TABLE IF EXISTS pricing_rules;

