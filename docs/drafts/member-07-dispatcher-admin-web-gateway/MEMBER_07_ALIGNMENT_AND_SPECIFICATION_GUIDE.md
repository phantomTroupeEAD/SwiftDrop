# Member 07 — Dispatcher/Admin UI & API Gateway Alignment Guide

> **Author / Domain Owner:** Member 7 — Dispatcher/Admin UI, Courier Web & Traefik Gateway (Bagya)  
> **Target Modules:** `web/src/features/dispatcher-admin/`, `core-api/internal/admin/`, `deploy/gateway/traefik/`  
> **Status:** Authoritative Architectural Alignment & Action Sheet for Team Meeting 2  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Domain Overview & Scope of Work

Member 7 straddles frontend operations, backend administrative capabilities, and edge ingress routing:
1. **Dispatcher & Admin Web Portal (`/ops/*`):** Live dispatch board, fleet capacity monitoring, delivery zone configuration, and system reports.
2. **Mobile Courier Web Application (`/courier/*`):** Mobile-first task list, barcode scan pickup, and proof-of-delivery (photo/signature) capture.
3. **Admin Backend Module (`core-api/internal/admin/`):** Delivery zones management and audit log recording.
4. **Traefik API Gateway (`deploy/gateway/traefik/`):** Edge ingress routing, TLS termination, path rewrites, CORS, and rate limiting.

### 1.1 In-Scope Responsibilities
- Interactive Dispatch Board (`/ops/dispatch`): Real-time view of unassigned parcels and active courier loads.
- Dispatcher reattempt authorization for `ATTEMPT_FAILED` parcels.
- Admin Delivery Zone CRUD endpoints (`/api/v1/admin/zones`).
- Admin Operational Reports (`/api/v1/admin/reports/operational`).
- Edge Traefik routing configuration with path-based routing.
- Rate limiting middleware at the gateway edge.
- Authoring `000002_create_delivery_zones.up.sql` and `000011_create_audit_logs.up.sql`.

### 1.2 Out-of-Scope Responsibilities
- Customer booking wizard and public tracker screens (Owned by Member 6).
- Courier shift state machine backend logic (Owned by Member 3).
- Kubernetes ingress controller infrastructure installation (Owned by Member 9).

---

## 2. Identified Flaws & Necessary Changes

1. **Missing Document 4 (API Contracts):** Member 7 omitted `doc-04-interface-and-integration-contract.md`. Section 3 below formalizes the official contracts.
2. **Gateway Port Conflict (Member 9 K5):** Member 7 proposed Traefik listening on port 8080.
   - *Fix:* Traefik binds to `:80` and `:443` externally. The Traefik dashboard binds to internal port `:8090`. Core API binds to `:8080`.
3. **Database Roles and Secrets (Member 9 K6):** In `doc-05`, Member 7 used `postgres-credentials` and role `swiftdrop_owner`.
   - *Fix:* Use secret `postgres-secrets` and runtime role `swiftdrop_app`.
4. **SSL Mode (Member 9 K7):** In `doc-08`, Member 7 configured `DATABASE_URL` with `sslmode=require`.
   - *Fix:* In-cluster Kubernetes connections use `sslmode=disable` (Agreement D11).
5. **Separate Compose File (Member 9 K10):** Delete `docker-compose.m7.yml` and merge all gateway requirements into `deploy/docker-compose.yml`.

---

## 3. Formal Document 4 Specification (Missing REST Contracts)

Member 7 must adopt the following REST contracts for `core-api/internal/admin/`:

### 3.1 Create Delivery Zone
- **Method / Path:** `POST /api/v1/admin/zones`
- **Auth Required:** Bearer JWT with role `ADMIN`
- **Request Body:**
  ```json
  {
    "id": "ZONE_COLOMBO_01",
    "name": "Colombo Central Commercial",
    "code": "CMB-01",
    "boundary_polygon": {
      "type": "Polygon",
      "coordinates": [[[79.84, 6.92], [79.86, 6.92], [79.86, 6.94], [79.84, 6.94], [79.84, 6.92]]]
    }
  }
  ```
- **Response (`201 Created`):** Returns zone object.

### 3.2 List Delivery Zones
- **Method / Path:** `GET /api/v1/admin/zones`
- **Auth Required:** Authenticated (`ADMIN`, `DISPATCHER`) or internal modules.
- **Response (`200 OK`):**
  ```json
  {
    "data": [
      {
        "id": "ZONE_COLOMBO_01",
        "name": "Colombo Central Commercial",
        "code": "CMB-01",
        "enabled": true
      }
    ]
  }
  ```

### 3.3 Dispatcher Unassigned Parcels Board Query
- **Method / Path:** `GET /api/v1/dispatch/board`
- **Auth Required:** Role `DISPATCHER` or `ADMIN`
- **Response (`200 OK`):**
  ```json
  {
    "data": {
      "unassigned_parcels": [
        {
          "id": "prcl_01JABC9988",
          "tracking_number": "SD-20261025-00123",
          "origin_zone_id": "ZONE_COLOMBO_01",
          "destination_zone_id": "ZONE_COLOMBO_02",
          "weight_kg": 2.50,
          "created_at": "2026-10-25T08:00:00Z"
        }
      ],
      "active_couriers": [
        {
          "courier_id": "usr_courier_bob",
          "employee_code": "CR-0042",
          "vehicle_type": "MOTORCYCLE",
          "current_zone_id": "ZONE_COLOMBO_01",
          "active_assignments": 3,
          "max_capacity": 25
        }
      ]
    }
  }
  ```

---

## 4. Database Migrations (Migrations 2 and 11)

### 4.1 Migration `000002_create_delivery_zones.up.sql`
*Runs 2nd in sequence, immediately after `users` and before `merchants`, `pricing_rules`, and `couriers`.*

```sql
CREATE TABLE delivery_zones (
    id VARCHAR(32) PRIMARY KEY, -- e.g. 'ZONE_COLOMBO_01'
    name VARCHAR(120) NOT NULL,
    code VARCHAR(32) NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    boundary_polygon JSONB, -- GeoJSON polygon geometry
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delivery_zones_code ON delivery_zones(code);
```

### 4.2 Migration `000011_create_audit_logs.up.sql`
```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID REFERENCES users(id),
    action VARCHAR(64) NOT NULL, -- 'ZONE_CREATED', 'MERCHANT_APPROVED', 'PARCEL_OVERRIDE'
    target_entity VARCHAR(64) NOT NULL,
    target_id VARCHAR(128) NOT NULL,
    changes JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_id, occurred_at DESC);
CREATE INDEX idx_audit_logs_target ON audit_logs(target_entity, target_id);
```

---

## 5. Member 9 DevOps & Gateway Configuration

| Config Parameter | Required Specification | Alignment Rule |
| :--- | :--- | :--- |
| **Gateway Ports** | External: `80` (HTTP), `443` (HTTPS); Dashboard: `8090` | Agreement A1 & A9 |
| **Routing Topology** | Single domain with path-based routing (Traefik) | Agreement A9 |
| **Rate Limiting** | Edge rate limiting: Public tracking (10 req/s, burst 50), Auth (5 req/s) | Agreement A9 |
| **CORS Origins** | Allowed: `https://swiftdrop.example.com` (local `http://localhost:3000`) | Agreement A9 |
| **Database Role** | Runtime: `swiftdrop_app`; Migrations: `swiftdrop_migrator` | Agreement D6 & K6 |
| **Migrations** | `000002_create_delivery_zones.up.sql` and `000011_create_audit_logs.up.sql` | Agreement D3 (2nd & 11th) |

---
*End of Member 07 Specification & Alignment Guide.*
