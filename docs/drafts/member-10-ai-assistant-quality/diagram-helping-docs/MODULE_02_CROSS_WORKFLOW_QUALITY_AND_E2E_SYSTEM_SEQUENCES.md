# Module 02 — Cross-Workflow Quality & E2E System Sequences Specification

> **Module Name:** Cross-Workflow Quality Assurance, System-Wide E2E Testing & Live Demo Infrastructure  
> **Repository Location:** `tests/e2e/` & `tests/e2e/mock-webhook-receiver/`  
> **Target Folder:** `docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/`  
> **Purpose:** Complete, granular textual specification of every system sequence, participating lifeline, inter-process communication protocol, request/response payload, and alternative exception path for Module 02. Designed specifically to provide a diagram designer with every detail required to construct formal **UML Sequence Diagrams** without containing visual diagrams.  
> **Specification Version:** `2.0.0` (Authoritative Baseline)  
> **Evaluation Scope:** **15 Marks** (Testing & Verification) + **15 Marks** (Live Demo Readiness)

---

## Table of Contents
1. [Lifeline Catalog & Communication Protocol Guide](#1-lifeline-catalog--communication-protocol-guide)
2. [SEQ-QA-01: Pure Web Customer Parcel Creation & Live SSE Tracking (Scenario 1A — Zero AI Bot)](#2-seq-qa-01-pure-web-customer-parcel-creation--live-sse-tracking-scenario-1a--zero-ai-bot)
3. [SEQ-QA-02: Multi-Tenant Merchant B2B Ordering & Asynchronous Webhook Dispatch (Scenario 2)](#3-seq-qa-02-multi-tenant-merchant-b2b-ordering--asynchronous-webhook-dispatch-scenario-2)
4. [SEQ-QA-03: Strict Multi-Tenant Webhook Isolation Verification (SEC-004 / SEC-005)](#4-seq-qa-03-strict-multi-tenant-webhook-isolation-verification-sec-004--sec-005)
5. [SEQ-QA-04: Webhook Transient Failure, Backoff Retries & Dead-Letter Queue (DLQ) Routing](#5-seq-qa-04-webhook-transient-failure-backoff-retries--dead-letter-queue-dlq-routing)
6. [SEQ-QA-05: Dispatcher Management Board Assignment with Concurrency Lock (Scenario 3 - Phase 1)](#6-seq-qa-05-dispatcher-management-board-assignment-with-concurrency-lock-scenario-3---phase-1)
7. [SEQ-QA-06: Courier Daily Route Run, Proof Photo Upload & State Finalization (Scenario 3 - Phase 2)](#7-seq-qa-06-courier-daily-route-run-proof-photo-upload--state-finalization-scenario-3---phase-2)
8. [SEQ-QA-07: Object-Level Authorization (IDOR) Cross-Tenant Security Verification (AT-004 / AT-006)](#8-seq-qa-07-object-level-authorization-idor-cross-tenant-security-verification-at-004--at-006)
9. [SEQ-QA-08: State Machine Boundary & Terminal State Immutability Verification (AT-007 / CHAOS-03)](#9-seq-qa-08-state-machine-boundary--terminal-state-immutability-verification-at-007--chaos-03)
10. [SEQ-QA-09: Rapid Concurrency & Idempotency Key Deduplication (CHAOS-01 / AT-003)](#10-seq-qa-09-rapid-concurrency--idempotency-key-deduplication-chaos-01--at-003)
11. [SEQ-QA-10: Idempotency Key Payload Tampering Detection (CHAOS-02)](#11-seq-qa-10-idempotency-key-payload-tampering-detection-chaos-02)
12. [SEQ-QA-11: Concurrent Dispatch Assignment Race Condition (CHAOS-13 / AT-013)](#12-seq-qa-11-concurrent-dispatch-assignment-race-condition-chaos-13--at-013)
13. [SEQ-QA-12: Consumer At-Least-Once Broker Redelivery Deduplication (CHAOS-07 / AT-010)](#13-seq-qa-12-consumer-at-least-once-broker-redelivery-deduplication-chaos-07--at-010)
14. [SEQ-QA-13: Hanging Webhook Receiver Socket Timeout (CHAOS-14)](#14-seq-qa-13-hanging-webhook-receiver-socket-timeout-chaos-14)
15. [SEQ-QA-14: Test Artifact Recording, Tracing & Visual Proof Capture (UC-DEMO-01 / UC-DEMO-02)](#15-seq-qa-14-test-artifact-recording-tracing--visual-proof-capture-uc-demo-01--uc-demo-02)

---

## 1. Lifeline Catalog & Communication Protocol Guide

When drawing UML Sequence Diagrams for Module 02, diagram designers must model the following standard participating **Lifelines (Objects / Actors)**:

1. **`Playwright / Pytest Test Orchestrator`:** The master Python test process executing browser scripts, HTTP API calls, and AMQP verifications.
2. **`Customer Browser (React UI :5173)`:** Chromium instance running the customer booking portal and tracking views.
3. **`Dispatcher Browser (React UI :5173)`:** Chromium instance running the operations dispatch board (`/ops/dispatch`).
4. **`Courier Mobile Browser (React UI :5173)`:** Mobile viewport Chromium instance running `/courier/deliveries`.
5. **`Core API Modular Monolith (:8080)`:** Identity, Parcels, Pricing, Dispatch, and Delivery domain controllers.
6. **`PostgreSQL Relational Store`:** Authoritative database tables (`parcels`, `parcel_status_history`, `outbox_events`).
7. **`Outbox Relay Daemon`:** Background polling daemon moving committed outbox rows to RabbitMQ.
8. **`RabbitMQ Broker`:** Topic exchange `swiftdrop.events.topic` and dead-letter queue `swiftdrop.dlq`.
9. **`Event Worker Consumer`:** Asynchronous worker verifying deduplication and dispatching HTTP webhooks.
10. **`Tracking API SSE Service (:8081)`:** Read-optimized tracking projection service serving live SSE streams.
11. **`Mock Webhook Container (:9999)`:** Dedicated multi-tenant test container capturing webhooks.
12. **`Visual HTML Dashboard (:9999/dashboard)`:** Web dashboard rendering tenant tabs and proof badges for evaluators.

---

## 2. SEQ-QA-01: Pure Web Customer Parcel Creation & Live SSE Tracking (Scenario 1A — Zero AI Bot)

Traces the core customer journey from authentication to booking and live tracking through the React frontend without touching the AI service.

### 2.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Customer Browser (React UI :5173)`
- `Core API Modular Monolith (:8080)`
- `PostgreSQL Relational Store`
- `Tracking API SSE Service (:8081)`

### 2.2 Chronological Message Sequence
1. `Playwright` $\to$ `Customer Browser`: `page.goto('/login')`.
2. `Customer Browser` $\to$ `Customer Browser`: Fill email `customer.a@swiftdrop.lk`, password `Password123!`; click submit.
3. `Customer Browser` $\to$ `Core API (:8080)`: `POST /api/v1/auth/login` with credentials.
4. `Core API (:8080)` $\to$ `Customer Browser`: Return HTTP `200 OK` with signed RS256 Bearer JWT.
5. `Customer Browser` $\to$ `Customer Browser`: Store JWT in `localStorage`; redirect to `/portal/dashboard`.
6. `Playwright` $\to$ `Customer Browser`: `page.goto('/portal/book')` (aliased `/customer/book`).
7. `Customer Browser` $\to$ `Customer Browser`: Select `ZONE_COLOMBO_01`, `ZONE_KANDY_01`, input weight `2.5` kg, dimensions `25x15x10` cm.
8. `Customer Browser` $\to$ `Customer Browser`: Click `[data-testid="booking-estimate-btn"]`.
9. `Customer Browser` $\to$ `Core API (:8080)`: `POST /api/v1/pricing/quotes` with dimensional parameters.
10. `Core API (:8080)` $\to$ `Customer Browser`: Return HTTP `200 OK {"fee_cents": 65000}`.
11. `Customer Browser` $\to$ `Customer Browser`: Display `"LKR 650.00"` in estimated fee badge locator.
12. `Customer Browser` $\to$ `Customer Browser`: Fill recipient name `"Kasun Perera"`, phone `"+94771234567"`, address `"42 Galle Road"`.
13. `Playwright` $\to$ `Customer Browser`: Click `[data-testid="booking-submit-btn"]`.
14. `Customer Browser` $\to$ `Core API (:8080)`: `POST /api/v1/parcels`  
    *Headers:* `Authorization: Bearer <jwt>`, `Idempotency-Key: <uuidv7>`  
    *Payload:* `{"origin_zone_id": "ZONE_COLOMBO_01", "destination_zone_id": "ZONE_KANDY_01", "weight_kg": 2.5, ...}`
15. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`: `BEGIN TX`  
    - `INSERT INTO parcels (id, tracking_number, status, ...) VALUES ('prcl_01', 'SD-20261021-04921', 'CREATED', ...)`  
    - `INSERT INTO outbox_events (id, event_type, payload, published) VALUES ('evt_01', 'com.swiftdrop.parcel.booked.v1', {...}, false)`  
    - `COMMIT TX`
16. `Core API (:8080)` $\to$ `Customer Browser`: Return HTTP `201 Created` with `tracking_number = "SD-20261021-04921"`.
17. `Customer Browser` $\to$ `Customer Browser`: Render confirmation modal. Playwright extracts tracking number from `[data-testid="booking-success-tracking-number"]`.
18. `Playwright` $\to$ `Customer Browser`: `page.goto('/tracking/SD-20261021-04921')`.
19. `Customer Browser` $\to$ `Tracking API SSE Service (:8081)`: HTTP GET `/api/v1/tracking/stream/SD-20261021-04921` (`Accept: text/event-stream`).
20. `Tracking API SSE Service (:8081)` $\to$ `Customer Browser`: Push initial SSE event: `event: status\ndata: {"status": "CREATED"}\n\n`.
21. `Customer Browser` $\to$ `Playwright`: Status badge locator `[data-testid="public-tracking-status-badge"]` displays `"CREATED"`. Test assertion passes.

---

## 3. SEQ-QA-02: Multi-Tenant Merchant B2B Ordering & Asynchronous Webhook Dispatch (Scenario 2)

Traces the asynchronous event publication flow from order creation through RabbitMQ to webhook delivery.

### 3.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Core API Modular Monolith (:8080)`
- `PostgreSQL Relational Store`
- `Outbox Relay Daemon`
- `RabbitMQ Broker`
- `Event Worker Consumer`
- `Mock Webhook Container (:9999)`
- `Visual HTML Dashboard (:9999/dashboard)`

### 3.2 Chronological Message Sequence
1. `Playwright` $\to$ `Mock Webhook Container (:9999)`: HTTP POST `/api/reset` (Flushes in-memory event logs).
2. `Mock Webhook Container (:9999)` $\to$ `Playwright`: HTTP `200 OK {"status": "cleared"}`.
3. `Playwright` $\to$ `Core API (:8080)`: `POST /api/v1/merchant/orders`  
   *Headers:* `X-API-Key: test_key_alpha` (Merchant Alpha: Apex Retail)  
   *Payload:* `{"recipient_name": "Alice", "destination_zone_id": "ZONE_COLOMBO_01", "fee_cents": 8000}`
4. `Core API (:8080)` $\to$ `Core API (:8080)`: Authenticate API key. Confirm tenant `mer_alpha_...` is `ACTIVE`.
5. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`: `BEGIN TX`  
   - Insert parcel record with `tenant_id = 'mer_alpha_...'`.  
   - Insert outbox event `parcel.booked.v1` (`published = false`).  
   - `COMMIT TX`
6. `Core API (:8080)` $\to$ `Playwright`: Return HTTP `201 Created {"data": {"tracking_number": "SD-MERCH-001"}}`.
7. `Outbox Relay Daemon` $\to$ `PostgreSQL Relational Store`: Query unpublished rows: `SELECT * FROM outbox_events WHERE published = false`.
8. `Outbox Relay Daemon` $\to$ `RabbitMQ Broker`: AMQP `basic.publish` to topic exchange `swiftdrop.events.topic` with routing key `parcel.booked.v1`.
9. `RabbitMQ Broker` $\to$ `Outbox Relay Daemon`: AMQP `basic.ack` (Broker confirms message is durable).
10. `Outbox Relay Daemon` $\to$ `PostgreSQL Relational Store`: `UPDATE outbox_events SET published = true WHERE id = $1`.
11. `RabbitMQ Broker` $\to$ `Event Worker Consumer`: Deliver message from queue `q.webhooks.dispatcher`.
12. `Event Worker Consumer` $\to$ `Event Worker Consumer`: Verify deduplication: `SELECT 1 FROM processed_events WHERE event_id = $1`.
13. `Event Worker Consumer` $\to$ `Event Worker Consumer`: Compute HMAC-SHA256 signature using Merchant Alpha secret (`whsec_alpha_...`).
14. `Event Worker Consumer` $\to$ `Mock Webhook Container (:9999)`: HTTP POST `/webhooks/merchant-alpha`  
    *Headers:* `Content-Type: application/json`, `X-SwiftDrop-Signature: sha256=<hmac>`, `X-SwiftDrop-Delivery-Attempt: 1`  
    *Payload:* CloudEvents JSON envelope (`tracking_number = "SD-MERCH-001"`).
15. `Mock Webhook Container (:9999)` $\to$ `Mock Webhook Container (:9999)`: Compute local HMAC; confirm signature match. Store in Alpha memory partition.
16. `Mock Webhook Container (:9999)` $\to$ `Event Worker Consumer`: HTTP `200 OK`.
17. `Mock Webhook Container (:9999)` $\to$ `Visual HTML Dashboard`: Increment Merchant Alpha event counter to `1`. Render CloudEvent card with valid HMAC badge.

---

## 4. SEQ-QA-03: Strict Multi-Tenant Webhook Isolation Verification (SEC-004 / SEC-005)

Traces the automated verification that Merchant Beta receives zero data when Merchant Alpha transacts.

### 4.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Mock Webhook Container (:9999)`
- `Visual HTML Dashboard (:9999/dashboard)`

### 4.2 Chronological Message Sequence
1. Immediately following completion of SEQ-QA-02 (Merchant Alpha booking):
2. `Playwright` $\to$ `Mock Webhook Container (:9999)`: HTTP GET `/api/merchants/mer_beta/events`.
3. `Mock Webhook Container (:9999)` $\to$ `Mock Webhook Container (:9999)`: Inspect Merchant Beta memory partition.
4. `Mock Webhook Container (:9999)` $\to$ `Playwright`: Return HTTP `200 OK` with JSON array `[]` (length == 0).
5. `Playwright` $\to$ `Playwright`: Assert `expect(beta_events.length).toBe(0)`. **Zero cross-talk verified.**
6. `Mock Webhook Container (:9999)` $\to$ `Visual HTML Dashboard`: Render prominent green indicator:  
   `[TENANT ISOLATION VERIFIED: ZERO DATA LEAKAGE]`.
7. `Playwright` $\to$ `Visual HTML Dashboard`: `page.goto('http://localhost:9999/dashboard')`.
8. `Playwright` $\to$ `Playwright`: `page.screenshot({ path: "test-results/artifacts/webhook-isolation-proof.png" })`.

---

## 5. SEQ-QA-04: Webhook Transient Failure, Backoff Retries & Dead-Letter Queue (DLQ) Routing

Traces how failing merchant webhooks are retried with compressed backoff in CI and routed to the dead-letter queue.

### 5.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Core API Modular Monolith (:8080)`
- `RabbitMQ Broker`
- `Event Worker Consumer`
- `Mock Webhook Container (:9999)`

### 5.2 Chronological Message Sequence
1. `Playwright` $\to$ `Core API (:8080)`: Book order for Chaos Merchant (`mer_chaos_...`).
2. Event propagates through PostgreSQL and Outbox Relay to RabbitMQ.
3. `RabbitMQ Broker` $\to$ `Event Worker Consumer`: Deliver CloudEvent.
4. `Event Worker Consumer` $\to$ `Mock Webhook Container (:9999)`: Attempt 1: HTTP POST `/webhooks/merchant-chaos`.
5. `Mock Webhook Container (:9999)` $\to$ `Event Worker Consumer`: HTTP `500 Internal Server Error`.
6. `Event Worker Consumer` $\to$ `Event Worker Consumer`: Detect 500 error. Check CI retry schedule: `WEBHOOK_RETRY_INTERVALS_MS=500,1000,2000`.
7. `Event Worker Consumer` $\to$ `Event Worker Consumer`: Sleep 500ms.
8. `Event Worker Consumer` $\to$ `Mock Webhook Container (:9999)`: Attempt 2: HTTP POST `/webhooks/merchant-chaos` (`X-SwiftDrop-Delivery-Attempt: 2`).
9. `Mock Webhook Container (:9999)` $\to$ `Event Worker Consumer`: HTTP `500 Internal Server Error`.
10. `Event Worker Consumer` $\to$ `Event Worker Consumer`: Sleep 1000ms.
11. `Event Worker Consumer` $\to$ `Mock Webhook Container (:9999)`: Attempt 3: HTTP POST `/webhooks/merchant-chaos` (`X-SwiftDrop-Delivery-Attempt: 3`).
12. `Mock Webhook Container (:9999)` $\to$ `Event Worker Consumer`: HTTP `500 Internal Server Error`.
13. `Event Worker Consumer` $\to$ `Event Worker Consumer`: Retries exhausted (3 attempts failed).
14. `Event Worker Consumer` $\to$ `RabbitMQ Broker`: AMQP `basic.publish` to dead-letter exchange `swiftdrop.dlq.exchange` routed to queue `swiftdrop.dlq`.
15. `Event Worker Consumer` $\to$ `RabbitMQ Broker`: AMQP `basic.ack` on original queue (remove from primary queue).
16. `Playwright` $\to$ `RabbitMQ Broker`: Pika client queries queue `swiftdrop.dlq`. Asserts dead-lettered CloudEvent is present. Test assertion passes.

---

## 6. SEQ-QA-05: Dispatcher Management Board Assignment with Concurrency Lock (Scenario 3 - Phase 1)

Traces how dispatchers assign parcels to couriers using PostgreSQL row locks.

### 6.1 Participating Lifelines
- `Dispatcher Browser (React UI :5173)`
- `Core API Modular Monolith (:8080)`
- `PostgreSQL Relational Store`

### 6.2 Chronological Message Sequence
1. `Dispatcher Browser` $\to$ `Core API (:8080)`: `GET /api/v1/dispatch/unassigned` (Authenticated with Dispatcher JWT).
2. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`: `SELECT * FROM parcels WHERE status = 'CREATED' AND courier_id IS NULL`.
3. `Core API (:8080)` $\to$ `Dispatcher Browser`: Return list of unassigned parcels.
4. `Dispatcher Browser` $\to$ `Dispatcher Browser`: Dispatcher selects parcel `prcl_01` and chooses Courier 1 (`usr_courier_bob`); clicks assign.
5. `Dispatcher Browser` $\to$ `Core API (:8080)`: `POST /api/v1/dispatch/assign`  
   *Payload:* `{"parcel_id": "prcl_01", "courier_id": "usr_courier_bob"}`
6. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`: `BEGIN TX`  
   `SELECT id, status, courier_id FROM parcels WHERE id = 'prcl_01' FOR UPDATE` (Acquires row-level lock).
7. `Core API (:8080)` $\to$ `Core API (:8080)`: Verify `status == 'CREATED'` and `courier_id IS NULL`.
8. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`:  
   - `UPDATE parcels SET status = 'ASSIGNED', courier_id = 'usr_courier_bob' WHERE id = 'prcl_01'`  
   - `INSERT INTO parcel_status_history (parcel_id, old_status, new_status, changed_by) VALUES ('prcl_01', 'CREATED', 'ASSIGNED', 'usr_disp_001')`  
   - `INSERT INTO outbox_events (event_type, payload) VALUES ('com.swiftdrop.parcel.assigned', {...})`  
   - `COMMIT TX`
9. `Core API (:8080)` $\to$ `Dispatcher Browser`: Return HTTP `200 OK {"status": "ASSIGNED"}`.
10. `Dispatcher Browser` $\to$ `Dispatcher Browser`: Render assignment success toast notification.

---

## 7. SEQ-QA-06: Courier Daily Route Run, Proof Photo Upload & State Finalization (Scenario 3 - Phase 2)

Traces how couriers advance deliveries to out-for-delivery, attach photo proofs, and finalize deliveries.

### 7.1 Participating Lifelines
- `Courier Mobile Browser (React UI :5173)`
- `Core API Modular Monolith (:8080)`
- `PostgreSQL Relational Store`
- `Customer Browser (React UI :5173)`

### 7.2 Chronological Message Sequence
1. `Courier Mobile Browser` $\to$ `Core API (:8080)`: `GET /api/v1/courier/deliveries` (Authenticated as Courier 1).
2. `Core API (:8080)` $\to$ `Courier Mobile Browser`: Return assigned deliveries list.
3. `Courier Mobile Browser` $\to$ `Core API (:8080)`: `PATCH /api/v1/courier/deliveries/prcl_01/status`  
   *Payload:* `{"status": "OUT_FOR_DELIVERY"}`
4. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`: Update status to `OUT_FOR_DELIVERY`; write status history and outbox event. Commit TX.
5. `Core API (:8080)` $\to$ `Courier Mobile Browser`: Return HTTP `200 OK`.
6. Courier arrives at destination. Attaches delivery photo (`sample_proof.jpg`) and types recipient name `"Sunil Shantha"`.
7. `Courier Mobile Browser` $\to$ `Core API (:8080)`: `PATCH /api/v1/courier/deliveries/prcl_01/status`  
   *Payload:* `{"status": "DELIVERED", "proof_type": "PHOTO_AND_SIGNATURE", "recipient_name": "Sunil Shantha", "proof_file_url": "https://storage.swiftdrop.lk/proofs/proof_01.jpg"}`
8. `Core API (:8080)` $\to$ `Core API (:8080)`: Validate proof photo URL and signatory presence:
   - If missing: reject with `422 Unprocessable Entity`.
   - If present: proceed to commit.
9. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`: `BEGIN TX`  
   - Update parcel status to `DELIVERED`.  
   - Insert status history with proof URL.  
   - Insert outbox event `parcel.delivered`.  
   - `COMMIT TX`
10. `Core API (:8080)` $\to$ `Courier Mobile Browser`: Return HTTP `200 OK`.
11. Outbox Relay and Tracking service propagate status update.
12. `Tracking API` $\to$ `Customer Browser`: Push live SSE event: `event: status\ndata: {"status": "DELIVERED"}`. Customer badge updates to `"DELIVERED"`.

---

## 8. SEQ-QA-07: Object-Level Authorization (IDOR) Cross-Tenant Security Verification (AT-004 / AT-006)

Traces automated security assertions proving IDOR protection across customers and couriers.

### 8.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Core API Modular Monolith (:8080)`
- `PostgreSQL Relational Store`

### 8.2 Chronological Message Sequence
#### Test Case 1: Customer A Accessing Customer B's Parcel (AT-004)
1. `Playwright` $\to$ `Core API (:8080)`: `GET /api/v1/parcels/prcl_belonging_to_B`  
   *Headers:* `Authorization: Bearer <Customer_A_JWT>`
2. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`: `SELECT sender_id, recipient_id FROM parcels WHERE id = 'prcl_belonging_to_B'`.
3. `Core API (:8080)` $\to$ `Core API (:8080)`: Compare `parcel.sender_id` (`usr_cust_002`) against caller token `sub` (`usr_cust_001`). Mismatch detected.
4. `Core API (:8080)` $\to$ `Playwright`: Return HTTP `403 Forbidden` (`INSUFFICIENT_PERMISSIONS`). Assertion passes.

#### Test Case 2: Courier 2 Mutating Courier 1's Assigned Parcel (AT-006)
5. `Playwright` $\to$ `Core API (:8080)`: `PATCH /api/v1/courier/deliveries/prcl_assigned_to_1/status`  
   *Headers:* `Authorization: Bearer <Courier_2_JWT>`  
   *Payload:* `{"status": "DELIVERED", "recipient_name": "Intruder"}`
6. `Core API (:8080)` $\to$ `PostgreSQL Relational Store`: `SELECT assigned_courier_id FROM parcels WHERE id = 'prcl_assigned_to_1'`.
7. `Core API (:8080)` $\to$ `Core API (:8080)`: Compare `assigned_courier_id` (`usr_cour_001`) against caller token `sub` (`usr_cour_002`). Mismatch detected.
8. `Core API (:8080)` $\to$ `Playwright`: Return HTTP `403 Forbidden` (`COURIER_NOT_ASSIGNED`). Assertion passes.

---

## 9. SEQ-QA-08: State Machine Boundary & Terminal State Immutability Verification (AT-007 / CHAOS-03)

Traces automated verification that state machine transitions are strictly linear and terminal states are immutable.

### 9.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Core API Modular Monolith (:8080)`
- `PostgreSQL Relational Store`

### 9.2 Chronological Message Sequence
#### Test Case 1: Illegal State Transition Leap (AT-007)
1. `Playwright` $\to$ `Core API (:8080)`: Attempt status update from `CREATED` directly to `DELIVERED` without assignment or dispatch.
2. `Core API (:8080)` $\to$ `Core API (:8080)`: Domain aggregate root evaluates valid transitions from `CREATED`: only `ASSIGNED` or `CANCELLED` permitted.
3. `Core API (:8080)` $\to$ `Playwright`: Return HTTP `409 Conflict` (`INVALID_STATUS_TRANSITION`). Zero outbox events written.

#### Test Case 2: Terminal State Mutation (CHAOS-03)
4. Parcel `prcl_01` is already in terminal state `DELIVERED`.
5. `Playwright` $\to$ `Core API (:8080)`: Attempt status update: `PATCH /api/v1/courier/deliveries/prcl_01/status` with `{"status": "OUT_FOR_DELIVERY"}`.
6. `Core API (:8080)` $\to$ `Core API (:8080)`: Aggregate root evaluates terminal state rule: `status IN ('DELIVERED', 'CANCELLED', 'RETURNED')` is immutable.
7. `Core API (:8080)` $\to$ `Playwright`: Return HTTP `409 Conflict` (`ILLEGAL_TERMINAL_STATE_MUTATION`). Assertion passes.

---

## 10. SEQ-QA-09: Rapid Concurrency & Idempotency Key Deduplication (CHAOS-01 / AT-003)

Traces rapid double-clicking deduplication ensuring exactly one database entity is created.

### 10.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Core API Modular Monolith (:8080)`
- `PostgreSQL Relational Store`

### 10.2 Chronological Message Sequence
1. `Playwright` dispatches 5 concurrent HTTP POST requests to `/api/v1/parcels` within 100ms sharing identical `Idempotency-Key: 0192a6b2-9d3e-7a1b`.
2. `Core API` Idempotency Filter receives all 5 calls concurrently:
   - **Call 1 (The Winner):**
     1. Checks cache: key not found. Inserts in-flight lock: `idempotency:0192a6b2-9d3e-7a1b = IN_PROGRESS`.
     2. Commits transaction to PostgreSQL: inserts 1 parcel row.
     3. Stores cached HTTP response payload in Redis/RAM: `status = 201, body = {...}`.
     4. Returns HTTP `201 Created` with tracking number `SD-101`.
   - **Calls 2 through 5 (Concurrent Replays):**
     1. Check cache: lock is `IN_PROGRESS` or response is cached.
     2. Wait for completion of Call 1 or retrieve cached response.
     3. Returns HTTP `200 OK` (or `201 Created`) with header `Idempotent-Replay: true` and identical body.
3. `Playwright` queries database: `SELECT COUNT(*) FROM parcels WHERE tracking_number = 'SD-101'`.
4. Asserts count is **strictly 1**. Exactly one parcel created.

---

## 11. SEQ-QA-10: Idempotency Key Payload Tampering Detection (CHAOS-02)

Traces defense against replaying an existing idempotency key with altered financial or dimensional parameters.

### 11.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Core API Modular Monolith (:8080)`

### 11.2 Chronological Message Sequence
1. Customer creates parcel with `Idempotency-Key: key_123` and `weight_kg: 2.5`. Receives 201 Created. Core API records `hash(payload)` in idempotency store.
2. Malicious user replays `Idempotency-Key: key_123` with altered payload: `weight_kg: 25.0` (attempting to ship a heavier package on original price).
3. `Core API` calculates SHA-256 hash of incoming payload.
4. Compares with cached hash for `key_123`.
5. Detects hash mismatch: `cached_hash != current_hash`.
6. `Core API` $\to$ `Playwright`: Return HTTP `409 Conflict` (`IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`). Transaction aborted.

---

## 12. SEQ-QA-11: Concurrent Dispatch Assignment Race Condition (CHAOS-13 / AT-013)

Traces how simultaneous courier assignments by two dispatchers are resolved safely via database row locking.

### 12.1 Participating Lifelines
- `Dispatcher 1`
- `Dispatcher 2`
- `Core API Modular Monolith (:8080)`
- `PostgreSQL Relational Store`

### 12.2 Chronological Message Sequence
1. Dispatcher 1 and Dispatcher 2 simultaneously submit courier assignments for unassigned parcel `prcl_01` at the exact same millisecond:
   - Dispatcher 1 assigns Courier A (`usr_courier_bob`).
   - Dispatcher 2 assigns Courier B (`usr_courier_alice`).
2. Core API handles both transactions concurrently:
   - **Transaction 1 (Dispatcher 1):**
     1. Executes `BEGIN TX`.
     2. Runs `SELECT id, status, courier_id FROM parcels WHERE id = 'prcl_01' FOR UPDATE`.
     3. Acquires row-level exclusive lock. Status is `CREATED`.
     4. Updates `courier_id = 'usr_courier_bob'`, `status = 'ASSIGNED'`.
     5. Commits transaction and releases row lock.
     6. Returns HTTP `200 OK` to Dispatcher 1.
   - **Transaction 2 (Dispatcher 2):**
     1. Executes `BEGIN TX`.
     2. Runs `SELECT ... FOR UPDATE`. Blocks waiting for Transaction 1 to release row lock.
     3. Transaction 1 commits; row lock is granted to Transaction 2.
     4. Reads updated row: `status` is now `'ASSIGNED'` (no longer `'CREATED'`) and `courier_id` is `'usr_courier_bob'`.
     5. Core API detects parcel is already assigned. Rolls back transaction.
     6. Returns HTTP `409 Conflict` (`PARCEL_ALREADY_ASSIGNED`) to Dispatcher 2.
3. System integrity preserved: parcel assigned to exactly one courier.

---

## 13. SEQ-QA-12: Consumer At-Least-Once Broker Redelivery Deduplication (CHAOS-07 / AT-010)

Traces message consumer deduplication preventing duplicate external effects when RabbitMQ redelivers events.

### 13.1 Participating Lifelines
- `RabbitMQ Broker`
- `Event Worker Consumer`
- `PostgreSQL Relational Store`
- `Mock Webhook Container (:9999)`

### 13.2 Chronological Message Sequence
1. `RabbitMQ Broker` delivers CloudEvent `evt_01HXYZ` to `Event Worker Consumer`.
2. `Event Worker Consumer` $\to$ `PostgreSQL Relational Store`: `SELECT 1 FROM processed_events WHERE event_id = 'evt_01HXYZ'`.
3. Not found. `Event Worker Consumer` executes transaction:
   - Inserts `evt_01HXYZ` into `processed_events`.
   - Dispatches webhook to `Mock Webhook Container (:9999)`.
4. Consumer crashes before sending `basic.ack` to RabbitMQ.
5. RabbitMQ detects unacknowledged message; redelivers identical message `evt_01HXYZ` (`redelivered = true`).
6. `Event Worker Consumer` restarts; consumes redelivered message.
7. `Event Worker Consumer` $\to$ `PostgreSQL Relational Store`: `SELECT 1 FROM processed_events WHERE event_id = 'evt_01HXYZ'`.
8. Found in `processed_events`.
9. `Event Worker Consumer` $\to$ `RabbitMQ Broker`: Send `basic.ack` immediately.
10. **Zero duplicate webhook calls dispatched.** External idempotency guaranteed.

---

## 14. SEQ-QA-13: Hanging Webhook Receiver Socket Timeout (CHAOS-14)

Traces how the Event Worker releases consumer threads when an external webhook endpoint hangs indefinitely.

### 14.1 Participating Lifelines
- `Event Worker Consumer`
- `Mock Webhook Container: Slow Endpoint (:9999/webhooks/merchant-slow)`
- `RabbitMQ Broker`

### 14.2 Chronological Message Sequence
1. `Event Worker Consumer` prepares HTTP POST dispatch for Latency Merchant.
2. `Event Worker Consumer` initializes async HTTP client with strict socket timeout: `timeout = 5.0s`.
3. `Event Worker Consumer` $\to$ `Mock Webhook Container`: HTTP POST `/webhooks/merchant-slow`.
4. `Mock Webhook Container` sleeps for 10.0 seconds without responding.
5. At exactly 5.0 seconds, `Event Worker Consumer` socket read timer triggers: `HTTPX.ReadTimeout: Socket timed out after 5000ms`.
6. `Event Worker Consumer` terminates TCP connection; worker thread released immediately (preventing thread exhaustion).
7. `Event Worker Consumer` records attempt failure; schedules exponential backoff retry.

---

## 15. SEQ-QA-14: Test Artifact Recording, Tracing & Visual Proof Capture (UC-DEMO-01 / UC-DEMO-02)

Traces the automated capture and packaging of submission artifacts for the 15-mark live demo evaluation.

### 15.1 Participating Lifelines
- `Playwright / Pytest Test Orchestrator`
- `Chromium Browser Instance`
- `Local Artifact Filesystem (test-results/artifacts/)`
- `Visual HTML Dashboard (:9999/dashboard)`

### 15.2 Chronological Message Sequence
1. `Playwright` starts Chromium context with flags: `--video=on --tracing=on`.
2. Tracing subsystem starts recording DOM snapshots and network traffic.
3. Multi-actor test scenarios execute across browser views.
4. Browser interactions complete. `Playwright` commands browser to open `http://localhost:9999/dashboard`.
5. Visual dashboard renders real-time feeds and the green `[TENANT ISOLATION VERIFIED: ZERO DATA LEAKAGE]` card.
6. `Playwright` $\to$ `Local Artifact Filesystem`: `page.screenshot({ path: "test-results/artifacts/webhook-isolation-proof.png", fullPage: true })`.
7. `Playwright` $\to$ `Local Artifact Filesystem`: `context.tracing.stop(path="test-results/artifacts/traces/trace.zip")`.
8. `Playwright` $\to$ `Local Artifact Filesystem`: Close browser context; flush recorded MP4 video stream to `test-results/artifacts/videos/scenario_run.mp4`.
9. `Playwright` compiles JUnit XML test report: `test-results/junit.xml` proving all 16 SRS acceptance tests passed.
10. Permanent submission artifacts ready for evaluator inspection.

---
*End of Module 02 System Sequences Specification.*
