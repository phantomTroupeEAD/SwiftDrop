# SwiftDrop Parcel Delivery & Courier Management System
## Document 1 â€” Module Scope & Responsibility Specification
### Member 2: Parcel Management & Pricing

## 1. Purpose

### 1.1 Module Purpose

The Parcel Management & Pricing module is responsible for creating and maintaining parcel booking records, calculating delivery fees, managing the parcel lifecycle within its agreed boundaries, and providing parcel history and receipt data.

It forms a core business capability of the SwiftDrop system. Customers and authorized merchants use its booking functionality to register parcels for delivery. The resulting parcel records are then consumed by other modules responsible for dispatch, courier operations, tracking, and event-driven integrations.

The module is implemented within the Go Core API as two closely related logical areas:

- **Parcel Management:** Owns the parcel aggregate, booking operations, tracking-number assignment, parcel state-transition rules, and parcel status history.
- **Pricing:** Owns delivery-fee estimation and the applicable pricing calculation rules.

These areas share business data and collaborate during booking, but their responsibilities should remain clearly defined.

### 1.2 Business Problem

Traditional courier operations may rely on phone calls, manual records, and spreadsheets to register parcels and calculate delivery charges. Such processes can lead to inconsistent pricing, duplicate bookings, incomplete parcel histories, and difficulty integrating with tracking and dispatch systems.

The SwiftDrop module addresses these problems by providing:

- A consistent and validated parcel-booking process.
- A deterministic and configurable delivery-fee calculation.
- A unique public tracking number for each successfully created parcel.
- Persistent parcel details, accepted pricing information, and status history.
- Controlled parcel-state transitions and protection against invalid operations.
- Reliable integration with other modules through agreed API and event contracts.

### 1.3 Objectives

The module shall aim to:

1. Accept valid parcel-booking requests from authorized customers and, through the agreed integration boundary, authorized merchants.
2. Validate parcel information and reject unsupported or incomplete requests.
3. Calculate delivery-fee estimates using team-approved pricing rules.
4. Persist the accepted booking price as a snapshot so that subsequent pricing-rule changes do not silently alter an existing booking.
5. Generate and maintain unique parcel identifiers and public tracking numbers.
6. Maintain the current parcel status and append-only status history.
7. Provide authorized parcel-history and receipt-data retrieval.
8. Prevent duplicate bookings when an eligible request is retried using the same idempotency key.
9. Persist relevant integration events through the agreed transactional outbox interface.
10. Provide automated tests, database migrations, API documentation, and reproducible setup instructions for the module.

## 2. Scope and Responsibilities

### 2.1 In-Scope Responsibilities

| Capability | Responsibility |
|---|---|
| Parcel booking | Receive, validate, and process parcel-creation requests for customers and authorized merchants via both quoted booking (`quote_id`) and direct booking paths. |
| Parcel data | Maintain authoritative parcel business records within the canonical PostgreSQL schema, including immutable JSONB address snapshots (`pickup_address`, `dropoff_address`). |
| Tracking number | Generate unique public tracking numbers in the canonical format `SD-YYYYMMDD-XXXXXX` and enforce database uniqueness with bounded collision retries. |
| Pricing and estimation | Calculate deterministic delivery fees via `POST /api/v1/pricing/quotes` using fixed complete tariffs per zone-pair and weight band across seven bands, threshold-based volumetric calculations (divisor 6000 cmÂ³/kg, 1 kg threshold, 30 kg max limits), backend quote expiry enforcement (15-minute TTL), and controlled missing-tariff rejection (`pricing_unavailable`). |
| Quote persistence | Persist 15-minute validity quote records in `pricing_quotes` to freeze fees prior to booking confirmation. |
| Price snapshot | Atomically preserve the accepted fee (`price_cents` in integer minor units), currency (`LKR`), and pricing-rule/version reference directly on the parcel record as an immutable historical record. |
| Parcel lifecycle | Validate permitted parcel-state transitions through the `Parcel` aggregate root (`CREATED`, `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY`, `DELIVERED`, `ATTEMPT_FAILED`, `RETURNED`, `CANCELLED`) with optimistic concurrency (`version`), rejecting illegal or direct SQL status updates. |
| Status history | Atomically record every valid status transition in `parcel_status_history` with UTC timestamp, actor ID/role, reason code, notes, and aggregate version. |
| Parcel history API | Provide authorized access to the parcel's chronological history via `GET /api/v1/parcels/{id}/history`. |
| Receipt data | Expose `GET /api/v1/parcels/{id}/receipt` providing the booking and pricing snapshot data required for receipt presentation without recalculating against current tariffs. |
| Idempotency | Enforce caller-scoped idempotency for non-idempotent mutations (`POST /api/v1/parcels`) using the canonical `idempotency_keys` table (24-hour retention, SHA-256 payload hash, HTTP 409 on payload mismatch). |
| Persistence | Implement and maintain module-owned PostgreSQL migrations (`000004_create_pricing_rules.up.sql`, `000005_create_parcels_and_quotes.up.sql`, `000010_create_idempotency_keys.up.sql`), constraints, and performance indexes. |
| Event production | Atomically write integration events (`parcel.booked.v1`, `parcel.cancelled.v1`) into the `outbox_events` table inside the business transaction for asynchronous relay by Member 8. |
| Quality assurance | Write unit, repository, and integration tests for booking, pricing, persistence, idempotency, privacy masking, and lifecycle rules. |
| Documentation | Maintain API contracts, pricing specifications, co-located module living documentation (`README.md`, `API.md`), test instructions, and operational runbooks. |

### 2.2 Explicit Exclusions

The module must not take ownership of capabilities assigned to other members.

| Excluded Capability | Owner | Boundary |
|---|---|---|
| User registration, password handling, JWT issuance, and Google sign-in | Member 1 | The module consumes the approved authentication middleware and RBAC context (`CUSTOMER`, `MERCHANT`, `COURIER`, `DISPATCHER`, `ADMIN`). |
| Courier assignment, dispatch board, fleet capacity, and route sequencing | Member 3 | Member 3 coordinates dispatch; courier assignment is recorded via the `Parcel` aggregate contract. |
| Courier delivery attempts and proof-of-delivery uploads | Member 3 | Member 3 owns `delivery_attempts` and proof assets (photos/signatures); Member 2 manages the resulting parcel status transitions (`ATTEMPT_FAILED`, `DELIVERED`). |
| Independent Tracking API and real-time SSE delivery | Member 5 | The Tracking API reads authoritative parcel data and projects privacy-sanitized milestones; it does not alter parcel state. |
| Customer and merchant user interfaces | Member 6 | Member 6 builds the frontend booking wizard, dashboard, and receipt view; frontend fee calculations are non-authoritative. |
| Dispatcher/admin interfaces and API Gateway configuration | Member 7 | Member 7 manages dispatcher/admin portals and Traefik Ingress routing/rate-limiting policies. |
| Outbox Relay daemon, RabbitMQ broker, Event Worker, notifications, and webhooks | Member 8 | Member 2 writes to `outbox_events`; Member 8 owns outbox polling, RabbitMQ delivery, notification dispatch, and merchant webhook retries/DLQ. |
| Platform-wide Docker Compose, Kubernetes, Helm, and observability configuration | Member 9 | Member 9 manages container orchestration, database pool budgets, and deployment manifests. |
| AI Assistant and MCP tools | Member 10 | The assistant consumes approved REST endpoints (`POST /api/v1/pricing/quotes`, `GET /api/v1/parcels/{id}`); zero direct database access. |
| Merchant credential issuance, API keys, and webhook configuration | Member 4 | Member 4 owns `POST /api/v1/merchant/parcels` transport and tenant verification, delegating core parcel creation to Member 2's application use case. |
| System-wide database governance | Database Coordinator / Member 9 | Member 2 owns its sequential migrations within the unified database schema conventions. |

The exclusions do not remove the need for collaboration. Other modules may invoke the parcel or pricing capabilities through approved interfaces without taking ownership of their business rules or persistence.

## 3. Bounded Context and Domain Ownership

### 3.1 Bounded Context

The Parcel Management & Pricing capability is a logical bounded context within the Core API. Its internal rules, domain objects, and use cases must be implemented within their designated packages.

The module must not require frontend code, tracking services, or event workers to reproduce its business rules independently.

The canonical Go Core API modular monolith directory structure is:

```text
core-api/
â”œâ”€â”€ internal/
â”‚   â”œâ”€â”€ parcels/
â”‚   â”‚   â”œâ”€â”€ domain/           # Pure domain models (Parcel aggregate root, status state machine, events, domain errors)
â”‚   â”‚   â”œâ”€â”€ application/      # Use cases (BookParcel, GetParcel, ListParcels, GetHistory, GetReceipt)
â”‚   â”‚   â”œâ”€â”€ infrastructure/   # PostgreSQL repositories, outbox adapters, tracking number generator
â”‚   â”‚   â””â”€â”€ transport/        # HTTP handlers, DTO mapping, input validation
â”‚   â”œâ”€â”€ pricing/
â”‚   â”‚   â”œâ”€â”€ domain/           # Tariff calculation engine, volumetric weight formulas, pricing rules
â”‚   â”‚   â”œâ”€â”€ application/      # Quote generation use case (CalculateQuoteUseCase)
â”‚   â”‚   â”œâ”€â”€ infrastructure/   # Pricing rule and quote PostgreSQL repositories
â”‚   â”‚   â””â”€â”€ transport/        # HTTP quote handlers, request/response DTOs
â”‚   â”œâ”€â”€ events/               # Shared outbox writer contract and event envelope
â”‚   â””â”€â”€ platform/             # Technical drivers (database pool, auth middleware, HTTP helpers)
â”œâ”€â”€ migrations/               # Canonical PostgreSQL migrations
â””â”€â”€ tests/                    # Integration and contract test suites
```

Following Clean Architecture rules, the `domain/` layer has zero external dependencies (no `database/sql`, no `net/http`, no RabbitMQ imports).

### 3.2 Parcel Management Ownership

The Parcel Management area owns:

- The `Parcel` aggregate root and its domain invariants.
- Booking creation use cases for both customers and merchants.
- Public tracking-number generation (`SD-YYYYMMDD-XXXXXX`) and uniqueness enforcement.
- The parcel's authoritative current status state machine.
- Immutable booking-time address snapshots (`pickup_address`, `dropoff_address` stored as JSONB).
- Parcel status-history audit records in `parcel_status_history`.
- Application use cases for authorized parcel details, history, and receipt data.
- The atomic transaction persisting parcel state, history, idempotency records, and outbox integration events.

### 3.3 Pricing Ownership

The Pricing area owns:

- The deterministic delivery-fee calculation engine (`POST /api/v1/pricing/quotes`).
- Retrieval interface for applicable tariffs based on canonical pickup and destination zones. *(Note: Member 7 owns admin screens and tariff-administration workflow; Member 2 owns calculation behavior and tariff-retrieval interface).*
- Strict validation of weight and dimensions against proposed operational limits (maximum actual parcel weight: 30.00 kg; maximum chargeable weight: 30.00 kg; reject quote requests exceeding either limit).
- Evaluation of chargeable weight under the selected Option A threshold policy:
  - If actual weight $\le 1\text{ kg}$: $\text{chargeable\_weight} = \text{actual\_weight}$ (dimensions do not affect pricing).
  - If actual weight $> 1\text{ kg}$: dimensions in cm are required; $\text{volumetric\_weight\_kg} = \frac{\text{length\_cm} \times \text{width\_cm} \times \text{height\_cm}}{6000}$; $\text{chargeable\_weight} = \max(\text{actual\_weight}, \text{volumetric\_weight})$.
  *(Proposed volumetric divisor: $6000\text{ cm}^3/\text{kg}$, provisional parameter pending team confirmation).*
- Preservation of full decimal precision for volumetric and chargeable weight without premature rounding prior to band selection.
- Assignment to exactly one of seven unambiguous, non-overlapping chargeable weight bands:
  - Band 1: $0 < W \le 0.5\text{ kg}$
  - Band 2: $0.5 < W \le 1.0\text{ kg}$
  - Band 3: $1.0 < W \le 2.0\text{ kg}$
  - Band 4: $2.0 < W \le 5.0\text{ kg}$
  - Band 5: $5.0 < W \le 10.0\text{ kg}$
  - Band 6: $10.0 < W \le 20.0\text{ kg}$
  - Band 7: $20.0 < W \le 30.0\text{ kg}$
- Tariff lookup evaluating the unique applicable, enabled pricing rule for the zone pair and weight band:
  $$\text{delivery\_fee} = \text{tariff}(\text{pickup\_zone\_id}, \text{destination\_zone\_id}, \text{weight\_band\_id})$$
  *(The tariff is the complete delivery fee for that combination; no separate common base fee, per-kilogram charge, distance fee, or service multiplier is added).*
- Controlled missing-tariff behavior: return a safe `pricing_unavailable` business error if no tariff exists; fail safely and flag a configuration problem if multiple applicable rules create ambiguity (no guessing or fallback rates).
- Strict integer minor-unit financial representation (`BIGINT` / `int64` in cents, currency `LKR`) â€” *Confirmed Project Standard*.
- Quote generation and persistence with a proposed 15-minute validity window (`expires_at = created_at + 15m`), enforced strictly on the backend upon booking confirmation.
- Creation and persistence of the immutable accepted-price snapshot embedded in confirmed bookings â€” *Confirmed Project Standard*.

> [!IMPORTANT]
> **Provisional Pricing Baseline Notice (Pending Team Approval):**
> The seven weight bands, 1 kg threshold policy, volumetric divisor ($6000\text{ cm}^3/\text{kg}$), 30 kg weight ceilings, fixed zone-pair/band tariff model, and 15-minute quote TTL represent the **current proposed Member 2 design baseline, pending formal team approval**. All example figures are illustrative, not commercial price promises. The underlying architectural invariantsâ€”server-side price calculation, integer minor units (LKR cents), quote persistence, backend expiry enforcement, missing-tariff rejection, and immutable accepted-price snapshots on `parcels`â€”are fixed and invariant.

The module must calculate prices on the server. It must not trust a client-submitted fee as authoritative.

### 3.4 Authoritative Data

PostgreSQL is the authoritative source for parcel business state.

The module owns the parcel-related tables assigned to it in the canonical database schema (`parcels`, `parcel_status_history`, `pricing_rules`, `pricing_quotes`, `idempotency_keys`). Other modules may read approved fields or invoke published operations, but they must not create independent, conflicting parcel or status tables.

The module must not treat RabbitMQ messages, frontend state, tracking caches, or AI responses as authoritative parcel records.

## 4. Dependencies and Integration Relationships

### 4.1 Upstream Dependencies

| Dependency | Provider | Required Input or Capability |
|---|---|---|
| Authentication and identity context | Member 1 | Verified caller identity (`user_id`), roles (`CUSTOMER`, `MERCHANT`, `COURIER`, `DISPATCHER`, `ADMIN`), and tenant context via JWT middleware. |
| API conventions | Shared Contract | Base path `/api/v1`, standard JSON envelopes (`{"data": ...}`, `{"error": ...}`), correlation IDs, and ISO 8601 UTC timestamps. |
| Database conventions | Shared Contract / Member 9 | Canonical identifiers (`UUIDv4`), migration sequencing (`migrations/NNNNNN_name.up.sql`), and exact integer minor-unit conventions (`BIGINT`). |
| Parcel lifecycle agreement | Members 2, 3, 5 | Approved 9-state lifecycle (`CREATED`, `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY`, `DELIVERED`, `ATTEMPT_FAILED`, `RETURNED`, `CANCELLED`), actor permissions, and ADR-004 failed-delivery semantics. |
| Pricing decisions | Member 2 / Team | Fixed complete tariff per zone-pair & weight band (7 bands), 1 kg threshold, divisor 6000 cmÂ³/kg, 30 kg limits, LKR minor units, zone IDs (`VARCHAR(64)`), and 15-minute quote validity. |
| Outbox/event interface | Member 8 | Canonical event envelope, `outbox_events` table contract, event types `parcel.booked.v1` and `parcel.cancelled.v1`. |
| Customer interface requirements | Member 6 | Booking wizard inputs, quote request DTO, receipt presentation fields. |
| Merchant integration requirements | Member 4 | Transport endpoint `POST /api/v1/merchant/parcels`, merchant tenant verification, and delegation to Member 2's booking use case. |

### 4.2 Downstream Consumers

| Consumer | Expected Interaction |
|---|---|
| Customer frontend â€” Member 6 | Requests fee quotes (`POST /api/v1/pricing/quotes`), creates bookings (`POST /api/v1/parcels`), and retrieves authorized parcel history and receipt data. |
| Merchant integration â€” Member 4 | In-process invocation of Member 2's `CreateParcelUseCase` within the authenticated merchant tenant context. |
| Dispatch & Couriers â€” Member 3 | Reads authoritative parcel state and invokes the `Parcel` aggregate methods to advance operational status (`ASSIGNED`, `PICKED_UP`, `OUT_FOR_DELIVERY`, `DELIVERED`, `ATTEMPT_FAILED`). |
| Tracking API â€” Member 5 | Reads authoritative parcel and status history records from PostgreSQL to construct read-optimized Server-Sent Events (SSE) and public tracking projections. |
| Event-driven backend â€” Member 8 | Outbox Relay poller queries unpublished rows from `outbox_events` and publishes them to RabbitMQ. |
| AI Assistant â€” Member 10 | Calls approved REST endpoints (`POST /api/v1/pricing/quotes`, `GET /api/v1/parcels/{id}`) through the API Gateway; zero direct database or message bus access. |
| Admin / Operations â€” Member 7 | Consumes parcel and pricing data for admin reporting and zone/tariff maintenance. |

### 4.3 Collaboration Boundaries

The following interface contracts govern collaboration across boundaries:
- **Delivery Attempts:** Member 3 owns `delivery_attempts` logs. When an attempt fails, Member 3 records the attempt and invokes Member 2's `parcel.RecordFailedAttempt(reasonCode, courierID)` to increment `attempt_count` (max 3) and transition to `ATTEMPT_FAILED`.
- **Proof of Delivery:** Member 3 captures photo/signature attachments in private object storage and provides the proof reference metadata when transitioning to `DELIVERED`.
- **Merchant Ingress:** Member 4 owns API key authentication and the external path `POST /api/v1/merchant/parcels`, calling Member 2's internal application layer directly without HTTP overhead.
- **Public Tracking Redaction:** Member 5 queries parcel records but strictly filters out private customer PII (recipient phone, street address, instructions) in accordance with Section 8.

## 5. Invariants and Business Guarantees

The module strictly enforces the following core business invariants:

### 5.1 Parcel Invariants

1. **Identity & Reference:** Every persisted parcel has an immutable internal `UUIDv4` identifier and a unique public tracking number in the format `SD-YYYYMMDD-XXXXXX`.
2. **Immutable Address Snapshots:** Pickup and drop-off addresses are stored as JSONB snapshots on the `parcels` table at booking creation. Subsequent customer profile updates never alter existing bookings.
3. **Validated Input Boundaries:** Positive actual weight up to proposed 30.000 kg maximum ($0 < W \le 30.000\text{ kg}$). Chargeable weight must also not exceed 30.000 kg. Dimensions must not exceed 150.00 cm per edge, with maximum combined girth ($L + 2(W+H)$) $\le 300.00$ cm.
4. **Aggregate Root State Machine:** Parcel status transitions can only occur through the `Parcel` aggregate root methods, enforcing legal transitions and rejecting direct SQL updates.
5. **Optimistic Concurrency Control:** Every parcel row includes an integer `version` column incremented on every transition. Stale concurrent updates fail with HTTP `409 Conflict`.
6. **Append-Only Status History:** Every accepted status transition creates an immutable record in `parcel_status_history` within the same transaction.
7. **Attempt Limits:** Delivery attempts are capped at 3 (`attempt_count <= 3`). Upon the third failure, the status must transition to terminal `RETURNED`.
8. **Privacy Protection:** Private parcel details and full recipient addresses are available only to authenticated senders, authorized merchants, assigned couriers, and dispatchers.

### 5.2 Pricing Invariants

1. **Deterministic Calculation:** Identical inputs evaluated against the same pricing rule version produce identical fees.
2. **Canonical Financial Representation:** All monetary amounts are stored as 64-bit integers (`BIGINT` / `int64`) in Sri Lankan Rupee minor units (**cents**). Floating-point arithmetic is strictly prohibited.
3. **Volumetric Standard & Threshold Policy:** Actual weight $\le 1\text{ kg}$ uses actual weight only. Actual weight $> 1\text{ kg}$ requires dimensions and uses volumetric divisor $6000\text{ cm}^3/\text{kg}$: $\text{volumetric\_kg} = (L \times W \times H) / 6000$, $\text{chargeable\_weight} = \max(\text{actual}, \text{volumetric})$. No rounding before band selection. Maximum actual and chargeable weight is 30 kg.
4. **Tariff Completeness:** Each applicable pricing rule specifies the complete delivery fee for the (pickup zone, destination zone, weight band) combination. No separate common base fee, per-kg charge, distance fee, or multiplier is added.
5. **Quote Immutability & Backend Expiry:** Generated quotes in `pricing_quotes` are valid for **15 minutes** (`expires_at = NOW() + 15m`). Backend strictly enforces expiry (`current_time >= expires_at` is expired) upon booking confirmation.
6. **Accepted Price Lock:** Confirmed bookings lock the accepted price (`price_cents`), currency (`LKR`), and rule/version reference permanently onto the `parcels` record. Subsequent rule changes never alter booked prices.
7. **No Fallback Guessing:** If no active, enabled pricing rule matches the requested route and weight band, the engine returns a controlled `pricing_unavailable` error. If multiple rules match ambiguously, fail safely. The system never fabricates a fee or falls back to default rates.

### 5.3 Idempotency and Transaction Invariants

1. **Booking Idempotency:** Parcel bookings require an `Idempotency-Key` header (UUIDv4). Duplicate requests with the same key and payload within 24 hours return the cached response with `Idempotent-Replay: true`.
2. **Payload Conflict Detection:** Reusing an idempotency key with a different request payload returns HTTP `409 Conflict` (`IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`).
3. **Transaction Atomicity:** Parcel creation commits the parcel aggregate, JSONB address snapshots, initial status history, idempotency record, and outbox event (`parcel.booked.v1`) within a single PostgreSQL transaction.
4. **Outbox Decoupling:** External notifications, webhooks, and RabbitMQ publishing are never executed inside the database transaction; they are handled asynchronously by Member 8's Outbox Relay.
5. **Database-Level Enforcement:** Foreign keys (`ON DELETE RESTRICT`), unique indexes (`idx_parcels_tracking_number`, `idempotency_keys_pkey`), and check constraints enforce domain guarantees at the storage engine level.

## 6. What Other Modules May Request

The Parcel Management & Pricing module exposes strict capabilities rather than allowing other modules to manipulate internal tables directly.

Approved callers may request:

- **Delivery Fee Quote:** `POST /api/v1/pricing/quotes` to calculate a fee and receive a 15-minute quote reference.
- **Parcel Booking:** `POST /api/v1/parcels` with validated inputs and mandatory `Idempotency-Key`.
- **Direct Merchant Booking:** Internal use case `parcels.CreateParcelUseCase` called by Member 4's merchant handler.
- **Parcel Details:** `GET /api/v1/parcels/{id}` subject to authenticated ownership and role scoping.
- **Parcel Listing:** `GET /api/v1/parcels` with bounded pagination (`page`, `per_page` $\le 100$).
- **Status History:** `GET /api/v1/parcels/{id}/history` returning chronological status milestones.
- **Receipt Data:** `GET /api/v1/parcels/{id}/receipt` returning accepted price snapshot data.
- **Lifecycle Transition Execution:** Invoking the `Parcel` aggregate methods to execute valid transitions (`ASSIGNED`, `PICKED_UP`, `OUT_FOR_DELIVERY`, `DELIVERED`, `ATTEMPT_FAILED`, `RETURNED`, `CANCELLED`).

Other modules must not:

- Execute direct SQL updates on parcel status (e.g. `UPDATE parcels SET current_status = ...`).
- Pass a client-supplied fee to be saved as the authoritative booking price.
- Bypass idempotency checks or input validation.
- Create competing parcel or status history tables in PostgreSQL.
- Publish events to RabbitMQ before the database transaction commits.
- Expose unmasked recipient PII to public tracking clients.

## 7. Ownership of Files, Tables, APIs, and Configuration

### 7.1 File Ownership

| Path | Ownership |
|---|---|
| `core-api/internal/parcels/` | Member 2 â€” Parcel aggregate root, booking use cases, lifecycle state machine, status history, and receipt data. |
| `core-api/internal/pricing/` | Member 2 â€” Pricing engine, volumetric calculations, tariff rules, quote persistence. |
| `core-api/migrations/000004_create_pricing_rules.up.sql` | Member 2 â€” Schema for `pricing_rules` and `pricing_quotes`. |
| `core-api/migrations/000005_create_parcels_and_quotes.up.sql` | Member 2 â€” Schema for `parcels` and `parcel_status_history`. |
| `core-api/migrations/000010_create_idempotency_keys.up.sql` | Member 2 (Shared technical driver) â€” Schema for `idempotency_keys`. |
| `core-api/tests/` | Member 2 contributes parcel and pricing unit, repository, and integration test suites. |
| `docs/drafts/member-02-parcels-pricing/` | Member 2 authoritative specifications (doc-01 through doc-08). |
| `core-api/internal/parcels/README.md`, `API.md` | Member 2 co-located living module documentation and living API contract. |
| `core-api/internal/pricing/README.md`, `API.md` | Member 2 co-located living module documentation and living API contract. |

### 7.2 Database Ownership

Member 2 owns the following PostgreSQL tables within the authoritative database schema:

- `parcels`: Core parcel aggregate, tracking number, status, JSONB address snapshots, accepted price snapshot, optimistic lock version.
- `parcel_status_history`: Append-only chronological audit log of all parcel status transitions.
- `pricing_rules`: Zone-pair tariff rates per weight band, complete fee in cents, currency, rule version, enabled state, and effective validity windows.
- `pricing_quotes`: Ephemeral 15-minute quote snapshots locking calculated prices prior to booking confirmation.
- `idempotency_keys`: Central idempotency store tracking caller scope, request SHA-256 hash, and cached HTTP responses.

Referenced external tables (`delivery_zones` owned by Member 7/Admin; `users` owned by Member 1; `merchants` owned by Member 4; `delivery_attempts` owned by Member 3; `outbox_events` owned by Member 8) are coordinated via foreign keys and application identifiers without transferring schema ownership.

### 7.3 API Ownership

Member 2 owns the implementation, OpenAPI specification, and maintenance of the following REST endpoints:

- `POST /api/v1/pricing/quotes` (Calculate delivery fee and generate 15-minute quote)
- `POST /api/v1/parcels` (Create parcel booking with idempotency key)
- `GET /api/v1/parcels` (List authenticated caller's parcels with bounded pagination)
- `GET /api/v1/parcels/{id}` (Get authorized parcel details)
- `GET /api/v1/parcels/{id}/history` (Get parcel status audit timeline)
- `GET /api/v1/parcels/{id}/receipt` (Get parcel booking receipt data)

The merchant-facing endpoint `POST /api/v1/merchant/parcels` is owned by Member 4 at the transport and auth layer, which calls Member 2's application use case.

### 7.4 Queue and Event Ownership

Member 2 does not own RabbitMQ broker configuration, exchange bindings, consumer workers, or webhook delivery.

Member 2 is responsible for:
- Writing integration events (`parcel.booked.v1`, `parcel.cancelled.v1`) into the `outbox_events` table inside the booking/cancellation database transaction.
- Providing consistent event payloads containing `event_id`, `event_type`, `aggregate_type: "parcel"`, `aggregate_id`, `version`, `timestamp`, `correlation_id`, and business payload.

### 7.5 Configuration Ownership

Member 2 defines module-specific environment configuration in `.env.example`:
- `PRICING_DEFAULT_CURRENCY=LKR`
- `PRICING_QUOTE_TTL_MINUTES=15`
- `PARCEL_MAX_WEIGHT_KG=30.0`
- `PARCEL_MAX_DIMENSION_CM=150.0`
- `IDEMPOTENCY_RETENTION_HOURS=24`

Real secrets must never be committed to source control.

## 8. Decision and Resolution Tracking

The table below summarizes the alignment between authoritative project sources, the comprehensive audit baseline, and decisions confirmed for implementation:

| Domain Area | Audit / Baseline Recommendation | Authoritative Resolution Status | Supporting Source | Affected Documents |
|---|---|---|---|---|
| **Pricing Endpoint & Quote Persistence** | Standardize on `POST /api/v1/pricing/quotes` with persisted quote record. | **RESOLVED:** Adopted canonical endpoint `POST /api/v1/pricing/quotes` and quote persistence in `pricing_quotes`. | Audit Â§4.A, `docs/contracts/pricing-rules.md` Â§6 | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Monetary Representation** | Store all monetary values as integer minor units (`BIGINT` / `int64` in cents) with currency `LKR`. Prohibit floating point. | **RESOLVED:** Canonical fields `fee_cents`, `price_cents`, `currency = 'LKR'` (complete tariff snapshot). Strict prohibition of floats. | Engineering Standards Â§5.2, `pricing-rules.md` Â§1 | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Pricing Model & Tariffs** | Fixed complete tariff per (pickup zone, destination zone, weight band). No separate base fee, per-kg fee, or multipliers. | **PROVISIONAL PROPOSAL (PENDING TEAM APPROVAL):** Complete fee model with 7 weight bands. Retrieval interface owned by Member 2; tariff administration workflow owned by Member 7. | Member 2 Baseline | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Volumetric Threshold & Divisor (6000)** | Actual weight $\le 1\text{ kg}$ exempt from volumetric pricing. Actual $> 1\text{ kg}$ requires dims and uses divisor 6000 cmÂ³/kg. Max actual/chargeable weight 30 kg. | **PROVISIONAL PROPOSAL (PENDING TEAM APPROVAL):** Option A threshold policy and divisor 6000 adopted as proposed design baseline. | Member 2 Baseline | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Quote Validity Period** | 15-minute price freeze window (`expires_at = created_at + 15m`). Backend expiry enforcement (`current_time >= expires_at`). | **PROVISIONAL PROPOSAL (PENDING TEAM APPROVAL):** Standard 15-minute quote TTL with backend enforcement upon booking confirmation. | Member 2 Baseline | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Address Data Model** | Replace separate `parcel_addresses` child rows with immutable `pickup_address JSONB NOT NULL` and `dropoff_address JSONB NOT NULL` directly on `parcels`. | **RESOLVED:** Single canonical model is JSONB snapshots on `parcels`. Separate `parcel_addresses` table removed from active design. | Audit Â§4.D, SRS Â§7.1 | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Tracking Number** | Format `SD-YYYYMMDD-XXXXXX` using Crockford's Base32 alphabet. Bounded collision retry (max 3). | **RESOLVED:** Standardized format `SD-YYYYMMDD-XXXXXX`. Tracking number is a public identifier, not an auth secret. | Audit Â§4.G, SRS FR-PAR-004 | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Parcel Lifecycle** | 9-state machine: `CREATED`, `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY`, `DELIVERED`, `ATTEMPT_FAILED`, `RETURNED`, `CANCELLED`. | **RESOLVED:** All transitions evaluated by `Parcel` aggregate root. Optimistic locking via `version INT`. | `domain-boundaries.md` Â§3, ADR-004 | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Failed Delivery** | Decouple `delivery_attempts` (Member 3) from `parcels` (Member 2). Increment `attempt_count` (max 3). | **RESOLVED:** Transient `ATTEMPT_FAILED` requires dispatcher rescheduling; terminal `RETURNED` upon 3rd failure. | ADR-004, SRS Â§13.2 | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Idempotency** | Header `Idempotency-Key` (UUIDv4), table `idempotency_keys` with caller scope, SHA-256 hash, 24h TTL, HTTP 409 on mismatch. | **RESOLVED:** Single canonical idempotency implementation shared via platform driver. | Audit Â§4.H, `api-conventions.md` Â§4 | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Database Migrations** | Sequential numbers: `000004_create_pricing_rules.up.sql`, `000005_create_parcels_and_quotes.up.sql`, `000010_create_idempotency_keys.up.sql`. `delivery_zones.id` is `VARCHAR(64)`. | **RESOLVED:** Adopted migration schedule aligned with Member 9 database coordination. | Audit Â§4.I, `database-schema.md` | Docs 1, 5, 8 |
| **Outbox Events** | Atomic commit of `parcels`, `parcel_status_history`, and `outbox_events` (`parcel.booked.v1`, `parcel.cancelled.v1`). | **RESOLVED:** Producer responsibility separated from Member 8 relay daemon. | Audit Â§4.J, `event-contracts.md` Â§2 | Docs 1, 2, 3, 4, 5, 6, 7, 8 |
| **Merchant Ingress** | Merchant route `POST /api/v1/merchant/parcels` owned by Member 4; invokes Member 2 use case in-process. | **RESOLVED:** Member 2 does not document competing merchant routes as canonical. | Audit Â§4.E, `domain-boundaries.md` Â§2 | Docs 1, 2, 3, 4 |
| **Tariff Administration** | Administrative write endpoints for zones and pricing rules (`/api/v1/admin/pricing-rules`). | **PENDING TEAM CONFIRMATION:** Candidate ownership belongs to Member 7 (Admin UI / Gateway) and Member 1 (Admin RBAC). Member 2 provides domain calculation interfaces. | Team Work Division Â§3 (Member 7) | Docs 1, 2, 3, 4 |
| **Receipt Rendering** | Downloadable PDF presentation vs structured JSON receipt endpoint. | **PENDING TEAM CONFIRMATION:** Member 2 standardizes `GET /api/v1/parcels/{id}/receipt` returning structured JSON receipt data; presentation/PDF download coordinated with Member 6. | SRS FR-PAR-008 | Docs 1, 2, 3, 4, 6 |

### 8.1 Pricing-Policy Dependency & Impact Analysis (If Pricing Policy Changes)

The delivery pricing policy has been architected to isolate mathematical formulas and tariff configurations from core parcel booking and state persistence. If the team changes the pricing policy, the following table details the exact impact and required modifications:

| Component / Layer | Stable / Invariant Aspect | Impact If Pricing Policy Changes | Specific Files / Tables Affected |
| :--- | :--- | :--- | :--- |
| **Core Architecture & Persistence** | PostgreSQL persistence, atomic transaction, `parcels` table schema, accepted-price snapshot storage, outbox events. | **Zero Impact:** Booking transactions, address snapshots, and parcel lifecycle remain completely unchanged. | `parcels`, `parcel_status_history`, `outbox_events` |
| **Monetary Standard** | Integer minor units (`BIGINT` in SQL, `int64` in Go, LKR cents). No binary floating-point. | **Zero Impact:** Any revised tariff must still be stored and calculated in integer minor units. | All schemas and domain structs |
| **API Contracts** | `POST /api/v1/pricing/quotes` endpoint path and base envelope (`quote_id`, `fee_cents`, `currency`, `expires_at`). | **DTO Alignment:** Breakdown fields reflect chargeable weight, selected band (Bands 1â€“7), and rule version. | `core-api/internal/pricing/API.md`, `doc-03`, `doc-04` |
| **Database Schema** | `pricing_quotes` and `pricing_rules` table existence and foreign key relationships. | **Column Alignment:** `pricing_rules` stores fixed complete fee per zone-pair and weight band. Max weight constraint reflects 30 kg limit. | `migrations/000004_create_pricing_rules.up.sql`, `doc-05` |
| **Quote Expiry Policy** | Quote verification during booking (`quote_id` lookup and backend freeze enforcement). | **TTL & Error Handling:** 15-minute window enforced at `current_time >= expires_at`. | `.env.example`, `doc-08`, Member 6 UI |
| **Validation Limits** | Bounded validation in Go transport DTOs before persistence. | **Boundary Checks:** Proposed 30 kg max actual weight and 30 kg max chargeable weight enforced. Required dimensions for actual weight $> 1$ kg. | `internal/pricing/transport/dto.go`, `doc-03` |
| **Automated Tests** | Test structure, mocks, transaction rollback assertions, idempotency checks. | **Test Suite Alignment:** Tests cover all 7 bands, boundary transitions, divisor 6000, 1 kg threshold, and 30 kg ceiling. | `doc-06`, `internal/pricing/domain/formula_test.go` |

## 9. Out-of-Scope Work

The following activities are strictly outside Member 2's implementation scope:

- Implementing Google OAuth, user registration, JWT token generation, or credential hashing (Member 1).
- Building courier route dispatching algorithms, driver workload balancing, or mobile delivery apps (Member 3).
- Storing high-resolution proof-of-delivery photos or raw SVG signature data in object storage (Member 3).
- Implementing the independently deployed Tracking API service, Redis caching, or SSE connections (Member 5).
- Building customer booking wizards or dispatcher boards in React (Members 6 and 7).
- Implementing the Outbox Relay poller daemon, RabbitMQ exchange topology, or consumer workers (Member 8).
- Managing platform-wide Kubernetes manifests, Helm charts, CI/CD pipelines, or Prometheus setups (Member 9).
- Building Python/FastAPI AI Assistant tools or LLM integrations (Member 10).

Member 2 remains responsible for providing the robust interfaces, data contracts, migrations, and test suites required for these components to integrate seamlessly.

## 10. Acceptance Criteria

The module is considered implementation-ready and acceptable when:

1. **Deterministic Pricing:** A valid quote request (`POST /api/v1/pricing/quotes`) deterministically calculates delivery fees using LKR integer cents, fixed complete tariffs across 7 weight bands, volumetric divisor 6000 cmÂ³/kg with 1 kg actual weight threshold, 30 kg max weight limits, and returns a 15-minute quote snapshot with backend expiry enforcement.
2. **Atomic Booking:** Submitting `POST /api/v1/parcels` with a valid quote or inputs atomically creates the parcel record, JSONB address snapshots, initial status history, idempotency record, and outbox event (`parcel.booked.v1`).
3. **Idempotency Guarantees:** Repeating an identical booking with the same `Idempotency-Key` returns the original cached response with `Idempotent-Replay: true`. Reusing a key with a different payload returns HTTP `409 Conflict`.
4. **Tracking Number Integrity:** Every booked parcel receives an unambiguous `SD-YYYYMMDD-XXXXXX` tracking number, backed by database unique constraints and collision retry logic.
5. **State Machine Invariants:** All lifecycle status changes are evaluated by the `Parcel` aggregate root, incrementing `version` for optimistic locking, rejecting direct SQL mutations, and creating immutable audit records in `parcel_status_history`.
6. **Failed Delivery Handling:** Delivery failures increment `attempt_count` up to 3; transient failures enter `ATTEMPT_FAILED` requiring dispatcher intervention, while the 3rd failure transitions to terminal `RETURNED`.
7. **Authorization & Privacy:** Cross-customer and cross-tenant access attempts return HTTP `403 Forbidden` or `404 Not Found`. Public tracking views strictly redact recipient phone numbers and street addresses.
8. **Receipt Historical Stability:** `GET /api/v1/parcels/{id}/receipt` retrieves accepted price snapshot values, unaffected by subsequent pricing-rule modifications.
9. **Migration Repeatability:** PostgreSQL migrations (`000004`, `000005`, `000010`) execute cleanly on clean databases and support reproducible automated test suites.
10. **Living Documentation Integrity:** Co-located living documentation (`README.md`, `API.md`) in `core-api/internal/parcels/` and `core-api/internal/pricing/` precisely mirrors implemented contracts.

## 11. Definition of Done

For every completed capability, Member 2 must verify that:

- The implementation matches the approved domain boundaries, API specifications, and database schema.
- Go code is formatted with `gofmt`, adheres to project standards, and passes `golangci-lint`.
- Automated unit and integration tests cover happy paths, boundary conditions, authorization denial, idempotency replay, and transaction rollbacks.
- Database migrations execute cleanly up and down in CI environments.
- Configuration templates contain safe placeholders with zero hardcoded credentials or secrets.
- Co-located module living documentation (`README.md` and `API.md`) and specifications are updated synchronously.
- Changes are verified against a local PostgreSQL instance before pull request review.itted on an appropriate feature branch and reviewed before merging.
- The feature can be demonstrated using the agreed local development environment.


