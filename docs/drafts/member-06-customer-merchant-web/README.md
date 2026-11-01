# Member 06 — Customer & Merchant Frontend Specifications

> **Owner:** Sajini, Member 6 — Customer & Merchant Frontend  
> **Status:** Draft Specifications Complete & Harmonized (Ready for Team Review in Meeting 2)  
> **Target Production Location:** `web/src/features/customer-merchant/` (Inside Unified React SPA)  
> **Governing Standards:** SwiftDrop SRS, ADR-001, ADR-002, ADR-003, `AGENTS.md`, and Team Agreements with Member 9  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Required Documents Checklist

All eight mandatory module specification documents have been authored and harmonized according to the team review structure:

- [x] [Document 1 — Module Scope & Responsibility Specification](./doc-01-scope-and-responsibilities.md)
- [x] [Document 2 — Functional Flows & Business Rules](./doc-02-functional-flows-and-business-rules.md)
- [x] [Document 3 — Internal Architecture & Design Specification](./doc-03-internal-architecture-and-design.md)
- [x] [Document 4 — Interface & Integration Contract](./doc-04-interface-and-integration-contract.md)
- [x] [Document 5 — Data & Persistence Specification](./doc-05-data-and-persistence.md)
- [x] [Document 6 — Testing Strategy & Acceptance Specification](./doc-06-testing-strategy-and-acceptance.md)
- [x] [Document 7 — Security, Failure & Recovery Specification](./doc-07-security-failure-and-recovery.md)
- [x] [Document 8 — Implementation, Deployment & Operations Guide](./doc-08-implementation-deployment-and-ops.md)

### Supporting Artifacts
- **API Inventory:** [`integration-inventory.json`](./integration-inventory.json) (Machine-readable inventory of consumed endpoints and transport contracts)
- **Reference Guide:** [`MEMBER_06_ALIGNMENT_AND_SPECIFICATION_GUIDE.md`](./MEMBER_06_ALIGNMENT_AND_SPECIFICATION_GUIDE.md) (Architectural action sheet for Meeting 2)

---

## 2. Executive Summary of Member 6 Domain

The **Customer & Merchant Frontend** delivers the primary external-facing user interfaces within the unified React SPA (`web/`):

1. **Customer Onboarding & Login:** Self-registration form (`/auth/register`) capturing customer details and shared login (`/auth/login`) with in-memory JWT storage and Google OAuth integration.
2. **Four-Step Guided Booking Wizard (`/portal/book`):**
   - Address input (sender/recipient)
   - Parcel dimensions & zone selection with Zod validation
   - Server-side fee quotation (`POST /api/v1/pricing/quotes`) with 15-minute countdown preview
   - Idempotent confirmation dispatching `POST /api/v1/parcels` with a stable UUIDv4 `Idempotency-Key` and receiving the authoritative tracking number (`SD-YYYYMMDD-XXXXX`).
3. **Public & Authenticated Live Tracking:**
   - Public view (`/track/:trackingNumber`) strictly masks private customer data (SEC-016), displaying only status, timestamps, and city/province.
   - Resilient `useSSE` hook with initial REST fetch, event deduplication, automatic reconnect re-fetching, and bounded polling fallback (every 15s, max 20 cycles).
4. **Merchant Self-Service Portal (`/portal/merchants/*`):**
   - Application status tracking (`PENDING`, `APPROVED`, `REJECTED`, `SUSPENDED`).
   - Order management and client-side validated CSV batch order upload UI.
   - Machine-to-machine API key management (`sd_live_*`) and webhook configuration (`whsec_*`) with one-time copy modal and zero browser storage persistence.
5. **Quality & DevOps Standards:**
   - Standardized `data-testid` attributes coordinated with Member 10 for Playwright E2E automation.
   - Clean compilation via `npm run build` targeting `dist/` served by Member 9's Nginx container on port 3000 driven by `VITE_API_BASE_URL=/api/v1`.

---

## 3. Alignment Resolution & Harmonization Matrix

This section summarizes the alignment issues resolved during the harmonization process, the specific documents updated, and any remaining cross-module dependencies:

| Alignment Issue | Previous State / Contradiction | Resolved Specification & Alignment Action | Documents Updated | Remaining Dependency / Action |
| :--- | :--- | :--- | :--- | :--- |
| **Pricing Quote Path** | Earlier text used `/pricing/quote` (singular) | Resolved to `POST /api/v1/pricing/quotes` (plural) conforming to Member 2's Doc 4 contract | `doc-02`, `doc-04`, `integration-inventory.json` | None. Path harmonized with Member 2. |
| **Tracking SSE Route** | Guide proposed `/tracking/{id}/stream`; M5 proposed `/tracking/public/{id}/stream` | Adopted `/api/v1/tracking/public/{trackingNumber}/stream` for public tracking stream; hook supports path aliasing | `doc-02`, `doc-03`, `doc-04`, `integration-inventory.json` | None. Confirmed with Member 5. |
| **Merchant Browser Auth vs. M2M Keys** | Ambiguity on whether browser stores or sends API keys | Explicitly established that the browser uses human **JWT Bearer** session (`tenant_id`). `X-API-Key` is strictly for backend servers | `doc-01`, `doc-02`, `doc-04`, `doc-07` | None. Aligned with Member 4's Doc 4 §3.1. |
| **Credential Persistence** | Risk of caching newly generated API keys or webhook secrets | Implemented **one-time modal display** with explicit copy warnings and strict zero persistence in browser storage or logs | `doc-01`, `doc-02`, `doc-05`, `doc-07` | None. Validated in security spec. |
| **E2E Test ID Alignment** | Minor naming differences between Guide and Member 10 specs | Implemented **dual-attribute support** (e.g. `quote-price-cents` + `booking-estimated-fee-display`) | `doc-03`, `doc-06` | Member 10 will verify against Playwright suite. |
| **DevOps & Container Handoff** | Unclear ownership of Dockerfile and static ports | Confirmed **Member 9 owns Dockerfile and Nginx**; Member 6 provides `npm run build` &rarr; `dist/` on port 3000 | `doc-01`, `doc-08` | None. Formally agreed in Agreement A3. |
| **Review File Naming** | Files were named `01-...` to `08-...` | Renamed via `git mv` to `doc-01-...` through `doc-08-...` matching team review convention | All files renamed | None. 100% consistent with other members. |
| **Receipt Format Contract** | Unspecified whether receipt is PDF or JSON | Frontend inspects `Content-Type`: downloads if binary PDF, renders print layout if JSON | `doc-04` | **Member 2** to confirm final format. |
| **Authenticated SSE Stream** | EventSource header limitations for private streams | Documented public SSE usage; private tracking falls back to REST polling until transport is agreed | `doc-04`, `doc-03` | **Member 5** to confirm private stream transport. |

---

## 4. Current Status & Review Disclaimer

These specifications are complete, harmonized, and ready for team review in Meeting 2. All workflows, schemas, and test scenarios represent planned designs; no application implementation or test execution is falsely claimed. Implementation will proceed once the team approves the specifications on `docs/module-spec-reviews`.
