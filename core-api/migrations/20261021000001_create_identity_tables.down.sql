-- Migration: 20261021000001_create_identity_tables.down.sql
-- Description: Clean rollback of identity schema

DROP INDEX IF EXISTS idx_refresh_tokens_family;
DROP INDEX IF EXISTS idx_refresh_tokens_hash;
DROP INDEX IF EXISTS idx_users_tenant_id;
DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_email_lower;

DROP TABLE IF EXISTS oauth_identities;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;
