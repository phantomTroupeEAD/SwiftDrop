# Member 04 — Merchant Integration & Webhooks Alignment Guide

> **Author / Domain Owner:** Member 4 — Merchant Integration & Webhooks (Nagitha)  
> **Target Module:** `core-api/internal/merchants/`  
> **Status:** Authoritative Architectural Alignment & Action Sheet for Team Meeting 2  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Domain Overview & Scope of Work

Member 4 owns the business integration surface: **Merchant Profiles & Onboarding**, **Merchant API Credentials (`sd_live_*`)**, **Webhook Endpoint Configuration**, and **Merchant Order Ingestion Proxying**.

### 1.1 In-Scope Responsibilities
- Merchant business profile onboarding (`POST /api/v1/merchants/register`).
- Merchant status lifecycle transitions (`PENDING` $\to$ `APPROVED` / `REJECTED` / `SUSPENDED` / `DISABLED`).
- API key generation with fast cryptographic hashing (SHA-256 with pepper).
- Webhook registration and validation (HTTPS, valid URL, event subscriptions).
- Server-side generation of webhook secrets (`whsec_...`) and encrypted persistence (AES-256-GCM).
- Defining the read-only database view `merchant_webhook_targets_view` consumed by Member 8's Event Worker.
- Merchant order submission endpoint (`POST /api/v1/merchant/orders`) delegating to Member 2's `CreateParcel` use-case.
- Multi-tenant data isolation (`X-Tenant-ID` and `merchant_id` query scoping).

### 1.2 Out-of-Scope Responsibilities
- Implementing HMAC signature calculation and HTTP dispatching at runtime (Owned by Member 8).
- Webhook retry backoff, dead-letter queue (DLQ) processing (Owned by Member 8).
- Parcel tariff calculation and tracking number generation (Owned by Member 2).
- Admin approval UI screens (Owned by Member 7).

---

## 2. Definitive Resolution of Open Questions (U-01 to U-47)

| Question ID | Issue / Dilemma | Authoritative Resolution |
| :--- | :--- | :--- |
| **U-01** | Table naming discrepancies | Use canonical names: `merchant_api_credentials` and `webhook_endpoints`. |
| **U-02** | Merchant status enum | Allowed values: `PENDING`, `APPROVED`, `REJECTED`, `SUSPENDED`, `DISABLED`. Reinstating `SUSPENDED` $\to$ `APPROVED` is permitted. |
| **U-03** | Webhook secret storage | **Do NOT use `secret_hash`**. Store symmetrically encrypted `secret_ciphertext BYTEA` using AES-256-GCM so Member 8 can decrypt at runtime for HMAC signing. |
| **U-04** | Webhook secret generation | Generated strictly by the server (`whsec_` + 32 random bytes), displayed once upon creation. Merchants cannot supply their own secret. |
| **U-05** | Route naming conventions | Self-service merchant config: `/api/v1/merchants/me/*`. B2B order integration: `/api/v1/merchant/orders`. |
| **U-06** | Webhook event names | Standardize on: `parcel.booked.v1`, `parcel.picked_up.v1`, `parcel.in_transit.v1`, `parcel.out_for_delivery.v1`, `parcel.delivered.v1`, `parcel.delivery_failed.v1`, `parcel.returned.v1`. |
| **U-08** | Worker config access | Member 8's worker connects using PostgreSQL role `swiftdrop_worker` and queries `merchant_webhook_targets_view`. |
| **U-11** | Optimistic concurrency | Add `version INT NOT NULL DEFAULT 1` to `merchants` table. |

---

## 3. Database Migrations (`000003_create_merchants_and_credentials.up.sql`)

Member 4 must consolidate all draft migration files into the canonical sequential migration:

```sql
-- 1. Merchants Table
CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    business_name VARCHAR(150) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'APPROVED', 'REJECTED', 'SUSPENDED', 'DISABLED'
    contact_phone VARCHAR(32) NOT NULL,
    contact_email VARCHAR(255) NOT NULL,
    address JSONB NOT NULL,
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Merchant API Credentials
CREATE TABLE merchant_api_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    key_prefix VARCHAR(16) NOT NULL, -- e.g. 'sd_live_a1b2c3d4'
    key_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA-256 hash of plaintext key
    label VARCHAR(64) NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT ARRAY['parcels:read', 'parcels:write'],
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Webhook Endpoints
CREATE TABLE webhook_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    target_url VARCHAR(512) NOT NULL,
    secret_ciphertext BYTEA NOT NULL, -- AES-256-GCM encrypted signing secret
    event_types TEXT[] NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Worker Read-Only Target View
CREATE OR REPLACE VIEW merchant_webhook_targets_view AS
SELECT 
    w.id AS endpoint_id,
    w.merchant_id,
    w.target_url,
    w.secret_ciphertext,
    w.event_types,
    m.status AS merchant_status
FROM webhook_endpoints w
JOIN merchants m ON m.id = w.merchant_id
WHERE w.is_active = TRUE AND m.status = 'APPROVED';

CREATE INDEX idx_merchants_owner ON merchants(owner_user_id);
CREATE INDEX idx_api_keys_lookup ON merchant_api_credentials(key_hash) WHERE revoked_at IS NULL;
CREATE INDEX idx_webhooks_merchant ON webhook_endpoints(merchant_id);
```

---

## 4. Security & Cryptographic Standards

### 4.1 API Key Format & Authentication
- Plaintext API key format: `sd_live_<32_random_hex_characters>` (e.g. `sd_live_4f89d3a1e2b5c7f8...`).
- Stored as SHA-256 hash: `key_hash = Hex(SHA256(plaintext_key + PEPPER))`.
- Authentication middleware validates:
  1. Key is active and not expired (`revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`).
  2. Associated merchant has `status = 'APPROVED'`.

### 4.2 Webhook Secret Cryptographic Protection
```go
// AES-256-GCM encryption using WEBHOOK_SECRET_ENCRYPTION_KEY
func EncryptWebhookSecret(plaintext string, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil { return nil, err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return nil, err }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return nil, err }
    return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}
```

### 4.3 Registration-Time SSRF Prevention
When registering `target_url`:
- Must start with `https://` (except in local development where `http://` is tolerated).
- Destination port must be `443` or `8443`.
- Reject hostname if it matches `localhost`, `127.0.0.1`, `169.254.*`, or AWS metadata IP.

---

## 5. Member 9 DevOps & Infrastructure Configuration

| Config Parameter | Required Specification | Alignment Rule |
| :--- | :--- | :--- |
| **Container & Port** | Runs inside `core-api` on port `:8080` | Agreement A1 |
| **Health Probes** | Core API probes: `/health/live` & `/health/ready` | Agreement A2 |
| **Kubernetes Secrets**| Mounted from `integrations-secrets` (`WEBHOOK_SECRET_ENCRYPTION_KEY`) | Agreement A5 |
| **Database Roles** | Runtime: `swiftdrop_app`; Worker reads view via: `swiftdrop_worker` | Agreement D6 & K8 |
| **Migration File** | `core-api/migrations/000003_create_merchants_and_credentials.up.sql` | Agreement D3 (3rd) |

---
*End of Member 04 Specification & Alignment Guide.*
