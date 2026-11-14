-- Migration: 20261113000001_add_parcel_item_details.down.sql
-- Owner: Member 2 (Parcel Management & Pricing)
-- Purpose: Revert addition of item_description and special_instructions columns

ALTER TABLE parcels
    DROP COLUMN IF EXISTS special_instructions,
    DROP COLUMN IF EXISTS item_description;
