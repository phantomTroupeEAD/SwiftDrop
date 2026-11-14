-- Migration: 20261112000001_create_email_otps.up.sql
-- Description: Persistent email OTP storage for registration and password reset verification

CREATE TABLE IF NOT EXISTS email_otps (
    email VARCHAR(255) PRIMARY KEY,
    code VARCHAR(32) NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    cooldown_end TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_email_otps_expires_at ON email_otps (expires_at);
