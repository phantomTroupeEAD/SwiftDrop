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
│   └── evidence.ts                # Automated full-page screenshot & .zip trace bundler for grading
│
├── flows/                         # End-to-End Journey Specifications
│   └── dry-run.spec.ts            # Offline verification test (10/10 passed)
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
