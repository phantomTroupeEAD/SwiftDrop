# SwiftDrop — Cross-Module End-to-End (E2E) Test Suite

[![Playwright](https://img.shields.io/badge/Playwright-1.42+-45ba4b?logo=playwright&logoColor=white)](https://playwright.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.4+-3178c6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![Node.js](https://img.shields.io/badge/Node.js-20.x%20%7C%2022.x-339933?logo=node.js&logoColor=white)](https://nodejs.org/)
[![Architecture](https://img.shields.io/badge/Architecture-Modular%20Monolith-orange)](../../docs/architecture/domain-boundaries.md)
[![Status](https://img.shields.io/badge/Status-Grounding%20Complete-brightgreen)]()

> **Domain Ownership:** Member 10 — Cross-Workflow Quality Coordination & AI Assistant  
> **Collaborating Domains:** Members 1 through 9 (All Domain Owners)  
> **Evaluation Rubric:** **15 Marks** (Cross-Module Integration, Multi-Actor Journeys, Examiner Dashboard, Automated Verification)  
> **Authoritative Living Journal:** [`LIVING_DOCUMENT.md`](./LIVING_DOCUMENT.md) (ADRs, Decision Logs, Invariant Proofs)

---

## 1. Executive Summary & Purpose

The **SwiftDrop Cross-Module E2E Test Suite** (`tests/e2e/`) is the central quality gate for the entire SwiftDrop logistics platform. While individual modules maintain unit tests (`core-api/internal/`, `services/messaging/`, `services/tracking/`), this test suite verifies **multi-actor transactions across physical boundaries**:

1. **Transactional Integrity:** From Customer Booking $\rightarrow$ PostgreSQL Outbox commit $\rightarrow$ RabbitMQ bus $\rightarrow$ Event Worker $\rightarrow$ Webhooks.
2. **Multi-Actor Browser Isolation:** Enforcing zero session or cookie crosstalk between Customer, Dispatcher, Courier, Merchant, and Admin sessions.
3. **Multi-Tenant Security (`SEC-004`/`SEC-005`):** Proving zero data leakage between independent merchant organizations.
4. **Real-Time Projection Accuracy:** Verifying instant milestone fan-out over Server-Sent Events (SSE) without polling.

---

## 2. System Architecture

```
                               CROSS-MODULE E2E TEST ARCHITECTURE
                               
 ┌────────────────────────────────────────────────────────────────────────────────────────┐
 │                         Playwright Multi-Actor Test Runner                             │
 │                                                                                        │
 │  [Customer Portal]    [Merchant Portal]   [Dispatcher Board]  [Courier PWA]  [Admin]   │
 │   Desktop 1280x800     Desktop 1366x768    Desktop 1440x900    Pixel 7 (Touch)1440x900 │
 └─────────────┬─────────────────┬───────────────────┬──────────────────┬───────────┬─────┘
               │                 │                   │                  │           │
               ▼                 ▼                   ▼                  ▼           ▼
 ┌────────────────────────────────────────────────────────────────────────────────────────┐
 │                      Traefik Edge API Gateway & Ingress (:80)                          │
 └─────────────┬─────────────────────────────┬──────────────────────────────┬─────────────┘
               │                             │                              │
               ▼                             ▼                              ▼
        [Core API: 8080]             [Tracking API: 8081]          [AI Assistant: 8000]
      PostgreSQL Monolith            SSE Projection Streams        FastMCP Natural Language
               │                                                            │
               ▼ (Atomic Outbox Commit)                                     ▼ (Read-Only)
        [Outbox Relay] ──► [RabbitMQ Bus] ──► [Event Worker] ──────► [mock-webhook:9999]
                                                                     Multi-Tenant Receiver &
                                                                     Visual Examiner Dashboard
```

---

## 3. Quickstart & Command Reference

### Prerequisites
- **Node.js:** `v20.x` or `v22.x` (LTS recommended)
- **npm:** `v10.x`

### Installation
```bash
cd tests/e2e
npm install
npx playwright install chromium
```

### Command Matrix

| Command | Environment | Description |
| :--- | :---: | :--- |
| **`npm run demo`** | Real Stack | **SwiftDrop Evaluator Demo-as-Code Showcase:** Complete 16-step end-to-end automated walkthrough with live Apple-grade HUD commentary, natural human-paced typing, admin setup, customer booking, dispatching, mobile courier POD, tracking, merchant onboarding, bulk CSV import (32 orders), API key generation, Swagger UI OpenAPI order execution, webhook delivery, and viewer inspection. |
| **`npm run demo:headed`** | Visual | Runs the Evaluator Demo-as-Code Showcase in a visible browser window with cinematic HUD commentary and natural human typing. |
| **`npm run test:12-users`** | Real Stack | **12-User Cross-Module Real-World Workflow Automation Suite:** Master 5-step serial journey executing all 12 distinct actors across all 6 roles (3 Customers, 2 Merchants, 3 Couriers, 2 Dispatchers, 1 Admin, 1 Public Guest) across the full parcel lifecycle. |
| **`npm run test:rbac-matrix`** | Real Stack | **Master Every-Button & Strict 6-Role RBAC Matrix Suite:** Exercises every UI button, modal, canvas, toggle, and external link across Courier PWA and Admin/Ops Console, synchronized with DB state, followed by strict 6-role privilege enforcement ("no less, no more"). |
| **`npm run test:admin-ops`** | Real Stack | **Master Admin Operations E2E Suite (Flow 14):** Validates PR #99 Admin UI Part 2: Merchants management (/ops/merchants, review, approval, suspend with reason validation, reinstate, disable), audit log integration, pricing rules management (/ops/pricing fallback & CRUD), and users pagination controls (/ops/users). |
| **`npm run test:edge-10`** | Real Stack | **Edge Suite 10 (Courier Offline Resilience & PWA Network Recovery):** Simulates mobile connectivity loss (`context.setOffline(true)`) during active delivery run, verifies graceful error alerts without state loss, reconnects (`context.setOffline(false)`), and asserts atomic POD completion. |
| **`npm run test:edge-11`** | Real Stack | **Edge Suite 11 (Concurrent Dispatch Assignment Race & PostgreSQL FOR UPDATE):** Fires simultaneous double assignment requests via `Promise.all` at the exact same millisecond; verifies hardware-level ACID row locking (`SELECT ... FOR UPDATE`), exactly one 201 Created, competing 409 Conflict, and zero double allocations in DB. |
| **`npm run test:edge-12`** | Real Stack | **Edge Suite 12 (Sustained Webhook Outage, Retry Backoff & DLQ Fault-Tolerance):** Sustained 500 error injection on chaos target (:9999/webhooks/merchant-chaos); asserts exponential retry backoff, attempts recorded in PostgreSQL `webhook_delivery_attempts`, and zero head-of-line blocking for healthy endpoints (Beta). |
| **`npm run test:cross-engine`** | Real Stack | **Cross-Engine Browser Matrix:** Runs tracking portal & customer workflows across Desktop Firefox (Gecko) and Desktop Safari (WebKit) to guarantee cross-browser layout & rendering parity. |
| **`npm run test:traffic`** | Real Stack | **Concurrent 6-Role System Load Simulation:** Multi-browser concurrent load across PUBLIC (SSE stream), CUSTOMER (parallel bookings), MERCHANT (CSV ingestion), DISPATCHER (zone assignment), COURIER (mobile POD), and ADMIN. |
| **`npm run test:vulnerabilities`** | Real Stack | **Exhaustive Flaw & Vulnerability Audit Matrix:** 15 security scenarios auditing state machine bypass, cross-tenant IDOR, privilege escalation, JWT tampering, and boundary fuzzing. |
| **`npm run test:live`** | Real Stack | **Master 5-Window Live System E2E** running against real Core API, live PostgreSQL DB, and physical daemons with zero route mocks. |
| **`npm run test:live-guardrails`** | Real Stack | **Live Security Guardrails** (Real Courier IDOR 403, RBAC 403, Tenant Isolation 403/404, Tracking PII masking). |
| **`npm run test:realtime`** | Live/Mocked | Master System 5-Window Real-Time Orchestration test across Customer, Applicant/Merchant, Admin, Dispatcher, and Courier simultaneously. |
| **`npm run test:security`** | Live/Mocked | Strict authorization guardrails: Courier IDOR protection, Merchant tenant isolation, RBAC routing gates, SEC-009 public PII masking. |
| **`npm run test:merchant-orders`** | Real Stack | **Master Unified Commercial Merchant E2E:** Mailpit OTP registration, REST approval, API keys, Webhooks matrix (Alpha/Beta/Gamma/Omega), 14 order API edge cases, multi-window dispatcher assignment, HMAC security & UI parity. |
| **`npm run test:merchant-orders-edge-cases`** | Real Stack | **Senior QA Orders Page Suite:** CSV Excel BOM parsing, embedded quotes/commas, partial failure retries, pagination boundaries (>20 orders), and cross-tenant IDOR isolation. |
| **`npm run test:merchant-api-keys`** | Real Stack | **Exhaustive 29-Scenario API Key Testing Suite:** UI modal security, form validations, quota limits (max 5), duplicate labels, expiration bounds, and cryptographic auth. |
| **`npm run test:combinatorial-webhooks`** | Real Stack | **Exhaustive 64-Combination Webhook Suite:** Full power set $\mathcal{P}(S)$ testing of all 6 subscribable events (0 to 6), 384 routing decisions, and Core API registration. |
| **`npm run test:merchant-webhooks`** | Real Stack | Merchant webhooks, event filtering combinations (Alpha/Beta/Gamma), multi-window dispatcher assignment, HMAC-SHA256 & deletion cutoff. |
| **`npm run test:dispatcher`** | Real/Mocked | **Exhaustive Dispatcher Portal Suite:** 8 single-page suites (zones, empty queue, shift management, unassign modal, reschedule modal, onboarding modal, and 500 retry). |
| **`npm run test:dispatcher-sync`** | Real Stack | **Multi-Portal Real-Time Live Sync:** Dispatcher shift control, real customer booking, unassign eviction, re-assignment, and mobile courier `OUT_FOR_DELIVERY` synchronization. |
| **`npm run test:dry-run`** | Offline | Runs the harness self-check verifying browser launch and multi-actor context isolation across all 5 profiles (**10/10 passing**). |
| **`npm run test:webhook`** | Offline | Runs in-memory unit tests for the mock webhook service (HMAC-SHA256, isolation, chaos 500, native 404 in **7ms**). |
| **`npm run test:ui`** | Interactive | Opens Playwright's visual desktop UI runner with DOM inspection, timeline scrub, and network logs. |
| **`npm run test:headed`** | Visual | Executes tests in visible browser windows (ideal for examiner live demos). |
| **`npm start`** | Standalone | Starts the Standalone Webhook Receiver & Visual Dashboard on **`http://localhost:9999/dashboard`**. |
| **`npm run test:report`** | Artifact | Opens the generated HTML test report with embedded traces and failure screenshots. |
| **`npm run typecheck`** | Strict | Validates strict TypeScript compilation (`tsc --noEmit`). |

---

## 4. Directory Layout

```
tests/e2e/
├── package.json                   # Standalone package manifest (@playwright/test, typescript, eventsource)
├── tsconfig.json                  # Strict TypeScript configuration with @fixtures and @helpers aliases
├── playwright.config.ts           # 5-actor multi-device project definitions & evidence retention rules
├── .gitignore                     # Ignores binary traces, videos, reports, and node_modules
│
├── fixtures/                      # Test Harness Fixtures
│   ├── authFixtures.ts            # Strict browser context isolation (customerPage, courierPage, etc.)
│   ├── testData.ts                # Canonical test accounts, tenant secrets, postal codes, dimensions
│   └── gatewayClient.ts           # Resilient REST API client with polling & health waiting
│
├── helpers/                       # Cross-Cutting Quality Helpers
│   ├── selectors.ts               # Authoritative data-testid contract registry across all 5 portals
│   ├── sseListener.ts             # Async Server-Sent Events stream subscriber for live tracking
│   ├── mailpitHelper.ts           # Real Mailpit transactional email OTP retrieval
│   ├── dbReset.ts                 # Clean DB reset leaving canonical Admin account
│   └── evidence.ts                # Automated full-page screenshot & .zip trace bundler for grading
│
├── demo/                          # Evaluator Demo-as-Code Showcase
│   ├── evaluator-demo.spec.ts     # Master 16-step serial demonstration walkthrough
│   └── helpers/                   # Demo-specific presenters & utilities
│       ├── hud.ts                 # Apple-grade glassmorphic presenter HUD banner
│       ├── typing.ts              # Natural human typing & fast-typing simulators
│       └── csvGenerator.ts        # Valid 32-order bulk merchant CSV generator
│
├── flows/                         # End-to-End Journey Specifications
│   ├── real-system-12-user-workflows.spec.ts # Master 12-User Cross-Module Real-World Workflow Automation Suite (12 Distinct Actors, All 6 Roles)
│   ├── real-world-12-user-workflows.spec.ts  # Canonical alias re-exporting real-system-12-user-workflows
│   ├── realtime-orchestration.spec.ts # Master 5-Window Real-Time Lifecycle (Customer, Applicant->Merchant, Admin, Dispatcher, Courier)
│   ├── realtime-auth-guardrails.spec.ts # Strict Security Guardrails (Courier IDOR, Tenant Isolation, RBAC, SEC-009)
│   ├── dry-run.spec.ts            # Offline verification test (10/10 passed)
│   ├── 01-customer-booking.spec.ts# Flow 1: Customer booking wizard, dynamic quote & receipt
│   ├── 02-dispatcher-assignment.spec.ts # Flow 2: Dispatcher board & courier capacity assignment
│   ├── 03-courier-delivery.spec.ts# Flow 3: Mobile PWA transit milestones & electronic signature POD
│   ├── 04-tracking-and-ai.spec.ts # Flow 4: Public tracking PII masking & grounded AI assistant
│   ├── 05-merchant-delivery-webhook-loop.spec.ts # Flow 5: Full-circle merchant webhook, courier delivery retry & messaging audit
│   ├── 06-customer-registration-mailpit.spec.ts # Flow 6: Customer registration & real email OTP verification via Mailpit
│   ├── realtime-user-traffic-load.spec.ts # Master Multi-Role Concurrent Load Simulation (6 concurrent browser sessions)
│   ├── flaw-and-vulnerability-matrix.spec.ts # Exhaustive System Vulnerability & Flaw Audit Matrix (15 security & invariant scenarios)
│   ├── every-button-and-rbac-matrix.spec.ts # Master Every-Button Action Synchronization & Strict 6-Role Privilege Matrix ("No Less, No More")
│   ├── 07-multi-parcel-lifecycle-to-delivered.spec.ts # Flow 7: Multi-parcel real lifecycle (registration -> book 6 parcels -> full delivery loop to DELIVERED)
│   ├── 08-merchant-portal-orders.spec.ts # Flow 8: Merchant portal onboarding, Mailpit OTP, REST approval, API order edge cases (14 scenarios) & UI parity
│   ├── 09-merchant-webhook-dispatch-lifecycle.spec.ts # Flow 9: Merchant webhooks, event filtering combinations, multi-window dispatcher assignment & HMAC security
│   ├── 12-dispatcher-portal-exhaustive.spec.ts # Flow 12: Exhaustive Dispatcher portal single-page suite (8 suites: zones, roster, unassign modal, reschedule modal, onboarding modal, retry)
│   ├── 13-dispatcher-cross-portal-sync.spec.ts # Flow 13: Dispatcher & cross-portal live sync (zero mocks: shift toggle, real booking, assignment, unassign eviction, courier out-for-delivery)
│   ├── 14-admin-ops-merchants-pricing.spec.ts  # Flow 14: Master Admin Ops Suite (Merchants CRUD & transitions, pricing rules, users pagination, audit log)
│   ├── edge-01-booking-validation.spec.ts through edge-09-auth-security-and-edge-cases.spec.ts # Complete Edge Case Suite (9 Specs: Booking, Pricing, Fleet, Exceptions, CSV, Security, Tracking, AI, Auth)
│   ├── edge-10-courier-offline-resilience.spec.ts # Edge Suite 10: Courier Offline Resilience, PWA Network Recovery & POD Integrity
│   ├── edge-11-concurrent-dispatch-race.spec.ts   # Edge Suite 11: Concurrent Dispatch Assignment Race & PostgreSQL FOR UPDATE Invariant
│   ├── edge-12-webhook-outage-dlq.spec.ts         # Edge Suite 12: Sustained Webhook Target Outage, Retry Backoff & DLQ Fault-Tolerance
│   └── member-06-merchant.spec.ts # Merchant profile & API key management
│
├── mock-webhook/                  # Standalone Webhook Receiver & Examiner Dashboard (:9999)
│   ├── Dockerfile                 # Alpine container image for deploy/docker-compose.yml
│   ├── package.json               # Microservice dependencies (Express, CORS, tsx)
│   ├── src/
│   │   ├── server.ts              # HMAC-SHA256 receiver, DLQ chaos endpoint, event store
│   │   └── server.test.ts         # In-memory test suite (5/5 passed in 7ms)
│   └── public/
│       └── dashboard.html         # Responsive examiner visual dashboard with live isolation badges
│
├── README.md                      # This document (Authoritative entry point & command reference)
├── LIVING_DOCUMENT.md             # Continuous living engineering journal, ADRs & audit ledger
└── PRE_TESTING_SETUP_ROADMAP.md   # Grounding checklist tracking all 6 quality pillars
```

---

## 5. Multi-Actor Viewport Profiles

To accurately simulate real-world physical devices, tests utilize 5 independent project profiles defined in `playwright.config.ts`:

| Project Profile | Device / Viewport | Touch | Role & Capabilities |
| :--- | :--- | :---: | :--- |
| **`customer-portal`** | Desktop Chrome (`1280 × 800`) | No | End-customer: Fee estimation, parcel booking, public tracking, AI chat |
| **`merchant-portal`** | Desktop Chrome (`1366 × 768`) | No | Merchant Tenant: Bulk shipments, API key management, webhook URL configuration |
| **`dispatcher-board`** | Desktop Chrome (`1440 × 900`) | No | Dispatcher: Live routing board, fleet capacity monitoring, courier assignment |
| **`courier-pwa`** | Google Pixel 7 (`375 × 812`) | **Yes** | Courier: Mobile task list, transit milestones, digital POD signature/photo upload |
| **`admin-portal`** | Desktop Chrome (`1440 × 900`) | No | System Administrator: Delivery zones, pricing tariffs, audit log inspection |

### Context Isolation Guarantee
Playwright tests **never share browser storage, cookies, or JWTs across actors**. Every actor executes inside an isolated `browser.newContext()` via [`fixtures/authFixtures.ts`](./fixtures/authFixtures.ts).

---

## 6. The UI Contract Standard (`data-testid`)

To decouple test stability from frontend CSS and Tailwind redesigns, all DOM queries must target canonical `data-testid` attributes defined in [`helpers/selectors.ts`](./helpers/selectors.ts).

### Contract Categories
- `SELECTORS.AUTH`: `input-email`, `input-password`, `btn-login`
- `SELECTORS.CUSTOMER`: `input-weight`, `btn-get-quote`, `quote-amount`, `btn-submit-booking`, `booking-confirmation-tracking-no`
- `SELECTORS.DISPATCHER`: `dispatch-unassigned-table`, `row-parcel-item`, `select-courier-dropdown`, `btn-assign-courier`
- `SELECTORS.COURIER`: `courier-task-list`, `btn-start-pickup`, `btn-mark-delivered`, `input-pod-signature-canvas`
- `SELECTORS.MERCHANT`: `merchant-api-key-display`, `input-webhook-url`, `btn-save-webhook`
- `SELECTORS.TRACKING`: `input-tracking-number`, `milestone-status-badge`, `sse-connection-indicator`
- `SELECTORS.AI_ASSISTANT`: `ai-chat-drawer`, `input-ai-query`, `btn-send-ai-query`, `ai-grounded-parcel-card`

```typescript
import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

test('sample booking flow', async ({ customerPage }) => {
  await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('2.5');
  await customerPage.locator(testId(SELECTORS.CUSTOMER.BTN_GET_QUOTE)).click();
  await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.QUOTE_AMOUNT))).toBeVisible();
});
```

---

## 7. Standalone Mock Webhook Service (`:9999`)

The mock webhook service acts as a simulated third-party merchant receiver. It receives HTTP webhooks dispatched by Member 8's Event Worker, verifies security, and displays evidence during grading.

### Endpoints

| Endpoint | Method | Purpose |
| :--- | :---: | :--- |
| `/webhooks/merchant-alpha` | `POST` | Validates HMAC-SHA256 using `whsec_alpha_...` and stores under Alpha. |
| `/webhooks/merchant-beta` | `POST` | Validates HMAC-SHA256 using `whsec_beta_...` and stores under Beta. |
| `/webhooks/merchant-chaos` | `POST` | Injects `HTTP 500` to test Event Worker retries and RabbitMQ DLQ routing (`swiftdrop.dlq`). |
| `/webhooks/merchant-slow` | `POST` | Sleeps for 10s to test Event Worker HTTP client timeout enforcement. |
| `/webhooks/*` (any other) | `POST` | Returns native **`HTTP 404 Not Found`** to verify handling of non-existent URLs. |
| `/api/events` | `GET` | Programmatic JSON event retrieval for automated Playwright test assertions. |
| `/api/reset` | `POST` | Flushes event history between test runs to guarantee idempotency. |
| `/dashboard` | `GET` | Responsive visual examiner dashboard with live isolation badges. |

### Examiner Dashboard (`http://localhost:9999/dashboard`)
Run `npm start` to open the visual dashboard. Features include:
- **Tenant Isolation Badge:** Real-time indicator displaying `[TENANT ISOLATION VERIFIED: ZERO DATA LEAKAGE]`.
- **Live Event Stream:** Auto-refreshing table showing CloudEvent IDs, types, timestamps, and HMAC status.
- **Payload Inspector:** Modal displaying full CloudEvents JSON bodies.
- **1-Click Test Triggers:** Interactive buttons to trigger sample delivery webhooks for live presentations.

---

## 8. Academic Grading & Trace Evidence (15-Mark Rubric)

For academic evaluation, this test suite automatically records rich trace evidence configured in `playwright.config.ts`:
- **Video Recordings (`.webm`):** Screen captures of multi-actor journeys saved to `artifacts/videos/`.
- **Playwright Traces (`.zip`):** Full DOM snapshots, console logs, and network waterfalls inspectable via `npx playwright show-trace <trace.zip>`.
- **Visual Failure Screenshots:** Captured automatically on assertion failure.
- **HTML Report:** Self-contained visual report generated at `artifacts/report/index.html`.

---

## 9. Living Documentation & Governance

Per repository engineering standards (`AGENTS.md`), this directory maintains two co-located documents:
- **Entry Point & User Guide (`README.md`):** This document — kept clean, polished, and focused on installation, commands, and architecture.
- **Authoritative Continuous Journal ([`LIVING_DOCUMENT.md`](./LIVING_DOCUMENT.md)):** Detailed architectural decision records (ADRs), 6 domain invariants, minute-by-minute activity logs, and debugging records.
- **Grounding Roadmap ([`PRE_TESTING_SETUP_ROADMAP.md`](./PRE_TESTING_SETUP_ROADMAP.md)): Phased status tracker across all 6 quality pillars.

---

## 10. Measurable Code Coverage & System Test Results

### 10.1 Go Backend Monolith Statement Coverage (`go test -cover`)

Total statements covered across all packages: **52.4% aggregate** with domain core layers reaching **85% - 98%**:

| Package / Domain Layer | Statement Coverage | Test Invariants Enforced |
| :--- | :---: | :--- |
| `internal/admin/domain` | **98.3%** | Delivery zones, role transitions, audit constraints |
| `internal/pricing/infrastructure` | **93.8%** | Dynamic tariffs, postal code matrices, dimensional surcharges |
| `internal/platform/middleware` | **94.1%** | JWT validation, RBAC enforcement, correlation ID propagation |
| `internal/platform/config` | **90.2%** | Environment variable defaults, strict type casting |
| `internal/parcels/domain` | **89.5%** | Aggregate root, parcel state machine, transition invariants |
| `internal/delivery/domain` | **89.3%** | Attempt counter ceilings (max 3), proof-of-delivery attachments |
| `internal/dispatch/domain` | **89.0%** | Fleet capacity checks, on-shift courier verification |
| `internal/pricing/domain` | **88.3%** | Volumetric pricing formulas, weight tiers |
| `internal/events` | **87.5%** | CloudEvents 1.0 JSON schemas, outbox serialization |
| `internal/admin/application` | **87.0%** | Admin governance use cases, tenant approval workflows |
| `internal/pricing/transport` | **86.9%** | Quote request parsing, currency formatting |
| `internal/parcels/transport` | **84.1%** | HTTP handler validation, idempotency key checks |
| `internal/parcels/application` | **83.9%** | Booking transaction boundaries, outbox write atomicity |
| `internal/parcels/infrastructure` | **81.2%** | PostgreSQL persistence, row-level locking queries |
| `internal/pricing/application` | **79.4%** | Tariff lookup, discount rules |
| `internal/admin/transport` | **75.5%** | Admin dashboard REST endpoints |
| `internal/identity/application` | **68.7%** | Password hashing, session refresh, role assignment |
| `cmd/api` | **67.1%** | Dependency injection wiring, graceful shutdown |
| **Monolith Aggregate Total** | **52.4%** | **Authoritative Core API Backend Coverage** |

### 10.2 Real-Time E2E System Test Matrix (Playwright)

| Spec File | Actors / Windows | Verified Flows & Invariants | Result |
| :--- | :---: | :--- | :---: |
| `real-system-12-user-workflows.spec.ts` | **12 Actors across All 6 Roles** (3 Customers, 2 Merchants, 3 Couriers, 2 Dispatchers, 1 Admin, 1 Public Guest) | Complete 5-step serial daily journey: multi-customer booking, real Mailpit signup, merchant API keys & webhooks, multi-order dispatch, courier shifts, door-to-door transit, digital signature POD, failure reason codes & reschedule recovery, admin audit logs, public tracking with SEC-009 PII masking | **PASSED** |
| `realtime-orchestration.spec.ts` | **5 Windows** (Customer A, Applicant B $\rightarrow$ Merchant, Admin, Dispatcher, Courier) | Dynamic registration, booking wizard, merchant application, admin approval, webhook registration, dispatcher assignment, mobile courier run, digital signature POD, wire webhook receiver on `:9999` | **PASSED** |
| `realtime-auth-guardrails.spec.ts` | Courier, Merchant, Customer, Public | Courier IDOR prevention (403), Merchant tenant isolation (403), RBAC dashboard navigation guards, SEC-009 public tracking PII masking | **PASSED** |
| `01-customer-booking.spec.ts` | Customer Portal | Complete 4-step booking wizard, dynamic quote retrieval, booking confirmation | **PASSED** |
| `02-dispatcher-assignment.spec.ts` | Dispatcher Board | Live unassigned parcels list, courier capacity evaluation, atomic assignment | **PASSED** |
| `03-courier-delivery.spec.ts` | Courier Mobile PWA | Route manifest, OUT_FOR_DELIVERY transition, digital signature capture POD | **PASSED** |
| `04-tracking-and-ai.spec.ts` | Public Visitor | SEC-009 sanitized public tracking, AI Assistant grounded delivery inquiries | **PASSED** |
| `05-merchant-delivery-webhook-loop.spec.ts` | Merchant Portal, Dispatcher, Courier | Full-circle delivery cycle, dual attempt retry after failure, 4-stage webhook delivery verification on mock receiver port 9999 | **PASSED** |
| `06-customer-registration-mailpit.spec.ts` | Customer Portal, Mailpit API | End-to-end customer registration flow with real email 6-digit OTP delivery through Mailpit (`:8025`), verification, and immediate login | **PASSED** |
| `07-multi-parcel-lifecycle-to-delivered.spec.ts` | Customer, Dispatcher, Courier | Full real-system lifecycle: dynamic user registration with real Mailpit OTP, booking 6 parcels, dispatcher assignment, courier run initiation, electronic POD signature completion, and verification that all 6 parcels reach DELIVERED status and visually render in the Customer Portal "Recent parcels" table (Zero Mocks) | **PASSED** |
| `08-merchant-portal-orders.spec.ts` | Merchant Portal, Mailpit, Core API | End-to-end merchant lifecycle: customer registration via Mailpit OTP, commercial application, REST approval, API key generation (read-only & full), 14 exhaustive edge cases (auth, validation, idempotency replay/conflict), multi-order generation, interactive UI search/filter verification, and post-revocation refusal | **PASSED** |
| `edge-01` through `edge-08` | Edge Cases (All Portals) | Fallback unmapped postal codes, quote expiration (410), max capacity rejection, 3-attempt ceiling transition, CSV bulk validation, one-time secret modal, SSE degradation, AI adversarial prompt injection | **PASSED** |
| `edge-09-auth-security-and-edge-cases.spec.ts` | Customer Portal (Auth Matrix) | 18-test exhaustive security & edge matrix: SQLi injection, XSS evasion, user enumeration defense, suspended accounts, whitespace normalization, role routing, malformed email gating, duplicate 409 conflict, OTP rate limiting (429), weak password policy, terms gating, OTP attempt ceiling (3 attempts), OTP single-use replay prevention, E.164 phone formats, state reset | **PASSED** |

