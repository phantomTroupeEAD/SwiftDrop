-- Migration: 20261021000002_seed_mock_users.up.sql
-- Description: Seed 5 canonical mock users across all 5 roles for development and team collaboration.

-- Pre-hashed passwords generated via StandardPasswordHasher ($sd$sha256$<salt>$<hash>):
-- Admin:        AdminPassword123!
-- Dispatcher:   Dispatcher123!
-- Courier:      CourierPassword123!
-- Merchant:     Merchant123!
-- Customer:     CustomerPassword123!

INSERT INTO users (id, email, password_hash, display_name, phone, role, tenant_id, status) VALUES
    ('018f4a7c-1001-7000-8000-000000000001', 'admin@swiftdrop.local', '$sd$sha256$79956f158e122617d8e9691ab75d0665$f87f3efe3d7748212fa39ffb08545ce443fcfaf3df46baf0111c9e6d8e9f20ba', 'System Administrator', '+94770000001', 'ADMIN', NULL, 'ACTIVE'),
    ('018f4a7c-1002-7000-8000-000000000002', 'dispatcher@swiftdrop.local', '$sd$sha256$eaf3eff86dfbaaeceea1939ca664a6cd$83e03fe6778e5bcca8c3319ef477ac7d3838325af53ee2fa4be6007763e793a1', 'Central Dispatcher', '+94770000002', 'DISPATCHER', NULL, 'ACTIVE'),
    ('018f4a7c-1003-7000-8000-000000000003', 'courier@swiftdrop.local', '$sd$sha256$6f19636f1cf0e6f3a9f047c0d003f736$98498e2cfe28af2307d3a02dc7e71a205e193f10c4a365a04277354baaf9f3f6', 'Kamal Perera (Courier)', '+94770000003', 'COURIER', NULL, 'ACTIVE'),
    ('018f4a7c-1004-7000-8000-000000000004', 'merchant@vendor.com', '$sd$sha256$6af7f1f27c2277e689a6bf7eb39fede5$dc09bf818fcfb7af753e4622dd88db7d4ba0711de03a5057911955bac9f5ff12', 'Colombo Tech Stores', '+94770000004', 'MERCHANT', 'mrc_01HMERCHANTTENANT00001', 'ACTIVE'),
    ('018f4a7c-1005-7000-8000-000000000005', 'customer@swiftdrop.local', '$sd$sha256$c271cc11a0beb226074d3a744477494d$dea45ec3435d22e4c1795402c22455766bddfba3ed1c73a786772ccbf9ff3c48', 'Nimal Fernando', '+94770000005', 'CUSTOMER', NULL, 'ACTIVE')
ON CONFLICT (email) DO NOTHING;

INSERT INTO user_roles (user_id, role_id) VALUES
    ('018f4a7c-1001-7000-8000-000000000001', 'ADMIN'),
    ('018f4a7c-1002-7000-8000-000000000002', 'DISPATCHER'),
    ('018f4a7c-1003-7000-8000-000000000003', 'COURIER'),
    ('018f4a7c-1004-7000-8000-000000000004', 'MERCHANT'),
    ('018f4a7c-1005-7000-8000-000000000005', 'CUSTOMER')
ON CONFLICT (user_id, role_id) DO NOTHING;
