# General (Non-Member) Sections of the Design Report — Fixes Required

> **Checked:** `SwiftDrop_System_Architecture_and_Design_Specification.pdf`, the shared parts: front matter (pp. 1–12), Ch. 1–3 (pp. 13–26), Ch. 14 (pp. 110–115), Ch. 15 (116–117), Ch. 16 (118–120), Ch. 17 (121–127), Ch. 18 general parts (128, 131 and Table 18.1), Ch. 19 (132–133), References/Appendices (134–137).
> **Compared against:** the repository code and migrations, `docs/architecture/*` (ADR-001…004, `database-schema.md`, `domain-boundaries.md`), `docs/contracts/*`, `AGENTS.md`/`CLAUDE.md`, and the member drafts (esp. Member 9's agreed health paths).
> **Limit:** text only. I could not render the PDF pages, so figures (2.1, 3.1, 14.1, 14.2, 17.1–17.6) were **not** visually checked; §8 lists what each must show.
> **Member chapters (4–13) are out of scope here**, except where a general chapter repeats their claims. Member-8 items are in `member-08-events-messaging/REPORT_FIXES_REQUIRED.md`.

Severity: **P1** = contradicts the code or authoritative repo docs; **P2** = inconsistent inside the report or incomplete; **P3** = wording.

---

## 1. Biggest problem: the report claims things the repository does not contain (P1)

The report is written in past tense ("implemented", "delivered", "verified") for work that the repository does not contain. Verified state of the repo today:

| Report claim | What actually exists |
| :--- | :--- |
| Release v1.0 includes "canonical PostgreSQL migrations (000001 through 000011)" (§17.1, §14.4) | **Four** migration files: identity tables, seed users, outbox, worker tables. They use timestamp names (`20261021000001_…`, `20261101000050_…`), not `00000N_…`. No zones, pricing, parcels, couriers, dispatch, merchants, idempotency or audit-log migrations exist. |
| "Working Argon2id auth, JWT provider, **Google OIDC**" (Table 18.1, Exec Summary p. 3, §1.2 obj. 4, §15.2 Adapter) | Hashing is **salted SHA-256, 10,000 rounds**; tokens are **HS256**; **no OAuth/OIDC code**; only `POST /api/v1/auth/login` and `GET /api/v1/auth/me` are routed (no register, refresh, logout, password reset). |
| Refresh-token rotation with family invalidation on replay (§17.2.3) | Table `refresh_tokens` with `family_id` exists, but **no refresh endpoint exists** to rotate or invalidate. Login issues the first token only. |
| "Docker Compose 10-service stack", Kubernetes manifests for all services, distroless Dockerfiles for all Go services, Helm charts (Ch. 12, Table 18.1) | No root `docker-compose` file, no Core API Dockerfile (only draft copies under `docs/drafts/*/doc8-local`), three Kubernetes files (`postgres`, `rabbitmq`, `keda`/`hpa`, one CronJob). Messaging's Dockerfile is `alpine`, not distroless. |
| Parcels/dispatch/merchants/tracking/AI "implemented" (Table 18.1, §18.3) | `core-api/internal/{parcels,pricing,dispatch,delivery,merchants,admin}` and `services/{tracking,ai-assistant}` contain only `README.md`/`API.md` (documentation). |
| "golangci-lint reports zero violations" and "npm test confirms 100% passing component tests" (§17.5) | Core API tests pass. `web/package.json` has **no `test` script** and there are **no frontend test files**, so the `npm test` claim is false. `golangci-lint` could not be run on my machine (toolchain mismatch), so "zero violations" is unproven by me. |

**Fix:** either (a) reword to "designed/specified" with a clear status table (Implemented in Release v1.0 / Specified, Phase 2), or (b) make the claims true before submission. Ch. 17 already limits the v1.0 baseline to "simple login with pre-seeded accounts" — apply that honesty everywhere (Exec Summary, §1.2, Table 18.1, §18.3 sheets, §19.1, Ch. 12). A marker comparing the PDF with the repo will find these.

Add a one-page **"Implementation status"** table near the Executive Summary: module → Specified / Implemented / Verified, and what each verification means.

---

## 2. Front matter (pp. 1–12)

| Item | Issue | Fix |
| :--- | :--- | :--- |
| Governance Charter (p. 2) | Calls the document "the definitive single source of truth". The repository's own precedence (`AGENTS.md` §1) is: SRS → `docs/contracts` & `docs/architecture` → Engineering Standards → `AGENTS.md` → code. | Say "authoritative for this submission; where it differs from the SRS, `docs/contracts` or `docs/architecture`, those prevail", and list known deviations (see §4). (P2) |
| Executive Summary (p. 3) | "defines and **verifies** the production baseline… featuring robust **Argon2id** hashing… federated **Google OAuth 2.0/OIDC**" — not implemented (see §1). Also "PostgreSQL read replicas" — no replica exists in the repo; ADR-002 says "read replica **/pool**". | Replace with the real baseline: Go monolith, SHA-256-10k hashing, HS256 JWT, RBAC middleware; list Argon2id/RS256/OIDC as ADR-003 target. Say "read replica or pooled read connections". (P1) |
| Table of Contents / Lists (pp. 4–10) | Re-generate after edits; check Figure 14.2/Table numbers and that every figure caption listed exists once. | Re-export. (P3) |
| Abbreviations (pp. 11–12) | Missing terms used in the report: DLX, KEDA, HPA, K3s, SKIP LOCKED, LKR, SRS, CQRS (if used), UUIDv7. | Add. (P3) |

---

## 3. Chapter 1 — Introduction, actors, RTM (pp. 13–16)

- **§1.2 objective 4:** "Argon2id… **RS256/HS256** dual-token JWT" — pick one truth: ADR-003 (accepted) says **RS256/EdDSA + Argon2id**; the code is **HS256 + SHA-256×10,000**. Write: "Target (ADR-003): …; Release v1.0 baseline: … (documented deviation)". (P1)
- **§1.2 objective 5:** "production-ready Kubernetes (K3s) deployment manifests" — only partial manifests exist (§1). (P1)
- **§1.3:** five human actors match the five roles in code. The "three automated actors" (Relay, Worker, AI) is right; add the **Reconciler** (CronJob) as an automated actor. (P3)
- **Table 1.1 (RTM)** data-store names do not match the authoritative `docs/architecture/database-schema.md` table list:

| Report | Authoritative / implemented |
| :--- | :--- |
| `quotes` | `pricing_quotes` |
| `dispatch_assignments` | `courier_assignments` |
| `merchant_api_keys` | `merchant_api_credentials` |
| `webhook_configs` / `merchant_webhook_configs` | `webhook_endpoints` |
| BR-08: `outbox_events`, RabbitMQ, DLQ | add `processed_events`, `notification_deliveries`, `webhook_deliveries`, `webhook_delivery_attempts` |
| BR-01: `users, roles, user_roles` | add `refresh_tokens`, `oauth_identities` |
| (missing) | `parcel_addresses`, `ai_conversations`, `audit_logs`, `idempotency_keys` |

  BR-05 says "load tests verifying **1,000** concurrent SSE streams" while §2.2 (p. 18) requires "up to **5,000** concurrent streaming connections". Make them equal. (P2)
- BR-01 verification: "Unit tests for **Argon2id**" → unit tests for the implemented SHA-256 hasher. (P1)

---

## 4. Chapter 2 — Requirements and use cases (pp. 17–20)

- **§2.2 item 5 (Supportability):** correlation header `X-Correlation-ID` → team agreement A10 and the event contract use **`X-Request-ID`**. Probes `/healthz, /readyz` → agreed and implemented paths are **`/health/live`, `/health/ready`, `/metrics`** (Member 9 README rule C1; Core API now serves them). (P1)
- **§2.2 item 4:** keep the sub-100 ms/200 rps and 5,000 SSE targets only if a test or plan backs them; otherwise say "target". (P3)
- **§2.1 hierarchy (F-01…F-28):** fine as a requirement list; make sure each Tier-1/2/3 item cites SRS IDs, since a marker will compare with the SRS. Tier 2 says "RabbitMQ consumer workers" for email — implemented (Member 8). (P3)
- **§2.3 (UC-10 → UC-18/19):** states assignments "immediately reflect on courier mobile devices and merchant dashboards" via the notification workflow. The worker sends **customer emails and merchant webhooks**, not courier-device pushes. Reword. (P2)
- **§2.4 (p. 20):** the Event Worker does **not** push telemetry to the Tracking API; the Tracking API consumes `q.tracking.updates`. (P1; also in member-8 file.)

---

## 5. Chapter 3 — Architecture, ADRs, stack (pp. 21–26)

- **§3.1/§3.3:** "Monolith + **3** lightweight specialized containers" but four deployables are listed (Tracking, Outbox Relay, Event Worker, AI Assistant) — plus the Reconciler CronJob. Make the count consistent. (P2)
- **§3.2:** state the Outbox Relay is **exactly one replica** (ADR-002 strict singleton) while Tracking/Core API/Worker scale (HPA/KEDA). (P2)
- **§3.4 ADR summaries differ from the accepted ADRs in `docs/architecture`:**
  - **ADR-003** (report): Argon2id + "access tokens signed with HMAC-SHA256". Repo ADR-003: **RS256/EdDSA** (public JWKS), Argon2id, hashed B2B API keys. State both, and flag the deviation. (P1)
  - **ADR-004** (report): "third failure → `RETURNED_TO_FACILITY`". Repo ADR-004: terminal state is **`RETURNED`**; a parcel has **9** states (CREATED, ASSIGNED, PICKED_UP, IN_TRANSIT, OUT_FOR_DELIVERY, DELIVERED, ATTEMPT_FAILED, RETURNED, CANCELLED). The report says "canonical **10-state** FSM" in Ch. 5, Ch. 18 (p. 128/130) and Table 18.1. Unify with the authoritative state list (notify Member 2). (P1)
  - ADR-001/002 summaries are consistent with the repo.
- **§3.5:** fine (row locking, partial indexes). "PostgreSQL **16**" here vs "PostgreSQL **15+**" in §4.1 — unify to 16. (P2)
- **§3.6 money:** BIGINT minor units is correct (authoritative doc also allows `NUMERIC(12,2)`; keep BIGINT as the chosen rule). Add: currency column `varchar(3)` default `LKR`. (P3)
- **§3.7 / Table 3.2:** "Distroless image <20 MB" as a Go advantage — the only Dockerfile in the repo is alpine-based. Soften to "can produce small images (alpine/distroless)". (P2) The memory numbers ("<45 MB baseline", "400–800 MB JVM") are unsourced; cite or soften. (P3)

---

## 6. Chapter 14 — Data architecture (pp. 110–115)

- **Count is inconsistent:** title and Fig. 14.1 say "**14** tables"; Table 14.1 lists **18** rows (users, roles, user_roles, oauth_identities, delivery_zones, pricing_rules, parcels, parcel_status_history, quotes, couriers, dispatch_assignments, delivery_attempts, merchants, merchant_api_keys, merchant_webhook_configs, outbox_events, worker_deduplication, idempotency_keys), and migration 11 adds `audit_logs`. Authoritative list adds `refresh_tokens`, `parcel_addresses`, `notification_deliveries`, `webhook_endpoints`, `webhook_deliveries`, `webhook_delivery_attempts`, `processed_events`, `ai_conversations`. Recount and use **one** number everywhere (ERD, Table 14.1, §14.2, Exec text). (P1)
- **Names to correct (authoritative → report):** `pricing_quotes` (not `quotes`), `courier_assignments` (not `dispatch_assignments`), `merchant_api_credentials`, `webhook_endpoints`, `processed_events` (composite PK `(consumer_name, event_id)`; `worker_deduplication` does not exist). (P1)
- **Column names:** authoritative `parcels.sender_id` (report: `customer_id`), `parcels.current_status` (report: `status`), `parcel_status_history.occurred_at` (report: `created_at`), `oauth_identities.provider_subject` (report: `provider_user_id`), `roles.id` is `VARCHAR(50)` role name (report: `INT`). `users` also has `display_name`, `phone`, `role`, `status`; `tenant_id` is `VARCHAR(100)`. (P1)
- **§14.4 migration plan:** the report's 11 numbered steps do not exist as such. Either (a) present them as the **logical dependency order** and add "implemented as timestamped files (the migrator sorts by filename)", or (b) rename to match the repo. State that migrations contain no `BEGIN/COMMIT` (the migrator wraps each file in a transaction). (P1)
- **§14.4 step 9 & §14.5 item 3 (outbox):** step 9 description and the partial-index definition are wrong (see member-8 file): real `idx_outbox_pending ON outbox_events (created_at ASC) WHERE status='PENDING'`. (P1)
- **§14.5 item 1:** "UUIDv4 primary keys". Repo standard is **UUIDv4 or UUIDv7** (events use UUIDv7). Say so. (P3)
- **§14.2 normalization:** "composite addresses … encapsulated as validated JSONB" conflicts with the authoritative `parcel_addresses` table. Decide one (Member 2) and align ERD + text. (P2)
- **Figure 14.2:** must show Member 4's `webhook_endpoints` before Member 8's delivery tables (FK).

---

## 7. Chapters 15–19 and appendices

### Ch. 15 (pp. 116–117) — patterns (P1/P2)
- **ISP example `EventPublisher`** — not in code (real: `events.OutboxWriter`).
- **Observer in `internal/events/`** — misattributed (RabbitMQ publish/subscribe, Tracking SSE subscribers are the observers).
- **Adapter "(Argon2, Google OIDC)"** — neither exists; real adapters: SHA-256 hasher, JWT provider, Postgres repositories, SMTP sender, AMQP publisher.
- **Circuit Breaker in Event Worker** — **not implemented**. Remove, or state "planned".
- **Factory `NewParcel`** and **Strategy in `internal/pricing/`** — the packages contain only docs; mark "specified". Verified real patterns: Repository, Transactional Outbox, Idempotent Consumer, Dependency Inversion via `OutboxWriter`/ports, `WithTx` (Unit of Work) in `platform/db`.
- **Unit of Work** is accurate only for `platform/db.WithTx` (new) — name it.

### Ch. 16 (pp. 118–120) — security (P1/P2)
- **Table 16.1 spoofing mitigation** "RS256/HS256… secret rotation; strict Argon2id" — fix per §1 (SHA-256×10,000 salted, constant-time compare; HS256; **no secret rotation implemented**; refresh-token revocation is a table only).
- **Table 16.2 RBAC matrix routes** differ from the contract/implementation: pricing is **`POST /api/v1/pricing/quote`** (not `/api/v1/quotes`); `/api/v1/auth/register` is **not implemented**; implemented protected routes today are `/api/v1/auth/me`, `/api/v1/admin/dashboard`, `/api/v1/courier/dashboard`, `/api/v1/merchant/portal`, plus the new `/api/v1/admin/events/*` and `/api/v1/merchant/webhook-deliveries*`. Mark each row **Implemented** or **Specified**.
- **Rate limiting "10 req/min on auth, 120 req/min on APIs"** — Traefik config lives in `deploy/gateway` (Member 7). Confirm the numbers exist there before citing as implemented.
- **§16.3 IDOR:** good. Add the real isolation rule used by the events API: merchant endpoints resolve `tenant_id` as a merchant UUID and return 404 for other tenants' rows; note the seeded `mrc_…` tenant is not a UUID (known gap).
- **§16.4 PII:** add **GDPR/APPI** → the report only mentions "international privacy regulations"; also Sri Lanka **PDPA No. 9 of 2022** is the applicable local law (the report is for a Sri Lankan operator; §1934 mentions GDPR/APPI only). Add the SMTP/log rules: emails masked in logs, webhook payloads strip `notify`. (P3)
- **Cookie flags:** report says refresh cookie is `Secure`; code sets `Secure: false` ("true in production"). State "Secure in production only". (P2)

### Ch. 17 (pp. 121–127) — baseline (P1)
- **§17.2.1:** says users use `VARCHAR(32)` keys prefixed `usr_`. Real DDL: **UUID** primary keys; `tenant_id VARCHAR(100)` holds `mrc_…`. Fix.
- **§17.2.2:** hash storage format "`<salt_hex>:<iterations>:<hash_hex>`" → real: **`$sd$sha256$<salt_hex>$<hash_hex>`** (10,000 iterations fixed in code).
- **§17.2.3:** remove/qualify refresh rotation and replay detection (no refresh endpoint; see §1). Access-token claims (`sub, email, role, tenant_id, iat, exp, jti`) — verify `email` is present in the actual claim struct (code shows `sub`, `tenant_id`, `jti`); list only claims that exist.
- **Table 17.1 (test accounts):** says accounts are "seeded into the database for grading". With PostgreSQL the seeded password hashes **do not verify** (login returns 401); the credentials work only in the **mock-storage fallback** (when PostgreSQL is not running). **A grader following the runbook against a real database will fail to log in.** Either fix the seed hashes (Member 1: regenerate with the real hasher) or state "mock mode" in the runbook. (P1)
- **§17.3 figures/numbers:** Admin console shows "1,428 users… 3,892 audit entries… 99.98% health", dispatcher "84 in-transit/210 drops", merchant "842 parcels", customer "LKR 9,450": these are **hard-coded demo data**, not from the backend. Add "illustrative mock data" so it is not read as system output. (P2)
- **§17.5:** fix the `npm test` and lint claims (§1). Paste real output (`go test -v ./internal/identity/...`) as evidence.
- **§17.1:** "migrations 000001 through 000011" → four migration files (§1).

### Ch. 18 general parts (pp. 128, 131) — workload (P2)
- **Table 18.1 "Key deliverables for Release v1.0 baseline"** repeats the Argon2id/OIDC/10-state/advisory-lock/“implemented” claims (see §1, §5). Reword each as Implemented vs Specified.
- **§18.4:** "Agreements A1–A13, migrations D1–D12" — these exist in Member 9's agreement document; cite its path or add a short appendix listing them, otherwise a reader cannot verify.
- Git flow: say what is actually enforced (CI workflow + `CODEOWNERS` currently maps `*` to two reviewers; no per-module owners).

### Ch. 19 (pp. 132–133) (P2)
- **§19.1** "atomic transactional outbox persistence, strict RBAC… eliminates architectural drift" — rephrase as design intent; Release v1.0 contains identity RBAC and (now) the outbox library.
- **Risk register** is missing risks the repo itself documents: (a) seeded credential mismatch with PostgreSQL, (b) merchant tenant ID not a UUID, (c) algorithm deviation from ADR-003, (d) single-instance Outbox Relay as a failure point (mitigated by lease + reconciler), (e) integration tests need Docker (`INTEGRATION=1`), (f) `-race`/lint only in Linux CI. Add 3–5 of these with owners.
- **§19.3:** roadmap says the Outbox Relay and Event Worker are Phase 2. They are implemented now; update to "delivered; remaining: integration with producers (Members 2/3/4) and Kubernetes deployment".

### References and Appendices (pp. 134–137) (P2)
- **Appendices A, B, C are one-line placeholders** ("contains full OpenAPI…", "provides complete SQL DDL…", "outlines exact step-by-step…") but the content is not in the PDF. A marker will notice. Either include the content (the OpenAPI JSON for events exists in `core-api/internal/events/transport/openapi.json`; the real migrations exist for identity/outbox/worker; the demo runbook exists in `member-08-events-messaging/DEMO_SCRIPT.md`) or remove the claims and point to repository paths.
- Reference list: [1]–[9] are fine; add RFC 6902-style references only if used. Add the SRS and the four ADRs as project references, and RabbitMQ/PostgreSQL docs if cited.

---

## 8. Diagram checklists (images not reviewed)
- **Fig. 2.1 (use cases):** actors = Customer, Merchant, Courier, Dispatcher, Admin + Relay, Worker, Reconciler, AI assistant (Tracking is a service, not an actor). Use-case IDs referenced in text (UC-02, UC-03, UC-10, UC-18, UC-19) must exist in the figure.
- **Fig. 3.1 (topology):** Traefik → Core API / Tracking API / Web / AI; Core API → PostgreSQL only (no arrow to RabbitMQ); Relay (1 replica) PostgreSQL → RabbitMQ; Worker ← RabbitMQ → SMTP and merchants; Tracking ← `q.tracking.updates`, Redis Pub/Sub; Reconciler CronJob.
- **Fig. 14.1 / 14.2:** table names exactly as in the authoritative list; counts matching the text; FK `webhook_deliveries → webhook_endpoints`.
- **Fig. 17.1–17.6:** screenshots of the demo UI — add a caption "demonstration data".

## 9. Search-and-fix checklist before re-export
- [ ] `Argon2id`, `RS256`, `OIDC`, `Google` → qualified as target/planned wherever the code differs.
- [ ] `X-Correlation-ID` → `X-Request-ID`; `/healthz`, `/readyz` → `/health/live`, `/health/ready`.
- [ ] `quotes`, `dispatch_assignments`, `merchant_api_keys`, `webhook_configs`, `worker_deduplication`, `customer_id`, `RETURNED_TO_FACILITY`, "10-state" → authoritative names/counts.
- [ ] "14 tables" and every "000001 … 000011" statement → one verified number / timestamped names.
- [ ] "100 %", "zero violations", "verified", "implemented" → backed by something you can name or removed.
- [ ] Appendices A–C filled or removed; all lists/figure numbers regenerated.
