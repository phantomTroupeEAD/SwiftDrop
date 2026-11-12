-- Migration: 20261110000002_create_parcel_tables.down.sql
-- Reverses 20261110000002_create_parcel_tables.up.sql.

DROP TRIGGER IF EXISTS trg_parcel_status_history_immutable ON parcel_status_history;
DROP FUNCTION IF EXISTS prevent_parcel_status_history_modification();
DROP TABLE IF EXISTS parcel_status_history;
DROP TABLE IF EXISTS parcels;

