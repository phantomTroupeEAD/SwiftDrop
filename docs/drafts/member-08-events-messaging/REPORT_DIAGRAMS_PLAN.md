# Member 08 — Report Section Plan: Use Case Diagrams, Architecture, Flowcharts & Justifications

> **Audience:** An AI assistant that will write the Member 08 section of the SwiftDrop design report (due **Nov 1**, submitted as a PDF).
> **Output format:** Markdown files with **Mermaid** diagrams (they render in GitHub, VS Code and most Markdown-to-PDF tools).
> **Module:** Member 08 — Event-Driven Backend & Messaging (Transactional Outbox, Outbox Relay, RabbitMQ, Event Worker, notifications, merchant webhooks, retries/DLQ, reconciliation).
> **Do not invent facts.** Every diagram must come from the source documents in §2. If sources disagree, use the "Canonical facts" table in §3.

---

## 1. Goal and grading context

The assignment's *Design Documentation* criteria (20 marks) ask for: use case diagrams, an ER diagram, an architecture proposal with justification, integration design (patterns and rationale), sequence/activity diagrams for key flows, and design patterns/principles. This plan covers Member 08's share of those:

| Assignment criterion | Member 08 contribution | Output file (§5) |
| :--- | :--- | :--- |
| Use Case Diagrams | Notification and webhook use cases for customer, merchant, operator, system | `01-use-cases.md` |
| Architecture Proposal + justification | Messaging architecture (outbox, relay, broker, worker, reconciler) | `02-architecture.md` |
| Integration Design (patterns, rationale) | Pub/sub, transactional outbox, at-least-once + idempotency, DB-driven retries | `03-integration-design.md` |
| Sequence / Activity Diagrams | Required flow 3 "Courier status update triggering notifications", plus webhook and retry flows | `04-sequence-flows.md` |
| Flowcharts / state machines | Outbox, delivery, retry, SSRF, reconciler flows | `05-flowcharts-and-state-machines.md` |
| ER Diagram (messaging part) | Outbox and delivery tables | `06-data-model.md` |
| Design Patterns & Principles | Observer, Repository, Strategy, Adapter, Outbox | `07-patterns-and-justifications.md` |

---

## 2. Source documents (read these first)

All paths are under `docs/drafts/member-08-events-messaging/` unless noted.

| Need | Read |
| :--- | :--- |
| Scope, actors, exclusions | `doc-01-scope-and-responsibilities.md` |
| Flows, state machines, retry matrix, SSRF algorithm, reconciler flow | `doc-02-functional-flows-and-business-rules.md` (§2 state machines, §3 flows 1–5, §4 SSRF, §6 retries, §7 reconciler) |
| Internal architecture, package layout, patterns, scaling | `doc-03-internal-architecture-and-design.md` |
| Event envelope, catalog, topology, webhook contract, access matrix | `doc-04-interface-and-integration-contract.md` |
| Tables and ER diagram | `doc-05-data-and-persistence.md` (§1 ERD, §4 DDL) |
| Failure modes and recovery | `doc-07-security-failure-and-recovery.md` |
| Deployment view (Kubernetes) | `doc-08-implementation-deployment-and-ops.md` |
| Decisions and rationale | `CHANGE_REQUESTS.md` (DEC-01 to DEC-04) |
| System-wide context | `docs/architecture/domain-boundaries.md`, `docs/architecture/adr-002-kubernetes-hosting.md`, `docs/contracts/event-contracts.md`, `docs/intial/SwiftDrop_SRS_v1.md` (FR-EVT-001–010, FR-REC-001–005, FR-MER-009–011) |

Existing Mermaid diagrams in `doc-02`, `doc-03`, `doc-04` and `doc-05` may be reused as a starting point, but **they are dense**. Redraw them to follow §4; do not copy them unchanged.

---

## 3. Canonical facts (use exactly these)

| Topic | Value |
| :--- | :--- |
| Components | **Core API** (producers), **Outbox Relay** (singleton, port 8082), **Event Worker** (port 8083, scales 1–3 pods with KEDA), **Reconciler** (CronJob every 10 min), **RabbitMQ**, **PostgreSQL**, **SMTP server** (Mailpit locally), **Merchant webhook endpoint** (external) |
| Exchange | `swiftdrop.events.topic` (topic, durable) |
| Queues | `q.tracking.updates` (consumed by Member 5), `q.notifications.worker`, `q.webhooks.dispatcher` |
| Dead-letter | exchange `swiftdrop.dlq.exchange` → queue `swiftdrop.dlq` (30-day TTL) |
| Events (examples) | `parcel.booked.v1`, `parcel.assigned.v1`, `parcel.picked_up.v1`, `parcel.in_transit.v1`, `parcel.out_for_delivery.v1`, `parcel.delivered.v1`, `parcel.delivery_failed.v1`, `courier.unassigned.v1`, `merchant.status_changed.v1` |
| Envelope fields | `event_id` (UUIDv7), `event_type`, `aggregate_type`, `aggregate_id`, `aggregate_version`, `schema_version` (string `"1.0.0"`), `occurred_at`, `correlation_id`, `producer`, `payload` |
| Outbox status | `PENDING → PUBLISHED` or `FAILED` |
| Delivery status | `PENDING → IN_PROGRESS → SUCCESS` or `FAILED` (retry returns to `PENDING`) |
| Retry schedule | 5 attempts: 0 s, 30 s, 2 min, 10 min, 30 min, ±20 % jitter (about 42.5 min total) |
| Webhook outcome rules | 2xx = success; 408, 429, 5xx, timeout = retry; other 4xx, redirects, SSRF-blocked = terminal failure |
| Delivery guarantee | At-least-once with idempotent effects (never claim exactly-once; SRS FR-EVT-010) |
| Webhook signature | HMAC-SHA256, header `X-SwiftDrop-Signature: t=<timestamp>,v1=<signature>`; secret decrypted (AES-256-GCM) from the `merchant_webhook_targets` view |
| Data tables | `outbox_events`, `processed_events`, `notification_deliveries`, `webhook_deliveries`, `webhook_delivery_attempts` |
| Read-only dependency | `merchant_webhook_targets` view (owned by Member 4) |
| Not owned by Member 08 | Parcel/dispatch business logic, merchant configuration, the Tracking API and Redis (Member 5), Kubernetes manifests (Member 9) |
| Reconciler limitation | Detecting a parcel missing its status history (FR-REC-003) is **partial** until Member 2 exposes a read-only view (decision DEC-04). State this honestly in the report. |

---

## 4. Readability rules for every diagram (mandatory)

1. **One idea per diagram.** If a diagram needs more than about **12 nodes** or **10 sequence messages**, split it into numbered parts (e.g. 4a, 4b).
2. **Short labels.** Node text ≤ 5 words. Put detail in the caption or the table below the diagram, not in the node. No raw SQL, JSON or code in nodes.
3. **Direction.** Use `flowchart LR` for pipelines and `flowchart TD` for decisions. Never mix both in one diagram.
4. **Group with `subgraph`** and give each group a plain-language title (e.g. "Core API", "Messaging Service", "External").
5. **Consistent colours** via `classDef` in every flowchart:
   - Core API / producers: light blue (`#dbeafe`)
   - Messaging components (relay, worker, reconciler): light green (`#dcfce7`)
   - Data stores and broker (PostgreSQL, RabbitMQ): light orange (`#ffedd5`)
   - External systems (SMTP, merchant endpoint): light grey (`#e5e7eb`)
   - Failure / dead-letter paths: light red (`#fee2e2`)
6. **Label every edge** that is not obvious (e.g. "publishes", "ack", "retry in 30 s").
7. **No HTML tags** (`<br/>`) and no special characters that break Mermaid (`$`, unescaped quotes, backticks). Wrap labels containing punctuation in quotes.
8. **Sequence diagrams:** use `autonumber`, at most **6 participants**, and `Note over` for key rules. Use `alt`/`else` for failure branches instead of separate diagrams only when the branch is short.
9. **Every diagram has:** a numbered title (`Figure 3.2 — …`), a one-sentence purpose, the diagram, and a short "How to read it" or "Key points" list of 2–4 bullets.
10. **Test each diagram** renders (paste into a Mermaid live editor or run `mmdc`). Fix syntax errors; never leave a broken block.
11. Use the **canonical names in §3** only. Do not rename components or events.

Diagram template:

````markdown
### Figure X.Y — <Title>
**Purpose:** <one sentence>

```mermaid
<diagram>
```

**Key points**
- <point>
- <point>
````

---

## 5. Output files

Create the folder `docs/drafts/member-08-events-messaging/report/` and these files. Each file starts with a one-paragraph introduction and ends with a "Traceability" table linking figures to SRS IDs and source docs.

### 5.1 `report/01-use-cases.md` — Use Case Diagrams

Use Mermaid `flowchart LR` styled as use case diagrams (actors on the left as rounded nodes; use cases as ovals inside a "Messaging System" subgraph). Mermaid has no native UML use case type, so label actors with the prefix "Actor:".

Produce:
- **Figure 1.1 — Overall messaging use cases.** Actors: Customer, Merchant system (external receiver), Platform operator (Admin/Dispatcher/DevOps), Core API (producer system), Scheduler (CronJob).
  Use cases: Receive delivery notification email; Receive signed webhook; Verify webhook signature; Retry failed delivery automatically; Inspect dead-lettered messages; Replay dead-lettered messages; Monitor queue health; Reconcile stuck events; Publish domain event.
- **Figure 1.2 — Producer view.** How Core API modules (Parcels, Dispatch, Delivery, Merchants) publish an event: use cases "Record event with state change" and "Rebuild event history". Keep to four use cases.
- **Figure 1.3 — Operator view.** Inspect backlog, inspect DLQ, replay DLQ, run reconciliation, review delivery history.
- A **use case description table** (one row per main use case): ID, actor, trigger, main flow (one line), failure case (one line), related requirement.

### 5.2 `report/02-architecture.md` — Architecture

- **Figure 2.1 — System context.** Messaging subsystem in the whole platform: Core API, Tracking API (Member 5), Merchant systems, customers' email, PostgreSQL, RabbitMQ. Show only data flow arrows; one-word labels.
- **Figure 2.2 — Container view.** The three Messaging binaries (Relay, Worker, Reconciler) plus the shared infrastructure, and who talks to whom. Mark Relay as "exactly 1 replica", Worker as "1–3 replicas (autoscaled)".
- **Figure 2.3 — Layered/internal structure.** The packages of the messaging service (broker, outbox, idempotency, notification, webhook, reconciler, health, config) grouped into layers: entrypoints, application logic, adapters, infrastructure. Use `doc-03` §2.
- **Figure 2.4 — Messaging topology.** Exchange, three queues with their bindings (`parcel.#`, `courier.#`, `merchant.#` as applicable), and the DLQ. Show which service consumes each queue.
- **Figure 2.5 — Deployment view.** Kubernetes objects: Outbox Relay Deployment (1), Event Worker Deployment plus KEDA ScaledObject (1–3), Reconciler CronJob, RabbitMQ StatefulSet, PostgreSQL, Mailpit/SMTP. Source: `doc-08` §6 and Member 9 agreements (`docs/drafts/member-09-devops-k8s/team-agreements-with-member-09.md`).
- **Component responsibility table:** component, responsibility, scaling rule, failure behaviour.
- **Architecture justification** (about 250 words): why a **separate relay and worker** instead of publishing from Core API; why a **modular monolith plus dedicated async services** (ties to `AGENTS.md` §4); why a **singleton relay** and an **autoscaled worker**.

### 5.3 `report/03-integration-design.md` — Integration Design & Rationale

- **Figure 3.1 — Integration patterns map.** One diagram showing where each pattern is used: In-process call (Core API → OutboxWriter), Publish/Subscribe (RabbitMQ), Competing consumers (worker pods), Request/Reply (worker → merchant HTTP, worker → SMTP), Read-only database view (worker ← `merchant_webhook_targets`).
- **Figure 3.2 — Delivery guarantee chain.** The path "commit → outbox row → publish with confirm → consume → ack" with the failure point and recovery for each step (use a table below it, not a crowded diagram).
- **Pattern comparison table:** Dual write vs Transactional outbox vs Change data capture; AMQP requeue vs DB-driven retry; Mock email vs real SMTP (Mailpit). Columns: option, pros, cons, decision.
- **Rationale section**, one short paragraph each (3–5 sentences), written for markers who have not read the SRS:
  1. Why a transactional outbox (dual-write problem).
  2. Why at-least-once with idempotent consumers, not exactly-once (SRS FR-EVT-010).
  3. Why retries live in PostgreSQL leases, not broker requeues (DEC-02).
  4. Why self-contained event payloads (DEC-03) and no cross-module table reads.
  5. Why one real SMTP code path with Mailpit (DEC-01).
  6. Why the worker never publishes secrets or URLs on the broker.

### 5.4 `report/04-sequence-flows.md` — Sequence Diagrams

Each diagram ≤ 6 participants, `autonumber`.

- **Figure 4.1 — Courier status update triggers notifications** (*assignment-required flow*). Participants: Courier app, Core API, PostgreSQL, Outbox Relay, RabbitMQ, Event Worker. Show: status change and outbox row in one transaction; relay publish with confirm; worker consumes, then email and webhook tasks created.
- **Figure 4.2 — Outbox relay publish loop.** Relay, PostgreSQL, RabbitMQ. Show claim batch, publish, confirm, mark published, and the NACK branch (stays pending, attempt counted).
- **Figure 4.3 — Customer email notification.** Event Worker, PostgreSQL, SMTP. Show claim lease, send, record result, and dedup record written only after the outcome.
- **Figure 4.4 — Merchant webhook delivery.** Event Worker, PostgreSQL, merchant endpoint. Show read target, decrypt secret, SSRF check, signed POST, success and retry paths.
- **Figure 4.5 — Failure and recovery.** Worker crash with lease expiry, reconciler resetting the lease, and a second attempt succeeding. Add a second small diagram 4.6 for "message to DLQ and operator replay".
- Below each diagram, a short numbered list explaining steps that need a business rule (for example "step 4: dedup happens after the effect, so a crash causes a retry, never a lost message").

### 5.5 `report/05-flowcharts-and-state-machines.md` — Flowcharts

- **Figure 5.1 — Outbox event state machine** (`stateDiagram-v2`): PENDING, PUBLISHED, FAILED, with the manual re-arm path.
- **Figure 5.2 — Delivery state machine** (webhooks and notifications): PENDING, IN_PROGRESS, SUCCESS, FAILED, and the retry loop.
- **Figure 5.3 — Retry decision flowchart** (`flowchart TD`): result of an attempt → success / transient / permanent → next attempt time or terminal. Include the 5-step schedule as a small table under it.
- **Figure 5.4 — Webhook response classification** (a decision table rendered as a flowchart): 2xx, 408/429/5xx, other 4xx, 3xx, timeout, SSRF block.
- **Figure 5.5 — SSRF defense flowchart** (`flowchart TD`): URL scheme check → resolve DNS → every IP checked against blocked ranges → connect to the checked IP → no redirects. Keep ≤ 10 nodes and list the blocked ranges in a table.
- **Figure 5.6 — Worker message handling flowchart:** receive → parse → duplicate? → create delivery task → ack; parse failure → DLQ.
- **Figure 5.7 — Reconciler flowchart:** stale outbox rows, expired leases, retention purge. State clearly what it does **not** do (never publishes to RabbitMQ; parcel-history checks pending DEC-04).
- **Figure 5.8 — Graceful startup and shutdown** (optional): health server first, then dependencies; on SIGTERM stop consuming, finish in-flight work, release leases.

### 5.6 `report/06-data-model.md` — Data Model (messaging part)

- **Figure 6.1 — ER diagram** (`erDiagram`) for the five Member 08 tables plus the read-only `merchant_webhook_targets` view (drawn as an external entity). Show keys and the key relationships only; list columns in a table below, not in the diagram (keep the diagram to a few attributes per entity).
- **Data dictionary table** per entity: purpose, key columns, important constraints (for example unique `(endpoint_id, event_id)`), retention (outbox 7 days, delivery rows 30 days, dedup ledger 35 days).
- A short note explaining why delivery tables carry no foreign key to `outbox_events` (different retention).

### 5.7 `report/07-patterns-and-justifications.md` — Patterns & Principles

- A table: pattern or principle, where it is used in Member 08, why it fits. Include: Transactional Outbox, Observer / publish-subscribe, Repository (outbox and delivery repositories), Strategy (backoff / retry schedule), Adapter (SMTP sender, HTTP client), Idempotent Receiver, Competing Consumers, Circuit of retries with DLQ (Dead Letter Channel), SOLID (single responsibility per binary; dependency inversion via `OutboxWriter` interface), DRY (one envelope).
- **Figure 7.1 — Class/interface diagram** (`classDiagram`): `OutboxWriter`, `AnyEnvelope`, `EventEnvelope`, `Publisher`, `Consumer`, `Sender` (email), `WebhookClient`, `BackoffStrategy`. ≤ 8 classes; no fields beyond key methods.
- **Design decisions & trade-offs table:** decision, alternative rejected, reason, known limitation (for example the singleton relay is a throughput cap; reconciler cannot check parcel history yet).
- **Risks and limitations** (honest, short): at-least-once means duplicates are possible; webhook receivers must dedupe by `X-SwiftDrop-Event-ID`; relay single point of throughput; dependence on other members' event payloads (`notify`, `merchant_id`).

### 5.8 `report/README.md`

An index listing the files in order, each with a one-line summary, and a **figure index** table (figure number, title, file). Add a "How to export to PDF" note (see §7).

---

## 6. Writing style for justifications

- Plain English, short paragraphs (3–5 sentences). Name the problem, the choice, the alternative, and the trade-off.
- Cite requirement IDs where relevant (FR-EVT-001–010, FR-REC-001–005, FR-MER-009–011, SEC-010/011). Use only IDs that exist in `docs/intial/SwiftDrop_SRS_v1.md`; verify before citing.
- No marketing language, no claims of "100%" or "perfect". State limitations.
- Refer to other members by role ("Member 2, Parcels") the first time, then by number.
- Do not copy long passages from `doc-01…08`; summarise.

---

## 7. Export to PDF (for the team)

Recommended path: render Mermaid to images, then convert Markdown to PDF.

1. Install Mermaid CLI: `npm install -g @mermaid-js/mermaid-cli`.
2. Render all diagrams in a file to PNG/SVG with `mmdc -i report/04-sequence-flows.md -o report/images/04.svg` (each Mermaid block becomes one image), or use VS Code with the "Markdown Preview Mermaid Support" and "Markdown PDF" extensions.
3. Check each PDF page: diagrams must not be cut off or tiny. If a diagram is too wide, split it or change `LR` to `TD`.

---

## 8. Quality checklist (the AI must confirm each item at the end)

- [ ] All 7 content files and `README.md` exist under `report/`.
- [ ] Every diagram renders without errors and follows §4 (≤ 12 nodes, short labels, colours, title, caption, key points).
- [ ] Required flow "Courier status update triggering notifications" is present (Figure 4.1).
- [ ] Use case diagrams exist for the overall system, producers and operators (Figures 1.1–1.3).
- [ ] Architecture covers context, container, internal layers, topology and deployment (Figures 2.1–2.5), with justification text.
- [ ] Integration patterns and the rationale for each decision are written (§5.3).
- [ ] Only canonical names from §3 are used; no `parcel.created`, `parcel.failed.v1`, `SENT`, or integer `schema_version`.
- [ ] Limitations stated honestly (FR-REC-003 partial, at-least-once duplicates, singleton relay).
- [ ] Every figure appears in the figure index and the traceability tables.
- [ ] No content invented beyond the source documents; anything uncertain is listed under "Open points" at the end of `report/README.md`.

## 9. Deliverable report back

When finished, the AI reports: files created, figure count, any source conflicts found (and which side it followed), and any open points the user must confirm with other members.
