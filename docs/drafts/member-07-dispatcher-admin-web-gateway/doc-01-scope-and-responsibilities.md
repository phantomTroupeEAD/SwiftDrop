# Document 1 — Module Scope & Responsibility Specification

> **Status:** Draft / Proposed (For Review in Meeting 2)  
> **Author:** Member 7 — Dispatcher/Admin UI, Courier Web, Traefik Gateway  
> **Target Module / Location:** `web/src/features/dispatcher-admin/` (ops + courier screens), `core-api/internal/admin/` (admin module), `deploy/gateway/traefik/` (gateway)  
> **Key Upstream Dependencies:** Member 1 (Auth, RBAC, JWT), Member 2 (Parcels, Pricing), Member 3 (Dispatch, Delivery), Member 4 (Merchants), Member 5 (Tracking SSE), Member 8 (Outbox, events), Member 9 (Compose, Kubernetes)  
> **Open Questions / Needs Clarification:** Section 11.3. The 26 Oct review closed U-02, U-03, U-05, U-07, U-09 to U-13 and U-15 to U-17 (decisions R-01 to R-21 in section 11.4). Still open: U-01 courier token storage, U-04 fee endpoint name, U-06 per-user limits, U-08 parcel correction owner, U-14 pricing overlap policy, plus the new U-63 to U-66

**Module:** Dispatcher/Admin/Courier Frontend, Admin Backend Module & API Gateway (Member 7) **Project:** SwiftDrop Parcel Delivery & Courier Management System **Based on:** SRS v1.0 (SWD-SRS-001), Engineering Standards v1.0, Team Work Division v1.0, and the team repository (README, AGENTS.md, ADR-001, ADR-002, ADR-003, ADR-004, domain-boundaries.md, web/README.md, security-notes.md, deploy/gateway, module API.md files) **Status:** Draft v4 (applies the 26 October docs audit: Member 3 and Member 4 contracts, the alignment guide, Member 9's team agreements, SRS requirement IDs). Items marked **\[TBD\]** wait for a teammate or a team decision. **Date:** 26 October 2026

> **What changed in v4.** Dispatch, courier and delivery calls now use Member 3's endpoints; merchant admin calls use Member 4's `/admin/merchants/**` routes, and merchant list and detail screens were added; zones have a text id (`ZONE_COLOMBO_01`) and no surcharge (fees live only in Member 2's pricing rules); a public zone list endpoint was added for Member 6; reports use the SRS route `/admin/reports/deliveries` and dispatchers may read them; gateway limits and CORS follow security-notes.md; the client never sends `X-Tenant-ID`; the full `/ops/*` route list is proposed for `web/README.md`. All review decisions are in section 11.4.

## 1. Purpose and business problem

SwiftDrop replaces phone calls and spreadsheets for booking, assigning and tracking parcels. Three groups of internal users need screens to do their daily work:

- **Dispatchers** decide which courier delivers which parcel, watch courier workload, and decide what happens after a failed delivery.
- **Administrators** approve businesses (merchants), manage users, set delivery zones and prices, and read reports.
- **Couriers** see their tasks on a phone, record proof of delivery, and report failed attempts.

All web traffic (customers, merchants, couriers, staff, public tracking) enters the system through one front door: the **Traefik gateway**.

This module provides four things:

1. The **dispatcher/admin web screens** (`/ops/*`).
2. The **courier mobile web screens** (`/courier/*`).
3. The **admin backend module** (`core-api/internal/admin/`), shared with Members 1 and 4.
4. The **gateway configuration**: rules that send each request to the right service and apply the agreed limits.

The screens and the gateway display and route. They do not decide business rules and do not store business data. The admin backend module stores only its own tables (section 8).

## 2. Functional responsibilities

### 2.1 Dispatcher/admin screens

| ID | Responsibility | Related requirement |
| --- | --- | --- |
| M7-UI-01 | Dispatch board for one zone at a time (the zone picker is required, because Member 3's board needs `zone_id`): unassigned and failed parcels with courier load | FR-ADM-001, FR-ADM-004 |
| M7-UI-02 | Assign a parcel to an on-shift courier (the new assignment has status `ASSIGNED`) | FR-ADM-002 |
| M7-UI-03 | Reassign: cancel the current assignment with a reason (`POST /dispatch/assignments/{id}/cancel`), then assign the new courier. There is no separate reassign endpoint (R-01) | FR-ADM-002, FR-ADM-003 |
| M7-UI-04 | Show courier workload (active assignments against maximum capacity) | FR-ADM-004 |
| M7-UI-05 | Show the real backend error on assignment failure (for example COURIER\_CAPACITY\_EXCEEDED, 409) and refresh the data | FR-ADM-003 |
| M7-UI-06 | Failed-delivery review at `/ops/failed-deliveries`: show the attempt count, reason code and attempt history (`GET /deliveries/parcels/{id}`), and let a dispatcher schedule a reattempt (`POST /dispatch/parcels/{id}/reschedule`, ATTEMPT\_FAILED to ASSIGNED, same or new courier). There is no dispatcher return button: Member 3 moves the parcel to RETURNED automatically after the 3rd failure or a refusal (R-01) | FR-ADM-001, FR-LIFE-002, ADR-004 |
| M7-UI-07 | Fleet screen at `/ops/couriers`: courier list, shift status, vehicle capacity, add a courier profile (Member 3's `/dispatch/couriers`) | FR-ADM-004 |
| M7-UI-08 | Merchant list (`/ops/merchants`, filter by status, search) and merchant detail (`/ops/merchants/:id`) with approve, reject, suspend, reinstate and disable. Every action sends `expected_version`; reject, suspend and disable also need a reason of 5 to 500 characters (R-02) | FR-ADM-005, FR-IAM-009 |
| M7-UI-09 | User and role management screen | FR-ADM-006 |
| M7-UI-10 | Delivery zone management screen | FR-ADM-007 |
| M7-UI-11 | Pricing rule management screen at `/ops/pricing` (versioned, effective dates). The screen only edits rules; Member 2 validates them and calculates fees **\[TBD: Member 2 must add the `/admin/pricing-rules` endpoints; the audit found none\]** | FR-ADM-007 |
| M7-UI-12 | Reports screen at `/ops/reports` (deliveries per day, success and failure rate over a period) for ADMIN and DISPATCHER; audit log explorer at `/ops/audit` for ADMIN only | FR-ADM-008, SEC-015 |
| M7-UI-13 | Admin parcel correction screen, reason required, audited **\[TBD: scope and endpoint, U-08\]** | FR-ADM-009, FR-LIFE-007 |
| M7-UI-14 | Login-protected area with role-based navigation (usability only) | Standards 4.3 |
| M7-UI-15 | Show loading, empty, success, validation error, 401, 403, 409 and 500 states on every screen | Standards 4.3 |
| M7-UI-16 | Accessible, responsive layout; status shown with text as well as colour; all parcel statuses shown (CREATED, ASSIGNED, PICKED\_UP, IN\_TRANSIT, OUT\_FOR\_DELIVERY, DELIVERED, ATTEMPT\_FAILED, RETURNED, CANCELLED) | SRS 9.4 |

**Screen routes (proposed for the route table in `web/README.md`, which today lists only four of them).** Member 6 shares the app shell and must agree before the table is changed.

| Route | Roles | Screen |
| --- | --- | --- |
| `/ops/dispatch` | DISPATCHER, ADMIN | Dispatch board for one zone |
| `/ops/couriers` | DISPATCHER, ADMIN | Fleet: couriers, shifts, capacity |
| `/ops/failed-deliveries` | DISPATCHER, ADMIN | Failed attempts and reattempt scheduling |
| `/ops/merchants`, `/ops/merchants/:id` | ADMIN | Merchant list, detail and status actions |
| `/ops/users` | ADMIN | Users and roles (needs Member 1's endpoints) |
| `/ops/zones` | ADMIN | Delivery zones and postal prefixes |
| `/ops/pricing` | ADMIN | Pricing rules (Member 2's endpoints) |
| `/ops/settings` | ADMIN | System settings |
| `/ops/reports` | ADMIN, DISPATCHER | Delivery reports (SRS: admin or authorized dispatcher) |
| `/ops/audit` | ADMIN | Audit log explorer (read only) |
| `/courier/tasks`, `/courier/tasks/:id`, `/courier/tasks/:id/pod`, `/courier/tasks/:id/fail` | COURIER | Courier task list, task detail, proof of delivery, failed attempt |

The current `web/README.md` route table also says `/ops/reports` is ADMIN only and describes `/ops/zones` as setting "base tariffs". Both lines need updating to match R-04 and R-16.

### 2.2 Courier mobile web screens

| ID | Responsibility | Related requirement |
| --- | --- | --- |
| M7-CR-01 | Courier task list at `/courier/tasks` from `GET /api/v1/courier/parcels` (scoped by the server to the signed-in courier), shift toggle (`POST /courier/shift`), start the run for one parcel (`POST /courier/parcels/{id}/start-run`) or for many (`POST /courier/runs/start-batch`) | FR-COU-001, FR-COU-002, FR-COU-003 |
| M7-CR-02 | Proof of delivery at `/courier/tasks/:id/pod`: optional photo upload first (`POST /api/v1/deliveries/{id}/proof-photo`, multipart, 10 MB), then `POST /api/v1/deliveries/{id}/complete` with `proof_type`, `signature_data`, `received_by_name`, `notes` and an `Idempotency-Key` | FR-COU-004, FR-COU-007 |
| M7-CR-03 | Failed attempt at `/courier/tasks/:id/fail` with Member 3's reason codes (RECIPIENT\_UNAVAILABLE, CUSTOMER\_REQUESTED\_RESCHEDULE, ACCESS\_RESTRICTED, INVALID\_ADDRESS, REFUSED\_BY\_RECIPIENT, DAMAGED\_IN\_TRANSIT, OTHER with a note of at least 15 characters), sent to `POST /api/v1/deliveries/{id}/failed` as `reason_code`, `notes`, `latitude`, `longitude`. The screen shows the answer: still ATTEMPT\_FAILED, or RETURNED after the 3rd attempt or a refusal | FR-COU-005, FR-LIFE-006, ADR-004 |
| M7-CR-04 | Work on slow phone connections; keep uploads within the 10 MB gateway limit | FR-COU-007, SEC-012 |

### 2.3 Admin backend module (`core-api/internal/admin/`, shared with Members 1 and 4)

| ID | Responsibility | Notes |
| --- | --- | --- |
| M7-AD-01 | Delivery zone administration: create and update zones (text id such as `ZONE_COLOMBO_01`, short code, name, optional description and boundary), postal code prefixes, enable and disable. **No money in zones**: fees and any zone surcharge are pricing rules owned by Member 2 (R-03, R-04) | `/api/v1/admin/zones/**` |
| M7-AD-02 | Public zone list for booking forms: enabled zones only (`id`, `code`, `name`), optional lookup by postal code (R-18) | `GET /api/v1/zones` (no sign-in) |
| M7-AD-03 | Delivery reports: deliveries per day and success/failure rate over a period, for ADMIN and DISPATCHER (R-16) | `GET /api/v1/admin/reports/deliveries` (SRS 6.2) |
| M7-AD-04 | Audit log queries (read only, never edit or delete) | `GET /api/v1/admin/audit-logs` |
| M7-AD-05 | Merchant admin routes: I host the HTTP routes and role check; Member 4's `merchants.AdminService` does the work and owns the rules (R-02) | `/api/v1/admin/merchants/**` |
| M7-AD-06 | System settings (allowlisted keys only) | `/api/v1/admin/settings/**` |
| M7-AD-07 | Keep this module's `README.md` and `API.md` updated in the same commit as any change | AGENTS.md |

The module's own rules: every `/admin/*` endpoint requires ADMIN, except the delivery report, which also allows DISPATCHER; audit logs are immutable; one postal prefix may not overlap a prefix of another zone. I write all of the admin module's Go code; Member 1 supplies the token middleware and Member 4 the `AdminService` interface (U-10 closed).

### 2.4 API Gateway (`deploy/gateway/traefik/`)

| ID | Responsibility | Status in repo |
| --- | --- | --- |
| M7-GW-01 | Route each public path to the correct service | Done: `/api/v1/tracking` to tracking, `/api/v1/ai` to AI assistant, `/api/v1` to core API, `/` to the frontend |
| M7-GW-02 | Rate limits: login, public tracking, merchant API, AI (SEC-007) | The repo file has only a tracking limit (30 per minute, burst 50) and **no login limiter**. My proposed config (Doc 7) uses the security-notes.md numbers per client IP: login 5/min burst 3, public tracking 30/min burst 10, merchant API 120/min burst 20, AI 10/min burst 2, plus 5 open live-update streams per IP. Per-user limits (courier 60/min, AI 10/min per user, merchant per API key) cannot be done at the gateway, which does not read tokens; the backends enforce them (U-06) |
| M7-GW-03 | Request size limit | Done: 10 MB on the core API route |
| M7-GW-04 | CORS allowlist (SEC-013) | Follows security-notes.md section 4 exactly: origins `https://swiftdrop.io`, `https://app.swiftdrop.io`, `https://portal.swiftdrop.io`, `http://localhost:3000`, `http://127.0.0.1:3000`; headers `Authorization, Content-Type, X-API-Key, Idempotency-Key, X-Request-ID`; exposed `X-Request-ID, Content-Length`; credentials allowed; max age 86400. `X-Tenant-ID` is **not** allowed. The repo file still differs (it allows X-Tenant-ID, not X-Request-ID, max age 3600, localhost only) and is replaced by my proposed file. With one domain and paths (A9) the web app itself is same-origin |
| M7-GW-05 | Support live updates (SSE) through the gateway | Decided (R-12): public stream `/api/v1/tracking/public/{trackingNumber}/stream` and signed-in stream `/api/v1/tracking/{trackingNumber}/stream`, response buffering off, long-lived connections allowed, heartbeat every 15 seconds. The AI service's MCP stream `/api/v1/ai/mcp/sse` gets the same no-buffering rule. Not load-tested yet |
| M7-GW-06 | Keep internal paths (health, metrics, database, broker, workers) unreachable from outside (SEC-017) | `/health/live`, `/health/ready` and `/metrics` are never routed (A2). Traefik listens on 80 and 443; its dashboard is on port 8090, internal only, with `insecure: false`; core-api keeps 8080 (R-10). The docker example in `deploy/gateway/README.md` (dashboard on 8080) must be updated. Tested by GS-12 in Doc 7 |
| M7-GW-07 | Pass on or create the request ID | Header `X-Request-ID` (team agreement A10, security-notes.md 4, SRS SEC-015). The alignment guide's `X-Correlation-ID` is **not** used, because A10 and the shared docs already chose `X-Request-ID`; every service logs it as `request_id` and the outbox copies it to `correlation_id` (R-11) |
| M7-GW-08 | Keep all gateway files in version control, rebuildable from a clean clone | traefik.yml and dynamic-routes.yml exist for Docker. I write ingressroutes.yaml and middlewares.yaml for Kubernetes. Member 9 writes values.yaml (Helm install) and the Docker Compose file |
| M7-GW-09 | Document the split of work between Cloudflare, Traefik and the backend | See section 10 |
| M7-GW-10 | Check that 401 and 403 responses pass through unchanged | Unchecked in ADR-001 |

### 2.5 Zone and pricing administration

Zones are mine; pricing rules are Member 2's. My screens show and edit both, but only Member 2's backend validates rules and calculates fees.

| Item | Fields |
| --- | --- |
| Delivery zone (mine, R-03) | `id` (text primary key chosen by the admin, `^ZONE_[A-Z0-9_]{2,27}$`, for example `ZONE_COLOMBO_01`; never changes), `code` (short unique code, `^[A-Z0-9_-]{2,32}$`, for example `CMB-01`), `name` (1 to 120 characters), `description` (optional), `boundary_polygon` (optional GeoJSON), `enabled`, `postal_code_prefixes` (3 to 5 digits, no overlap with another zone), `version`, `created_at`, `updated_at`. **No surcharge or other money field** (R-04) |
| Pricing rule (Member 2) | id, origin\_zone\_id, destination\_zone\_id, service\_level, weight and size limits, base\_fee\_cents, per\_kg\_fee\_cents, currency, enabled, effective\_from, effective\_to, version. A zone surcharge, if the team wants one, is a pricing rule, not a zone field |

| Method and path | Purpose | Owner |
| --- | --- | --- |
| GET /api/v1/admin/zones | List zones with prefixes | Me |
| POST /api/v1/admin/zones | Create a zone | Me |
| GET, PATCH /api/v1/admin/zones/{id} | Read or edit a zone (needs `version`) | Me |
| PATCH /api/v1/admin/zones/{id}/status | Enable or disable a zone (needs `version` and `reason`) | Me |
| GET /api/v1/zones | Public list of enabled zones for booking forms | Me |
| GET, POST /api/v1/admin/pricing-rules; PATCH /api/v1/admin/pricing-rules/{id} and /{id}/status | Pricing rules | Member 2 (**requested**) |

Invalid rules show the backend's message, for example "Minimum weight must be less than maximum weight". For overlapping rules the backend returns 409 Conflict naming the existing rule, if Member 2 forbids overlaps (U-14).

## 3. Explicit exclusions (what this module must never do)

| # | This module must NEVER | Who does it instead |
| --- | --- | --- |
| X-01 | Decide who is allowed to see or do something (roles, ownership, tenant checks) | Member 1 and each backend module |
| X-02 | Decide whether a status change, assignment or approval is valid | Parcel, dispatch and merchant modules (Members 2 to 4) |
| X-03 | Calculate prices or fees in the frontend | Pricing module (Member 2) |
| X-04 | Change parcel status, or send raw database updates | Parcel aggregate in Core API |
| X-05 | Connect to PostgreSQL, RabbitMQ or Redis from the frontend or gateway. The admin module may touch only its own tables, through the Core API | Backend services |
| X-06 | Copy business data into the frontend as a second source of truth | PostgreSQL via Core API |
| X-07 | Choose arbitrary status strings or send actor identity from the client | Backend derives actor from credentials |
| X-08 | Publish or consume events | Member 8 and the domain owners |
| X-09 | Run extra proxy layers without a documented reason | Member 9 decides the deployment shape |
| X-10 | Keep secrets, API keys or tokens in browser storage or committed config | Secret management (Member 9) |
| X-11 | Build customer, merchant or public tracking screens | Member 6 |
| X-12 | Own cluster setup, Traefik installation or TLS certificates | Member 9 |
| X-13 | Import code between `features/customer-merchant/` and `features/dispatcher-admin/` (shared code goes in `src/shared/`) | Frontend rule |

## 4. Upstream and downstream dependencies

### 4.1 Upstream (what I need from others)

| From | What I need | Status |
| --- | --- | --- |
| Member 1 | User list, role change, disable account and end-sessions endpoints (FR-ADM-006, FR-IAM-007; the audit found the last two missing); token middleware; shared audit writer interface | **\[TBD\]**: endpoints not written yet |
| Member 2 | `/admin/pricing-rules` endpoints and the pricing overlap policy; parcel correction endpoint | **\[TBD\]**: endpoints not written yet |
| Member 3 | Dispatch, fleet, courier and delivery endpoints | **Agreed**: Member 3 Doc 4 (R-01) |
| Member 4 | Merchant admin service and status rules | **Agreed**: Member 4 Doc 4 and OpenAPI, `merchants.AdminService` (R-02) |
| Member 5 | SSE paths and 15-second heartbeat (R-12); rate limits for public tracking | **Agreed** (load test still to do) |
| Member 6 | Shared app shell; the `/ops/*` route table in `web/README.md`; uses my public zone list `GET /api/v1/zones` | **\[TBD\]**: route table change to agree |
| Member 8 | Confirmation that no worker or broker is exposed externally | Agreed (alignment guide 4.7) |
| Member 10 | AI route `/api/v1/ai/*` (chat and MCP stream) and the AI rate limit | **Agreed** (R-17) |
| Member 9 | Traefik v3.1, ports, paths on one domain, dashboard 8090, `X-Request-ID`, health paths, secrets and roles, one Compose file | **Agreed** (team agreements A1 to A10, D1 to D12) |

### 4.2 Downstream (who depends on me)

| Who | What they depend on |
| --- | --- |
| Dispatchers, admins, couriers | Working screens |
| Members with public APIs (1, 2, 3, 4, 5, 10) | Correct routes to their services |
| Member 9 | Gateway config that deploys cleanly into the cluster |
| Member 6 | Shared app shell, gateway route for the frontend, and the public zone list for booking forms |

## 5. Bounded context, domain ownership and authoritative data

- **Bounded context:** Admin & Operations context (`core-api/internal/admin/`, owned with Members 1 and 4), plus a presentation and edge layer in front of the other contexts.
- **Domain ownership:** the dispatcher/admin/courier screens, the admin module, and the gateway routing and policy files.
- **Authoritative data owned:** only the admin module's tables (section 8). For everything else, PostgreSQL through the Core API is the source of truth, and anything the screens hold is a temporary copy. If they differ, the backend answer wins.
- **Vocabulary:** screens use the team's agreed terms (Parcel, Assignment, Courier, Merchant, Tracking Event, Delivery Attempt) exactly as in the API contracts.

## 6. Invariants this module must protect

| ID | Rule that must always hold | How I protect it |
| --- | --- | --- |
| INV-01 | Hiding a button is never treated as security | Backend 403 is tested separately from UI visibility |
| INV-02 | The UI never invents business state, prices or valid status changes | Display what the API returns; refresh after errors |
| INV-03 | The UI never sends actor identity or an arbitrary status | Send only fields in the API contract |
| INV-04 | The gateway never makes permission decisions | Authorization stays in the backend |
| INV-05 | Internal endpoints are not reachable from the internet | Route tests confirm they are blocked or not found |
| INV-06 | Every public route goes to exactly one intended service | Route map plus smoke tests |
| INV-07 | Live-update connections are not buffered or cut early | Test of the SSE route through the gateway |
| INV-08 | No secrets are committed or placed in browser code | Placeholder-only example config; secret scan before merge |
| INV-09 | The gateway setup can be rebuilt from version control alone | Clean-clone test |
| INV-10 | No tokens or personal data in logs or on-screen errors | Show safe messages only |
| INV-11 | Access tokens stay in memory only; never in localStorage or sessionStorage | API client uses the in-memory token and the refresh cookie |
| INV-12 | Audit logs are never edited or deleted | Admin module offers read-only audit queries |
| INV-13 | Money is shown from backend integer cents in LKR, never calculated in the browser | Use the shared formatting helper |

## 7. What other modules may request from this module

| Request | From | How |
| --- | --- | --- |
| Add, change or remove a route | Members 1, 4, 5, 8, 9, 10 | Written route-change request (path, service, access level, limits); I update the route map and config and notify them |
| A special gateway setting for a service | Member 5 and others | Same request |
| A new rate limit or size limit | Any owner | Same request with a proposed number |
| A new admin, dispatcher or courier screen or field | Members 1 to 4 | Written request; I add it to the screen list |
| Confirmation that a route works | Member 9 | I provide smoke test results |

Other modules may not edit my gateway files or screen code directly without a pull request that I review.

## 8. Ownership of files, tables, APIs, queues and configuration

| Item | Owned by me? | Details |
| --- | --- | --- |
| Frontend code | Yes | `web/src/features/dispatcher-admin/` (dispatch-board, fleet, courier-mobile, zones, reports) |
| Shared app shell, API client, design system | Shared with Member 6 | `web/src/app/`, `web/src/shared/` |
| Admin backend module | Shared with Members 1 and 4 | `core-api/internal/admin/` |
| Gateway route and policy files | Yes (with Member 9) | In `deploy/gateway/traefik/`: `traefik.yml` and `dynamic-routes.yml` (Docker, already in the repo) plus `ingressroutes.yaml` and `middlewares.yaml` (Kubernetes, I write them) |
| Module README and API list | Yes | README.md and API.md next to each module, updated in the same commit as changes |
| Route map and responsibility document | Yes | In `docs/` |
| Test checklists, frontend tests, gateway smoke tests | Yes | Vitest and React Testing Library, Playwright, route smoke tests |
| Database tables | **Yes, admin module only** | `delivery_zones`, `zone_postal_codes`, `system_settings` (migration `000002_create_delivery_zones`) and `audit_logs` (migration `000011_create_audit_logs`, created once by me as the table owner; Member 1 and Member 4 write rows through the shared audit writer) |
| Message queues and events | **No** | I publish or consume nothing |
| Business APIs outside admin | **No** | I only consume them (section 9) |
| Frontend environment settings | Yes | API base address (`VITE_API_BASE_URL`) only, no secrets |
| Cluster install of Traefik, TLS certificates, secrets | **No** | Member 9 |

Kubernetes files (`ingressroutes.yaml`, `middlewares.yaml`) are mine and live in `deploy/gateway/traefik/kubernetes/`. Member 9 writes `values.yaml` (Traefik Helm install) and the one Compose file, `deploy/docker-compose.yml` (A7). I send my Compose needs to Member 9 as a proposal; I keep no Compose file of my own (K10). My Kubernetes gateway files go in `deploy/gateway/traefik/` as ADR-001 says. Note for Member 9: ADR-002 names the Kubernetes folder `deploy/k8s/`, but the repo uses `deploy/kubernetes/`.

## 9. APIs consumed

The "Status" column says whether the owner has written the endpoint in their Doc 4 (26 October).

| Method and path | Purpose | Owner | Status |
| --- | --- | --- | --- |
| POST /api/v1/auth/login, /refresh, /logout | Staff sign-in, silent refresh, sign-out | Member 1 | Documented |
| GET /api/v1/dispatch/board?zone\_id= | Unassigned and failed parcels plus on-shift couriers with load, for one zone (`zone_id` required) | Member 3 | Agreed (Member 3 Doc 4 3.4) |
| POST /api/v1/dispatch/assignments | Assign `{parcel_id, courier_id}` with `Idempotency-Key`; result status `ASSIGNED`. Errors: 400 PARCEL\_NOT\_ELIGIBLE\_FOR\_ASSIGNMENT, 400 COURIER\_NOT\_ON\_SHIFT, 409 COURIER\_CAPACITY\_EXCEEDED, 409 PARCEL\_ALREADY\_ASSIGNED | Member 3 | Agreed (3.5) |
| POST /api/v1/dispatch/assignments/{id}/cancel | Unassign with `cancellation_reason` (used for reassignment) | Member 3 | Agreed (3.6) |
| POST /api/v1/dispatch/parcels/{id}/reschedule | Reattempt after ATTEMPT\_FAILED: `target_courier_id`, `scheduled_date`, `preferred_time_window`, `dispatch_notes`; 400 MAX\_ATTEMPTS\_EXCEEDED | Member 3 | Agreed (3.7) |
| GET, POST /api/v1/dispatch/couriers; GET /api/v1/dispatch/couriers/{id} | Fleet list (filters `zone_id`, `status`, `vehicle_type`), profile, add courier | Member 3 | Agreed (3.1 to 3.3) |
| POST /api/v1/courier/shift | Courier shift toggle (ON\_SHIFT, OFF\_SHIFT, ON\_BREAK) | Member 3 | Agreed (3.8) |
| GET /api/v1/courier/parcels | Courier's own task list | Member 3 | Agreed (3.9) |
| POST /api/v1/courier/parcels/{id}/start-run; POST /api/v1/courier/runs/start-batch | Courier starts the run (ASSIGNED to OUT\_FOR\_DELIVERY) | Member 3 | Agreed (3.10, 3.11) |
| POST /api/v1/deliveries/{id}/proof-photo | POD photo, multipart, 10 MB | Member 3 | Agreed (3.12) |
| POST /api/v1/deliveries/{id}/complete | Complete delivery with proof and `Idempotency-Key` | Member 3 | Agreed (3.13) |
| POST /api/v1/deliveries/{id}/failed | Failed attempt `{reason_code, notes, latitude, longitude}`; answer is ATTEMPT\_FAILED or RETURNED | Member 3 | Agreed (3.14) |
| GET /api/v1/deliveries/parcels/{id} | Attempt history and proof | Member 3 | Agreed (3.15) |
| GET /api/v1/admin/merchants?status=&q=&page=&per\_page= ; GET /api/v1/admin/merchants/{id} | Merchant list and detail | Route: me; logic: Member 4 | Agreed (Member 4 OpenAPI) |
| POST /api/v1/admin/merchants/{id}/approve, /reject, /suspend, /reinstate, /disable | Merchant status change, body `{expected_version, reason}` (reason 5 to 500 characters, required for reject, suspend, disable); 409 on a stale version | Route: me; logic: Member 4 | Agreed (Member 4 OpenAPI) |
| GET, POST /api/v1/admin/zones; GET, PATCH /api/v1/admin/zones/{id}; PATCH /api/v1/admin/zones/{id}/status | Zone screen | Admin module (me) | Mine |
| GET /api/v1/zones | Public list of enabled zones (also `?postal_code=`) for booking forms | Admin module (me) | Mine, new (R-18) |
| GET /api/v1/admin/reports/deliveries?from=&to= | Deliveries per day, success and failure rate; ADMIN and DISPATCHER | Admin module (me) | Mine; SRS 6.2 name (R-16) |
| GET /api/v1/admin/audit-logs | Audit explorer, ADMIN only | Admin module (me) | Mine |
| GET /api/v1/admin/settings; PUT /api/v1/admin/settings/{key} | System settings | Admin module (me) | Mine |
| User list, role change, disable account, end sessions | User management | Member 1 | **Requested** (FR-ADM-006, FR-IAM-007) |
| GET, POST /api/v1/admin/pricing-rules; PATCH /api/v1/admin/pricing-rules/{id} and /{id}/status | Pricing management | Member 2 | **Requested** (the audit found no pricing-rule endpoints in Member 2's drafts) |
| Audited parcel correction | Admin correction | Member 2 | **Requested** (U-08) |

Every **Requested** row must be confirmed by its owner before the matching screen is finished. The API client sends `Authorization` on every call, `Idempotency-Key` where the owner's contract asks for it, and `X-Request-ID`. It **never** sends `X-Tenant-ID`, `X-User-ID` or `X-User-Role`: the backend reads the tenant and role from the verified token, and the gateway removes those headers if a client sends them (R-15).

## 10. Responsibility split: Cloudflare, Traefik and the backend

In the repo, Traefik acts as both the Kubernetes Ingress controller and the API gateway, so there is one proxy hop inside the cluster. Cloudflare sits in front for DNS, DDoS protection and HTTPS. **Member 9 confirmed this split matches ADR-001 section 2. \[TBD: update the Team Work Division wording (Ingress and gateway as separate steps) to match.\]**

| Concern | Cloudflare | Traefik | Backend |
| --- | --- | --- | --- |
| DNS, DDoS shield, first HTTPS hop | Yes | No | No |
| TLS to the cluster, restoring real client IP | No | Yes (Member 9 installs; I configure trusted IPs) | No |
| Path-to-service routing | No | Yes | No |
| Rate limits (per pod, in memory) | No | Yes | Business limits and backups |
| Request size limits | No | Yes | Backup checks possible |
| CORS | No | Yes | No |
| Check that the login token is valid | No | No | Yes, always |
| Merchant API key check | No | No | Yes, always |
| Roles, ownership, tenant checks | No | No | Yes, always |
| Remove private data from public tracking | No | No | Tracking API |

**TLS and headers.** In production Cloudflare handles public HTTPS. Traffic from Cloudflare to Traefik uses a Cloudflare Origin certificate stored as the Kubernetes Secret `traefik-origin-tls`, which Member 9 sets up; I only reference TLS in the IngressRoutes. Locally, plain HTTP on port 80 is used; SRS SEC-001 allows HTTP only on loopback or private networks. The gateway passes `Authorization` and `X-API-Key` headers through unchanged.

## 11. Assumptions, decisions and out-of-scope work

### 11.1 Assumptions

- A-01: Traefik is the gateway and Ingress controller (ADR-001).
- A-02: Dispatcher, admin and courier screens live in one shared React app with Member 6 (web/README).
- A-03: Reassignment is "cancel the assignment with a reason, then assign again" (Member 3 Doc 4 3.6).
- A-04: Until teammates' APIs exist, I build against the example responses in their Doc 4 files and mark them as not final.

### 11.2 Decisions made earlier (still valid)

| ID | Decision | Source |
| --- | --- | --- |
| D-01 | Access token in memory (15 minutes). Refresh token in an HttpOnly, Secure, SameSite=Strict cookie, rotated on every use. The client refreshes on a 401. Couriers: ADR-003 gives them a 14-day refresh token (see U-01) | ADR-003 |
| D-02 | The gateway applies CORS | ADR-001 |
| D-03 | The gateway does not check tokens; the backend does | ADR-001 |
| D-04 | One shared web app; staff go to `/ops/*`, couriers to `/courier/*` | web/README |
| D-05 | Folder layout fixed (section 8) | domain-boundaries.md |
| D-06 | Failed attempt is separate from parcel status; maximum 3 attempts; a dispatcher or admin schedules a reattempt; Member 3 returns the parcel automatically after the 3rd failure or a refusal | ADR-004, Member 3 |
| D-07 | Money is LKR in integer cents, shown from backend values only | web/README, database-schema.md 3.2 |
| D-08 | Gateway is Traefik v3.1 (image traefik:v3.1); it replaced Kong | ADR-001, Member 9 |
| D-09 | Ports: core-api 8080, tracking-api 8081, ai-assistant 8000, web 3000, outbox-relay 8082, event-worker 8083. Health and metrics paths are not routed publicly | Member 9 A1, A2 |
| D-11 | I write ingressroutes.yaml and middlewares.yaml; Member 9 writes values.yaml and the one Compose file | Member 9 A7, A9 |
| D-12 | Production TLS by Cloudflare plus an origin certificate Secret; local use is plain HTTP, which SRS SEC-001 allows only on loopback or private networks | Member 9, ADR-002, SRS SEC-001 |

### 11.3 Unresolved decisions

| ID | Decision | Decide with | Status (26 Oct) |
| --- | --- | --- | --- |
| U-01 | Courier token storage: ADR-003 allows encrypted local storage for couriers, but the frontend rules forbid localStorage for tokens | Members 1, 9 | Open |
| U-02 | Report route and who may read reports | Members 1, 3 | **Closed by R-16** |
| U-03 | Zone surcharge format | Members 1, 2 | **Closed by R-04** (no surcharge in zones) |
| U-04 | Fee endpoint name: `/pricing/estimate`, `/pricing/quote` or `/pricing/quotes` (SRS 6.2) | Member 2 | Open (the audit recommends `/pricing/quotes`) |
| U-05 | AI assistant route | Members 9, 10 | **Closed by R-17** |
| U-06 | Per-user limits (courier, AI, merchant key) inside the backends | Members 1, 4, 10 | Partly closed: gateway numbers fixed by R-13; per-user limits still need an owner (see U-50) |
| U-07 | Reassignment and reattempt rules | Member 3 | **Closed by R-01** |
| U-08 | Is admin parcel correction a screen I build, and which endpoint does it call? | Team lead, Member 2 | Open |
| U-09 | Request ID header, domain, CORS origins | Member 9 | **Closed by R-11 and R-14** |
| U-10 | Which parts of the admin Go code I write | Members 1, 4 | **Closed**: I write all of it (section 2.3) |
| U-11 | Paths on one domain or subdomains | Whole group | **Closed**: paths on one domain (A9) |
| U-12 | Traefik dashboard port | Member 9 | **Closed by R-10**: 8090, internal only |
| U-13 | Who implements zone and pricing-rule CRUD | Members 2, 1, 4 | **Closed**: zones by me, pricing rules by Member 2 (R-03, R-04) |
| U-14 | Overlapping pricing rules: forbidden or resolved by priority | Member 2 | Open |
| U-15 | Public tracking burst | Members 5, 9 | **Closed by R-13**: burst 10 |
| U-16 | SSE path | Member 5 | **Closed by R-12** |
| U-17 | SSE heartbeat | Members 5, 9 | **Closed by R-12**: 15 seconds |
| U-63 | Add the `/ops/*` route table (section 2.1) to `web/README.md` and fix the `/ops/reports` and `/ops/zones` rows | Member 6 | New |
| U-64 | `database-schema.md` says `audit_logs.target_id` is uuid, but zone ids and setting keys are text. Change it to `varchar(128)` and add the `reason` column (SEC-015 asks for a reason) | Member 1, Tech Lead | New (replaces U-38) |
| U-65 | Member 2 must add the `/admin/pricing-rules` endpoints that my pricing screen needs | Member 2 | New |
| U-66 | Member 1 must add disable-account and end-sessions endpoints for the users screen | Member 1 | New (same as audit item for Member 1; extends U-53) |

### 11.4 Decisions applied in v4 (26 October review)

These decisions come from the owners' Doc 4 files, the alignment guide, Member 9's team agreements (A, D, K items), security-notes.md and the SRS. Every later document refers to them by number.

| ID | Decision | Source |
| --- | --- | --- |
| R-01 | Dispatch and courier calls use Member 3's routes only: `/dispatch/board?zone_id=` (required), `/dispatch/couriers`, `/dispatch/assignments` (status `ASSIGNED`), `/dispatch/assignments/{id}/cancel`, `/dispatch/parcels/{id}/reschedule`, `/courier/shift`, `/courier/parcels`, `/courier/parcels/{id}/start-run`, `/courier/runs/start-batch`, `/deliveries/{id}/proof-photo`, `/deliveries/{id}/complete`, `/deliveries/{id}/failed`, `/deliveries/parcels/{id}`. The older names (`reassign`, `reattempt`, `return`, `/courier/parcels/{id}/deliver`, `/fail-attempt`, assignment status `ACTIVE`) are dropped. RETURNED is set by Member 3 after the 3rd failure or REFUSED\_BY\_RECIPIENT | Member 3 Doc 4 |
| R-02 | Merchant admin uses `/admin/merchants`, `/admin/merchants/{id}` and `/admin/merchants/{id}/{approve, reject, suspend, reinstate, disable}` with `expected_version` and a reason (5 to 500 characters) for reject, suspend and disable. My admin module hosts the routes; Member 4's `merchants.AdminService` holds the logic. Statuses: PENDING, APPROVED, REJECTED, SUSPENDED, DISABLED | Member 4 Doc 4, OpenAPI |
| R-03 | `delivery_zones.id` is `VARCHAR(32)` text chosen by the admin (`ZONE_COLOMBO_01`), so Members 2 and 3 can reference it; `code` is a separate short unique code | Alignment guide 3.1, database-schema.md |
| R-04 | Zones hold **no money**. All fees, including any per-zone surcharge, are Member 2's pricing rules | Docs audit (pricing ownership), database-schema.md |
| R-05 | `audit_logs` uses the shared column names: `actor_id`, `actor_role`, `action`, `target_type`, `target_id`, `previous_state`, `new_state`, `client_ip`, `user_agent`, `request_id`, `created_at`, plus `reason` (U-64) | database-schema.md, SRS SEC-015 |
| R-06 | Migrations use the 6-digit sequence: `000002_create_delivery_zones` (zones, prefixes, settings) and `000011_create_audit_logs`. If the team's D2 vote picks timestamps, only the file names change | Alignment guide 6, AGENTS.md 7 |
| R-07 | Health and metrics paths: `/health/live`, `/health/ready`, `/metrics`, internal only | Member 9 A2, SRS 6.2 |
| R-08 | Database: name `swiftdrop_db`; roles `swiftdrop_migrator` (DDL) and `swiftdrop_app` (runtime); Kubernetes secrets `postgres-secrets` and `postgres-migrator-secrets`; `sslmode=disable` inside the cluster | Member 9 A5, D6, D11, K6, K7 |
| R-09 | No Compose file of my own; my services go into `deploy/docker-compose.yml` through a proposal to Member 9 | Member 9 A7, K10 |
| R-10 | Traefik entry points 80 and 443; dashboard 8090, internal only; core-api 8080 | Alignment guide 3.3, Member 9 A9 |
| R-11 | Request ID header is `X-Request-ID` (not `X-Correlation-ID`); logged as `request_id` | Member 9 A10, security-notes.md 4 |
| R-12 | SSE: `/api/v1/tracking/public/{trackingNumber}/stream` and `/api/v1/tracking/{trackingNumber}/stream`, 15-second heartbeat, no buffering | Alignment guide 3.4 |
| R-13 | Gateway rate limits per client IP: login 5/min burst 3; public tracking 30/min burst 10; merchant API 120/min burst 20; AI 10/min burst 2; 5 open streams per IP. Per-user limits are enforced by the backends | security-notes.md 3, SRS SEC-007 |
| R-14 | CORS exactly as security-notes.md section 4 | security-notes.md 4, SRS SEC-013 |
| R-15 | Clients never send `X-Tenant-ID`, `X-User-ID` or `X-User-Role`; the gateway strips them; the tenant comes from the token | ADR-003, web/README "Tenant Context", alignment guide 4.1 |
| R-16 | Reports: `GET /api/v1/admin/reports/deliveries`, readable by ADMIN and DISPATCHER | SRS 6.2, FR-ADM-008 |
| R-17 | AI routes stay under `/api/v1/ai/*` (`/api/v1/ai/chat`, `/api/v1/ai/mcp/sse`); the SRS line `/assistant/chat` must be updated by the Tech Lead | Member 9 A9, Member 10 Doc 4 |
| R-18 | New public endpoint `GET /api/v1/zones` (enabled zones, optional `postal_code` lookup) for Member 6's booking forms | Docs audit, Member 6 Doc 4 Q3 |
| R-19 | Requirement IDs follow the SRS: FR-ADM, FR-COU, FR-LIFE, FR-IAM, SEC-001 to SEC-017 (SRS 8), NFR-PERF. IDs such as FR-DSP, FR-POD and NFR-REL do not exist | SRS |
| R-20 | The `/ops/*` route list in section 2.1 is proposed for `web/README.md` (U-63) | Docs audit |

### 11.5 Out of scope for this module

- Customer, merchant and public tracking screens (Member 6).
- Business logic and tables of other modules, events and queues.
- Cluster setup, certificates, monitoring stack (Member 9).
- Live GPS, maps and payments (out of scope for the whole project, SRS 1.5). Zone map drawing is out unless the team confirms it.
- Kubernetes autoscaling settings, except routes for those services.

## 12. Responsibility matrix (summary)

O = owns, C = consumes or is consulted, N = must not do.

| Area | Member 7 | Member 1 | Member 2 | Member 3 | Member 4 | Member 5 | Member 6 | Member 8 | Member 9 | Member 10 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Dispatcher/admin/courier screens | O | C | C | C | C |  | C (shared shell) |  |  |  |
| Admin backend module | O (shared) | O (shared) |  |  | O (shared) |  |  |  |  |  |
| Authorization decisions | N | O |  |  |  |  |  |  |  |  |
| Assignment and delivery rules | N |  |  | O |  |  |  |  |  |  |
| Merchant approval rules | N |  |  |  | O |  |  |  |  |  |
| Pricing and parcel rules | N |  | O |  |  |  |  |  |  |  |
| Tracking and SSE service | N |  |  |  |  | O |  |  |  |  |
| Customer and merchant screens | N |  |  |  |  |  | O |  |  |  |
| Gateway route and policy files | O | C |  |  | C | C |  | C | C | C |
| Traefik install, TLS, cluster | N |  |  |  |  |  |  |  | O |  |
| Queues and events | N |  |  |  |  |  |  | O |  |  |

## 13. Sign-off

To be reviewed and approved by Members 1, 2, 3, 4, 5, 6, 8, 9 and 10 before implementation begins.

| Member | Approved (date) | Notes |
| --- | --- | --- |
| Member 1 |  |  |
| Member 2 |  |  |
| Member 3 |  |  |
| Member 4 |  |  |
| Member 5 |  |  |
| Member 6 |  |  |
| Member 8 |  |  |
| Member 9 |  |  |
| Member 10 |  |  |
