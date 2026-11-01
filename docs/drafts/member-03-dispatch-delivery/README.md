# Member 03 - Dispatch, Couriers & Proof of Delivery Specifications

> **Owner:** Member 03  
> **Status:** Pre-Draft Alignment & Specification Stage  
> **Target Production Location:** `core-api/internal/dispatch/` and `core-api/internal/delivery/`

---

## 1. Pre-Draft Alignment

- [x] [decisions-and-questionnaire.md](decisions-and-questionnaire.md) — Comprehensive architectural, domain invariant, and persistence decision matrix.

---

## 2. Required Documents Checklist

- [x] [doc-01-scope-and-responsibilities.md](doc-01-scope-and-responsibilities.md) — Module purpose, in-scope/out-of-scope capabilities, domain invariants, persona workflows, and acceptance criteria.
- [x] [doc-02-functional-flows-and-business-rules.md](doc-02-functional-flows-and-business-rules.md) — Complete operational workflows, sequence diagrams, state machines, and business rules matrix.
- [x] [doc-03-internal-architecture-and-design.md](doc-03-internal-architecture-and-design.md) — Clean architecture package layout, domain models, application use cases, repository interfaces, and atomic transaction design.
- [x] [doc-04-interface-and-integration-contract.md](doc-04-interface-and-integration-contract.md) — Complete REST endpoint contracts (requests, responses, errors) and 6 versioned outbox event schemas.
- [x] [doc-05-data-and-persistence.md](doc-05-data-and-persistence.md) — Authoritative PostgreSQL DDL migrations, performance indexes, row-locking strategies, and data retention policies.
- [x] [doc-06-testing-strategy-and-acceptance.md](doc-06-testing-strategy-and-acceptance.md) — Testing pyramid, unit test suites, PostgreSQL integration tests, concurrency race tests (AT-013), and courier IDOR verification (AT-006).
- [x] [doc-07-security-failure-and-recovery.md](doc-07-security-failure-and-recovery.md) — Threat modeling (STRIDE), courier IDOR defense, pre-signed URL security, failure modes (R2 outages, mobile signal dropouts), and disaster recovery queries.
- [x] [doc-08-implementation-deployment-and-ops.md](doc-08-implementation-deployment-and-ops.md) — Dependency injection wiring in `wiring.go`, environment variable inventory, local Docker Compose setup, Kubernetes deployment specs, Prometheus observability metrics, and operational runbooks.
