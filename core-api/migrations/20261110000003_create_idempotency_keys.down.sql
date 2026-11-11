-- Migration: 20261110000003_create_idempotency_keys.down.sql
-- Reverses 20261110000003_create_idempotency_keys.up.sql.

DROP INDEX IF EXISTS idx_idempotency_keys_expires_at;
DROP TABLE IF EXISTS idempotency_keys;

