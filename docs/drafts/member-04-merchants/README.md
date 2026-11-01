# Member 04 - Merchant Integration & Webhooks Specifications

> **Owner:** Member 04
> **Status:** Initial Draft Space for Review in Meeting 2
> **Target Production Location:** See docs/architecture/domain-boundaries.md

---

## Required Documents Checklist

- [x] doc-01-scope-and-responsibilities.md
- [x] doc-02-functional-flows-and-business-rules.md
- [x] doc-03-internal-architecture-and-design.md
- [x] doc-04-interface-and-integration-contract.md
  - [x] `contracts/merchants-openapi.v1.yaml` (OpenAPI 3.1, passes `redocly lint`)
  - [x] `contracts/webhook-config-read-contract.sql` (read-only view for Member 8)
  - [x] `contracts/merchant.status_changed.v1.schema.json` (proposed event, U-09)
- [x] doc-05-data-and-persistence.md
  - [x] `migrations-draft/` (4 up/down pairs — tested on PostgreSQL 16, see doc6-tests)
- [x] doc-06-testing-strategy-and-acceptance.md
  - [x] `doc6-tests/merchants-domain/` (119 Go tests passing, -race, 96.2% coverage)
  - [x] `doc6-tests/migrations/` (41 DB assertions passing, up/down/up)
- [x] doc-07-security-failure-and-recovery.md (§2: C-1, C-2, C-4 applied; C-3 needs ADR amendment)
- [x] doc-08-implementation-deployment-and-ops.md
  - [x] `doc8-local/` runnable merchants slice (Docker Compose; 49/49 end-to-end checks, graceful shutdown, config refusal)
