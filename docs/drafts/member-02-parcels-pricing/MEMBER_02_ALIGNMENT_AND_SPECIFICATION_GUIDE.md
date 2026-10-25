# Member 02 — Parcel Management & Pricing Engine Alignment & Specification Guide

> **Author / Domain Owner:** Member 2 — Parcel Management & Pricing (Hansi)  
> **Target Modules:** `core-api/internal/parcels/` and `core-api/internal/pricing/`  
> **Status:** Authoritative Architectural Alignment & Action Sheet for Team Meeting 2  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Domain Overview & Scope of Work

Member 2 owns the core business heart of SwiftDrop: the **Parcel Aggregate Root**, the **Parcel Booking Lifecycle**, and the **Pricing & Tariff Engine**.

### 1.1 In-Scope Responsibilities
- Delivery fee calculation engine with zone-based tariff matrices.
- Pricing quote generation with expiration timestamps (15-minute validity).
- Customer parcel booking with accepted-price snapshots.
- Tracking number generation (`SD-YYYYMMDD-XXXXX`).
- Parcel aggregate root state machine invariant enforcement.
- Optimistic concurrency control (`version` column) on parcel records.
- Immutable audit timeline recording in `parcel_status_history`.
- Transactional Outbox event generation (`parcel.booked.v1`, `parcel.cancelled.v1`).
- Shared parcel creation use-case reused by Member 4 (Merchant order ingestion).
- Owner of the shared `idempotency_keys` persistence table.

### 1.2 Out-of-Scope Responsibilities
- Courier assignments and dispatch board operations (Owned by Member 3).
- Proof-of-delivery attachments and delivery attempt recording (Owned by Member 3).
- Merchant API key authentication (Owned by Member 4).
- Public tracking SSE stream serving (Owned by Member 5).
- Webhook signing and network event dispatch (Owned by Member 8).

---

## 2. Document Audit & Current State

- **Current Status in `docs/drafts/`:** All 8 draft documents (`doc-01` to `doc-08`) were submitted.
- **Identified Flaws:** The submitted documents are overly cautious, containing numerous open placeholders ("needs team agreement", "baseline not confirmed"), conflicting route names (`/pricing/quotes` vs `/pricing/estimate`), and lack concrete tariff calculation formulas and integer cents data types.
- **Immediate Action:** Replace speculative placeholders with the binding rules defined below.

---

## 3. Authoritative Resolutions & Design Decisions

### 3.1 Pricing Engine Tariff Formula (Binding Standard)
The delivery fee calculation MUST execute using deterministic integer minor units (`bigint` cents in LKR):

$$\text{Volumetric Weight (kg)} = \frac{\text{Length (cm)} \times \text{Width (cm)} \times \text{Height (cm)}}{5000}$$
$$\text{Chargeable Weight (kg)} = \max(\text{Actual Weight}, \text{Volumetric Weight})$$
$$\text{Base Fee} = \text{Rule.base\_fee\_cents}$$
$$\text{Weight Fee} = \text{ceil}(\max(0, \text{Chargeable Weight} - 1.0)) \times \text{Rule.per\_kg\_fee\_cents}$$
$$\text{Total Fee (cents)} = \text{Base Fee} + \text{Weight Fee}$$

*Example:* A 2.5 kg parcel (20x20x20 cm) between Colombo zones:
- Volumetric weight = $(20 \times 20 \times 20) / 5000 = 1.6 \text{ kg}$.
- Chargeable weight = $\max(2.5, 1.6) = 2.5 \text{ kg}$.
- If `base_fee_cents` = 35000 (LKR 350.00 for 1st kg) and `per_kg_fee_cents` = 10000 (LKR 100.00/kg):
- Total Fee = $35000 + (2 \times 10000) = 55000 \text{ cents}$ (LKR 550.00).

### 3.2 Canonical API Endpoints
- `POST /api/v1/pricing/quotes` — Calculate delivery fee quote (Valid for 15 minutes).
- `POST /api/v1/parcels` — Book a parcel (Requires `Idempotency-Key` header).
- `GET /api/v1/parcels` — List caller-owned parcels (supports pagination `?page=1&limit=20`).
- `GET /api/v1/parcels/{id}` — Get parcel details with price snapshot.
- `GET /api/v1/parcels/{id}/history` — Get parcel audit history.
- `GET /api/v1/parcels/{id}/receipt` — Get JSON receipt payload.

### 3.3 Transactional State Invariants
Direct SQL updates on `current_status` are **strictly prohibited**. State changes must occur via the `Parcel` aggregate root within a single PostgreSQL transaction:
```go
tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
// 1. Fetch parcel FOR UPDATE
// 2. Evaluate state transition invariant: parcel.CanTransitionTo(newStatus)
// 3. Increment version and update parcel
// 4. Insert into parcel_status_history
// 5. Insert into outbox_events
// 6. tx.Commit()
```

---

## 4. Database Migrations (Migrations 4, 5, and 10)

### 4.1 Migration `000004_create_pricing_rules.up.sql`
```sql
CREATE TABLE pricing_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    origin_zone_id VARCHAR(32) NOT NULL REFERENCES delivery_zones(id) ON DELETE RESTRICT,
    destination_zone_id VARCHAR(32) NOT NULL REFERENCES delivery_zones(id) ON DELETE RESTRICT,
    base_fee_cents BIGINT NOT NULL, -- LKR cents
    per_kg_fee_cents BIGINT NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pricing_rules_lookup 
ON pricing_rules (origin_zone_id, destination_zone_id, enabled);
```

### 4.2 Migration `000005_create_parcels_and_quotes.up.sql`
```sql
CREATE TABLE pricing_quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID NOT NULL REFERENCES users(id),
    origin_zone_id VARCHAR(32) NOT NULL REFERENCES delivery_zones(id),
    destination_zone_id VARCHAR(32) NOT NULL REFERENCES delivery_zones(id),
    actual_weight_kg NUMERIC(6, 3) NOT NULL,
    length_cm NUMERIC(6, 2) NOT NULL,
    width_cm NUMERIC(6, 2) NOT NULL,
    height_cm NUMERIC(6, 2) NOT NULL,
    chargeable_weight_kg NUMERIC(6, 3) NOT NULL,
    fee_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'LKR',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE parcels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tracking_number VARCHAR(32) NOT NULL UNIQUE,
    sender_id UUID NOT NULL REFERENCES users(id),
    merchant_id UUID REFERENCES merchants(id),
    current_status VARCHAR(32) NOT NULL DEFAULT 'CREATED',
    weight_kg NUMERIC(6, 3) NOT NULL,
    length_cm NUMERIC(6, 2) NOT NULL,
    width_cm NUMERIC(6, 2) NOT NULL,
    height_cm NUMERIC(6, 2) NOT NULL,
    origin_zone_id VARCHAR(32) NOT NULL REFERENCES delivery_zones(id),
    destination_zone_id VARCHAR(32) NOT NULL REFERENCES delivery_zones(id),
    pickup_address JSONB NOT NULL,
    dropoff_address JSONB NOT NULL,
    recipient_name VARCHAR(120) NOT NULL,
    recipient_phone VARCHAR(32) NOT NULL,
    fee_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'LKR',
    quote_id UUID REFERENCES pricing_quotes(id),
    attempt_count INT NOT NULL DEFAULT 0,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE parcel_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id UUID NOT NULL REFERENCES parcels(id) ON DELETE CASCADE,
    from_status VARCHAR(32),
    to_status VARCHAR(32) NOT NULL,
    changed_by_user_id UUID REFERENCES users(id),
    reason TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_parcels_tracking ON parcels(tracking_number);
CREATE INDEX idx_parcels_sender ON parcels(sender_id, created_at DESC);
CREATE INDEX idx_parcels_merchant ON parcels(merchant_id, created_at DESC);
CREATE INDEX idx_history_parcel ON parcel_status_history(parcel_id, occurred_at ASC);
```

### 4.3 Migration `000010_create_idempotency_keys.up.sql`
```sql
CREATE TABLE idempotency_keys (
    key VARCHAR(128) NOT NULL,
    caller_id UUID NOT NULL,
    endpoint VARCHAR(128) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    response_code INT NOT NULL,
    response_body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (key, caller_id)
);

CREATE INDEX idx_idempotency_expiry ON idempotency_keys(expires_at);
```

---

## 5. Member 9 DevOps & Infrastructure Configuration

| Config Parameter | Required Specification | Alignment Rule |
| :--- | :--- | :--- |
| **Container & Port** | Runs inside `core-api` on port `:8080` | Agreement A1 |
| **Health Probes** | Shared Core API probes: `/health/live` & `/health/ready` | Agreement A2 |
| **Database Pool** | `DATABASE_MAX_OPEN_CONNS=25`, `DATABASE_MAX_IDLE_CONNS=10` | Agreement D8 |
| **Database Role** | Runtime: `swiftdrop_app`; Migrations: `swiftdrop_migrator` | Agreement D6 |
| **Migrations** | `000004_create_pricing_rules.up.sql`, `000005_create_parcels_and_quotes.up.sql`, `000010_create_idempotency_keys.up.sql` | Agreement D3 |

---
*End of Member 02 Specification & Alignment Guide.*
