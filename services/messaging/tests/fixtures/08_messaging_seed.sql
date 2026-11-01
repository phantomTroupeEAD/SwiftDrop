-- ==============================================================================
-- Fixture: 08_messaging_seed.sql
-- Purpose: Seed data for messaging integration tests and local development.
-- ==============================================================================

-- 1. Seed Merchant
INSERT INTO merchants (id, business_name, status)
VALUES ('0192e2b3-7c8a-7890-a123-000000000042', 'Ceylon Organics Ltd', 'APPROVED')
ON CONFLICT (id) DO NOTHING;

-- 2. Seed User
INSERT INTO users (id, email, full_name, role)
VALUES ('0192e2b3-7c8a-7890-a123-000000000001', 'admin@swiftdrop.local', 'System Admin', 'ADMIN')
ON CONFLICT (email) DO NOTHING;

-- 3. Seed Webhook Endpoint
-- Note: secret_ciphertext is a 12-byte nonce + 32-byte ciphertext + 16-byte tag (60 bytes)
-- Encrypted with key "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
-- Plaintext secret: "whsec_test_secret_for_local_merchant_123"
-- Target: http://localhost:8088/webhook
INSERT INTO webhook_endpoints (
    id, merchant_id, url, secret_ciphertext, secret_key_version, secret_last4,
    subscribed_event_types, enabled, created_by
)
VALUES (
    '0192e2b3-7c8a-7890-a123-000000000088',
    '0192e2b3-7c8a-7890-a123-000000000042',
    'http://localhost:8088/webhook',
    decode('000102030405060708090a0b27b9c6f2df80a905a761ef7655079a405fb557088b901614f179116e053cb377197b0eeacbf09eb1283d5a9fae804f5e714271424b95333f28ea96', 'hex'),
    1,
    'e123',
    ARRAY['parcel.booked.v1', 'parcel.out_for_delivery.v1', 'parcel.delivered.v1', 'parcel.delivery_failed.v1', 'parcel.returned.v1'],
    true,
    '0192e2b3-7c8a-7890-a123-000000000001'
)
ON CONFLICT (id) DO NOTHING;

