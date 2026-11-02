# Module 02 — Cross-Workflow Quality & E2E Testing Architecture Specification

> **Module Name:** Cross-Workflow Quality Assurance, System-Wide E2E Testing & Live Demo Infrastructure  
> **Repository Location:** `tests/e2e/` & `tests/e2e/mock-webhook-receiver/`  
> **Assigned Container Port:** `9999` (`mock-webhook`)  
> **Target Folder:** `docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/`  
> **Purpose:** Authoritative architectural specification designed to serve as the direct blueprint for drawing the **Module 02 Quality Assurance & E2E Testing Topology Diagram**. Contains exhaustive technical details on the multi-actor Playwright execution engine, transactional outbox verification pipeline, standalone `mock-webhook:9999` container, real-time HTML dashboard, and chaos penetration architecture without containing visual diagrams.  
> **Specification Version:** `2.0.0` (Authoritative Baseline)  
> **Evaluation Scope:** **15 Marks** (Testing & Verification) + **15 Marks** (Live Demo Readiness)

---

## 1. Quality Framework Topology & Technology Stack

### 1.1 Architecture & Stack Philosophy
The cross-workflow testing system in `tests/e2e/` is built entirely in **Python**, providing a unified, reproducible test harness capable of driving real browser UI sessions, querying asynchronous event brokers, and asserting microservice contracts:

- **Browser Automation:** `pytest-playwright` controlling headless and headed Chromium browser sessions against the React web application (`http://localhost:5173` or `http://traefik:80`).
- **Asynchronous HTTP & Streaming Client:** `httpx` (`>=0.27.0`) for high-concurrency REST endpoint assertions and streaming Server-Sent Events (SSE) tracking streams.
- **Message Broker AMQP Client:** `pika` (`>=1.3.2`) for inspecting RabbitMQ topic exchanges (`swiftdrop.events.topic`), worker consumer queues, and dead-letter queues (`swiftdrop.dlq`).
- **Core Test Runner & Fixtures:** `pytest` (`>=8.2.0`), `pytest-asyncio`, and `pytest-cov` managing test fixtures, session lifecycles, and database seed verifications.

---

## 2. Multi-Actor Test Execution Engine Architecture

The framework coordinates automated testing across 5 distinct business roles, orchestrating real browser sessions and API calls to prove full lifecycle progression:

```
tests/e2e/
├── ui/                                # Browser End-to-End Test Suites (Playwright)
│   ├── test_scenario_1a_customer.py  # Scenario 1A: Pure Web Customer (Core Rubric — No AI Bot)
│   ├── test_scenario_3_courier.py   # Scenario 3: Dispatcher Assignment & Courier Proof Upload
│   ├── test_scenario_4_ai_bot.py    # Scenario 4: AI Delivery Assistant & FastMCP Execution
│   └── test_ui_chaos.py             # UI Chaos: Double-clicks, invalid form boundaries
├── api/                               # Asynchronous & Security Test Suites (HTTPX + Pika)
│   ├── test_merchant_webhooks.py    # Scenario 2: Merchant B2B, RabbitMQ & DLQ Verification
│   ├── test_security_idor.py        # Scenario 5: Cross-Cutting IDOR & State Transition Guards
│   └── test_chaos_and_vulnerabilities.py # 16 Automated System Chaos & Attack Penetrations
├── fixtures/                          # Reusable Test Fixtures & Deterministic Accounts
│   ├── test_users.py                # Pre-seeded customer, courier, and dispatcher tokens
│   └── sample_proof.jpg             # Standardized proof photo for courier delivery verification
├── mock-webhook-receiver/             # Standalone Dedicated Test Container Artifact (:9999)
│   ├── server.py                    # Multi-tenant webhook receiver & SSE dashboard server
│   ├── templates/dashboard.html     # Real-time Tailwind visual dashboard UI
│   └── Dockerfile                   # Lightweight container packaging for Docker Compose
└── requirements-test.txt              # Test harness dependencies
```

### 2.1 Scenario 1A: Pure Web Customer Journey (Core Rubric — Zero AI Bot)
- **Objective:** Fulfills the assignment's primary customer journey requirement completely through the React UI without touching the AI service.
- **Automated Workflow:**
  1. Playwright opens Chromium browser; navigates to `/login`.
  2. Authenticates using Customer A credentials (`customer.a@swiftdrop.lk` / `Password123!`); asserts session token saved.
  3. Navigates to `/portal/book` (aliased `/customer/book`).
  4. Selects `ZONE_COLOMBO_01` $\to$ `ZONE_KANDY_01`, enters weight 2.5 kg, dimensions (25x15x10 cm).
  5. Clicks `[data-testid="booking-estimate-btn"]`; asserts `[data-testid="booking-estimated-fee-display"]` displays `"LKR 650.00"` within 2 seconds.
  6. Enters recipient details; clicks `[data-testid="booking-submit-btn"]` with auto-generated client `Idempotency-Key`.
  7. Extracts tracking number from confirmation card (`SD-\d{8}-[A-Za-z0-9]+`).
  8. Navigates to `/tracking/{trackingNumber}`; connects to SSE live stream; asserts badge reads `"CREATED"`.

### 2.2 Scenario 2: Merchant B2B, RabbitMQ & Webhook Pipeline
- **Objective:** Validates automated merchant order generation, transactional outbox commitments, RabbitMQ topic exchange publication, Event Worker webhook dispatches, and dead-letter queue routing.
- **Automated Workflow:**
  1. Flushes mock webhook receiver memory: `POST http://localhost:9999/api/reset`.
  2. Merchant Alpha submits order via `POST /api/v1/merchant/orders` with header `X-API-Key: test_key_alpha`.
  3. Asserts Core API returns `201 Created` with tracking number `SD-MERCH-001`.
  4. Inspects PostgreSQL: verifies `parcels` and `outbox_events` (`parcel.booked.v1`) committed atomically.
  5. Inspects RabbitMQ exchange `swiftdrop.events.topic`: verifies CloudEvent published by Outbox Relay within 3 seconds.
  6. Event Worker dispatches CloudEvent to `http://mock-webhook:9999/webhooks/merchant-alpha` with header `X-SwiftDrop-Signature`.
  7. Mock webhook receiver verifies HMAC-SHA256 signature; logs event in memory; responds `200 OK`.
  8. **Strict Multi-Tenant Isolation Check:** Queries `GET http://localhost:9999/api/merchants/mer_beta/events` $\rightarrow$ Asserts Merchant Beta received **ZERO events**.
  9. **DLQ Failure Check:** Merchant Chaos dispatches to `merchant-chaos` (responding 500); worker retries with compressed backoff (`500ms, 1000ms, 2000ms`); asserts message routed to `swiftdrop.dlq`.

### 2.3 Scenario 3: Dispatcher Assignment & Courier Proof-of-Delivery
- **Objective:** Proves operational parcel state progression from `CREATED` $\to$ `ASSIGNED` $\to$ `OUT_FOR_DELIVERY` $\to$ `DELIVERED` with mandatory photo proof.
- **Automated Workflow:**
  1. Playwright logs in as Dispatcher (`dispatcher@swiftdrop.lk`).
  2. Opens `/ops/dispatch`; locates unassigned parcel; selects Courier 1 (`usr_courier_bob`); clicks assign.
  3. Asserts parcel status advances to `ASSIGNED`.
  4. Logs out; logs in as Courier 1 (`courier.kandy@swiftdrop.lk`).
  5. Opens mobile web portal `/courier/deliveries`; asserts parcel appears in daily route list.
  6. Advances status to `OUT_FOR_DELIVERY` via status dropdown.
  7. Attaches mock photo proof (`sample_proof.jpg`) via file input locator.
  8. Enters recipient signatory name `"Sunil Shantha"`; submits proof.
  9. Asserts status transitions to `DELIVERED`.

### 2.4 Scenario 4: AI Delivery Assistant & FastMCP Conversational Flow (Bonus)
- **Objective:** Demonstrates conversational tracking, tariff quoting, 1-click booking pre-fill, and external FastMCP tool inspection (**+5 Bonus Marks**).
- **Automated Workflow:**
  1. Playwright opens chat drawer via `[data-testid="ai-chat-toggle-btn"]`.
  2. Queries fee for 2.5 kg box from Colombo to Kandy.
  3. Asserts assistant bubble displays LKR 650.00 fee.
  4. Asserts pre-filled action card button `[data-testid="ai-chat-action-btn"]` is rendered.
  5. Clicks action button; verifies browser redirects to `/portal/book` with form fields populated.
  6. In-process FastMCP Python client runs `tools/call` for `track_parcel_status`; asserts structured JSON output.

### 2.5 Scenario 5: Cross-Cutting Security & IDOR Invariants
- **Objective:** Validates object-level authorization, courier route boundaries, and state transition immutability.
- **Automated Workflow:**
  1. Customer IDOR Test: Customer A attempts to fetch Customer B's parcel (`403 Forbidden`).
  2. Courier Route Isolation Test: Courier 2 attempts to deliver Courier 1's parcel (`403 Forbidden`).
  3. Illegal State Transition Test: Attempts to advance parcel directly from `CREATED` to `DELIVERED` (`409 Conflict`).

---

## 3. Standalone Multi-Tenant Mock Webhook Container (`mock-webhook:9999`)

During live grading demonstrations and automated CI workflows, asynchronous background processing through message brokers is invisible. To provide irrefutable proof of event delivery and tenant security, Member 10 deploys a dedicated test artifact container:

### 3.1 Network Topology & Container Deployment
- **Container Service Name:** `mock-webhook`
- **Container Listening Port:** Internal & Host Port `9999` (`0.0.0.0:9999`).
- **Network Placement:** Attached directly to Docker Compose network `swiftdrop-network`.
- **SSRF Test-Mode Integration:**
  - In production, Member 8's Event Worker enforces strict HTTPS on port 443 and blocks private IP ranges.
  - In dev/test environments, Member 8 supports dev-mode flags:
    - `WEBHOOK_ALLOW_INSECURE_HTTP=true` (allows HTTP protocol for local hosts)
    - `WEBHOOK_ALLOWED_HOSTS=mock-webhook,localhost,127.0.0.1` (whitelists test container hostname)
  - Allows Event Worker to dispatch webhooks across the Docker bridge to `http://mock-webhook:9999/webhooks/...` without violating production security guardrails.

### 3.2 Multi-Tenant Partitioning
The mock receiver provisions distinct endpoints and signing secrets for separate test tenants:
1. **Merchant Alpha (Apex Retail):**
   - Tenant ID: `mer_alpha_11111111-1111-1111-1111-111111111111`
   - Target Endpoint: `POST /webhooks/merchant-alpha`
   - Shared Signing Secret: `whsec_alpha_test_secret_32_bytes_len!`
   - Behavior: Verifies HMAC-SHA256 signature; logs event in Alpha memory partition; responds `200 OK`.
2. **Merchant Beta (Zenith Logistics):**
   - Tenant ID: `mer_beta_22222222-2222-2222-2222-222222222222`
   - Target Endpoint: `POST /webhooks/merchant-beta`
   - Shared Signing Secret: `whsec_beta_test_secret_32_bytes_leng!`
   - Behavior: Distinct memory partition. Used by Playwright to assert that 0 events are received when Merchant Alpha transacts.
3. **Chaos Merchant (Failure & DLQ Simulation):**
   - Target Endpoint: `POST /webhooks/merchant-chaos`
   - Behavior: Always responds with HTTP `500 Internal Server Error`. Used to trigger Event Worker retry exhaustion and verify routing into `swiftdrop.dlq`.
4. **Latency Merchant (Timeout Simulation):**
   - Target Endpoint: `POST /webhooks/merchant-slow`
   - Behavior: Sleeps for 10 seconds before responding, verifying that the Event Worker enforces its 5-second socket timeout and releases worker threads safely.

---

## 4. Real-Time Visual HTML Dashboard Architecture

The mock webhook container serves an interactive, real-time visual dashboard at `http://localhost:9999/dashboard`:

- **Real-Time Streaming / Polling:** Browser UI automatically polls `/api/events` every 2 seconds or listens to Server-Sent Events stream (`/api/stream`).
- **Tabbed Multi-Tenant Feeds:** Separate visual tabs for Merchant Alpha Feed, Merchant Beta Feed, and Chaos DLQ Log.
- **The Tenant Isolation Proof Indicator:**
  - Displays a prominent green verification card: `[TENANT ISOLATION VERIFIED: ZERO DATA LEAKAGE]`.
  - Proves that when Merchant Alpha creates an order, Merchant Alpha's event counter increments to `1`, while Merchant Beta's event counter remains strictly at `0`.
- **HMAC Cryptographic Verification Badges:** Each captured event card displays a green badge: `[HMAC-SHA256: VALID]`, showing the received signature vs computed signature.
- **CloudEvent Payload Accordions:** Evaluators can click to expand the full CloudEvent JSON envelope (`specversion`, `type`, `source`, `data.tracking_number`) and delivery headers (`X-SwiftDrop-Delivery-Attempt`).
- **Live Evidence Capture:** Playwright test suites navigate to `http://localhost:9999/dashboard` at the end of Scenario 2 and capture a full-page PNG screenshot (`test-results/artifacts/webhook-isolation-proof.png`) as verifiable submission proof for the **15-mark live demo rubric**.

---

## 5. Transactional Outbox, Broker & DLQ Verification Pipeline

The quality framework validates the complete asynchronous reliability pipeline:

1. **Transactional Outbox Commitment Check:**
   - Verifies that status updates and outbox events commit in the **exact same database transaction**. If the parcel record fails validation, zero outbox rows exist.
2. **Outbox Relay Durability Check:**
   - Outbox Relay polls rows where `published = false`.
   - Publishes CloudEvent to RabbitMQ topic exchange `swiftdrop.events.topic`.
   - Updates `published = true` only after receiving broker ACK.
3. **Consumer Idempotency Check:**
   - Test framework publishes duplicate message IDs.
   - Verifies Event Worker checks `processed_events` table; duplicate messages are acknowledged without re-dispatching duplicate webhooks or customer notifications.
4. **CI Compressed Retry Schedule:**
   - Production retry backoff spans ~42 minutes (30s, 2m, 8m, 32m).
   - In CI/test environments, Member 8 configures `WEBHOOK_RETRY_INTERVALS_MS=500,1000,2000`.
   - The test harness asserts all 3 retry attempts and subsequent routing to `swiftdrop.dlq` in under 5 seconds.

---

## 6. 16 Mandatory SRS Acceptance Tests Architecture (AT-001 – AT-016)

The quality harness provides automated verification for all 16 SRS Acceptance Tests:

- `AT-001`: Customer register/login, quote preview, and parcel booking verified via Playwright UI.
- `AT-002`: Negative weight and missing address rejection verified; asserts 0 database rows created.
- `AT-003`: Idempotency verification: replaying same key returns cached parcel with `Idempotent-Replay: true`.
- `AT-004`: Tenant IDOR isolation: Customer A reading Customer B returns `403 Forbidden`.
- `AT-005`: Public tracking milestone sanitization: asserts absence of street address and phone number.
- `AT-006`: Courier isolation: unauthorized courier updating another courier's parcel returns `403 Forbidden`.
- `AT-007`: State machine guard: invalid transition `CREATED` $\to$ `DELIVERED` returns `409 Conflict`.
- `AT-008`: Transactional outbox atomicity: verifies parcel and outbox rows committed together.
- `AT-009`: Outbox relay confirmation: verifies published flag set only after broker ACK.
- `AT-010`: Consumer idempotency: duplicate RabbitMQ message results in zero duplicate side effects.
- `AT-011`: Merchant isolation: suspended merchant order creation rejected with `403 Forbidden`.
- `AT-012`: Webhook signature & retry: valid HMAC verified; 500 error triggers backoff retry to DLQ.
- `AT-013`: Dispatcher concurrency: simultaneous assignments handled via optimistic locking (`409 Conflict`).
- `AT-014`: Google OIDC validation: forged state/nonce rejected safely with `401 Unauthorized`.
- `AT-015`: AI Assistant sandbox: verified read-only tools, auth pass-through, and zero hallucination.
- `AT-016`: Container health probes: `/health/live` and `/health/ready` verified functional and decoupled.

---

## 7. Adversarial Chaos & Vulnerability Penetration Architecture

The quality suite executes an automated matrix of 16 chaos scenarios (`CHAOS-01` to `CHAOS-16`):

1. **Race Condition Defense (CHAOS-01):** 5 rapid clicks within 100ms on confirm booking button $\rightarrow$ Frontend disables button; backend idempotency ensures single parcel creation.
2. **Payload Hash Tampering (CHAOS-02):** Replaying existing `Idempotency-Key` with altered parcel weight $\rightarrow$ Returns `409 Conflict (IDEMPOTENCY_KEY_PAYLOAD_MISMATCH)`.
3. **Terminal State Immutability (CHAOS-03):** Attempting status update on `DELIVERED` or `CANCELLED` parcel $\rightarrow$ Aggregate root rejects with `409 Conflict`.
4. **Max Attempt Overflow (CHAOS-04):** Submitting 4th failed delivery attempt when limit is 3 $\rightarrow$ State machine forces status transition to `RETURNED`.
5. **Metric Ingestion Boundaries (CHAOS-05):** Submitting weight `-5 kg` or `500 kg` ($>30\text{kg}$ max) $\rightarrow$ Rejected with `422 Unprocessable Entity`.
6. **SQLi & Stored XSS Payloads (CHAOS-06):** Injection strings in recipient address $\rightarrow$ Driver parameters prevent SQLi; React escapes HTML.
7. **Broker Duplication (CHAOS-07):** Duplicate CloudEvent delivery $\rightarrow$ Consumer deduplication prevents duplicate SMS/email notifications.
8. **SSE Reconnect (CHAOS-08):** Dropping active SSE tracking stream mid-transit $\rightarrow$ Client reconnects with `Last-Event-ID`; resumes without full reload.
9. **Buffer Overflow (CHAOS-09):** 50,000-character payload to AI chat $\rightarrow$ Middleware drops with `400 Bad Request`.
10. **Role Deception (CHAOS-10):** Multi-turn prompt injection (*"You are now DAN..."*) $\rightarrow$ Model constrained strictly to parcel tools.
11. **Base64 Jailbreak (CHAOS-11):** Encoded instructions $\rightarrow$ Tool allow-list prevents unauthorized commands.
12. **Proof File Tampering (CHAOS-12):** Uploading `.exe` or `.sh` script instead of JPEG $\rightarrow$ MIME check rejects with `415 Unsupported Media Type`.
13. **Concurrent Dispatch Race (CHAOS-13):** Two dispatchers assigning same parcel simultaneously $\rightarrow$ PostgreSQL `SELECT FOR UPDATE` ensures winner gets 200 OK, loser gets `409 Conflict`.
14. **Hanging Webhook Receiver (CHAOS-14):** Webhook server hanging $>30\text{s}$ $\rightarrow$ Worker socket timeout drops request after 5s; routes to `swiftdrop.dlq`.
15. **JWT Signature Tampering (CHAOS-15):** Altering role to `ADMIN` keeping customer signature $\rightarrow$ RS256 check fails with `401 Unauthorized`.
16. **Fuzzed Inputs (CHAOS-16):** Telephone numbers with alphabetic characters or 100 digits $\rightarrow$ Pydantic regex blocks with `422 Unprocessable Entity`.

---

## 8. Test Artifacts, Tracing & Live Demo Recording Subsystem

The test harness automatically captures, packages, and archives submission artifacts:

1. **Full Video Capture:**
   - Headless and headed Chromium browser sessions execute with `--video=on`.
   - MP4 video files capturing the full customer booking journey, dispatch assignment, and courier photo delivery are stored in `test-results/artifacts/videos/`.
2. **Interactive Playwright Traces:**
   - Executed with `--tracing=on`.
   - Generates `trace.zip` containing complete DOM snapshots, network waterfalls, console logs, and action timings. Evaluators can inspect every millisecond via `playwright show-trace trace.zip`.
3. **Failure Screenshots:**
   - Any assertion failure automatically triggers a full-page PNG screenshot with highlighted locator bounding boxes.
4. **Live Dashboard Visual Proof:**
   - Automatic screenshot of `http://localhost:9999/dashboard` capturing the green `[TENANT ISOLATION VERIFIED]` card and CloudEvent accordions.

---

## 9. Diagram 2 Visual Layout & Construction Guide

When drawing the **Module 02 Quality Assurance & E2E Testing Topology Diagram**, structure the visual layout as follows:

1. **Left Side: Test Orchestrator (`tests/e2e/` Test Runner Boundary):**
   - Box for `Pytest Test Harness`.
   - Inside, place:
     - `Playwright Browser Runner` (controlling Customer Browser, Dispatcher Browser, Courier Mobile Browser).
     - `HTTPX Async REST & SSE Client`.
     - `Pika RabbitMQ Client`.
     - `Chaos & Security Attack Engine`.
     - `Artifact Recorder` (producing `video.mp4`, `trace.zip`, and screenshots).
2. **Center: System Under Test (The 10-Member SwiftDrop Topology):**
   - Box for `React Web Application (:5173)` (Customer Portal, Dispatch Board, Courier View).
   - Box for `Traefik API Gateway (:80)`.
   - Box for `Core API Modular Monolith (:8080)`.
   - Box for `Tracking API (:8081)`.
   - Box for `PostgreSQL Database` showing `parcels` and `outbox_events` tables.
   - Box for `Outbox Relay Daemon`.
   - Box for `RabbitMQ Broker` showing `swiftdrop.events.topic` exchange and `swiftdrop.dlq`.
   - Box for `Event Worker Consumer` showing `processed_events` deduplication.
3. **Right Side: Dedicated Test Artifact (`mock-webhook:9999` Container Boundary):**
   - Box for `mock-webhook Container (:9999)` attached to `swiftdrop-network`.
   - Inside, place:
     - `Receiver Endpoints`: `/webhooks/merchant-alpha` (Apex Retail), `/webhooks/merchant-beta` (Zenith Logistics), `/webhooks/merchant-chaos` (500 Error).
     - `HMAC-SHA256 Verification Engine`.
     - `In-Memory Partitioned Event Store`.
     - `Real-Time HTML Dashboard Engine (/dashboard)` with arrows pointing to Evaluator Browser.
4. **Directional Interaction Flows:**
   - Flow 1 (Customer UI): Playwright Customer $\to$ React UI $\to$ Core API $\to$ Tracking API SSE.
   - Flow 2 (Dispatcher & Courier): Playwright Dispatcher/Courier $\to$ React Ops $\to$ Core API.
   - Flow 3 (Async Webhook & DLQ): Core API $\to$ Outbox Table $\to$ Relay $\to$ RabbitMQ $\to$ Event Worker $\to$ `mock-webhook:9999`.
   - Flow 4 (Isolation Verification): Playwright $\to$ queries `/api/merchants/mer_beta/events` on `mock-webhook` asserting 0 events received.

---
*End of Module 02 Architecture Specification.*
