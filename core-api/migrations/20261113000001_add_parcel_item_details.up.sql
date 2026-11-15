-- Migration: 20261113000001_add_parcel_item_details.up.sql
-- Owner: Member 2 (Parcel Management & Pricing)
-- Purpose: Add item_description and special_instructions columns to parcels table

ALTER TABLE parcels
    ADD COLUMN IF NOT EXISTS item_description TEXT NULL,
    ADD COLUMN IF NOT EXISTS special_instructions TEXT NULL;
