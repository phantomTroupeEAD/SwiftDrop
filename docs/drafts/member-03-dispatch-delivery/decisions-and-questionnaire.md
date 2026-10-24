# Member 03 Architecture Decisions & Module Alignment Questionnaire

> **Role:** Member 03 — Dispatch, Couriers & Proof of Delivery  
> **Status:** Pre-Draft Alignment Questionnaire  
> **Target Modules:** `core-api/internal/dispatch/` and `core-api/internal/delivery/`  
> **Purpose:** Resolve all critical domain, architectural, security, state machine, and data persistence decisions before authoring the 8 mandatory module specifications.

---

## Instructions for Review

Below is the comprehensive decision matrix covering every aspect of the Member 3 workload. Each item contains:
1. **The Question / Decision Needed**
2. **Context & Why It Matters**
3. **Recommended Answer & Architectural Justification**
4. **Alternative Options Considered**

Please review each question. If you agree with the recommended answers, you can confirm them directly or provide custom adjustments.

---

## Part 1: Fleet Management & Courier Profiles (`internal/dispatch`)

### Q1.1: Courier Profile Provisioning & Account Creation
- **Question:** How are courier profiles created and linked to authenticated system users?
- **Context:** Couriers need both a login account (`users` table owned by Member 1) and operational fleet metadata (`couriers` table owned by Member 3).
- **Recommended Answer:**
  - **Two-Step Provisioning via Admin/Dispatcher:**
    1. An Administrator creates the user account in `users` with role `COURIER` (via Member 1's identity module).
    2. Member 3's module exposes `POST /api/v1/dispatch/couriers` (accessible by `ADMIN` and `DISPATCHER`) to attach courier operational metadata: `user_id`, `employee_code` (e.g., `CR-0042`), `vehicle_type`, `max_capacity`, and `phone_number`.
    3. `couriers.user_id` is both Primary Key and Foreign Key referencing `users.id` with `ON DELETE RESTRICT`.
- **Justification:** Preserves clean bounded context separation. Identity & credential management remains strictly with Member 1, while fleet operational metadata is owned by Member 3.
- **Alternatives Considered:** Auto-creating courier profile on first login (risks unvalidated couriers joining the fleet), single wide table (violates modular monolith boundary).

---

### Q1.2: Courier Shift Lifecycle & Availability
- **Question:** How does a courier transition between active and inactive operational states?
- **Context:** A dispatcher must never assign parcels to an offline, off-shift, or suspended courier.
- **Recommended Answer:**
  - **Courier Shift State Machine:**
    - States: `OFF_SHIFT` (default), `ON_SHIFT` (active), `ON_BREAK`, `SUSPENDED`.
    - **Self-Service Shift Toggle:** An authenticated courier can toggle their shift status (`POST /api/v1/courier/shift` with `{ status: "ON_SHIFT" | "OFF_SHIFT" | "ON_BREAK" }`).
    - **Dispatcher/Admin Suspension:** Dispatchers/Admins can set `SUSPENDED` or force `OFF_SHIFT` if a courier is unresponsive.
    - **Ineligibility Rule:** The assignment engine strictly filters for `status = 'ON_SHIFT'` and `is_active = true`.
- **Justification:** Real-world logistics requires couriers to signal when they are on the road. Preventing assignments to off-shift drivers avoids packages sitting idle in the depot.
- **Alternatives Considered:** Automatic shift based on login/logout (inaccurate because logging out of the web portal does not mean a courier in the field stopped delivering).

---

### Q1.3: Vehicle Types & Capacity Ceilings
- **Question:** What vehicle types are supported, what are their default capacity ceilings, and how is overload prevented?
- **Context:** Assigning 40 parcels to a motorcycle courier will fail physical delivery and breach SLAs.
- **Recommended Answer:**
  - **Vehicle Classifications & Default Parcel Limits:**
    - `MOTORCYCLE`: Max **25** active parcels (System Default for urban parcel delivery).
    - `VAN`: Max **60** active parcels (Suburban & bulk courier runs).
    - `TRUCK`: Max **120** active parcels (Heavy freight & inter-hub transfers).
  - **Capacity Rule:** Each courier has a configurable `max_capacity` column on their profile.
  - **Invariant Enforcement:** Before assigning a parcel, the application evaluates:
    $$\text{current\_active\_assignments} + 1 \le \text{max\_capacity}$$
    If violated, the system returns `409 Conflict` with error code `COURIER_CAPACITY_EXCEEDED`.
- **Justification:** Protects couriers from overload, maintains delivery feasibility, and satisfies SRS §4.7.
- **Alternatives Considered:** Weight/volume volumetric bin-packing (over-engineered for MVP/assignment scope; count-based capacity with max weight limit is standard and robust).

---

## Part 2: Dispatcher Operations & Parcel Assignment (`internal/dispatch`)

### Q2.1: Manual vs. Automated Parcel Assignment
- **Question:** How are parcels assigned to couriers—manual dispatcher selection, batch auto-assignment, or both?
- **Context:** SRS §4.7 requires dispatchers to assign parcels, while some operations benefit from rule-based auto-assign suggestions.
- **Recommended Answer:**
  - **Dispatcher-Driven Manual Assignment with Smart Suggestions (Baseline):**
    - Primary workflow: Dispatcher views unassigned parcels (`GET /api/v1/dispatch/unassigned`), selects a courier from active list (`GET /api/v1/dispatch/workloads`), and assigns (`POST /api/v1/dispatch/assignments`).
    - Smart filter: The dispatch board sorts/filters couriers by:
      1. Zone match (`courier.zone_code == parcel.delivery_zone_code`).
      2. Lowest current workload (`active_assignments ASC`).
      3. Available capacity margin (`max_capacity - active_assignments > 0`).
    - Optional/Stretch Auto-Assign endpoint: `POST /api/v1/dispatch/auto-assign` to assign top pending parcels to the least-loaded available couriers in the zone.
- **Justification:** Directly matches the core assignment brief requirements without unnecessary algorithmic complexity, while giving dispatchers full operational control.
- **Alternatives Considered:** Black-box automated routing engine (hard to test, prone to unpredictable edge cases in demo).

---

### Q2.2: Concurrency & Race Condition Protection
- **Question:** How does the system prevent two dispatchers from simultaneously assigning the same parcel to different couriers?
- **Context:** In high-volume operations, two dispatchers looking at the board simultaneously could click "Assign" on the same package at the exact same second.
- **Recommended Answer:**
  - **Pessimistic Locking & State Validation in PostgreSQL Transaction:**
    ```sql
    BEGIN;
    -- Lock parcel row and verify eligibility
    SELECT id, current_status, version 
    FROM parcels 
    WHERE id = $1 
    FOR UPDATE;

    -- Verify current_status IN ('CREATED', 'ATTEMPT_FAILED')
    -- If already ASSIGNED or OUT_FOR_DELIVERY, ROLLBACK and return 409 Conflict

    -- Lock courier and verify capacity
    SELECT id, max_capacity 
    FROM couriers 
    WHERE user_id = $2 AND status = 'ON_SHIFT' 
    FOR UPDATE;

    -- Count active assignments
    SELECT COUNT(*) FROM courier_assignments 
    WHERE courier_id = $2 AND status = 'ASSIGNED';

    -- Insert assignment record & update parcel state
    COMMIT;
    ```
- **Justification:** Guaranteed atomicity and safety at the database engine level. Prevents race conditions completely with zero chance of dual-assignment.
- **Alternatives Considered:** Optimistic concurrency tokens (`ETag` / `version` column) alone without DB lock (can lead to retry storms under concurrent clicks).

---

### Q2.3: Reassignment & Cancellation Semantics
- **Question:** Can a parcel be unassigned or reassigned to another courier, and what happens to the previous assignment record?
- **Context:** A courier's vehicle may break down or a courier may become sick while parcels are assigned.
- **Recommended Answer:**
  - **Explicit Reassignment Workflow with Immutable Assignment History:**
    - **Depot / Before Departure (`ASSIGNED` status):** If the parcel is still at the hub, the dispatcher simply reassigns it via `POST /api/v1/dispatch/assignments`. The old assignment row is updated (`status = 'CANCELLED'`, `ended_at = NOW()`), and a new assignment row is created for the replacement courier. The parcel state remains `ASSIGNED`.
    - **In Field / On the Road (`OUT_FOR_DELIVERY` status):** If a courier's vehicle breaks down or an emergency occurs while parcels are in transit:
      1. An operational interruption is recorded via `POST /api/v1/deliveries/{id}/failed` with reason code `DISPATCH_INTERRUPTED` or `OTHER`. This safely transitions the parcel to `ATTEMPT_FAILED` (flagged as operational so it does not penalize the recipient's 3-attempt customer ceiling).
      2. When the parcel is returned to the depot or physically transferred to the replacement courier, the Dispatcher executes reassignment via `POST /api/v1/dispatch/assignments`, transitioning the parcel to `ASSIGNED` or back into `OUT_FOR_DELIVERY` under the new courier.
    - Required authorization: `DISPATCHER` or `ADMIN` only. All reassignments create an immutable audit record in `courier_assignments`.
- **Justification:** Keeps a complete historical audit trail of who was assigned when, rather than overwriting existing records.
- **Alternatives Considered:** Deleting the old assignment row (destroys audit history and breaks accountability).

---

## Part 3: Courier Workflow & Delivery Execution (`internal/delivery`)

### Q3.1: Courier Daily Workload & Permitted Visibility
- **Question:** What parcel information can a courier see on their device/mobile web view?
- **Context:** Data minimization and privacy regulations mandate that couriers see operational details needed for delivery, but not private business billing or unrelated customer data.
- **Recommended Answer:**
  - **Endpoint:** `GET /api/v1/courier/parcels` (filtered by authenticated `jwt.user_id`).
  - **Visible Operational Fields:**
    - Parcel ID & Tracking Number
    - Recipient Name, Delivery Address, City/Postal Code, Recipient Contact Phone Number
    - Package Weight, Dimensions, Handling Instructions (e.g., "Fragile", "Leave at door")
    - Scheduled Delivery Window / Priority
    - Current Attempt Count (e.g. "Attempt 2 of 3")
  - **Redacted / Hidden Fields:**
    - Sender's internal merchant tariff rates, merchant profit margins, merchant API keys.
    - Other couriers' parcels or unassigned packages.
- **Justification:** Satisfies SRS §4.6 (`FR-COU-001`, `FR-COU-002`) and privacy rules while providing everything needed to physically deliver the package.
- **Alternatives Considered:** Exposing full parcel entity (leaks merchant pricing and billing metadata to drivers).

---

### Q3.2: Parcel Transit Milestone Milestones & State Flow
- **Question:** Which exact milestone transitions does the courier trigger directly?
- **Context:** State transitions must be consistent across Member 2 (Parcels) and Member 3 (Delivery).
- **Consolidated Decision (Post-Review Alignment):**
  - **Courier-Triggered Transitions:**
    1. **`OUT_FOR_DELIVERY`:** Courier collects assigned package(s) from depot and starts the field delivery run (`POST /api/v1/courier/parcels/{id}/start-run` or batch `POST /api/v1/courier/runs/start-batch`). Physical collection and run departure are consolidated into a single operational action to eliminate redundant micro-status latency.
    2. **`DELIVERED`:** Courier hands over package and submits proof (`POST /api/v1/deliveries/{id}/complete`).
    3. **`ATTEMPT_FAILED`:** Courier reports inability to deliver (`POST /api/v1/deliveries/{id}/failed`).
  - **Hub / Dispatcher Transitions:**
    - `ASSIGNED`: Dispatcher assigns package to an on-shift courier (`POST /api/v1/dispatch/assignments`).
    - `IN_TRANSIT`: Marked when package is in linehaul transit between sorting hubs.
    - `RETURNED`: Terminal return marked upon 3rd failed attempt or recipient refusal.
- **Justification:** Eliminates friction in the courier mobile app by consolidating pickup and departure into one atomic transition (`ASSIGNED` $\rightarrow$ `OUT_FOR_DELIVERY`), while maintaining an immutable status audit trail.
- **Alternatives Considered:** Requiring separate `POST /pickup` and `POST /start-run` roundtrips (rejected during review as unnecessary operational overhead).

---

## Part 4: Proof of Delivery (POD) & Verification (`internal/delivery`)

### Q4.1: Supported Proof-of-Delivery Types
- **Question:** What proof types must be supported for marking a package `DELIVERED`?
- **Context:** SRS §4.6 and §13.2 mention recipient name, signature, and photo upload.
- **Recommended Answer:**
  - **Supported Proof Types (`proof_type` enum):**
    1. `PHOTO`: Photo of package at doorstep / delivered safe place (standard for contactless delivery).
    2. `SIGNATURE`: Digitize signature drawn by the **recipient / customer** on the courier's screen, accompanied by recipient verification details.
    3. `RECIPIENT_CONFIRMATION`: Recipient name + relationship (e.g., "Jane Doe - Sister").
  - **Mandatory Recipient & Courier Verification Invariants:**
    - **Recipient Identity:** When `SIGNATURE` is used, the system **MUST capture BOTH** `received_by_name` (who accepted the delivery) AND `signature_data` (Base64/SVG stroke points, length $\ge 50$). Recipient identity must never be anonymous.
    - **Courier Identity:** The delivery person does **NOT** type their own name or sign. Their identity (`courier_id`, employee code) is automatically and securely extracted from the authenticated JWT token (`auth.UserID`) and stamped into the database audit trail.
    - A parcel **CANNOT** transition to `DELIVERED` without at least one valid proof payload:
      - If `PHOTO`: Valid uploaded image reference URL required.
      - If `SIGNATURE`: Valid SVG/base64 signature string AND `received_by_name` required.
      - If `RECIPIENT_CONFIRMATION`: `received_by_name` required (`length >= 2`).
- **Justification:** Provides flexibility for both attended delivery (signature/name) and unattended/safe-place delivery (photo).
- **Alternatives Considered:** Photo mandatory for all deliveries (impractical when customer signs in person).

---

### Q4.2: Photo Upload & Object Storage Architecture
- **Question:** How are proof-of-delivery photos uploaded, stored, and served without storing large binary data in PostgreSQL?
- **Context:** Storing images in PostgreSQL causes database bloat and performance degradation. SRS §4.6 (`FR-COU-007`) mandates storing only metadata/URLs in the database.
- **Recommended Answer:**
  - **Two-Tier Storage Abstraction (`ObjectStorageClient`):**
    - **Interface Definition:**
      ```go
      type ObjectStorage interface {
          Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error)
          GetPreSignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
      }
      ```
    - **Development / Demo Mode (`STORAGE_DRIVER=local`):**
      - Photos saved to `core-api/uploads/proofs/` directory and served via authenticated endpoint.
    - **Production / Cloud Mode (`STORAGE_DRIVER=s3`):**
      - Direct S3-API compatible upload to **Cloudflare R2** (recommended for zero-egress fees), AWS S3, or local MinIO bucket.
      - Because Cloudflare R2 and MinIO conform to the AWS S3 API standard, the exact same Go `aws-sdk-go-v2` client handles all three via configurable `S3_ENDPOINT`, `S3_BUCKET`, and credentials.
      - Pre-signed URLs generated with 15-minute TTL for secure viewing.
    - **PostgreSQL Persistence:**
      - Only the storage key/relative path (e.g., `proofs/2026/10/prcl_123_pod.jpg`) is saved in `delivery_attempts.proof_attachment_url`.
- **Justification:** Fully adheres to clean architecture, prevents database bloat, and works identically in local Compose and cloud/Kubernetes environments.
- **Alternatives Considered:** Storing Base64 image strings in PostgreSQL `TEXT` columns (strictly prohibited by engineering standards).

---

## Part 5: Failed Delivery Semantics & 3-Attempt Lifecycle ([ADR-004](file:///e:/Projects/SwiftDrop/docs/architecture/adr-004-failed-delivery-semantics.md))

### Q5.1: Standardized Failure Reason Codes & Mandatory Explanations
- **Question:** What failure reason codes are accepted when a delivery attempt fails, and when is a note mandatory?
- **Context:** ADR-004 mandates standardized codes to eliminate ambiguous courier inputs.
- **Recommended Answer:**
  - **Standardized Codes (`failure_reason_code`):**
    1. `RECIPIENT_UNAVAILABLE`: Nobody home, phone unanswered.
    2. `CUSTOMER_REQUESTED_RESCHEDULE`: Customer requested delivery on another date/time.
    3. `ACCESS_RESTRICTED`: Gated building, security denied entry, hostile animal.
    4. `INVALID_ADDRESS`: Street number missing, wrong street, building demolished.
    5. `REFUSED_BY_RECIPIENT`: Recipient explicitly refused acceptance.
    6. `DAMAGED_IN_TRANSIT`: Package found damaged prior to delivery.
    7. `OTHER`: Any unclassified reason.
  - **Validation Invariant:** If `reason_code == 'OTHER'`, `notes` is **mandatory and must meet a configurable minimum character length**:
    - Configured via environment/app settings: `DELIVERY_FAILURE_NOTE_MIN_LENGTH` (default: `15`).
    - Prevents single-word or vague courier notes (e.g. "no", "cant") while allowing easy policy adjustments without code modifications.
- **Justification:** Standard codes enable automated analytics and reporting (`FR-ADM-008`), while freeform notes ensure edge cases are documented.
- **Alternatives Considered:** Free-text only reasons (makes reporting impossible), rigid codes with no custom notes (couriers cannot explain unusual field situations).

---

### Q5.2: Attempt Counter Increment & Terminal `RETURNED` Trigger
- **Question:** How does the 3-attempt ceiling work in practice, and what happens on the 3rd failure?
- **Context:** Packages cannot stay in limbo forever. SRS §13.2 and ADR-004 define a maximum of 3 delivery attempts.
- **Recommended Answer:**
  - **Attempt Lifecycle Rules:**
    1. Each failed attempt creates an immutable row in `delivery_attempts` with `attempt_number = parcels.attempt_count + 1`.
    2. `parcels.attempt_count` is incremented atomically.
    3. **If `attempt_count < 3`:**
       - Parcel status becomes `ATTEMPT_FAILED`.
       - Package is returned to the local hub at end of shift.
       - Awaiting dispatcher review.
    4. **If `attempt_count >= 3` OR reason is `REFUSED_BY_RECIPIENT`:**
       - Parcel status automatically moves to `RETURNED` (terminal state).
       - No further delivery attempts are permitted.
       - Integration event `parcel.returned.v1` is committed to the outbox.
       - **Downstream Actions Triggered by `parcel.returned.v1`:**
         - **Outbox Relay (Member 8):** Relays the event to RabbitMQ exchange `swiftdrop.events`.
         - **Tracking API (Member 5):** Updates public and authenticated tracking status to `RETURNED` with the explanation: *"Delivery unsuccessful after maximum attempts; package being returned to sender."*
         - **Notification Worker (Member 8):** Dispatches SMS/Email to both the customer and the sender/merchant regarding the return.
         - **Merchant Webhook Worker (Member 8):** Pushes signed webhook `parcel.returned` to the merchant's endpoint so the merchant's e-commerce system can trigger automated restocking, customer outreach, or refunds.
         - **Physical Hub Operations:** The package is flagged for transfer into the depot's "Return-to-Sender / Reverse Logistics" holding bin.
- **Justification:** Fully enforces ADR-004 invariants, prevents infinite delivery loops, and protects business profitability.
- **Alternatives Considered:** Allowing infinite attempts if customer keeps rescheduling (incurs unsustainable courier costs).

---

### Q5.3: Dispatcher Authorization Gate for Reattempts
- **Question:** Can a courier self-schedule a reattempt, or is dispatcher authorization strictly required?
- **Context:** Couriers might want to reattempt immediately on their way back, but dispatchers need to review addresses or coordinate customer availability.
- **Recommended Answer:**
  - **Strict Dispatcher Authorization Gate:**
    - Couriers **CANNOT** directly transition a parcel from `ATTEMPT_FAILED` back to `OUT_FOR_DELIVERY`.
    - Only a user with role `DISPATCHER` or `ADMIN` can authorize a reattempt via `POST /api/v1/dispatch/parcels/{id}/reschedule`.
    - The dispatcher can reassign to the same courier or a different courier for the next shift.
- **Justification:** Directly satisfies SRS §13.2 ("Separate delivery attempt outcome from parcel lifecycle; allow reattempt only through explicit dispatcher action").
- **Alternatives Considered:** Allowing couriers to click "Reattempt Now" (bypasses dispatch oversight and creates billing/workload discrepancies).

---

## Part 6: Architecture, Persistence & Events

### Q6.1: Clean Architecture Layer Structure for Member 3
- **Question:** How are Member 3's two bounded contexts structured inside `core-api/internal/`?
- **Context:** `AGENTS.md` mandates clean package separation: `domain/` $\to$ `application/` $\to$ `infrastructure/` $\to$ `transport/`.
- **Recommended Answer:**
  - **Two Parallel Clean Architecture Modules:**
    ```
    core-api/internal/
    ├── dispatch/
    │   ├── domain/           # Courier entity, CourierAssignment aggregate, VehicleType VO, errors
    │   ├── application/      # AssignCourier, ReassignCourier, GetDispatchBoard, ToggleShift use cases
    │   ├── infrastructure/   # PostgresCourierRepo, PostgresAssignmentRepo
    │   └── transport/        # HTTP handlers (/api/v1/dispatch/*, /api/v1/courier/shift)
    └── delivery/
        ├── domain/           # DeliveryAttempt entity, ProofOfDelivery value object, ReasonCodes
        ├── application/      # CompleteDelivery, RecordFailedAttempt, GetCourierParcels use cases
        ├── infrastructure/   # PostgresDeliveryRepo, LocalObjectStorage, S3ObjectStorage
        └── transport/        # HTTP handlers (/api/v1/deliveries/*, /api/v1/courier/parcels)
    ```
- **Justification:** Separates operational assignment (`dispatch`) from doorstep delivery confirmation (`delivery`), matching DDD bounded contexts while sharing the same modular monolith runtime.
- **Alternatives Considered:** Merging into a single `couriers` package (violates single-responsibility and creates oversized files).

---

### Q6.2: Single PostgreSQL Transaction Contract
- **Question:** How are parcel status updates, assignment records, audit history, and outbox events kept consistent?
- **Context:** If a status updates but the outbox event or audit history fails to insert, data corruption occurs.
- **Recommended Answer:**
  - **All-in-One Transaction Execution:**
    Every operational mutation executes within a single `sql.Tx`:
    1. Lock and validate `parcels` row.
    2. Insert/update `courier_assignments` or `delivery_attempts`.
    3. Update `parcels.current_status` and increment `parcels.attempt_count` (if attempt).
    4. Insert immutable audit entry in `parcel_status_history`.
    5. Insert integration event record in `outbox_events` (for Member 8's relay).
    6. `tx.Commit()`
- **Justification:** Guarantees zero phantom state transitions, 100% audit compliance, and strict transactional outbox reliability.
- **Alternatives Considered:** Updating parcel state first and publishing events asynchronously in a goroutine (risks lost events if process crashes before publishing).

---

### Q6.3: Integration Events Emitted by Member 3 & Worker Enrichment
- **Question:** What exact event schemas does Member 3 publish to `outbox_events`, and how does the Event Worker know where to route notifications and merchant webhooks?
- **Context:** Member 8 (Messaging) and Member 5 (Tracking) rely on stable, versioned event schemas, while merchants need automated delivery callbacks.
- **Recommended Answer:**
  - **4 Canonical Events Emitted by Member 3:**
    1. `parcel.assigned.v1`: `{ parcel_id, tracking_number, courier_id, assigned_by, assigned_at }`
    2. `parcel.out_for_delivery.v1`: `{ parcel_id, tracking_number, courier_id, started_at }`
    3. `parcel.delivered.v1`: `{ parcel_id, tracking_number, courier_id, proof_type, proof_url, received_by, delivered_at }`
    4. `parcel.delivery_failed.v1`: `{ parcel_id, tracking_number, courier_id, attempt_number, reason_code, next_action, attempted_at }`
    *(Plus terminal `parcel.returned.v1` on 3rd attempt failure).*
  - **How Downstream Event Workers Resolve Routing & Recipients (Architecture Invariant):**
    - **Merchant Webhook Dispatch:**
      1. When the Event Worker consumes a `parcel.*` event from RabbitMQ, it inspects `parcel_id`.
      2. It queries PostgreSQL to check if the parcel belongs to an authorized `merchant_id`.
      3. If yes, it loads active webhook endpoints from the `webhook_endpoints` table (configured by Member 4 in `internal/merchants`).
      4. It serializes the event according to the standard webhook schema in `docs/contracts/webhook-security.md`, signs it with an `HMAC-SHA256` signature using the merchant's secret, and performs an HTTP POST with retry/DLQ support.
    - **Customer SMS / Email Notifications:**
      1. The parcel record in PostgreSQL (created by Member 2) stores `recipient_phone`, `recipient_email`, and `recipient_name`.
      2. The Notification Worker reads these contact details to dispatch templated SMS/Email messages (e.g. *"Your package is out for delivery with Courier Bob"*, *"Package delivered; signed by Jane"*).
      3. Member 3 is **not** responsible for sending emails or webhooks directly; Member 3 only commits the domain event to `outbox_events` within the transactional boundary.
- **Justification:** Perfectly aligns with DDD, the transactional outbox pattern, and `docs/contracts/event-contracts.md`. Keeps Member 3's Go core clean while enabling fully decoupled asynchronous fan-out.

---

### Q6.4: Seed Data & Local Demo Scenarios
- **Question:** What deterministic test/demo couriers and dispatch scenarios should be seeded for local testing and presentation?
- **Context:** The team needs reproducible demo scenarios where a courier logs in, sees their parcels, delivers with proof, and marks a failure.
- **Recommended Answer:**
  - **Deterministic Seed Data:**
    - **Courier 1 (`courier_bob`):** Motorcycle, ZONE_COLOMBO_METRO, `ON_SHIFT`, 3 active assignments (`prcl_demo_01`, `prcl_demo_02`, `prcl_demo_03`).
    - **Courier 2 (`courier_alice`):** Van, ZONE_GREATER_WESTERN, `ON_SHIFT`, 1 active assignment (`prcl_demo_04`).
    - **Courier 3 (`courier_charlie`):** Motorcycle, ZONE_COLOMBO_METRO, `OFF_SHIFT`, 0 assignments (for testing off-shift assignment rejection).
    - **Dispatcher (`dispatcher_dan`):** Pre-configured with credentials for testing the dispatch board.
    - **Demo Parcels:**
      - 2 parcels in `CREATED` status ready for assignment.
      - 1 parcel in `ATTEMPT_FAILED` (attempt 1) ready for reattempt.
      - 1 parcel in `ATTEMPT_FAILED` (attempt 2) ready for final attempt / return test.
- **Justification:** Guarantees every feature can be demonstrated immediately on a clean clone without manual data entry.
- **Alternatives Considered:** Empty database on startup (forces manual setup before every demo or test run).

---

## Summary of Next Steps

Once you review and confirm these answers:
1. We will check off the questionnaire in `docs/drafts/member-03-dispatch-delivery/`.
2. We will systematically author the **8 mandatory documents** (`doc-01` to `doc-08`) in `docs/drafts/member-03-dispatch-delivery/` exactly following the verified baseline.
3. We will update the living documentation (`README.md` and `API.md`) in `core-api/internal/dispatch/` and `core-api/internal/delivery/`.
