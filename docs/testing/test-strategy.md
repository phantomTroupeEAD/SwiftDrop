# SwiftDrop — Master Test Strategy & Traceability Matrix

> **Authority:** Fulfills SRS §12 (Testing and Verification), §16 (Requirements Traceability Matrix), and Engineering Standards §12.  
> **Quality Goal:** Measurable test coverage with strict invariant enforcement and zero untested security boundaries.  
> **Primary Owner:** Member 10 (AI Assistant & Cross-Workflow Quality)  
> **Collaborators:** All 10 Domain Owners.

---

## 1. Test Pyramid & Execution Levels

SwiftDrop implements a multi-tier test pyramid combining isolated unit tests, containerized integration tests, HTTP contract tests, and end-to-end journey tests:

| Test Level | Scope & Objective | Frameworks & Tooling | Target Execution Point | Mandatory Coverage |
| :--- | :--- | :--- | :--- | :--- |
| **Unit** | Domain aggregates, pricing formulas, state transitions, validation rules, token parsing. | Go `testing` + `testify`<br>Vitest (React)<br>Pytest (Python) | Pre-commit / PR CI | **$\ge$ 70%** line coverage of core business packages. |
| **Integration** | Real database transactions, outbox poller, RabbitMQ pub/sub, consumer idempotency. | Go `testing`<br>`testcontainers-go`<br>Docker Compose | PR CI & Nightly | All atomic transactions and messaging queues. |
| **API & Security** | Ingress routing, RBAC authorization, tenant isolation, IDOR, rate limiting, SSRF defense. | Go HTTP test recorder / `httptest`<br>Postman / Newman | PR CI | 100% of protected `/api/v1/*` endpoints. |
| **Frontend Component** | Form validation, role-based view rendering, live milestone updates, error banners. | React Testing Library + Vitest | PR CI | Critical customer & courier forms. |
| **End-to-End (E2E)** | Full customer booking $\rightarrow$ dispatch $\rightarrow$ courier delivery $\rightarrow$ receipt download. | Playwright (TypeScript) | Pre-Release / Demo Validation | Core multi-actor business journeys. |
| **Resilience & Chaos** | Broker outage, database connection cut, duplicate webhook delivery, worker crash. | Custom Go test scripts | Pre-Deployment Acceptance | Outbox durability and consumer recovery. |

---

## 2. Mandatory Acceptance Tests (AT-001 through AT-016)

Per **SRS §12.2**, the following 16 acceptance tests are strictly required and verifiable in CI:

| Acceptance Test ID | Test Title & Invariant Verified | Primary Owning Member | Traceable Requirement | Verification Status |
| :--- | :--- | :--- | :--- | :--- |
| **AT-001** | Customer registration/login, fee quote preview, and valid parcel creation. | Member 1, Member 2 | `FR-IAM-001`, `FR-PAR-001` | **Automated** (`core-api/tests`) |
| **AT-002** | Invalid parcel inputs (negative weight, missing address) rejected with 422; no partial DB write. | Member 2 | `FR-PAR-004`, `SEC-006` | **Automated** (Unit/API) |
| **AT-003** | Idempotency enforcement: repeating booking request with same key returns cached response, single parcel. | Member 2, Platform | `FR-PAR-007` | **Automated** (Integration) |
| **AT-004** | Tenant & Object Isolation (IDOR): Customer A cannot read Customer B's parcels or receipts (HTTP 403/404). | Member 1, Member 2 | `SEC-002`, `SEC-005` | **Automated** (Security test) |
| **AT-005** | Public Tracking Masking: `/tracking/public/{num}` returns milestones without recipient address or phone. | Member 5 | `SEC-009`, `FR-TRK-001` | **Automated** (Tracking test) |
| **AT-006** | Courier Authorization: Valid courier can update assigned parcel; unauthorized courier is rejected (HTTP 403). | Member 3 | `FR-COU-002`, `SEC-002` | **Automated** (Dispatch test) |
| **AT-007** | State Machine Guard: Illegal transition (`CREATED` $\rightarrow$ `DELIVERED`) rejected; zero outbox events written. | Member 2 | `FR-LIFE-002`, `ADR-004` | **Automated** (Domain unit) |
| **AT-008** | Atomic Commit: Status update atomically writes to `parcels`, `parcel_status_history`, and `outbox_events`. | Member 2, Member 8 | `FR-LIFE-005`, `FR-EVT-001` | **Automated** (DB Integration) |
| **AT-009** | Outbox Relay Durability: Events published only after RabbitMQ confirmation; relay recovers after broker restart. | Member 8 | `FR-EVT-002`, `FR-EVT-003` | **Automated** (Messaging test) |
| **AT-010** | Consumer Idempotency: Duplicate RabbitMQ delivery does not duplicate notifications or webhooks. | Member 8 | `FR-EVT-005`, `SEC-011` | **Automated** (Worker test) |
| **AT-011** | Merchant Isolation: Unapproved/suspended merchants cannot book parcels; API key scoped to tenant. | Member 4 | `FR-MER-002`, `FR-MER-004` | **Automated** (Merchant test) |
| **AT-012** | Merchant Webhook Signature & Retry: Webhook payload includes valid HMAC; retried on 500 error with backoff. | Member 4, Member 8 | `FR-MER-008`, `SEC-010` | **Automated** (Worker test) |
| **AT-013** | Dispatcher Concurrency: Concurrent courier assignments on same parcel handled via optimistic locking (409 Conflict). | Member 3 | `FR-ADM-002`, `SRS §7.2` | **Automated** (Dispatch test) |
| **AT-014** | Google OIDC Validation: OIDC code exchange validates state/nonce and safely rejects forged tokens. | Member 1 | `FR-IAM-002`, `ADR-003` | **Automated** (Auth test) |
| **AT-015** | AI Assistant Tool Sandbox: AI uses read-only backend endpoints, respects RBAC, and never invents unverified facts. | Member 10 | `FR-AI-001` to `FR-AI-008` | **Automated** (AI test suite) |
| **AT-016** | Deployment & Probes: Kubernetes `/health/live` & `/health/ready` probes functional; reconciliation CronJob executes safely. | Member 9 | `SRS §11.2`, `ADR-002` | **Automated** (K8s verification) |

---

## 3. Requirements Traceability Matrix (RTM)

Maps all assignment functional and security specifications directly to code paths and test suites:

| Assignment Requirement | Functional Spec IDs | Owning Code Path | Test Suite Location |
| :--- | :--- | :--- | :--- |
| **Identity & Access** | `FR-IAM-001` to `009`, `SEC-001` to `004` | `core-api/internal/identity/` | `core-api/internal/identity/..._test.go` |
| **Parcel Booking & Fee** | `FR-PAR-001` to `008`, `FR-PRC-001` to `005` | `core-api/internal/parcels/`, `pricing/` | `core-api/internal/parcels/..._test.go`, `pricing/..._test.go` |
| **Parcel Lifecycle** | `FR-LIFE-001` to `007` | `core-api/internal/parcels/domain/` | `core-api/internal/parcels/domain/parcel_test.go` |
| **Dispatch & Couriers** | `FR-COU-001` to `007`, `FR-ADM-001` to `004` | `core-api/internal/dispatch/`, `delivery/` | `core-api/internal/dispatch/..._test.go` |
| **Merchant Integration** | `FR-MER-001` to `011`, `SEC-010`, `SEC-011` | `core-api/internal/merchants/` | `core-api/internal/merchants/..._test.go` |
| **Live Tracking & SSE** | `FR-TRK-001` to `007`, `SEC-016` | `services/tracking/` | `services/tracking/..._test.go` |
| **Outbox & Messaging** | `FR-EVT-001` to `010` | `core-api/internal/events/`, `services/messaging/`| `services/messaging/..._test.go` |
| **Admin Operations** | `FR-ADM-005` to `009` | `core-api/internal/admin/` | `core-api/internal/admin/..._test.go` |
| **AI Assistant** | `FR-AI-001` to `008`, `SEC-017` | `services/ai-assistant/` | `services/ai-assistant/tests/test_assistant.py` |
| **Edge & Ingress Gateway**| `ADR-001`, `ADR-002`, `SEC-014`, `SEC-015` | `deploy/gateway/traefik/` | `deploy/tests/gateway_test.sh` |

---

## 4. Test Execution Commands Reference

### Run Core API Unit & Integration Tests
```bash
cd core-api
# Run all unit tests with coverage profile
go test -v -race -coverprofile=coverage.out ./internal/...

# View coverage summary
go tool cover -func=coverage.out
```

### Run Frontend Component Tests
```bash
cd web
npm run test:unit
```

### Run Messaging & Worker Tests
```bash
cd services/messaging
go test -v -race ./...
```

### Run Python AI Assistant Tests
```bash
cd services/ai-assistant
pytest tests/ -v --cov=app
```

### Run Full System End-to-End Verification
```bash
# Execute repository-wide verification script
./scripts/verify.sh
```
