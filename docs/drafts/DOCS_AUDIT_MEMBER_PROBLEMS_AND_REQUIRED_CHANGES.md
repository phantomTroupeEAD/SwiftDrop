# SwiftDrop Docs Audit — Problems per Member & Required Changes in `docs/`

> **Date:** 2026-10-26 · **Branch:** `docs/module-spec-reviews`
> **Scope:** All 10 member draft folders in `docs/drafts/`, compared with the authoritative docs (`docs/architecture`, `docs/contracts`, `docs/operations`, `docs/security`, `docs/testing`), the SRS, the work division, `AGENTS.md`, the Master Audit, and Member 9's team agreements (A/D/K items).
> **Method:** Four read-only review passes (M1–3, M4–5, M6–7, M8–10) with file-level evidence. I re-checked a sample directly (the `/livez` vs `/health/*` conflict, the KEDA and CronJob manifests, the `parcel.created` naming, `/pricing/quote`, the missing M7 `doc-04`) and all held. The rest of the findings are from the review passes: treat line numbers as pointers, and confirm each before editing.
> **Nothing was edited.** This file is the only change.

---

## 1. Headline problems (cross-cutting)

1. **The "alignment guides" claim fixes that the member's `doc-01…08` never received.** M1, M2, M3, M4, M5 and M7 guides say "resolved / 100% harmonized", but the underlying docs still carry the old names, `Pending`/`U-xx` markers and TODOs. The Master Audit repeats the claims. Reviewers reading `doc-0x` will see the old contracts.
2. **Requirement IDs are wrong in many guides.** FR-AUTH, FR-PRC/BOK, FR-DSP, FR-POD, NFR-REL-002, NFR-001 do not exist in the SRS. Several SEC-xxx IDs are mapped to the wrong meaning (SEC-016 is "container hardening" in the SRS but used for "public tracking privacy" by M5, M6, `security-notes.md`, ADR-004; privacy is SEC-009). Real IDs: FR-IAM-001..009, FR-PAR, FR-LIFE, FR-COU, FR-TRK-001..007, FR-MER-001..011, FR-EVT-001..010, FR-REC-001..005, SEC-001..017.
3. **Four naming decisions are open and each member picked a different answer.** These are the root of most contract mismatches:
   - **JSON casing/envelope:** snake_case + `{data}` (api-conventions) vs camelCase (M1, M5 doc-04, M6).
   - **Health paths:** `/health/live`, `/health/ready` (SRS, Agreement A2) vs `/livez`, `/readyz` (authoritative runbook, deployment guide, AT-016, M4, M7).
   - **Migration naming:** 6-digit sequential (Master, AGENTS.md) vs timestamps (Agreement D2, still "vote needed").
   - **Object storage env names:** `S3_*` (M3, K15) vs `OBJECT_STORAGE_*` (env doc, Master).
4. **Event contract is out of sync everywhere.** `parcel.created` (authoritative `webhook-security.md`, `sequence-diagrams.md`, M10) vs `parcel.booked.v1` (`event-contracts.md`, M8); `parcel.delivery_failed.v1` vs `parcel.delivery.failed` (ADR-004) vs `parcel.failed.v1` (M8 drafts); `merchant.webhook_dispatch.v1` still in the catalog though M4 and M8 reject it; `in_transit`, `returned`, `cancelled`, `courier.unassigned`, `merchant.*` missing from the catalog; ID formats `evt_…`/`prcl_…` vs UUIDs.
5. **Deploy manifests contradict the agreements** (verified): KEDA watches queue `parcel.events`, one trigger, max 2, secret key `amqp-uri`; the reconcile CronJob uses `swiftdrop/core-api:latest` with `/app/server reconcile …`. Agreements say two queues, max 3, `RABBITMQ_URL`, SHA-tagged `messaging` image, `/usr/local/bin/reconcile`.
6. **Parcel state machine is inconsistent.** M2 has no `OUT_FOR_DELIVERY`/`ATTEMPT_FAILED`; M3 jumps `ASSIGNED → OUT_FOR_DELIVERY`, skipping `PICKED_UP`/`IN_TRANSIT`. `domain-boundaries.md` and ADR-004 define the authoritative path.
7. **Schema drift.** `database-schema.md` does not match what members actually designed (users/roles model, `parcel_addresses` vs JSONB, courier/assignment/attempt columns and enums, webhook/notification/processed_events tables, `refresh_tokens`, `merchants.status` DISABLED, zone fields, audit columns).
8. **Member 7 has no `doc-04` file**, which the required 8-document set (`drafts/README.md` §3) mandates. Confirmed missing.
9. **Many items are still "pending" with no owner decision:** D2, D4, D6, D10, K15, M9's sign-off table (empty), M1/M5/M6 marked ❌ in M9's tracker, HPA range, Redis for tracking, hosting choice.

---

## 2. Problems by member

Severity: **C** = breaks integration/build, **H** = contract mismatch that will fail, **M** = inconsistency, **L** = cleanup.

### Member 1 — Identity (`member-01-identity`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| C | User model differs: M1 uses `users.role`, `full_name`, `is_active`; `database-schema.md` has `roles`/`user_roles`, `display_name`, `status ACTIVE/SUSPENDED`. Master endorses the single role column. `refresh_tokens` missing from the schema file. | Decide one model, then update M1 doc-05/08 **and** `database-schema.md`. |
| H | JWT: M1 defaults to HS256/`JWT_SECRET`; ADR-003 and the env doc require RS256/EdDSA, `JWT_PRIVATE_KEY_PEM`/`JWT_PUBLIC_KEY_PEM`, `JWT_ACCESS_EXP_MINUTES`, JWKS at `/.well-known/jwks.json`. | RS256 only; use env-doc names; add JWKS endpoint to doc-04. |
| H | FR-IAM-007/009 gaps: no suspend/disable-account or revoke-sessions endpoint (M7 needs it); audit is vague (`AUTH_*` names vs expected `MERCHANT_APPROVED` style). M4 needs `RoleAssigner.GrantMerchantRole(tx)` and `AuditWriter.Write(tx)` from M1; they don't exist. `TenantResolver` has M1 calling M4 (inverted dependency, import-cycle risk). | Add endpoints and interfaces; invert the dependency (M4 supplies, or JWT carries `tenant_id` resolved on refresh). |
| H | camelCase responses, 400 for validation (conventions say 422), `usr_` vs UUID IDs, courier refresh 7 days vs ADR-003's 14. | Align to api-conventions and ADR-003. |
| M | OAuth path `/auth/google` (SRS, M1, M6) vs `/auth/oauth/google` (ADR-003, sequence-diagrams, env doc). | Pick one (SRS: `/auth/google`) and fix ADR-003/env doc/diagrams. |
| M | Seeds `@swiftdrop.local` (docs) vs `.com` (guide); Compose DB URL uses `swiftdrop:swiftdrop123` not `swiftdrop_app`. | Update docs. |
| M | Wrong IDs: hashing mapped to SEC-001 (is SEC-002), audit to SEC-009 (is SEC-015), FR-ADM-004 for roles (is FR-ADM-006); "verified email" optional vs FR-IAM-001. | Correct. |
| L | 10 s refresh grace window vs ADR-003 instant family invalidation. | Align. |

### Member 2 — Parcels & Pricing (`member-02-parcels-pricing`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| C | State machine omits `OUT_FOR_DELIVERY`/`ATTEMPT_FAILED`, uses `FAILED`; contradicts `domain-boundaries.md` and ADR-004. | Adopt the authoritative table; drop `FAILED`; 3rd failed attempt forces `RETURNED`. |
| H | Pricing: no service-level multiplier (pricing-rules.md); example arithmetic wrong (55000 vs 51000 at baseline 8000); `float64`/`math.Ceil`; `pricing_rules`/`pricing_quotes` miss `version`, `per_volumetric_kg_fee_cents`, `rule_version_id`. | Follow `pricing-rules.md`; use integer arithmetic. |
| H | Tables: `parcel_addresses` dropped for JSONB (schema file and Master still list it); `price_cents` vs `fee_cents`; history columns missing (`actor_role`, `reason_code`, `note`, `parcel_version`); `ON DELETE CASCADE` violates RESTRICT rule. | Decide JSONB vs table; update both sides. |
| H | Idempotency table: PK `(key, caller_id)` with FK to `users` vs authoritative `key` + `caller_scope`; M4 uses `MERCHANT:<uuid>`, which cannot satisfy a users FK. Retention "open" vs 24 h in api-conventions. | Use `caller_scope`; set 24 h. |
| H | Events: M2 emits only booked/cancelled; M8 and M4 expect M2 to emit picked_up/delivered/returned; no endpoint produces picked_up/in_transit; no `recipient_email`/`notify`/`merchant_id` (M8 CR-M02-01); `parcel.cancelled.v1` not in the catalog. | Decide event ownership (also see `domain-boundaries.md` edit); add fields. |
| H | No cancel endpoint though the event implies one; M7 expects `/admin/pricing-rules` CRUD from M2 (FR-ADM-007) and M2 lists none. | Add endpoints. |
| M | Quote response names differ across members (`fee_cents`/`expires_at` vs `amount_minor`/`valid_until`); merchant path `/merchant/parcels` vs SRS `/merchant/orders`; zone ID examples differ. | One shared contract (see §3). |
| M | Open TODOs remain (doc-02, 03, 04, 05, 08); doc-07 reuses SEC-001..018 for its own test IDs, colliding with the SRS; doc-01 cites no FR IDs; DB pool and DB name/role differ from D6/D8. | Clean up. |

### Member 3 — Dispatch & Delivery (`member-03-dispatch-delivery`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| C | `ASSIGNED → OUT_FOR_DELIVERY` skips `PICKED_UP`/`IN_TRANSIT`, so those events never fire; M2 has no such state. | Follow `domain-boundaries.md`, or get an explicit rule there. |
| C | Raw SQL `UPDATE parcels` and inserts into `parcel_status_history`/`outbox_events` from M3 code violate domain boundaries; outbox insert omits NOT NULL `correlation_id`/`aggregate_version`; history misses `actor_role`/`parcel_version`; delivery use case never checks status = `OUT_FOR_DELIVERY`. | Go through M2's aggregate interface and `OutboxWriter.Write`. |
| H | No partial UNIQUE index on active assignment per parcel (FR-ADM-003); reattempt reuses `ASSIGNED`; cascade deletes. | Add index; fix reattempt path per ADR-004. |
| H | Schema differs from authoritative and from its own docs (couriers `BIKE`, status values/column name; `courier_assignments` column names/status enum; `delivery_attempts` proof fields). | Reconcile; then update `database-schema.md`. |
| H | Events: `prcl_`/`usr_` IDs, no `merchant_id`/`notify`; delivered payload carries recipient name and `proof_url` (PII) instead of `proof_id`; `courier.unassigned.v1` not in catalog; decisions Q6.3 says the worker reads `parcels`/`webhook_endpoints` directly (boundary + role violation); `OutboxWriter.WriteEvent` vs M8's `Write`; courier phone and free-text notes in payloads. | See M8 CR-M03-01/02. |
| H | "100% harmonized with M7" is false: M7 calls `/courier/parcels/{id}/deliver`, `/fail-attempt`; M3 has `/deliveries/{id}/complete`, `/failed`; SRS endpoints unused. | Agree one route set with M7. |
| M | `DISPATCH_INTERRUPTED` outside ADR-004 code list and DB CHECK; FR-COU-006 and FR-LIFE-006/007 untraced; invented FR-DSP/FR-POD IDs; seed UUIDs M1/M2 never create; `S3_*` vs env doc; metric label `courier_id` violates A11; presigned upload (guide) vs multipart (doc-04). | Clean up. |

### Member 4 — Merchants (`member-04-merchants`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| C | Guide's "authoritative" migration (§6) uses `target_url`, `event_types`, `is_active`; doc-05 and the contract SQL use `url`, `subscribed_event_types`, `enabled`, `secret_key_version`. Guide/Master claim M8 uses `merchant_webhook_targets_view`; M8 actually uses `merchant_webhook_targets`. | Canonical name `merchant_webhook_targets`, columns per doc-05/contract SQL; fix guide and Master. |
| C | API paths: guide `POST /merchants/register`, `/merchant/parcels`, `PUT /admin/merchants/{id}/status` vs doc-04 `/merchants/applications`, SRS `/merchant/orders`, three variants for admin approval (M7 differs again). | Use SRS `/merchant/orders`; agree admin routes with M7; update guide and M2 guide. |
| H | Key/credential design differs: prefix `char(8)` vs `VARCHAR(16)` vs ADR-003 format; lookup by hash vs by prefix; scopes `parcels:read/write` vs `parcels:create/read` vs `tracking:read`; hash scheme (HMAC+pepper) not in schema file; webhook port 8443 accepted by validator but rejected by EC-13 and `webhook-security.md`. | One definition; fix schema file and validator. |
| H | Wrong IDs: FR-MER-009/010/011 and SEC-010/011 mis-described; SEC-005 uncited. | Correct. |
| H | DB roles `swiftdrop_core_api`/`swiftdrop_tracking` (K8) vs `swiftdrop_app`/`swiftdrop_tracking_ro`; migrations `000100…`/timestamps vs `000003`, and `000003` collides with M3's draft; `/livez`; `HTTP_ADDR`/`JWT_PUBLIC_KEY_PEM` env names; pepper env `MERCHANT_API_KEY_PEPPER` vs `API_KEY_PEPPER`; secret names vs A5. Guide misstates D2 as mandated. | Apply K8/A2/A5, fix numbering. |
| M | U-15..U-47 still open; `merchant.approved.v1` vs `merchant.status_changed.v1`; in_transit/returned not in catalog; `parcel.delivery.failed` naming; tenant_id vs `owner_user_id` handling unconfirmed with M1; rate-limit path `/merchants/*` vs real `/merchant/*`. | Close/mark; fix patterns. |

### Member 5 — Tracking (`member-05-tracking`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| C | Guide SQL selects `parcels.destination_zone_id` and `parcel_status_history.reason`; schema has `parcel_addresses.zone_id` (DROPOFF) and `reason_code`/`note`; `swiftdrop_tracking_ro` has no grant on `parcel_addresses`; `delivery_zones` columns wrong. | Rewrite query; add grant; never expose free-text `note`. |
| C | Guide says "Member 8's worker publishes to Redis `tracking:updates`"; M8/`event-contracts.md` say M5 consumes `q.tracking.updates` and M8 has no Redis link. Guide code never consumes RabbitMQ. A shared queue does not broadcast to all replicas. | Define: consume queue → republish to Redis (or per-replica exclusive queue), `event_id` dedupe. M9 must decide on Redis. |
| H | Wrong IDs (FR-TRK shifted; FR-TRK-006/007 missing; SEC-008 vs SEC-009; NFR-001 nonexistent). doc-01…08 cite none. | Correct. |
| H | Path/event drift: canonical stream path differs among guide, doc-04, M6, Master (two places), SRS, ADR-002, sequence-diagrams, env doc; authenticated history `/tracking/{n}` vs `/tracking/{n}/history`; SSE event `tracking-update` vs `status_update` vs `milestone_updated`; DTO camelCase vs snake_case + envelope. | Choose one set (recommended: `/tracking/public/{n}`, `/tracking/public/{n}/stream`, `/tracking/{n}/history`) and apply everywhere, including SRS. |
| H | Naming: `tracking-api/` vs `services/tracking/`; role `tracking_reader`/`swiftdrop_tracking`/`swiftdrop_tracking_ro`; sample `SWD-EXAMPLE-001` vs `SD-YYYYMMDD-XXXXX` (5 vs 6 chars); `/livez`; `PORT` vs `TRACKING_HTTP_ADDR`. | Align. |
| M | SSE `id` not replayable but Last-Event-ID claimed; code block missing closing brace/fence; 5-slot channel drops events; `estimated_delivery` and `zone_id` exposed with no source/permission. | Fix. |
| M | Pending: DB role, pool (D8 = 5×4), health paths, Redis, HPA 2–5 vs 2–4, error envelope; `EventSource` cannot send Bearer for authenticated SSE; `tenant_id` checks unspecified. | Decide. |

### Member 6 — Customer/Merchant Web (`member-06-customer-merchant-web`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| H | SSE: listens only for `tracking-update`; M5's guide emits `status_update`. | M5 picks the name; M6 listens to both until then. |
| H | Tracking DTO: bare camelCase object vs M5's `{data:{…snake_case}}`; M6 shows "origin city", M5 exposes destination only. | Align to api-conventions. |
| H | Naming: `perPage`, `totalRecords`, `reason`, `amountMinor`, `quoteId` vs api-conventions `per_page`, `total_items`, `details[].issue`, `amount_cents`; M1 returns camelCase. Money fields disagree four ways (`amount_minor`, `fee_cents`, `amount_cents`, `*Minor`); `service_level` missing; weight/dimension limits differ from M2. | Decide once (§3, #1). |
| H | Merchant approval: JWT carries no `tenant_id` until next login, M6 never refreshes → `/merchants/me/**` fails after APPROVED. | Force refresh/re-login; M1 resolves tenant on refresh. |
| H | M4 DTO mismatches: `key`/`prefix` vs `api_key`/`key_prefix`/`key_id`; `secret`/`id`/`status` vs `signing_secret`/`webhook_id`/`enabled`; BRN field not accepted (`additionalProperties:false`); no rejection reason; DISABLED status missing; flat CSV vs nested order body; hard-coded `parcel.in_transit.v1` (not in catalog); inventory JSON missing five M4 endpoints; no cancel-order API. | Align with M4 OpenAPI; use `GET /merchants/webhook-event-types`. |
| H | Webhook delivery history UI unaddressed (M4 U-13); M8 has the tables but no read API. | Descope in M6 doc-01 **or** request a merchant-scoped read endpoint via M4. |
| M | Missing: AI chat widget (M10 expects it; routes `/customer/book` vs `/portal/book`), accessibility (SRS §9.4), address book, profile prefill, email verification. | Add or explicitly exclude. |
| M | Wrong ID: SEC-016 for tracking privacy. `VITE_API_BASE_URL` vs env doc `VITE_API_GATEWAY_URL`; paths hard-coded with `/api/v1` despite base URL; polling 15 s×20 vs M5's 30 s; gateway burst 50 vs security-notes 10; README says SSE path "Confirmed with M5" but M5 differs; `/auth/me` called on session cookie but needs Bearer; stale M6 guide; dev proxy bypasses tracking-api; `services/tracking-api/` path. | Fix. |

### Member 7 — Dispatcher/Admin Web & Gateway (`member-07-dispatcher-admin-web-gateway`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| H | **No `doc-04-interface-and-integration-contract.md`** (verified); only an inline "Document 4" in the guide, which diverges from M3/M4. All README checklist boxes unchecked. | Create the file, reconciled with M3/M4. |
| H | Endpoints not in M3: `reassign`, `reattempt`, `return`, `/courier/parcels/{id}/deliver`, `/fail-attempt`; board without required `zone_id`; assignment status `ACTIVE`. | Align to M3 (`/assignments/{id}/cancel`, `/reschedule`, `/deliveries/{id}/complete|failed`). |
| H | Merchant admin routes `/merchants/{id}/approve…` vs M4 `/admin/merchants/**` (needs `expected_version`, reason; `reinstate`, `disable`); merchant list/detail screens missing. | Use M4's routes. |
| H | Guide vs doc-01…08 contradictions: zone id `uuid` vs `VARCHAR(32)`, migrations timestamped vs `000002/000011`, `/livez`, `postgres-credentials`/`swiftdrop_owner`, `docker-compose.m7.yml` (K10 forbids), port 8080, `X-Request-ID` vs `X-Correlation-ID`, open U-/TBD items. | Apply the guide's decisions to doc-01…08. |
| H | Wrong IDs (FR-DSP-003, FR-POD-*, NFR-REL-002; SEC-001/002/008/009/014/015 mis-mapped). | Correct. |
| H | Schema: zones gain `base_surcharge_cents` (conflicts with pricing ownership), `zone_postal_codes`, `system_settings`; `audit_logs` columns differ from the authoritative shape (needs `actor_role`, `target_type`, `previous_state`, `new_state`, `client_ip`, `request_id`); no zone-list endpoint for customers (M6 needs one). | Reconcile; add a public/zone-list endpoint. |
| M | Rate limits/CORS: no login limiter configured though claimed; burst values differ from security-notes; CORS origin and headers differ; guide strips client `X-Tenant-ID` while `web/README.md` says the client sends it. Report route `/admin/reports/operational` vs SRS `/deliveries`; dispatcher access to reports; `/ops/*` routes missing from `web/README.md`; AI route `/api/v1/ai` vs SRS `/assistant/chat`. | Align. |

### Member 8 — Events & Messaging (`member-08-events-messaging`) — your module
Already covered in earlier replies; remaining open items:
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| H | RabbitMQ user/URL: `swiftdrop_app@…` and `localhost` vs A5 (`swiftdrop`, `rabbitmq.swiftdrop.svc.cluster.local`); per-role `*_DATABASE_URL` vs one `DATABASE_URL` per module secret. | Align with A5. |
| H | `doc-07` still `CREATE ROLE …`, `GRANT CONNECT ON DATABASE swiftdrop`; D6 says M9 bootstraps roles; DB is `swiftdrop_db`; `swiftdrop_reconciler` not in D6; migrate as `swiftdrop_admin` vs `swiftdrop_migrator`. | Remove `CREATE ROLE`; fix names. |
| H | Secret names `messaging-*-secrets` vs A5 pattern; `WEBHOOK_SECRET_KEY_V1` vs `WEBHOOK_SECRET_ENCRYPTION_KEY`; `target_url` in README vs M4's `url`. | Rename. |
| M | Guide line ~153 says KEDA 2–10 pods; everything else 1–3; guide lines ~71/95 still describe the old design. | Fix/mark as history. |
| M | `SENT` status in doc-03/doc-04; migration names in `REVIEW_FINDINGS.md`; doc-05 note about M3 `000003`; FR-REC-003 "100% covered" but reconciler cannot read parcel tables (DEC-04). | Fix, mark partial. |
| M | Event contract/schema items in CR-AUTH-01/02 still "Proposed". | Send to Tech Lead / M9. |

### Member 9 — DevOps (`member-09-devops-k8s`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| C | Authoritative ops docs use `/livez`/`/readyz`/`/health`; SRS and A2 use `/health/live`, `/health/ready`. | Change ops docs/AT-016 (see §3). |
| C | Existing `deploy/kubernetes/*` contradict the agreements (verified for KEDA and CronJob; Postgres memory 512Mi vs 1Gi, `-U postgres` probes). | Rewrite per A3/A5/A8/D8. |
| H | D6 omits a reconciler role; doc-05 proposes `swiftdrop_reconciler`; grants list gives worker `SELECT` on `webhook_endpoints` where the agreement says the view. | Decide; fix grants. |
| H | K11 items not applied inside M9 docs: flat `.yml` layout vs A8 `.yaml` folders; reserved 6-digit range vs D2; migrate Job `backoffLimit: 2` vs D1 `0`; `core-api-secrets [TBD]`; `swiftdrop_outbox_pending_count` vs K19. | Update after votes. |
| H | D2 (migration naming) "vote needed" but Master says ratified; integrations secret list lacks `WEBHOOK_SECRET_ENCRYPTION_KEY`, `SMTP_*`, still lists SendGrid/Twilio; no Mailpit anywhere in M9. | Vote, then edit; add Mailpit to Compose and CI. |
| M | Pending: D2, D4, D6, D10, sign-off table empty, M1/M5/M6 ❌, K17/K18 still ⏳ (M8 says done), HPA numbers, requests/limits, backup retention, hosting (Master says decided, M9 says pending); pool arithmetic fine (105/150) but env doc defaults to 25; K15; `postgres-credentials`/`rabbitmq-credentials`/`guest:guest` in env doc; reconciler CronJob mounts `rabbitmq-secrets` though it never connects to RabbitMQ. | Close items. |

### Member 10 — AI Assistant & Quality (`member-10-ai-assistant-quality`)
| Sev | Problem | Fix |
| :-- | :-- | :-- |
| C | Webhook E2E uses `http://localhost:9999/webhook`: blocked by M8's SSRF guard, rejected by M4 (https/443 only), and unreachable from a container anyway. | Dev-only SSRF bypass flag and test receiver hostname, agreed with M4/M8 and documented in `webhook-security.md`. |
| C | Calls `POST /api/v1/merchants/orders`; M4 owns `/merchant/orders`. | Fix. |
| H | Asserts `parcel.created`; contract and M8 say `parcel.booked.v1` (authoritative docs themselves disagree). DLQ/queue names (`swiftdrop.webhooks.dlq`, `swiftdrop.webhooks.orders`) vs `swiftdrop.dlq`, `q.webhooks.dispatcher`; expects 3 retries vs 5 attempts (~42 min; needs compressed schedule). | Align. |
| H | CI services list omits migrations, relay, worker, Mailpit, gateway; uses `docker compose up` instead of `make up` (A7), `seeds/dev_seed.sql` instead of `make seed` (K13). | Fix. |
| H | Orphan endpoints `/system/policies`, `/admin/policies*` defined by no other member or SRS FR. | Remove or get an owner. |
| M | Tool names differ from FR-AI-007 (`estimate_shipping_fee`, `get_my_active_parcels`, no `get_delivery_history`); SSE path; env names (`INTERNAL_SERVICE_KEY` with insecure default vs `AI_SERVICE_ROLE_TOKEN`; `LLM_API_KEY` vs `OPENAI_API_KEY`; `LLM_BASE_URL`, etc. missing from env doc); rate limits 10/30 vs "20 rpm". | Align. |
| L | Open handoff boxes to M2, M5, M9; `/api/v1/ai/mcp/sse` not in M7 routes. | Close. |

---

## 3. Required changes in `docs/` (authoritative files, outside drafts)

Group by file; each change has the member(s) whose decision it depends on.

| File | Change | Needs decision from |
| :-- | :-- | :-- |
| `docs/operations/runbook.md` (l.14–17) | `/livez`, `/readyz`, `/health` → `/health/live`, `/health/ready`, `/metrics`; add relay 8082 and worker 8083; Traefik port per A9. | Tech Lead (A2) |
| `docs/operations/deployment-guide.md` (l.141), `docs/testing/test-strategy.md` (AT-016, l.46) | Same probe-path change. | Tech Lead |
| `docs/operations/environment-variables.md` | Secrets → `postgres-secrets`, `postgres-migrator-secrets`, `rabbitmq-secrets`, `<service>-<module>-secrets`; `RABBITMQ_URL` default user `swiftdrop` (drop `guest:guest`); per-service pool sizes (core 12, tracking 5, relay 5, worker 10, reconciler 2); replace SendGrid/Twilio with `SMTP_*`; add `WEBHOOK_SECRET_ENCRYPTION_KEY`, `MERCHANT_API_KEY_*`, `LLM_BASE_URL`, `TRACKING_API_URL`, `AI_RATE_LIMIT_*`, `AI_SESSION_*`, `PROBE_PORT`, `JWT_ISSUER`; resolve `AI_SERVICE_ROLE_TOKEN` vs `INTERNAL_SERVICE_KEY`; object storage names (K15); web variables (`VITE_API_BASE_URL`, `VITE_TRACKING_SSE_URL`); `GOOGLE_REDIRECT_URI`; tracking env (`TRACKING_HTTP_ADDR`, `REDIS_URL`). | M9, M1, M3, M5, M6, M10 |
| `docs/contracts/event-contracts.md` | One name for the booking event; add `in_transit`, `returned`, `cancelled`, `courier.unassigned`, `merchant.approved`/`status_changed`; remove `merchant.webhook_dispatch.v1`; add `merchant_id` and `notify`; ID format (UUIDv7 vs `evt_…`), `timestamp` vs `occurred_at`, `schema_version`; `q.tracking.updates` binding and consumer semantics; DLQ exchange `swiftdrop.dlq.exchange`; `processed_events.event_id` type; fix producer modules (delivered/returned); `proof_id`. | M8, M2, M3, M4, M5 |
| `docs/architecture/database-schema.md` | Apply final M1/M2/M3 table and column decisions; add `refresh_tokens`, `processed_events`, outbox `correlation_id`/`producer`/`occurred_at`, delivery tables (`IN_PROGRESS`, `lease_expires_at`, `webhook_delivery_attempts`, `attempt_count`), `merchants.status` DISABLED and `version`, webhook endpoint secret fields, `merchant_webhook_targets` view, tracking-role grants (incl. `parcel_addresses`), D6 role list, `swiftdrop_db`, zone fields, audit column names, unique index on active assignment, ownership list (`idempotency_keys`, `ai_conversations`). | M1, M2, M3, M4, M7, M8, M9 |
| `docs/architecture/domain-boundaries.md` | Who writes outbox events for each `parcel.*` event; whether `ASSIGNED → OUT_FOR_DELIVERY` batch start is allowed; Tracking reads via read-only role (not "views/tables"); route set for tracking/SSE/AI. | Tech Lead, M2, M3, M5 |
| `docs/architecture/adr-002-kubernetes-hosting.md` | Tracking SSE route (l.97); HPA range (2–4 vs 2–5); Redis in the deployment table if kept; hosting question once voted. | M5, M9 |
| `docs/architecture/adr-003-authentication-strategy.md` | `sk_live_` → `sd_live_`; callback `/auth/google/callback` (and `/auth/google` vs `/auth/oauth/google`); courier refresh lifetime; `tenant_id` refresh on `/auth/refresh`; suspend/disable and RoleAssigner/AuditWriter contract; JWKS path. | M1, M4 |
| `docs/architecture/adr-004-failed-delivery-semantics.md` | `parcel.delivery.failed` → `parcel.delivery_failed.v1`; SEC-016 → SEC-009. | Tech Lead |
| `docs/architecture/sequence-diagrams.md` | `parcel.created` → agreed name; `/pricing/quote` (l.24), `/parcels/book` (l.30), `/merchants/parcels` (l.79) → `/pricing/quotes`, `/parcels`, `/merchant/orders`; SSE route and event name (l.143, l.168); OAuth path. | M2, M4, M5 |
| `docs/contracts/api-conventions.md` | Decide snake_case vs camelCase, UUID vs prefixed IDs, error envelope and 400 vs 422; add tracking routes and SSE format; merchant routes; public field set (city only, no `zone_id`); idempotency header names. | Tech Lead (all members adapt) |
| `docs/contracts/pricing-rules.md` | `/pricing/quote` → `/pricing/quotes` (l.115); quote response field names; zone IDs; integer rounding. | M2 |
| `docs/contracts/webhook-security.md` | `parcel.created`/`parcel.delivery.failed` list (l.10); dev/test SSRF bypass policy; allowed ports. | M4, M8, M10 |
| `docs/security/security-notes.md` | Re-key SEC IDs to the SRS (§8): privacy is SEC-009, containers SEC-016, rate limiting SEC-007, CORS SEC-013, audit SEC-015; rate limits for `/api/v1/merchant/*` and tracking stream; CORS origins/headers; burst values. | Tech Lead, M7 |
| `docs/testing/test-strategy.md` (l.35, 61) | SEC-016 → SEC-009. | — |
| `docs/drafts/README.md`, `AGENTS.md` §7 | Record the migration-naming decision (D2) and the final 11-step order (D3 vs Master §6.4); resolve the `000003` collision between M3 and M4; note that M7 `doc-04` must exist. | M9, Tech Lead |
| `web/README.md` | Route table (`/ops/*`, `/portal/*`); remove `X-Tenant-ID` claim and `ROLE_` prefix; pin API variable. | M6, M7 |
| `deploy/**` (not docs, but contradicts them) | KEDA: two triggers, max 3, `RABBITMQ_URL`; CronJob: SHA-tagged `messaging` image, `/usr/local/bin/reconcile`, no RabbitMQ secret; Postgres 1Gi; per-service `.yaml` layout; gateway login limiter, `X-Request-ID`, credentials CORS. | M9, M7 |
| SRS (`intial/SwiftDrop_SRS_v1.md` l.369, 378) | Record final tracking and report routes. | Tech Lead |

---

## 4. Decisions to make first (they unblock most fixes)

| # | Decision | Recommendation |
| :-- | :-- | :-- |
| 1 | JSON style and error envelope | snake_case + `{data}`; M1, M5, M6 adapt |
| 2 | Health paths | `/health/live`, `/health/ready` (SRS wins) |
| 3 | Migration naming (D2) and `000003` collision | Decide in the group; delete the losing text everywhere |
| 4 | Booking event name | `parcel.booked.v1` (matches `event-contracts.md` and M8); update `parcel.created` users |
| 5 | Failure event name | `parcel.delivery_failed.v1` |
| 6 | Parcel state path and who emits which `parcel.*` event | Follow `domain-boundaries.md`; M3 writes through M2's aggregate |
| 7 | Tracking route set and SSE event name; Redis yes/no | `/tracking/public/{n}`, `/stream`, `/{n}/history`; decision by M5 + M9 |
| 8 | User/roles model and `parcel_addresses` vs JSONB | One model; update `database-schema.md` |
| 9 | Reconciler DB role and FR-REC-003 scope | Add `swiftdrop_reconciler` in D6; M2 provides a read-only consistency view |
| 10 | Dev-only SSRF bypass for E2E tests | Explicit env flag, off in production, documented in `webhook-security.md` |
| 11 | Webhook delivery history for merchants | Read API via M4, or descope in M6 |
| 12 | Object-storage env names (K15) | One set in the env doc |

---

## 5. Suggested order of work

1. Hold a short decision meeting for §4 items 1–10; record them in the Master Audit.
2. Edit authoritative `docs/` files (§3), starting with `api-conventions.md`, `event-contracts.md`, `database-schema.md`, `domain-boundaries.md`, and the ops docs.
3. Each member then updates `doc-01…08` (not only the guide) and removes stale `Pending`/`U-xx` markers.
4. Member 7 writes `doc-04`; Member 9 rewrites `deploy/` manifests and fills the sign-off table.
5. Re-run this audit before Meeting 2 sign-off.
