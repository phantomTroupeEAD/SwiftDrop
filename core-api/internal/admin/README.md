# Admin & Operations Module (`internal/admin`)

> **Domain:** Delivery zones, postal code prefixes, runtime system settings, audit trail
> **Owner:** Member 7 (Dispatcher/Admin Frontend & API Gateway). `audit_logs` is shared with Member 1.
> **Boundary:** Core API modular monolith module | **API contract:** [API.md](API.md)

---

## 1. Purpose

The admin module holds the back-office configuration that other modules read:

- **Delivery zones:** create, edit, enable and disable zones and their postal code prefixes.
- **Public zone lookup:** list enabled zones and find the zone for a postal code, for booking forms.
- **System settings:** a small allowlist of runtime values (for example the dispatch board refresh interval).
- **Audit trail:** every admin change writes one append-only row to `audit_logs`, and admins can read the latest rows.

Zones carry **no money**. Fees, including any per-zone charge, belong to Member 2's pricing rules.

---

## 2. Business invariants

1. **Admin only.** Every `/api/v1/admin/*` route needs a valid JWT with role `ADMIN`. A missing or invalid token is `401`; any other role is `403`. Only `GET /api/v1/zones` is public, and it shows `id`, `code` and `name` only.
2. **No overlapping prefixes.** A prefix (3 to 5 digits) belongs to one zone. A prefix that starts another zone's prefix, or is started by it (`001` and `00100`), is refused with `409 ZONE_POSTAL_CODE_CONFLICT`. The check runs inside the write transaction under a PostgreSQL advisory lock (`pg_advisory_xact_lock(7700001)`), so concurrent writers cannot both pass it. The `zone_postal_codes` primary key is the last safety net.
3. **Optimistic concurrency.** Zone and setting writes must send the current `version`. A stale version is `409 CONCURRENT_MODIFICATION`. One `PATCH` raises the version by exactly one, however many fields it changes. A `PATCH` that changes nothing writes nothing and adds no audit row.
4. **Change and audit commit together.** The zone or setting write and its `audit_logs` row share one transaction. If the audit insert fails, the change is rolled back.
5. **Audit rows are append-only.** A trigger refuses every `UPDATE` and `DELETE` on `audit_logs` (SRS SEC-015). Audit snapshots are redacted: values under keys that look like secrets (`password`, `secret`, `token`, `api_key`, `authorization`, `cookie`, `signature`) become `[REDACTED]`.
6. **Settings allowlist.** Only these keys can be changed, each with its own type and range:

   | Key | Type | Allowed |
   | :-- | :-- | :-- |
   | `reports.max_range_days` | integer | 1 to 366 |
   | `ops.board_refresh_seconds` | integer | 10 to 300 |
   | `ops.maintenance_banner` | string | at most 200 characters |

   Any other key is `422 SETTING_UNKNOWN`, so secrets or arbitrary keys cannot be stored this way. A value is at most 4096 bytes.
7. **A reason is required** (10 to 500 characters) to enable or disable a zone and to change a setting. It is stored in `audit_logs.reason`.
8. **Zone ids never change.** `id` (for example `ZONE_COLOMBO_01`) is the key other modules reference. `code` (for example `CMB-01`) can be changed.

---

## 3. Domain model

- **`Zone`** (aggregate root, `domain/zone.go`): id, code, name, description, prefixes, enabled, version. It owns its prefixes and records events (`ZONE_CREATED`, `ZONE_CHANGED`, `ZONE_STATUS_CHANGED`).
- **Value objects** (`domain/values.go`): `ZoneID` (`^ZONE_[A-Z0-9_]{2,27}$`), `ZoneCode` (`^[A-Z0-9_-]{2,32}$`), `PostalPrefix` (`^[0-9]{3,5}$`).
- **Domain service** (`domain/prefix_policy.go`): `PrefixOverlapPolicy` compares a zone's prefixes with other zones' prefixes.
- **Domain errors** (`domain/errors.go`): invalid id, code, name, description or prefix; no prefixes; duplicate prefix in one zone; already enabled or disabled.

---

## 4. Layers

```
internal/admin/
├── domain/          Zone aggregate, value objects, overlap policy. Standard library only.
├── application/     Use cases (CreateZone, UpdateZone, SetZoneEnabled, UpdateSetting),
│                    settings allowlist, error kinds, Store/Tx ports.
├── infrastructure/  PostgreSQL Store (database/sql + lib/pq): advisory lock, FOR UPDATE loads,
│                    version-checked updates, audit insert, audit redaction, error-code mapping.
└── transport/       HTTP handlers, error-to-HTTP mapping, Auth port.
```

- **Auth:** `transport.Auth` is a port. `cmd/api/wiring_admin.go` adapts Member 1's `SecurityMiddleware` to it, so this module does not import `internal/identity`.
- **Errors:** handlers map application error kinds to the standard envelope (`platform/httpx.WriteError`). Unknown errors become a generic `500`; the raw error goes to the server log only, never to the client.
- **Wiring:** `registerAdminModule` in `cmd/api/wiring_admin.go`. The module needs PostgreSQL; without a database it is not registered (fatal in production, a notice locally).

---

## 5. Events

This module writes **no outbox events** and consumes none. Zone and setting changes are recorded in `audit_logs` only. If another module later needs to react to zone changes, add a domain event through the outbox in the same transaction.

---

## 6. Data ownership

| Table | Migration | Notes |
| :-- | :-- | :-- |
| `delivery_zones` | `20261109000100_admin_create_delivery_zones` | `id VARCHAR(32)` PK, `code` unique, `enabled`, `version`, `boundary_polygon JSONB` (not used by the API yet) |
| `zone_postal_codes` | `20261109000100_admin_create_delivery_zones` | `prefix` PK, `zone_id` FK `ON DELETE CASCADE` |
| `system_settings` | `20261109000100_admin_create_delivery_zones` | `key` PK, `value JSONB`, `value_type`, `version`, `updated_by` FK `users(id)`. Seeds the 3 allowlisted keys |
| `audit_logs` | `20261109000200_admin_create_audit_logs` | Shared with Member 1. Append-only trigger; `UPDATE`/`DELETE` revoked from `swiftdrop_app` when that role exists |

Other modules may write `audit_logs` rows in their own transactions. The columns and the `actor_role` CHECK (`ADMIN`, `DISPATCHER`, `COURIER`, `MERCHANT`, `CUSTOMER`, `SYSTEM`) are the contract.

---

## 7. Tests

| Layer | File | Covers |
| :-- | :-- | :-- |
| Domain | `domain/*_test.go` | value object rules, zone changes and versioning, overlap policy |
| Application | `application/zones_test.go`, `settings_rules_test.go` | lock taken, audit row per change, rollback when the audit write fails, version and reason rules, settings allowlist |
| Infrastructure | `infrastructure/redact_test.go` | secrets never reach audit snapshots |
| Infrastructure (integration) | `infrastructure/postgres_integration_test.go` | 20-way overlapping prefix race (1 wins), 10-way stale version race (1 wins), audit rollback, append-only trigger, settings round trip, public lookup |
| Transport | `transport/http_test.go` | 401/403 on every admin route, public fields only, error mapping, no leak of SQL text, input limits |
| Wiring | `cmd/api/wiring_admin_test.go` | adapter keeps the 401/403 contract with real identity JWTs |

Integration tests run only with `INTEGRATION=1`:

```bash
INTEGRATION=1 TEST_DATABASE_URL="postgres://USER:PASSWORD@localhost:5432/swiftdrop_test?sslmode=disable" \
  go test -v -count=1 ./internal/admin/infrastructure/...
```

They apply only the migrations this module needs (identity's `users` table and the two admin migrations, recorded in `schema_migrations`), so a broken migration in another module cannot hide an admin failure. They create zones with ids starting `ZONE_T_` and delete those zones afterwards. Audit rows they write stay, because `audit_logs` is append-only. Use a test database.

---

## 8. Status

| Area | Status |
| :-- | :-- |
| Zones, postal prefixes, public lookup | Done |
| System settings (allowlist) | Done |
| Audit trail (write and read latest) | Done. No filters or paging beyond `limit` yet |
| Delivery report (`GET /api/v1/admin/reports/deliveries`, SRS) | Not started. Needs read interfaces from Members 2 and 3 |
| Merchant approval routes | Not in this module. Needs Member 4's admin facade |
| Metrics (`admin_changes_total`, `admin_conflicts_total`) | Not started. Core API has no metrics package yet |
| Demo zone seed data | Not included. Admins create zones through the API |

### Known limitation in shared code

`platform/httpx.WriteError` reads the correlation id only from the `X-Request-ID` request header, but `platform/middleware.RequestID` stores a generated id in the context under its own key type. Without a client-sent header, error envelopes from other modules have an empty `correlation_id`. This module works around it in `transport.withRequestIDHeader`; the platform owner should fix `httpx` to use `middleware.CorrelationID`.

---

## 9. Changelog

| Date | Change | Author |
| :-- | :-- | :-- |
| 2026-10-21 | Module baseline created | Architecture setup |
| 2026-10-25 | Contract draft (surcharges, `/system-settings`) | Post-submission alignment |
| 2026-11-09 | Implemented zones, postal prefixes, settings and audit trail, ported from the Member 7 draft (`docs/drafts/member-07-dispatcher-admin-web-gateway/doc8-local`) onto `database/sql` and the Core API platform. Removed surcharges (pricing owns fees) and the unimplemented report contract | Member 7 |
