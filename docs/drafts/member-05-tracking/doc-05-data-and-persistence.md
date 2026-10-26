# Document 5 — Data & Persistence Specification

**Project:** SwiftDrop — Parcel Delivery & Courier Management System  
**Module:** Tracking API & Real-Time Updates  
**Module Owner:** Member 5 (Senuji)  
**Version:** 1.0.0 — Post-Audit Harmonized  
**Status:** Approved & Ratified  
**Primary Technology:** PostgreSQL 16, pgx/v5 driver, dedicated read-only role `swiftdrop_tracking_ro`  
**Target Codepath:** `services/tracking/internal/repository/`  
**Related Documents:** Documents 1–4, Member 2 `doc-05-data-and-persistence.md`, Member 9 Agreements (D6, D8)

---

## 1. Persistence Philosophy & Data Ownership

The Tracking API is an **ephemeral, read-only projection service**. 

### 1.1 Invariants
1. **PostgreSQL Is the Single Source of Truth:** All authoritative parcel statuses and historical milestones originate from PostgreSQL. Neither Redis Pub/Sub nor RabbitMQ messages are durable stores.
2. **Zero Schema Ownership:** Member 5 owns **zero database migrations** and **no tables**. All DDL migrations are authored by Member 2 / Member 3 and executed by Member 9 (`swiftdrop_migrator`) in `core-api/migrations/`.
3. **No Mutations:** Under no circumstances does the Tracking API issue `INSERT`, `UPDATE`, `DELETE`, or DDL statements.
4. **Physical Role Enforcement:** The service connects exclusively through the dedicated read-only role **`swiftdrop_tracking_ro`** (Agreement D6).

---

## 2. Authoritative Schema Binding & Physical ERD

The Tracking API binds directly to the ratified schema managed by Member 2 (Migrations `000004` and `000005`):

```mermaid
erDiagram
    parcels ||--o{ parcel_addresses : "has addresses (PICKUP / DROPOFF)"
    parcels ||--o{ parcel_status_history : "has historical milestones"
    parcel_addresses }o--|| delivery_zones : "references zone"

    parcels {
        uuid id PK
        varchar tracking_number UK "SD-YYYYMMDD-XXXXXX"
        varchar current_status "CREATED, ASSIGNED, PICKED_UP, IN_TRANSIT, OUT_FOR_DELIVERY, DELIVERED, ATTEMPT_FAILED, RETURNED, CANCELLED"
        uuid sender_id FK "Customer ID"
        uuid merchant_id FK "Merchant Tenant ID"
        timestamptz created_at
        timestamptz updated_at
    }

    parcel_addresses {
        uuid id PK
        uuid parcel_id FK
        varchar address_type "PICKUP, DROPOFF"
        varchar street_line1 "REDACTED"
        varchar street_line2 "REDACTED"
        varchar city "PUBLIC: EXPOSED"
        varchar postal_code "AUTHENTICATED ONLY"
        varchar zone_id FK "REDACTED from public"
    }

    parcel_status_history {
        uuid id PK
        uuid parcel_id FK
        varchar from_status
        varchar to_status "New Status"
        varchar reason_code "E.g. RECIPIENT_UNAVAILABLE"
        text note "INTERNAL ONLY - NEVER EXPOSE PUBLICLY"
        varchar actor_role "CUSTOMER, MERCHANT, COURIER, DISPATCHER"
        timestamptz occurred_at
    }

    delivery_zones {
        varchar id PK
        varchar name "Zone Name"
    }
```

---

## 3. Data Dictionary for Tracking Projections

| Table | Column | Type | Nullable | Exposure Level | Tracking Usage |
|---|---|---|:---:|---|---|
| `parcels` | `id` | `UUID` | No | Internal / Auth | Join key for status history and addresses. |
| `parcels` | `tracking_number` | `VARCHAR(32)` | No | **Public** | Primary lookup key from URL path. |
| `parcels` | `current_status` | `VARCHAR(32)` | No | **Public** | Current authoritative shipment status. |
| `parcels` | `sender_id` | `UUID` | No | Auth Only | Verified against JWT `sub` for customer ownership. |
| `parcels` | `merchant_id` | `UUID` | Yes | Auth Only | Verified against JWT `tenant_id` for merchant ownership. |
| `parcels` | `created_at` | `TIMESTAMPTZ` | No | **Public** | Booking date/time milestone. |
| `parcels` | `updated_at` | `TIMESTAMPTZ` | No | **Public** | Last status change timestamp. |
| `parcel_addresses` | `city` | `VARCHAR(64)` | No | **Public** | Destination city exposed on public tracking page. |
| `parcel_addresses` | `postal_code` | `VARCHAR(16)` | Yes | Auth Only | Visible to authenticated owner; redacted on public endpoint. |
| `parcel_addresses` | `street_line1` | `VARCHAR(255)` | No | **Confidential** | **Strictly Redacted (SEC-009)**. |
| `parcel_addresses` | `zone_id` | `VARCHAR(32)` | Yes | Internal / Auth | **Strictly Redacted from public response (SEC-009)**. |
| `parcel_status_history`| `to_status` | `VARCHAR(32)` | No | **Public** | Status code of chronological milestone. |
| `parcel_status_history`| `reason_code` | `VARCHAR(64)` | Yes | Auth Only | Failure or cancellation code; mapped to description for public. |
| `parcel_status_history`| `note` | `TEXT` | Yes | **Confidential** | **Internal operational notes; STRICTLY REDACTED**. |
| `parcel_status_history`| `actor_role` | `VARCHAR(32)` | No | Auth Only | Role that triggered milestone (`COURIER`, `MERCHANT`). |
| `parcel_status_history`| `occurred_at` | `TIMESTAMPTZ` | No | **Public** | Milestone event timestamp in UTC. |

---

## 4. Dedicated Read-Only Database Role (`swiftdrop_tracking_ro`)

Per DevOps Agreement **D6**, Member 9 provisions the dedicated read-only role during database bootstrapping.

### 4.1 Required Role Grants (Agreement D6)

```sql
-- Role definition
CREATE ROLE swiftdrop_tracking_ro WITH LOGIN PASSWORD '${TRACKING_DB_PASSWORD}';

-- Database and schema access
GRANT CONNECT ON DATABASE swiftdrop_db TO swiftdrop_tracking_ro;
GRANT USAGE ON SCHEMA public TO swiftdrop_tracking_ro;

-- Specific read-only table grants
GRANT SELECT ON parcels TO swiftdrop_tracking_ro;
GRANT SELECT ON parcel_addresses TO swiftdrop_tracking_ro;
GRANT SELECT ON parcel_status_history TO swiftdrop_tracking_ro;
GRANT SELECT ON delivery_zones TO swiftdrop_tracking_ro;

-- Enforce zero modification privileges
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM swiftdrop_tracking_ro;
GRANT SELECT ON parcels, parcel_addresses, parcel_status_history, delivery_zones TO swiftdrop_tracking_ro;
```

---

## 5. Optimized Repository Queries

### 5.1 Public Tracking Query (Audit Fix: Correct Join on `parcel_addresses`)

The initial draft incorrectly attempted to select `parcels.destination_zone_id`. The ratified schema stores destination address information in `parcel_addresses` with `address_type = 'DROPOFF'`.

```sql
-- 1. Fetch parcel and destination city
SELECT 
    p.id,
    p.tracking_number,
    p.current_status,
    p.created_at,
    p.updated_at,
    pa.city AS destination_city
FROM parcels p
LEFT JOIN parcel_addresses pa 
    ON p.id = pa.parcel_id AND pa.address_type = 'DROPOFF'
WHERE p.tracking_number = $1;

-- 2. Fetch chronological milestone history (excluding internal notes)
SELECT 
    to_status,
    reason_code,
    occurred_at
FROM parcel_status_history
WHERE parcel_id = $1
ORDER BY occurred_at ASC;
```

### 5.2 Authenticated Tracking Query with Ownership Verification

```sql
-- Fetch parcel with ownership keys and dropoff address
SELECT 
    p.id,
    p.tracking_number,
    p.current_status,
    p.sender_id,
    p.merchant_id,
    p.created_at,
    p.updated_at,
    pa.city,
    pa.postal_code,
    pa.zone_id
FROM parcels p
LEFT JOIN parcel_addresses pa 
    ON p.id = pa.parcel_id AND pa.address_type = 'DROPOFF'
WHERE p.tracking_number = $1;

-- Fetch complete milestone history including actor_role and reason_code
SELECT 
    to_status,
    reason_code,
    actor_role,
    occurred_at
FROM parcel_status_history
WHERE parcel_id = $1
ORDER BY occurred_at ASC;
```

---

## 6. Connection Pool & Statement Timeout Budget (Agreement D8)

To preserve database capacity across all 10 microservices, Member 9 ratified connection budget **Agreement D8**:

| Configuration Parameter | Value | Rationale |
|---|---|---|
| `DATABASE_URL` | `postgres://swiftdrop_tracking_ro:****@postgres.swiftdrop.svc:5432/swiftdrop_db?sslmode=require` | Secure service connection string. |
| `MaxOpenConns` | **5** | Hard cap per pod. Across 2–4 pods, max total pool is 10–20 connections. |
| `MaxIdleConns` | **2** | Keeps connection churn low. |
| `ConnMaxLifetime` | **15 minutes** | Prevents stale connections across pod recycles. |
| `ConnMaxIdleTime` | **5 minutes** | Releases idle connections during off-peak hours. |
| `statement_timeout` | **3000ms** (`3s`) | Prevents slow unindexed queries from hanging the pool. |

---

## 7. Index Verification

The Tracking API relies on two critical indexes maintained in Member 2's migration scripts:

1. **`idx_parcels_tracking_number`:** `CREATE UNIQUE INDEX idx_parcels_tracking_number ON parcels(tracking_number);`
   - Guarantees $O(1)$ B-tree index lookup for public and authenticated requests.
2. **`idx_parcel_status_history_parcel_id`:** `CREATE INDEX idx_parcel_status_history_parcel_id ON parcel_status_history(parcel_id, occurred_at ASC);`
   - Enables fast index scans for milestone timeline generation without disk sorting.

---

## 8. Ratified Persistence Decisions Table

| Persistence Item | Ratified Resolution | Reference |
|---|---|---|
| **Database Role** | `swiftdrop_tracking_ro` (dedicated read-only user) | Agreement D6 |
| **Max Open Connections** | **5** connections per pod | Agreement D8 |
| **Statement Timeout** | **3000ms** | Service configuration |
| **Destination City Source** | Joined from `parcel_addresses` where `address_type = 'DROPOFF'` | Member 2 schema |
| **History Reason Column** | `reason_code` (VARCHAR); internal `note` (TEXT) is strictly redacted | SEC-009 / Audit |
| **Zone ID Exposure** | Excluded from public DTO; city only is exposed | SEC-009 / Audit |
| **Migrations Ownership** | Zero migrations owned by M5; managed centrally by M2/M9 | Agreement D3 |

---
*End of Document 5 — Data & Persistence Specification (v1.0.0)*