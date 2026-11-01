# Document 1 — Module Scope & Responsibility Specification

> **Status:** Draft / Proposed (For Review in Meeting 2)  
> **Author:** Member 7 — Dispatcher/Admin UI, Courier Web, Traefik Gateway  
> **Target Module / Location:** `web/src/features/dispatcher-admin/` (ops + courier screens), `core-api/internal/admin/` (admin module), `deploy/gateway/traefik/` (gateway)  
> **Key Upstream Dependencies:** Member 1 (Auth, RBAC, JWT), Member 2 (Parcels, Pricing), Member 3 (Dispatch, Delivery), Member 4 (Merchants), Member 5 (Tracking SSE), Member 8 (Outbox, events), Member 9 (Compose, Kubernetes)  
> **Open Questions / Needs Clarification:** U-01 to U-17 in section 11.3: paths vs subdomains, dispatch board endpoint, reports owner, missing endpoints (merchant approval, users, pricing, fleet), money format, courier token storage, tracking burst limit, SSE path, heartbeat 20 s vs 15 s

**Module:** Dispatcher/Admin/Courier Frontend, Admin Backend Module & API Gateway (Member 7) **Project:** SwiftDrop Parcel Delivery & Courier Management System **Based on:** SRS v1.0 (SWD-SRS-001), Engineering Standards v1.0, Team Work Division v1.0, and the team repository (README, AGENTS.md, ADR-001, ADR-002, ADR-003, ADR-004, domain-boundaries.md, web/README.md, security-notes.md, deploy/gateway, module API.md files) **Status:** Draft v3 (fixes from the v2 review). Items marked **\[TBD\]** wait for a teammate or a team decision. **Date:** 24 October 2026

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
| M7-UI-01 | Dispatch board: unassigned parcels with courier load, from one board endpoint | FR-ADM-001, FR-ADM-004 |
| M7-UI-02 | Assign a parcel to an active courier | FR-ADM-002 |
| M7-UI-03 | Reassign a courier where the backend allows it, with a reason **\[TBD: no reassign endpoint or reason field yet\]** | SRS 5.3 |
| M7-UI-04 | Show courier workload (active assignments against maximum capacity) | FR-ADM-004 |
| M7-UI-05 | Show the real backend error on assignment failure (for example COURIER\_CAPACITY\_EXCEEDED, 409) and refresh the data | FR-ADM-003 |
| M7-UI-06 | Failed-delivery review: show the attempt count and failure reason, and let a dispatcher choose reattempt (ATTEMPT\_FAILED to OUT\_FOR\_DELIVERY, maximum 3 attempts) or return the parcel (RETURNED) **\[TBD: endpoint not documented\]** | ADR-004 |
| M7-UI-07 | Fleet screen: courier status, vehicle capacity, shifts | web/README |
| M7-UI-08 | Merchant approval screen (approve, suspend; reject and disable only if the backend supports them) | FR-ADM-005 |
| M7-UI-09 | User and role management screen | FR-ADM-006 |
| M7-UI-10 | Delivery zone management screen | FR-ADM-007 |
| M7-UI-11 | Pricing rule management screen (versioned, effective dates) **\[TBD: Member 2 proposed endpoints (section 2.5); implementer to confirm\]** | FR-ADM-007 |
| M7-UI-12 | Reports screen (volume, success rate, failed attempts, SLA, audit log explorer) | FR-ADM-008 |
| M7-UI-13 | Admin parcel correction screen, reason required **\[TBD: confirm this is my scope\]** | FR-ADM-009 |
| M7-UI-14 | Login-protected area with role-based navigation (usability only) | Standards 4.3 |
| M7-UI-15 | Show loading, empty, success, validation error, 401, 403, 409 and 500 states on every screen | Standards 4.3 |
| M7-UI-16 | Accessible, responsive layout; status shown with text as well as colour; all parcel statuses shown (CREATED, ASSIGNED, PICKED\_UP, IN\_TRANSIT, OUT\_FOR\_DELIVERY, DELIVERED, ATTEMPT\_FAILED, RETURNED, CANCELLED) | SRS 9.4 |

Planned routes from web/README: `/ops/dispatch` (dispatcher, admin), `/ops/couriers` (dispatcher, admin), `/ops/zones` (admin), `/ops/reports` (admin). Routes for merchants, users and pricing are not defined yet; proposed `/ops/merchants`, `/ops/users`, `/ops/pricing` **\[TBD: confirm with Member 6, who shares the app shell\]**.

### 2.2 Courier mobile web screens

| ID | Responsibility | Related requirement |
| --- | --- | --- |
| M7-CR-01 | Courier task list at `/courier/tasks` (pickups and deliveries, barcode scan simulation) | web/README |
| M7-CR-02 | Proof-of-delivery capture at `/courier/tasks/:id/pod` (photo upload, recipient signature) | web/README |
| M7-CR-03 | Failed-delivery report with the standard reason codes (RECIPIENT\_UNAVAILABLE, CUSTOMER\_REQUESTED\_RESCHEDULE, ACCESS\_RESTRICTED, INVALID\_ADDRESS, REFUSED\_BY\_RECIPIENT, DAMAGED\_IN\_TRANSIT, OTHER with a note of at least 15 characters), sent to `POST /api/v1/courier/parcels/{id}/fail-attempt` | ADR-004 |
| M7-CR-04 | Work on slow phone connections; keep uploads within the 10 MB gateway limit | ADR-001 |

### 2.3 Admin backend module (`core-api/internal/admin/`, shared with Members 1 and 4)

| ID | Responsibility | Notes |
| --- | --- | --- |
| M7-AD-01 | Delivery zone administration: create and update zones, postal code mappings, base surcharge | `POST /api/v1/admin/zones` exists |
| M7-AD-02 | Operational reports: volume, success rate, failed attempts, SLA | `GET /api/v1/admin/reports/operational` exists; SRS 6.2 calls it `/admin/reports/deliveries` (U-02) |
| M7-AD-03 | Audit log queries (read only, never edit or delete) | Not yet in API.md |
| M7-AD-04 | Merchant approval, in coordination with Member 4 | Approval status lives in the merchants module |
| M7-AD-05 | Keep this module's `README.md` and `API.md` updated in the same commit as any change | AGENTS.md |

The module's own rules: every endpoint requires the ADMIN role; audit logs are immutable; one postal code may not belong to conflicting zones. **\[TBD: agree with Members 1 and 4 which parts of the Go code I write.\]**

### 2.4 API Gateway (`deploy/gateway/traefik/`)

| ID | Responsibility | Status in repo |
| --- | --- | --- |
| M7-GW-01 | Route each public path to the correct service | Done: `/api/v1/tracking` to tracking, `/api/v1/ai` to AI assistant, `/api/v1` to core API, `/` to the frontend |
| M7-GW-02 | Rate limits: login, public tracking, merchant API, AI | Tracking only in the file (30 per minute, burst 50). security-notes.md says burst 10 for public tracking, so the burst must be agreed (U-15). Member 5 proposes 30 public lookups per minute and 5 concurrent live-update connections per IP, not yet load-tested. Login, merchant and AI limits missing. Courier and AI limits in security-notes.md are per user, which the gateway cannot see, so they must be per IP here or done in the backend |
| M7-GW-03 | Request size limit | Done: 10 MB on the core API route |
| M7-GW-04 | CORS allowlist | Gateway applies it (ADR-001). Allowlist is localhost only; the real domain is not decided (placeholder swiftdrop.example.com). If one domain with paths is chosen (Member 9's proposal, group vote pending), the web app needs no CORS. Differences from security-notes.md to fix: (1) the file allows X-Tenant-ID but not X-Request-ID, security-notes.md is the reverse; (2) max age 3600 in the file, 86400 in security-notes.md; (3) security-notes.md wants credentials allowed and lists swiftdrop.io origins, not swiftdrop.example.com |
| M7-GW-05 | Support live updates (SSE) through the gateway | Member 5 proposes route /api/v1/tracking/public/{trackingNumber}/stream, response buffering off, long-lived connections allowed, and a heartbeat every 20 seconds. Not agreed yet: the path differs in tracking API.md (/api/v1/tracking/{tracking\_number}/stream), ADR-002 (/api/v1/tracking/sse/\*) and SRS 6.2 (/api/v1/tracking/{trackingNumber}/events), and ADR-002 and deploy/gateway/README.md say a 15-second heartbeat (U-16, U-17). The /api/v1/tracking route covers every path, but buffering and limit rules need the exact one. Settings are not load-tested and the test is still unchecked in ADR-001 |
| M7-GW-06 | Keep internal paths (health, metrics, database, broker, workers) unreachable from outside | Needs explicit check and test. Dashboard `insecure: true` must be off in production. Member 9 says /health/live, /health/ready and /metrics must not be routed publicly, and proposes moving the dashboard to port 8090 because core-api uses 8080. If agreed, the docker example in deploy/gateway/README.md (which maps 8080 for the dashboard) must be updated too |
| M7-GW-07 | Pass or create a request/correlation ID **\[TBD: not in config; confirm header name with Member 9\]** | Missing |
| M7-GW-08 | Keep all gateway files in version control, rebuildable from a clean clone | traefik.yml and dynamic-routes.yml exist for Docker. I write ingressroutes.yaml and middlewares.yaml for Kubernetes. Member 9 writes values.yaml (Helm install) and the Docker Compose file |
| M7-GW-09 | Document the split of work between Cloudflare, Traefik and the backend | See section 10 |
| M7-GW-10 | Check that 401 and 403 responses pass through unchanged | Unchecked in ADR-001 |

### 2.5 Zone and pricing administration (Member 2 proposal, not final)

Member 2 proposed these fields and endpoints. Member 2 handles the backend fee calculation; my screens only show and edit rules and display the backend's validation messages.

| Item | Proposed fields |
| --- | --- |
| Delivery zone | id, name, code (unique), description (optional), enabled, created\_at, updated\_at. To confirm: postal codes, cities or areas, and the surcharge the admin module already stores |
| Pricing rule | id, origin\_zone\_id, destination\_zone\_id, service\_level, min\_weight\_kg, max\_weight\_kg, size limits, base\_fee, currency, enabled, effective\_from, effective\_to, version, created\_at, updated\_at |

| Method and path | Purpose |
| --- | --- |
| GET /api/v1/admin/zones | List zones |
| POST /api/v1/admin/zones | Create a zone |
| PATCH /api/v1/admin/zones/{id} | Edit a zone |
| PATCH /api/v1/admin/zones/{id}/status | Enable or disable a zone |
| GET /api/v1/admin/pricing-rules | List pricing rules |
| POST /api/v1/admin/pricing-rules | Create a rule |
| PATCH /api/v1/admin/pricing-rules/{id} | Edit a rule |
| PATCH /api/v1/admin/pricing-rules/{id}/status | Enable or disable a rule |

Invalid rules show a clear message from the backend, for example "Minimum weight must be less than maximum weight" or "Base fee cannot be negative". For overlapping rules, the backend detects the conflict and, if overlaps are prohibited, returns 409 Conflict naming the existing rule so the admin can correct the details.

**To confirm with Member 2:** whether overlapping rules are prohibited or resolved by priority; the pricing formula, currency, weight and size limits, and extra charges; who implements the zone and pricing-rule CRUD endpoints.

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
| Member 1 | User and role endpoints; backend 403 behaviour; refresh and logout endpoint details; agreement on ADMIN-only admin module versus dispatcher report access | **\[TBD\]** |
| Member 2 | Zone and pricing fields and endpoints are proposed (section 2.5). Still to confirm: pricing formula, currency, overlap policy, fee endpoint name (estimate or quote), money format (integer cents) | **\[TBD\]** |
| Member 3 | Reassignment endpoint and reason field, reattempt and return endpoint, courier fleet endpoints, fail-attempt contract | **\[TBD\]** |
| Member 4 | Merchant list, approve, reject, suspend, disable endpoints | **\[TBD\]** |
| Member 5 | SSE path, buffering, heartbeat and rate limits are proposed (M7-GW-02, M7-GW-05); confirm after testing. Member 5 also asked which gateway is used: Traefik v3.1 | **\[TBD\]** |
| Member 6 | Shared app shell, route list, shared components | **\[TBD\]** |
| Member 8 | Confirmation that no worker or broker is exposed externally | **\[TBD\]** |
| Member 10 | AI route (/api/v1/ai) confirmation and the AI rate limit (per IP at the gateway, per user in the backend) | **\[TBD\]** |
| Member 9 | Answered: Traefik v3.1, service ports, file layout, TLS. Still open: real domain, paths versus subdomains, dashboard port, correlation header | **\[TBD\]** |

### 4.2 Downstream (who depends on me)

| Who | What they depend on |
| --- | --- |
| Dispatchers, admins, couriers | Working screens |
| Members with public APIs (1, 2, 3, 4, 5, 10) | Correct routes to their services |
| Member 9 | Gateway config that deploys cleanly into the cluster |
| Member 6 | Shared app shell and gateway route for the frontend |

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
| Database tables | **Yes, admin module only** | `delivery_zones`, `zone_postal_codes`, `system_settings` |
| Message queues and events | **No** | I publish or consume nothing |
| Business APIs outside admin | **No** | I only consume them (section 9) |
| Frontend environment settings | Yes | API base address (`VITE_API_BASE_URL`) only, no secrets |
| Cluster install of Traefik, TLS certificates, secrets | **No** | Member 9 |

Kubernetes files (`ingressroutes.yaml`, `middlewares.yaml`) are mine and live in `deploy/gateway/traefik/`. Member 9 writes `values.yaml` (Traefik Helm install) and the Docker Compose file, which runs Traefik with small test containers named core-api, tracking-api, ai-assistant and web, so every route can be tested before the real services exist. My Kubernetes gateway files go in `deploy/gateway/traefik/` as ADR-001 says. Note for Member 9: ADR-002 names the Kubernetes folder `deploy/k8s/`, but the repo uses `deploy/kubernetes/`.

## 9. APIs consumed

The "Repo status" column shows what the module `API.md` files document today.

| Method and path | Purpose | Owner | Repo status |
| --- | --- | --- | --- |
| POST /api/v1/auth/login | Staff sign-in | Member 1 | Documented |
| POST /api/v1/auth/refresh, /logout | Silent token refresh and sign-out | Member 1 | Referenced in ADR-003, not in API.md |
| GET /api/v1/dispatch/board | Unassigned parcels and active couriers with load | Member 3 | Documented. SRS 6.2 uses two endpoints (GET /dispatch/unassigned and GET /dispatch/workloads); the repo merged them into one board |
| POST /api/v1/dispatch/assignments | Assign (parcel\_id, courier\_id). Errors: 400 PARCEL\_NOT\_ELIGIBLE\_FOR\_ASSIGNMENT, 404, 409 COURIER\_CAPACITY\_EXCEEDED | Member 3 | Documented. No reassign or reason field |
| Reattempt or return a failed parcel | Dispatcher decision after ATTEMPT\_FAILED | Members 2, 3 | **Requested** |
| Fleet: courier status, capacity, shifts | Fleet screen | Member 3 | **Requested** |
| POST /api/v1/courier/parcels/{id}/fail-attempt | Courier failed-delivery report | Member 3 | Named in ADR-004, not in API.md |
| GET /api/v1/courier/deliveries; POST /api/v1/courier/parcels/{id}/status | Courier task list and status update | Member 3 | Named in SRS 6.2, not in API.md. **Member 3 to confirm** |
| Proof-of-delivery upload | Courier POD screen | Member 3 | **Requested** |
| POST /api/v1/admin/zones | Create or update a zone | Admin module | Documented |
| GET /api/v1/admin/zones; PATCH /api/v1/admin/zones/{id} and /{id}/status | Zone screen | Admin module | **Proposed by Member 2; implementer to confirm** |
| GET /api/v1/admin/reports/operational | Reports | Admin module | Documented. SRS 6.2 names it GET /api/v1/admin/reports/deliveries (see U-02) |
| Audit log query | Audit explorer | Admin module | **Requested** |
| Merchant list, approve, reject, suspend, disable | Merchant approval | Member 4 | **Requested** (only webhooks and API keys exist; statuses are PENDING, APPROVED, SUSPENDED) |
| User list, role change, disable account | User management | Member 1 | **Requested** (module path mentions /users, no endpoint listed) |
| GET and POST /api/v1/admin/pricing-rules; PATCH /api/v1/admin/pricing-rules/{id} and /{id}/status | Pricing management | Member 2 | **Proposed by Member 2; implementer to confirm** |
| Audited parcel correction | Admin correction | Member 2 | **Requested** |

Every **Requested** row must be confirmed by its owner before the matching screen is finished. The API client also sends `Authorization` on every call, `Idempotency-Key` on creating requests, and `X-Tenant-ID` where it applies.

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
- A-03: Reassignment uses the assignment endpoint with a reason **\[TBD: not confirmed by Member 3\]**.
- A-04: Until teammates' APIs exist, I build against agreed example responses and mark them as not final.

### 11.2 Decisions made or proposed so far

| ID | Decision | Source |
| --- | --- | --- |
| D-01 | Access token in memory (15 minutes). Refresh token in an HttpOnly, Secure, SameSite=Strict cookie (7 days), rotated on every use. The client refreshes on a 401. Couriers differ: ADR-003 gives them a 14-day refresh token (see U-01) | ADR-003 |
| D-02 | The gateway applies CORS | ADR-001 |
| D-03 | The gateway does not check tokens; the backend does | ADR-001 |
| D-04 | One shared web app; staff go to `/ops/*`, couriers to `/courier/*` (web/README section 1 wrongly sends couriers to `/ops/*`; its route table uses `/courier/*`, which I follow. Section 1 needs a fix) | web/README |
| D-05 | Folder layout fixed (section 8) | domain-boundaries.md |
| D-06 | Failed attempt is separate from parcel status; maximum 3 attempts; only a dispatcher or admin can approve a reattempt (domain-boundaries.md 3.1 says dispatcher only; see U-07) | ADR-004 |
| D-07 | Money is LKR in integer cents | web/README |
| D-08 | Gateway is Traefik v3.1 (image traefik:v3.1); it replaced Kong | ADR-001, Member 9 |
| D-09 | Ports: core-api 8080, tracking-api 8081, ai-assistant 8000, web 3000 (3000 proposed; ADR-002 says 80). Health and metrics paths are not routed publicly | Member 9 |
| D-10 | Live tracking: stream route, no buffering, heartbeat, 30 lookups per minute and 5 connections per IP (proposed, not load-tested). Path and heartbeat interval are still open (U-16, U-17) | Member 5 (proposed) |
| D-11 | I write ingressroutes.yaml and middlewares.yaml; Member 9 writes values.yaml and the Docker Compose file | Member 9 |
| D-12 | Production TLS by Cloudflare plus an origin certificate Secret; local use is plain HTTP on port 80, which SRS SEC-001 allows only on loopback or private networks | Member 9, ADR-002, SRS SEC-001 |
| D-13 | Zone and pricing-rule fields and endpoints (section 2.5) | Member 2 (proposed) |

### 11.3 Unresolved decisions

| ID | Decision | Decide with |
| --- | --- | --- |
| U-01 | Courier token storage: ADR-003 allows encrypted local storage for couriers, but the frontend rules forbid localStorage for tokens. ADR-003 also gives couriers a 14-day refresh token, not 7 days | Members 1, 9 |
| U-02 | Reports: admin module and web/README say ADMIN only, but SRS 6.2 allows "Admin/authorized dispatcher". The endpoint name also differs: `/admin/reports/operational` (repo) or `/admin/reports/deliveries` (SRS 6.2) | Members 1, 3 |
| U-03 | Zone surcharge format: zone API sends "3.50" but money rule is integer cents | Members 1, 2 |
| U-04 | Fee endpoint name: `/pricing/estimate` (module docs), `/pricing/quote` (frontend rules) or `/pricing/quotes` (SRS 6.2) | Member 2 |
| U-05 | AI assistant route: Member 9 confirms /api/v1/ai; the SRS says /assistant/chat, so the SRS reference needs updating | Members 9, 10 |
| U-06 | Exact rate limits for login, merchant API and AI routes, and per-IP replacements for the per-user limits in security-notes.md | Members 1, 4, 10, 9 |
| U-07 | Reassignment rules and reason field; who may approve a reattempt (ADR-004: dispatcher or admin; domain-boundaries.md: dispatcher only) | Member 3 |
| U-08 | Is admin parcel correction a screen I build? | Team lead |
| U-09 | Correlation ID header name, real domain (placeholder swiftdrop.example.com), production CORS origins | Member 9 |
| U-10 | Which parts of `core-api/internal/admin/` Go code I write | Members 1, 4, team lead |
| U-11 | Paths on one domain (Member 9's proposal) or subdomains (ADR-002). My vote is requested | Whole group |
| U-12 | Traefik dashboard on port 8090 (proposed) because core-api uses 8080. My OK is requested | Member 9 |
| U-13 | Zone model: Member 2 lists no postal codes or surcharge, but the admin module stores them. Who implements zone and pricing-rule CRUD? | Members 2, 1, 4 |
| U-14 | Overlapping pricing rules: forbidden (409 Conflict naming the rule) or resolved by priority; pricing formula and currency | Member 2 |
| U-15 | Public tracking burst: 50 (gateway file) or 10 (security-notes.md) | Members 5, 9 |
| U-16 | SSE path: Member 5's /api/v1/tracking/public/{trackingNumber}/stream, tracking API.md's /api/v1/tracking/{tracking\_number}/stream, ADR-002's /api/v1/tracking/sse/\*, or SRS 6.2's /api/v1/tracking/{trackingNumber}/events | Member 5 |
| U-17 | SSE heartbeat: 20 seconds (Member 5) or 15 seconds (ADR-002, deploy/gateway/README.md) | Members 5, 9 |

### 11.4 Out of scope for this module

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
