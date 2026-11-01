-- Migration: 20261021000002_seed_mock_users.up.sql
-- Description: Seed 5 canonical mock users across all 5 roles for development and team collaboration.

-- Pre-hashed passwords generated via StandardPasswordHasher ($sd$sha256$<salt>$<hash>):
-- Admin:        AdminPassword123!
-- Dispatcher:   Dispatcher123!
-- Courier:      CourierPassword123!
-- Merchant:     Merchant123!
-- Customer:     CustomerPassword123!

INSERT INTO users (id, email, password_hash, display_name, phone, role, tenant_id, status) VALUES
    ('018f4a7c-1001-7000-8000-000000000001', 'admin@swiftdrop.local', '$sd$sha256$a1b2c3d4e5f60718$48db16503c58bbfd9b15d6541f53d2d14aa4beaa3f8dcbebb1a6c4b22c4f74d9', 'System Administrator', '+94770000001', 'ADMIN', NULL, 'ACTIVE'),
    ('018f4a7c-1002-7000-8000-000000000002', 'dispatcher@swiftdrop.local', '$sd$sha256$b2c3d4e5f6071829$52d3a987efc1234567890abcdef1234567890abcdef1234567890abcdef12345', 'Central Dispatcher', '+94770000002', 'DISPATCHER', NULL, 'ACTIVE'),
    ('018f4a7c-1003-7000-8000-000000000003', 'courier@swiftdrop.local', '$sd$sha256$c3d4e5f60718293a$63e4b098fed2345678901bcdef2345678901bcdef2345678901bcdef23456', 'Kamal Perera (Courier)', '+94770000003', 'COURIER', NULL, 'ACTIVE'),
    ('018f4a7c-1004-7000-8000-000000000004', 'merchant@vendor.com', '$sd$sha256$d4e5f60718293a4b$74f5c109afe3456789012cdef3456789012cdef3456789012cdef34567', 'Colombo Tech Stores', '+94770000004', 'MERCHANT', 'mrc_01HMERCHANTTENANT00001', 'ACTIVE'),
    ('018f4a7c-1005-7000-8000-000000000005', 'customer@swiftdrop.local', '$sd$sha256$e5f60718293a4b5c$85a6d210b0f4567890123def4567890123def4567890123def45678', 'Nimal Fernando', '+94770000005', 'CUSTOMER', NULL, 'ACTIVE')
ON CONFLICT (email) DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('018f4a7c-1001-7000-8000-000000000001', 'ADMIN'),
    ('018f4a7c-1002-7000-8000-000000000002', 'DISPATCHER'),
    ('018f4a7c-1003-7000-8000-000000000003', 'COURIER'),
    ('018f4a7c-1004-7000-8000-000000000004', 'MERCHANT'),
    ('018f4a7c-1005-7000-8000-000000000005', 'CUSTOMER')
ON CONFLICT (user_id, role_id) DO NOTHING;
