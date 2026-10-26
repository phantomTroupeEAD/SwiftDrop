# Module 02 — Cross-Workflow Quality & E2E Testing Activity Flows Specification

> **Module Name:** Cross-Workflow Quality Assurance, System-Wide E2E Testing & Live Demo Infrastructure  
> **Repository Location:** `tests/e2e/` & `tests/e2e/mock-webhook-receiver/`  
> **Target Folder:** `docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/`  
> **Purpose:** Complete, granular textual specification of all system activities, control flows, decision branches, concurrent forks/joins, and terminal states for Module 02. Designed specifically to provide a diagram designer with every detail required to construct formal **UML Activity Diagrams** (with swimlanes/partitions) without containing visual diagrams.  
> **Specification Version:** `2.0.0` (Authoritative Baseline)  
> **Evaluation Scope:** **15 Marks** (Testing & Verification) + **15 Marks** (Live Demo Readiness)

---

## Table of Contents
1. [Activity Modeling Principles & Swimlane Notation Guide](#1-activity-modeling-principles--swimlane-notation-guide)
2. [Primary Activity Flow 1: Multi-Actor End-to-End Delivery & Verification Lifecycle](#2-primary-activity-flow-1-multi-actor-end-to-end-delivery--verification-lifecycle)
3. [Primary Activity Flow 2: Asynchronous Outbox, RabbitMQ, Webhook & DLQ Pipeline Activity](#3-primary-activity-flow-2-asynchronous-outbox-rabbitmq-webhook--dlq-pipeline-activity)
4. [Primary Activity Flow 3: Automated Chaos & Invariant Penetration Testing Activity](#4-primary-activity-flow-3-automated-chaos--invariant-penetration-testing-activity)
5. [Primary Activity Flow 4: Automated Test Evidence & Trace Capture Lifecycle Activity](#5-primary-activity-flow-4-automated-test-evidence--trace-capture-lifecycle-activity)
6. [Comprehensive UML Element Dictionary for Diagram 2](#6-comprehensive-uml-element-dictionary-for-diagram-2)

---

## 1. Activity Modeling Principles & Swimlane Notation Guide

When converting the specifications in this document into a visual UML Activity Diagram, the diagram designer must organize activities into the following **8 vertical or horizontal Swimlanes (Partitions)**:

1. **`Playwright / Pytest Test Orchestrator`:** The master Python test harness controlling browser contexts, HTTPX requests, and AMQP inspections.
2. **`React Web Portal (:5173)`:** The frontend web application (Customer Portal, Dispatch Board, Courier Mobile View).
3. **`Core API Modular Monolith (:8080)`:** Identity, Parcels, Pricing, Dispatch, and Merchants domain controllers.
4. **`PostgreSQL Database`:** Authoritative business store (`parcels`, `parcel_status_history`, `outbox_events`).
5. **`Outbox Relay Daemon`:** Transactional outbox background poller publishing events to RabbitMQ.
6. **`RabbitMQ Broker`:** Topic exchange (`swiftdrop.events.topic`) and dead-letter queue (`swiftdrop.dlq`).
7. **`Event Worker Consumer`:** Asynchronous consumer verifying idempotency and dispatching signed webhooks.
8. **`Mock Webhook Container & Dashboard (:9999)`:** Dedicated multi-tenant test container capturing webhooks and serving the visual HTML dashboard.

---

## 2. Primary Activity Flow 1: Multi-Actor End-to-End Delivery & Verification Lifecycle

This activity flow models the complete multi-actor journey across Customer, Dispatcher, and Courier boundaries, proving state progression from booking to proof of delivery.

### 2.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** Playwright test runner launches Scenario 1A and Scenario 3 suites.
- **Preconditions:** All microservices running (`make up`); seed accounts loaded (`make seed`).

### 2.2 Step-by-Step Activity Flow

#### Phase 1: Customer Booking & Live Tracking (Scenario 1A — Zero AI Bot)
1. **Activity (`Playwright Test Orchestrator`):** Launch headless Chromium browser. Navigate to `/login`.
2. **Activity (`React Web Portal`):** Render login form. Ingest Customer A credentials (`customer.a@swiftdrop.lk` / `Password123!`); submit.
3. **Activity (`Core API`):** Verify password hash. Issue signed asymmetric RS256 Bearer JWT.
4. **Activity (`React Web Portal`):** Store JWT in `localStorage`. Redirect browser to `/portal/dashboard`.
5. **Activity (`Playwright Test Orchestrator`):** Navigate to `/portal/book` (aliased `/customer/book`).
6. **Activity (`React Web Portal`):** Ingest origin zone (`ZONE_COLOMBO_01`), destination zone (`ZONE_KANDY_01`), and parcel dimensions (2.5 kg). Click `[data-testid="booking-estimate-btn"]`.
7. **Activity (`Core API`):** Compute tariff quote via pricing engine. Return HTTP `200 OK {"fee_cents": 65000}`.
8. **Activity (`React Web Portal`):** Render `"LKR 650.00"` in estimated fee badge. Ingest recipient name and street address.
9. **Activity (`Playwright Test Orchestrator`):** Generate client UUIDv7 `Idempotency-Key`. Click `[data-testid="booking-submit-btn"]`.
10. **Activity (`Core API`):** Execute atomic transaction: insert `parcels` record (status `CREATED`) and insert `outbox_events` record (`parcel.booked.v1`). Return HTTP `201 Created` with tracking number `SD-20261021-04921`.
11. **Activity (`React Web Portal`):** Render confirmation card. Playwright captures tracking number from DOM locator `[data-testid="booking-success-tracking-number"]`.
12. **Activity (`Playwright Test Orchestrator`):** Navigate to `/tracking/{trackingNumber}`.
13. **Activity (`React Web Portal`):** Establish SSE live tracking stream with Tracking API (`GET /api/v1/tracking/stream/{trackingNumber}`).
14. **Activity (`React Web Portal`):** Receive initial milestone event. Verify status badge reads `"CREATED"`.

#### Phase 2: Dispatcher Assignment (Scenario 3)
15. **Activity (`Playwright Test Orchestrator`):** Open new browser session. Navigate to `/login`.
16. **Activity (`React Web Portal`):** Authenticate Dispatcher (`dispatcher@swiftdrop.lk`). Redirect to `/ops/dispatch`.
17. **Activity (`React Web Portal`):** Query Core API `GET /api/v1/dispatch/unassigned`. Render unassigned parcels table.
18. **Activity (`Playwright Test Orchestrator`):** Select target parcel row; select Courier 1 (`usr_courier_bob`) from dropdown; click `[data-testid="dispatch-assign-btn"]`.
19. **Activity (`Core API`):** Initiate database transaction. Execute row-level lock (`SELECT FOR UPDATE`). Verify parcel status is `CREATED`.
20. **Activity (`PostgreSQL Database`):** Update parcel record: `courier_id = 'usr_courier_bob'`, `status = 'ASSIGNED'`. Insert audit record into `parcel_status_history`. Insert outbox event `parcel.assigned`. Commit transaction.
21. **Activity (`React Web Portal`):** Render assignment success toast notification.

#### Phase 3: Courier Delivery & Proof Upload (Scenario 3)
22. **Activity (`Playwright Test Orchestrator`):** Open mobile browser viewport. Navigate to `/login`.
23. **Activity (`React Web Portal`):** Authenticate Courier 1 (`courier.kandy@swiftdrop.lk`). Navigate to `/courier/deliveries`.
24. **Activity (`React Web Portal`):** Fetch assigned deliveries from Core API. Verify target parcel appears in daily route list.
25. **Activity (`Playwright Test Orchestrator`):** Select parcel card; select `OUT_FOR_DELIVERY` from status dropdown; click submit.
26. **Activity (`Core API`):** Verify caller is assigned courier. Update status to `OUT_FOR_DELIVERY`. Insert status history and outbox event.
27. **Activity (`Playwright Test Orchestrator`):** Attach test delivery photo (`sample_proof.jpg`) via file input locator. Enter recipient signatory name `"Sunil Shantha"`.
28. **Activity (`Playwright Test Orchestrator`):** Click `[data-testid="courier-submit-proof-btn"]`.
29. **Activity (`Core API`):** Validate proof photo URL and signatory name presence.
    - **Decision Diamond:** *Are proof photo and recipient name present?*
      - *[No]:* Reject status transition with `422 Unprocessable Entity` (`PROOF_ATTACHMENT_REQUIRED`).
      - *[Yes]:* Advance status to `DELIVERED`. Insert completion record into `parcel_status_history`. Atomically commit transaction.
30. **Activity (`React Web Portal`):** Update tracking SSE stream in Customer browser session: status badge updates dynamically to `"DELIVERED"`.
31. **Terminal Node ($\bigodot$):** Multi-actor end-to-end lifecycle verified successfully.

---

## 3. Primary Activity Flow 2: Asynchronous Outbox, RabbitMQ, Webhook & DLQ Pipeline Activity

This activity flow models the asynchronous event publishing pipeline, verifying transactional reliability, HMAC-SHA256 signature generation, multi-tenant isolation, and dead-letter queue routing.

### 3.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** Pytest test runner triggers Scenario 2 (`test_merchant_webhooks.py`).
- **Preconditions:** Core API, PostgreSQL, Outbox Relay, RabbitMQ, Event Worker, and `mock-webhook:9999` are active.

### 3.2 Step-by-Step Activity Flow

1. **Activity (`Playwright / Pytest Test Orchestrator`):** Issue HTTP POST to `http://localhost:9999/api/reset`.
2. **Activity (`Mock Webhook Container & Dashboard`):** Flush all in-memory event stores, tenant feeds, and delivery counters. Respond `200 OK`.
3. **Activity (`Playwright / Pytest Test Orchestrator`):** Dispatch HTTP POST to Core API: `POST /api/v1/merchant/orders` with header `X-API-Key: test_key_alpha` (Merchant Alpha: Apex Retail).
4. **Activity (`Core API`):** Authenticate API key. Validate tenant status is `ACTIVE`.
5. **Activity (`PostgreSQL Database`):** Execute atomic transaction:
   - Insert new parcel record with `tenant_id = 'mer_alpha_...'`.
   - Insert outbox event record into `outbox_events` (`event_type = 'com.swiftdrop.parcel.booked.v1'`, `payload = {...}`, `published = false`).
   - Commit transaction atomically.
6. **Activity (`Core API`):** Return HTTP `201 Created` with tracking number `SD-MERCH-001`.
7. **Activity (`Outbox Relay Daemon`):** Poller wakes up on 500ms interval. Executes `SELECT * FROM outbox_events WHERE published = false ORDER BY created_at FOR UPDATE SKIP LOCKED`.
8. **Activity (`Outbox Relay Daemon`):** Format database row into standard CloudEvents 1.0 JSON envelope.
9. **Activity (`RabbitMQ Broker`):** Receive AMQP `basic.publish` to topic exchange `swiftdrop.events.topic` with routing key `parcel.booked.v1`.
10. **Activity (`RabbitMQ Broker`):** Route message to durable queue `q.webhooks.dispatcher`. Send publisher ACK back to Outbox Relay.
11. **Activity (`PostgreSQL Database`):** Outbox Relay receives broker ACK. Executes `UPDATE outbox_events SET published = true WHERE id = $1`.
12. **Activity (`Event Worker Consumer`):** Worker daemon consumes message from queue `q.webhooks.dispatcher`.
13. **Activity (`Event Worker Consumer`):** Extract `event_id`. Inspect deduplication table: `SELECT 1 FROM processed_events WHERE event_id = $1`.
    - **Decision Diamond:** *Has event already been processed?*
      - *[Yes (Duplicate Message)]:* Acknowledge message immediately (`basic.ack`). Exit branch.
      - *[No (New Event)]:* Insert `event_id` into `processed_events`. Retrieve merchant webhook URL and secret key.
14. **Activity (`Event Worker Consumer`):** Compute HMAC-SHA256 signature using Merchant Alpha secret (`whsec_alpha_...`):
    `signature = "sha256=" + hmac_sha256(secret, payload_bytes).hex()`.
15. **Activity (`Event Worker Consumer`):** Dispatch HTTP POST across internal Docker bridge to `http://mock-webhook:9999/webhooks/merchant-alpha` with headers:
    - `Content-Type: application/json`
    - `X-SwiftDrop-Signature: sha256=...`
    - `X-SwiftDrop-Delivery-Attempt: 1`
16. **Activity (`Mock Webhook Container & Dashboard`):** Ingest webhook payload. Extract signature header.
    - Compute local HMAC digest using registered Alpha secret.
    - **Decision Diamond:** *Does received HMAC match computed HMAC?*
      - *[No (Signature Mismatch)]:* Return HTTP `401 Unauthorized`. Log error.
      - *[Yes (Valid HMAC)]:* Record event in Alpha memory store. Respond HTTP `200 OK`.
17. **Activity (`Mock Webhook Container & Dashboard`):** Update real-time visual dashboard (`/dashboard`):
    - Increment Merchant Alpha event counter to `1`.
    - Render received CloudEvent accordion with valid signature badge.
18. **Activity (`Playwright / Pytest Test Orchestrator`):** Execute **Strict Multi-Tenant Isolation Check**:
    - Query `GET http://localhost:9999/api/merchants/mer_beta/events`.
    - **Decision Diamond:** *Did Merchant Beta receive 0 events?*
      - *[No (Data Leaked)]:* Test assertion fails with `AssertionError: Tenant isolation violated`.
      - *[Yes (Strict Isolation)]:* Test assertion passes. Dashboard renders highlighted green badge: `[TENANT ISOLATION VERIFIED: ZERO DATA LEAKAGE]`.

#### Phase 2: Failure & Dead-Letter Queue (DLQ) Verification
19. **Activity (`Playwright / Pytest Test Orchestrator`):** Trigger order creation for Merchant Chaos (`mer_chaos_...`).
20. **Activity (`Event Worker Consumer`):** Dispatch webhook to `http://mock-webhook:9999/webhooks/merchant-chaos`.
21. **Activity (`Mock Webhook Container & Dashboard`):** Chaos endpoint responds with HTTP `500 Internal Server Error`.
22. **Activity (`Event Worker Consumer`):** Detect HTTP 500 error. Check delivery attempt counter.
    - **Decision Diamond:** *Is delivery attempt $< 3$?*
      - *[Yes (Attempts 1 & 2)]:* Apply compressed backoff delay (`500ms`, `1000ms`). Increment attempt counter. Retry HTTP POST.
      - *[No (Attempt 3 Failed)]:* Exhaust retry policy.
23. **Activity (`RabbitMQ Broker`):** Event Worker publishes message to dead-letter exchange `swiftdrop.dlq.exchange` routed to queue `swiftdrop.dlq`.
24. **Activity (`Playwright / Pytest Test Orchestrator`):** Pytest Pika client inspects queue `swiftdrop.dlq`. Asserts failed CloudEvent is present.
25. **Terminal Node ($\bigodot$):** Async reliability and DLQ routing verified.

---

## 4. Primary Activity Flow 3: Automated Chaos & Invariant Penetration Testing Activity

This activity flow models the automated execution of the 16 adversarial penetration and edge-case attacks.

### 4.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** Pytest executes `test_chaos_and_vulnerabilities.py`.
- **Preconditions:** System operational.

### 4.2 Step-by-Step Activity Flow

1. **Activity (`Playwright / Pytest Test Orchestrator`):** Loop through chaos test matrix (`CHAOS-01` to `CHAOS-16`).
2. **Activity (`Core API & Security Middleware`):** Ingest adversarial payload.
   - **Branch Matrix (Execution per Attack Type):**
     - *Attack 1 (Rapid Double-Click Race - CHAOS-01):* Ingest 5 concurrent booking requests with same `Idempotency-Key` within 100ms. Deduplication filter ensures only 1 request proceeds to SQL insert; remaining 4 return cached record with header `Idempotent-Replay: true`.
     - *Attack 2 (Idempotency Payload Tampering - CHAOS-02):* Ingest cached `Idempotency-Key` with modified parcel weight. Backend computes SHA-256 payload hash mismatch; aborts with `409 Conflict`.
     - *Attack 3 (Terminal State Mutation - CHAOS-03):* Ingest status patch on parcel already marked `DELIVERED`. Domain aggregate root checks status; rejects with `409 Conflict`.
     - *Attack 4 (Attempt Limit Overflow - CHAOS-04):* Ingest 4th delivery attempt failure when limit is 3. State machine forces status to `RETURNED`.
     - *Attack 5 (Negative & Boundary Metrics - CHAOS-05):* Ingest weight `500 kg` or `-5 kg`. Pydantic validator rejects with `422 Unprocessable Entity`.
     - *Attack 6 (SQLi / XSS Injection - CHAOS-06):* Ingest `Robert'); DROP TABLE parcels;--`. Parameterized SQL drivers treat payload as literal text string; zero SQL injection.
     - *Attack 7 (Buffer Overflow - CHAOS-09):* Ingest 50,000-character prompt to AI chat. Gateway drops with `400 Bad Request`.
     - *Attack 8 (Tampered JWT - CHAOS-15):* Ingest customer JWT with modified `"role": "ADMIN"`. Asymmetric RS256 signature verification detects mismatch; aborts with `401 Unauthorized`.
3. **Activity (`Playwright / Pytest Test Orchestrator`):** Assert that expected defensive status code (`400`, `401`, `403`, `409`, `422`) was received.
4. **Terminal Node ($\bigodot$):** Chaos penetration suite completed with zero invariant breaches.

---

## 5. Primary Activity Flow 4: Automated Test Evidence & Trace Capture Lifecycle Activity

This activity flow models the automatic capture, packaging, and archiving of submission artifacts for the 15-mark live demo evaluation.

### 5.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** Test execution starts with flags `--video=on --tracing=on`.
- **Preconditions:** Chromium browser binaries installed.

### 5.2 Step-by-Step Activity Flow

1. **Activity (`Playwright Test Orchestrator`):** Initialize browser context with tracing enabled:
   `context.tracing.start(screenshots=True, snapshots=True, sources=True)`.
2. **Activity (`Playwright Test Orchestrator`):** Execute browser interaction steps (Scenario 1A, Scenario 3, Scenario 4).
3. **Activity (`Playwright Test Orchestrator`):** Detect test completion or failure.
   - **Decision Diamond:** *Did any assertion fail?*
     - *[Yes (Failure)]:* Capture full-page PNG screenshot with locator bounding boxes highlighted. Store in `test-results/artifacts/screenshots/`.
     - *[No (Success)]:* Navigate to `http://localhost:9999/dashboard`; capture full-page screenshot of the webhook visual dashboard as tenant isolation proof.
4. **Activity (`Playwright Test Orchestrator`):** Stop tracing and archive:
   `context.tracing.stop(path="test-results/artifacts/traces/trace.zip")`.
5. **Activity (`Playwright Test Orchestrator`):** Close browser context; flush recorded video stream to `test-results/artifacts/videos/scenario_run.mp4`.
6. **Activity (`Playwright Test Orchestrator`):** Compile JUnit XML test report (`test-results/junit.xml`) detailing pass/fail status across all 16 SRS acceptance tests.
7. **Terminal Node ($\bigodot$):** Submission artifacts bundled and ready for evaluator grading.

---

## 6. Comprehensive UML Element Dictionary for Diagram 2

To assist the diagram designer in drawing **Activity Diagram 2**, use this exact dictionary:

| UML Element Type | Element Name / Label | Swimlane / Partition |
| :--- | :--- | :--- |
| **Initial Node** | `Launch Playwright Suite` | Playwright / Pytest Orchestrator |
| **Action Node** | `Authenticate Customer A via /login` | React Web Portal (:5173) |
| **Action Node** | `Validate Credentials & Issue JWT` | Core API Monolith (:8080) |
| **Action Node** | `Select Zones & Request Fee Estimate` | React Web Portal (:5173) |
| **Action Node** | `Compute Tariff via Pricing Engine` | Core API Monolith (:8080) |
| **Action Node** | `Submit Booking with Idempotency-Key`| React Web Portal (:5173) |
| **Action Node** | `Atomic Insert: Parcel + Outbox Event`| PostgreSQL Database |
| **Action Node** | `Capture Generated Tracking Number` | Playwright / Pytest Orchestrator |
| **Action Node** | `Establish SSE Live Tracking Stream` | React Web Portal (:5173) |
| **Action Node** | `Authenticate Dispatcher & Open Board`| React Web Portal (:5173) |
| **Action Node** | `Assign Courier 1 with SELECT FOR UPDATE`| Core API Monolith (:8080) |
| **Action Node** | `Update Status to ASSIGNED` | PostgreSQL Database |
| **Action Node** | `Authenticate Courier on Mobile View`| React Web Portal (:5173) |
| **Action Node** | `Advance Status to OUT_FOR_DELIVERY` | Core API Monolith (:8080) |
| **Action Node** | `Upload Delivery Photo & Signatory Name`| React Web Portal (:5173) |
| **Decision Diamond**| `Proof Photo Attached?` | Core API Monolith (:8080) |
| **Action Node** | `Advance Status to DELIVERED` | PostgreSQL Database |
| **Action Node** | `Update Live Tracking SSE Stream` | React Web Portal (:5173) |
| **Action Node** | `Reset Mock Webhook Receiver Cache` | Mock Webhook Container (:9999) |
| **Action Node** | `Submit Merchant Order via API Key` | Playwright / Pytest Orchestrator |
| **Action Node** | `Poll Unpublished Outbox Rows` | Outbox Relay Daemon |
| **Action Node** | `Publish CloudEvent to Topic Exchange`| RabbitMQ Broker |
| **Action Node** | `Update Outbox Row published = true` | PostgreSQL Database |
| **Action Node** | `Consume Event & Deduplicate ID` | Event Worker Consumer |
| **Action Node** | `Compute HMAC-SHA256 Signature` | Event Worker Consumer |
| **Action Node** | `Dispatch Webhook to mock-webhook:9999`| Event Worker Consumer |
| **Decision Diamond**| `HMAC Signature Matches Secret?` | Mock Webhook Container (:9999) |
| **Action Node** | `Log Event in Alpha Memory Partition` | Mock Webhook Container (:9999) |
| **Action Node** | `Update Dashboard Feed & Counters` | Live Visual HTML Dashboard |
| **Action Node** | `Query Beta Events & Assert Count == 0`| Playwright / Pytest Orchestrator |
| **Decision Diamond**| `Tenant Isolation Preserved?` | Playwright / Pytest Orchestrator |
| **Action Node** | `Render Green Isolation Proof Badge` | Live Visual HTML Dashboard |
| **Decision Diamond**| `Delivery Attempt < 3?` | Event Worker Consumer |
| **Action Node** | `Apply Compressed Backoff & Retry` | Event Worker Consumer |
| **Action Node** | `Route Message to swiftdrop.dlq` | RabbitMQ Broker |
| **Action Node** | `Capture Trace.zip, Video & Screenshot`| Playwright / Pytest Orchestrator |
| **Activity Final** | `15-Mark Verification Complete` | Playwright / Pytest Orchestrator |

---
*End of Module 02 Activity Flows Specification.*
