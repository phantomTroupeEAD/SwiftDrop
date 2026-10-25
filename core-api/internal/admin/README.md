# Admin & Operations Module (`internal/admin`)

> **Domain:** Geographic Delivery Zones, System Configuration, Operational Reports & Audit Oversight  
> **Primary Owners:** Member 7 (Admin Frontend & Gateway), Member 1 (Security & Identity), Member 4 (Merchant Approval)  
> **Clean Architecture Boundary:** Modular Monolith Internal Module | **Port:** Core API `:8080`

---

## 1. Module Overview & Responsibilities
The `admin` module handles system-wide backoffice operations and configuration that transcend individual domain contexts.

### Key Responsibilities:
- **Geographic Delivery Zone Administration:** Create, update, and enable/disable zones, postal code prefix mappings, GeoJSON boundary polygons, and base surcharges.
- **Runtime System Settings:** Key-value configuration store for operational runtime toggles (e.g., auto-assignment switches, attempt thresholds).
- **System-Wide Operational Reports:** Aggregate delivery success rates, first-attempt rates, failed attempts, and SLA compliance metrics across zones and date intervals.
- **Cross-Domain Audit Trail Logging & Queries:** Append-only immutable recording and inspection of administrative, operational, and security state modifications.
- **Merchant Account Approval Workflows:** Backoffice coordination with `internal/merchants` for merchant application review.

---

## 2. Domain Invariants & Business Rules
1. **Strict Role Guard (SEC-002):** All administrative endpoints require the `ADMIN` role (with read-only report inspection permitted for `DISPATCHER`). No customer, merchant, or courier may access administrative queries or configurations.
2. **Postal Code Prefix Non-Overlap:** One postal code prefix may NOT be assigned to multiple active operational delivery zones. Enforced in application use case via PostgreSQL advisory locking.
3. **Immutable Audit Logs (SEC-009):** Audit logs and historical transition records can be queried but CANNOT be updated or deleted by any administrative user or runtime process. Enforced via database triggers.
4. **Monetary Precision:** All surcharges and financial metrics are stored and computed in integer minor units (`base_surcharge_cents BIGINT`). Zero floating-point representation.
5. **Optimistic Concurrency Control:** Updates to delivery zones must supply the current `version` integer to prevent lost updates during concurrent administrative edits.

---

## 3. Clean Layer Organization
```
internal/admin/
├── domain/                  # Zone aggregate root, ZoneCode, PostalPrefix value objects, domain errors
├── application/             # ManageZones, UpdateSettings, GenerateReport, AuditLogger use cases & ports
├── infrastructure/          # PostgresAdminRepository (pgx with advisory locks), PostgresReportingQuery
└── transport/               # HTTP handlers (/api/v1/admin/*) and JSON DTO mapping
```

---

## 4. Database Ownership & Migrations
This module owns the following PostgreSQL tables:
- `delivery_zones` (Migration `000002_create_delivery_zones.up.sql`): `id VARCHAR(32) PRIMARY KEY`, `code VARCHAR(32) UNIQUE`, `name`, `base_surcharge_cents`, `currency`, `enabled`, `version`, `boundary_polygon`.
- `zone_postal_codes` (Migration `000002_create_delivery_zones.up.sql`): `prefix VARCHAR(10) PRIMARY KEY`, `zone_id VARCHAR(32) REFERENCES delivery_zones(id) ON DELETE CASCADE`.
- `system_settings` (Migration `000002_create_delivery_zones.up.sql`): `key VARCHAR(64) PRIMARY KEY`, `value JSONB`, `value_type`, `description`, `updated_by`.
- `audit_logs` (Migration `000011_create_audit_logs.up.sql`): `id UUID PRIMARY KEY`, `actor_id UUID`, `action`, `target_entity`, `target_id`, `changes JSONB`, `ip_address`, `occurred_at`. Protected by append-only trigger.

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Realigned to v2.0.0: standardized VARCHAR(32) zone IDs, integer cents surcharges, complete API contracts, and advisory-locked domain model | Post-Submission Alignment |
