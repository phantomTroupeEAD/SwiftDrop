# Member 03 — Dispatch, Couriers & Proof of Delivery Alignment Guide

> **Author / Domain Owner:** Member 3 — Dispatch, Couriers & Proof of Delivery (Poorna / Kalani)  
> **Target Modules:** `core-api/internal/dispatch/` and `core-api/internal/delivery/`  
> **Status:** Authoritative Architectural Alignment & Action Sheet for Team Meeting 2  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Domain Overview & Scope of Work

Member 3 owns the operational delivery pipeline: **Courier Fleet Capacity & Shifts**, **Dispatcher Parcel Assignment**, **In-Transit Milestones**, **Delivery Attempt Failure Classifications**, and **Proof-of-Delivery (POD) Media Attachments**.

### 1.1 In-Scope Responsibilities
- Courier operational profiles (`couriers` table) and shift toggling (`ON_SHIFT`, `OFF_SHIFT`, `ON_BREAK`, `SUSPENDED`).
- Fleet capacity constraints by vehicle type (`MOTORCYCLE`: 25, `VAN`: 60, `TRUCK`: 120).
- Courier assignment (`POST /api/v1/dispatch/assign`) and reassignment with reason audit.
- Unassigned dispatch board queries (`GET /api/v1/dispatch/board`).
- Courier mobile tasks endpoint (`GET /api/v1/courier/tasks`).
- Pickup confirmation (`ASSIGNED` $\to$ `PICKED_UP`).
- Sorting hub scans (`PICKED_UP` $\to$ `IN_TRANSIT`).
- Final mile dispatch (`IN_TRANSIT` $\to$ `OUT_FOR_DELIVERY`).
- Delivery completion with proof (`OUT_FOR_DELIVERY` $\to$ `DELIVERED`).
- Delivery attempt failure recording (`OUT_FOR_DELIVERY` $\to$ `ATTEMPT_FAILED`) and terminal return (`RETURNED`) after 3 failed attempts (ADR-004).
- Proof-of-delivery storage integration (Cloudflare R2 in production, MinIO locally).

### 1.2 Out-of-Scope Responsibilities
- User account and password management for couriers (Owned by Member 1).
- Delivery zone geometric polygons and zone management (Owned by Member 7).
- Traefik routing and courier web screens (Owned by Member 7).
- Public tracking lookups (Owned by Member 5).
- Webhook notifications to merchants (Owned by Member 8).

---

## 2. Identified Flaws & Necessary Changes

1. **Migration Sequencing (Member 9 K12):** Member 3's draft placed all dispatch tables in `000003_dispatch_and_delivery.up.sql`. This crashed because foreign keys referenced `parcels` and `delivery_zones` before they were created.
   - *Fix:* Split into two sequential migrations: `000006_create_couriers.up.sql` (after zones and users) and `000007_create_dispatch_and_delivery.up.sql` (after parcels).
2. **Readiness Probe Cascading Failure (Member 9 K14):** Member 3's `doc-08` included a connectivity check to Cloudflare R2 / MinIO in the Core API `/health/ready` probe.
   - *Fix:* Core API `/health/ready` checks **PostgreSQL only** (Agreement A2). An R2 outage must only fail photo uploads with HTTP 503, not take down the entire API.
3. **Seed SQL Location (Member 9 K13):** Member 3 placed seed data inside `core-api/migrations/seeds/`.
   - *Fix:* Migrations must contain schema DDL only. Seed data is moved to `deploy/seeds/` and executed via `make seed` (Agreement D12).
4. **Environment Variables for Storage (Member 9 K15):** Adopt standardized names: `OBJECT_STORAGE_DRIVER`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`.
5. **Local Database Connection (Member 9 K16):** Connect using database `swiftdrop_db` and user `swiftdrop_app` (never `postgres:postgres`).

---

## 3. Database Migrations (Migrations 6 and 7)

### 3.1 Migration `000006_create_couriers.up.sql`
```sql
CREATE TABLE couriers (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    employee_code VARCHAR(32) NOT NULL UNIQUE, -- e.g. 'CR-0042'
    vehicle_type VARCHAR(32) NOT NULL, -- 'MOTORCYCLE', 'VAN', 'TRUCK'
    max_capacity INT NOT NULL DEFAULT 25,
    operational_status VARCHAR(32) NOT NULL DEFAULT 'OFF_SHIFT', -- 'OFF_SHIFT', 'ON_SHIFT', 'ON_BREAK', 'SUSPENDED'
    current_zone_id VARCHAR(32) REFERENCES delivery_zones(id) ON DELETE SET NULL,
    phone_number VARCHAR(32) NOT NULL,
    last_active_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_couriers_operational ON couriers(operational_status, current_zone_id);
```

### 3.2 Migration `000007_create_dispatch_and_delivery.up.sql`
```sql
CREATE TABLE courier_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id UUID NOT NULL REFERENCES parcels(id) ON DELETE CASCADE,
    courier_id UUID NOT NULL REFERENCES couriers(user_id) ON DELETE RESTRICT,
    assigned_by_user_id UUID NOT NULL REFERENCES users(id),
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE', -- 'ACTIVE', 'COMPLETED', 'REASSIGNED', 'CANCELLED'
    notes TEXT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    unassigned_at TIMESTAMPTZ
);

CREATE TABLE delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id UUID NOT NULL REFERENCES parcels(id) ON DELETE CASCADE,
    courier_id UUID NOT NULL REFERENCES couriers(user_id) ON DELETE RESTRICT,
    attempt_number INT NOT NULL,
    outcome VARCHAR(32) NOT NULL, -- 'SUCCESS', 'FAILED'
    failure_reason_code VARCHAR(64), -- 'RECIPIENT_UNAVAILABLE', 'INVALID_ADDRESS', 'PREMISES_INACCESSIBLE', 'RECIPIENT_REJECTED'
    proof_type VARCHAR(32), -- 'PHOTO', 'SIGNATURE', 'OTP'
    proof_url TEXT,
    recipient_signature TEXT,
    received_by_name VARCHAR(120),
    latitude NUMERIC(10, 8),
    longitude NUMERIC(11, 8),
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_assignments_courier ON courier_assignments(courier_id, status);
CREATE INDEX idx_attempts_parcel ON delivery_attempts(parcel_id, attempt_number ASC);
```

---

## 4. Operational State Machine & Capacity Invariant

### 4.1 Invariant Capacity Check
Before assigning a parcel to a courier, execute inside the assignment transaction:
```sql
SELECT COUNT(*) FROM courier_assignments 
WHERE courier_id = $1 AND status = 'ACTIVE';
```
If `active_count + 1 > courier.max_capacity`, return HTTP `409 Conflict` with error code `COURIER_CAPACITY_EXCEEDED`.

### 4.2 Proof-of-Delivery Photo Upload Flow
1. Mobile Courier Web requests a presigned upload URL: `POST /api/v1/courier/tasks/{id}/proof-upload-url`.
2. Core API generates an S3/R2 presigned `PUT` URL valid for **15 minutes**.
3. Mobile client uploads the binary JPEG/PNG directly to Cloudflare R2 / MinIO.
4. Courier client marks delivery complete:
   ```json
   POST /api/v1/courier/tasks/{id}/deliver
   {
     "proof_type": "PHOTO",
     "proof_url": "https://r2.swiftdrop.com/proofs/prcl_123.jpg",
     "received_by": "Jane Doe"
   }
   ```
5. Transaction commits: Updates parcel to `DELIVERED`, records `delivery_attempts`, inserts `parcel_status_history`, and writes `parcel.delivered.v1` to `outbox_events`.

---

## 5. Member 9 DevOps & Infrastructure Configuration

| Config Parameter | Required Specification | Alignment Rule |
| :--- | :--- | :--- |
| **Container & Port** | Runs inside `core-api` on port `:8080` | Agreement A1 |
| **Health Probes** | Core API probes: `/health/live` & `/health/ready` (PostgreSQL ONLY) | Agreement A2 & K14 |
| **Object Storage** | Cloudflare R2 (Prod) / MinIO (Local) via `integrations-secrets` | Agreement A5 & K15 |
| **Database Role** | Runtime: `swiftdrop_app`; Migrations: `swiftdrop_migrator` | Agreement D6 |
| **Migrations** | `000006_create_couriers.up.sql` and `000007_create_dispatch_and_delivery.up.sql` | Agreement D3 (6th & 7th) |

---
*End of Member 03 Specification & Alignment Guide.*
