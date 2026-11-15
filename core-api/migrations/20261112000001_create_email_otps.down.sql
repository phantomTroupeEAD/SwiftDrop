-- Migration: 20261112000001_create_email_otps.down.sql
-- Description: Drop persistent email OTP storage table

DROP TABLE IF EXISTS email_otps;
