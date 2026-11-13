# Delivery Records Module (`internal/delivery`)

> **Domain:** Delivery Attempts, Proof of Delivery (Photo/Signature) & Failure Reason Tracking  
> **Primary Owner:** Member 3 — Dispatch, Couriers & Proof of Delivery  
> **Clean Architecture Boundary:** Modular Monolith Internal Module

---

## 1. Module Overview & Responsibilities
The `delivery` module governs physical courier interactions at the recipient's doorstep: capturing proof of delivery (POD), handling customer recipient confirmation, recording delivery failure reasons, and managing re-attempt policies.

### Key Responsibilities:
- Recording successful delivery with verified proof attachments (secure photo upload, recipient signature, or attended confirmation).
- Logging failed delivery attempts with structured ADR-004 reason codes.
- Incrementing attempt counters (triggering return-to-hub after 3 failed attempts or refusal).
- Coordinating with `internal/parcels` to execute the terminal `DELIVERED`, intermediate `ATTEMPT_FAILED`, or terminal `RETURNED` state transition.
- Scoping courier daily manifests with strict operational data minimization.

---

## 2. Domain Invariants & Business Rules
1. **Mandatory Proof for Completion:** A parcel CANNOT transition to `DELIVERED` without a valid proof record (photo URL, recipient signature data with stroke length $\ge 50$ and recipient name, or OTP/confirmation).
2. **Assigned Courier Check (AT-006 IDOR Defense):** Only the courier currently assigned to the parcel may record a delivery, start a run, or log a failed attempt.
3. **Attempt Limit (ADR-004):** A maximum of 3 delivery attempts are permitted. On the 3rd failed attempt or upon `REFUSED_BY_RECIPIENT`, the parcel transitions to terminal `RETURNED`.
4. **Required Failure Classification:** Every failed attempt MUST record one of the 7 approved ADR-004 reason codes. Reason `OTHER` requires a detailed note of at least 15 characters.

---

## 3. Layer Breakdown
```
internal/delivery/
├── API.md                   # Living REST API Specifications & Request/Response Contracts
├── domain/                  # Pure Domain Logic (Zero external dependencies)
│   ├── delivery_attempt.go      # DeliveryAttempt entity, IsTerminal(), outcome validation
│   ├── delivery_attempt_test.go # Unit tests for attempt ceiling, outcomes & terminal flags
│   ├── proof_of_delivery.go     # ProofOfDelivery value object, photo/signature/confirmation rules
│   ├── proof_of_delivery_test.go# Unit tests for proof validation & stroke data lengths
│   ├── failure_reason.go        # FailureReasonCode enum (7 ADR-004 codes) & note length checks
│   ├── failure_reason_test.go   # Unit tests for reason validation & immediate return flags
│   ├── geo_coordinate.go        # GeoCoordinate value object (-90..90, -180..180 bounds)
│   ├── geo_coordinate_test.go   # Unit tests for GPS coordinate boundaries
│   └── errors.go                # Sentinel domain errors (ErrMissingDeliveryProof, etc.)
├── application/             # Use Cases, Command Handlers & Ports
│   ├── collaborators.go         # TransactionManager, ParcelLifecycleUpdater, OutboxWriter, StorageClient
│   ├── repository.go            # DeliveryAttemptRepository, ActiveAssignmentFinder, CourierManifestReader
│   ├── commands.go              # Inbound command DTOs and result envelopes
│   ├── start_delivery_run.go    # StartDeliveryRunUseCase (Single & batch run start from depot)
│   ├── complete_delivery.go     # CompleteDeliveryUseCase (POD verification & IDOR protection AT-006)
│   ├── record_failed_attempt.go # RecordFailedAttemptUseCase (ADR-004 3-attempt ceiling & terminal return)
│   ├── get_courier_manifest.go  # GetCourierManifestQueryHandler (Operational data minimization)
│   ├── get_delivery_history.go  # GetDeliveryHistoryQueryHandler & UploadProofPhotoUseCase
│   └── application_test.go      # In-memory mock tests for IDOR defense, ADR-004 & proof completion
├── infrastructure/          # Persistence Drivers & Object Storage Adapters
│   ├── postgres_delivery_repo.go      # PostgresDeliveryRepository (delivery_attempts driver)
│   ├── postgres_assignment_finder.go  # PostgresAssignmentFinder (Active courier verification & closing)
│   ├── postgres_manifest_reader.go    # PostgresManifestReader (Optimized manifest query with data minimization)
│   ├── local_storage_client.go        # LocalStorageClient (Local filesystem storage for dev/CI)
│   ├── s3_storage_client.go           # S3StorageClient (Cloudflare R2 / AWS S3 client with SigV4 presigning)
│   ├── storage_factory.go             # Storage driver factory reading OBJECT_STORAGE_DRIVER
│   ├── local_storage_client_test.go   # Unit tests for local file storage & path traversal guards
│   ├── s3_storage_client_test.go      # Unit tests for SigV4 presigned URL generation
│   ├── storage_factory_test.go        # Unit tests for storage environment variable switching
│   └── postgres_infra_test.go         # Infrastructure constructors & smoke tests
└── transport/               # HTTP REST Handlers, Routing & Error Mappers
    ├── doc.go               # Transport package overview
    ├── requests.go          # Inbound request DTOs with validation rules
    ├── responses.go         # Outbound single-resource JSON response envelopes
    ├── error_mapper.go      # RFC 7807 / SwiftDrop canonical error status mapper
    ├── delivery_handlers.go # Handlers for POD photo uploads, completion, failed attempts & history
    ├── manifest_handlers.go # Handlers for daily courier manifests and delivery run commencement
    ├── routes.go            # ServeMux route registration with RBAC middleware
    └── transport_test.go    # Unit tests with httptest covering AT-006 IDOR, validation & completion
```

---

## 4. Database Ownership
- `delivery_attempts`: Attempt ID, parcel ID, courier ID, attempt number (1-3), outcome (`SUCCESS`, `FAILED`), failure reason code, proof type (`PHOTO`, `SIGNATURE`, `RECIPIENT_CONFIRMATION`), proof attachment URL / storage path, signature data, received by name, GPS coordinates, notes, attempted_at timestamp. Migrated via `20261110000005_create_dispatch_and_delivery.up.sql` (Canonical Sequence 000005).

---

## 5. Events Emitted
- `parcel.delivered.v1`: Published upon successful delivery with proof.
- `parcel.delivery_failed.v1`: Published upon failed attempt.
- `parcel.returned.v1`: Published upon 3rd failed attempt or refusal.

---

## 6. System Wiring & Dependency Injection
All delivery components are centrally registered in `core-api/cmd/api/wiring_dispatch_delivery.go` via `registerDispatchAndDeliveryModules(mux, dbConn, security)`.
- **Authoritative Database Persistence:** Concrete PostgreSQL persistence drivers and cloud/local object storage clients are instantiated to uphold ACID invariants.
- **Fail-Fast Offline Invariant:** When the database connection is unavailable (`dbConn == nil`), delivery operations fail immediately with `503 Service Unavailable` (`DATABASE_UNAVAILABLE`) rather than writing to in-memory state.
- **Agreement A2 Preserved:** Object storage is strictly excluded from Kubernetes readiness probes (`/health/ready`).

---

## 7. Acceptance Criteria & Concurrency Verification
- **AT-006 Courier IDOR Protection Test (`tests/at_006_courier_idor_test.go`):** Proves that only the legitimately assigned courier can perform delivery operations on a parcel; foreign couriers attempting `start-run`, `proof-photo`, `complete`, or `failed` are barred with `403 Forbidden` (`COURIER_NOT_ASSIGNED`).
- **ADR-004 Delivery Attempt Ceiling Test (`tests/adr_004_delivery_ceiling_test.go`):** Proves that sequential failed attempts 1 and 2 transition to `ATTEMPT_FAILED`, while attempt 3 transitions to terminal `RETURNED` and closes the assignment; any 4th attempt is strictly barred; parcels refused by recipient transition immediately to `RETURNED`.
- **Transaction Atomicity Test (`tests/delivery_atomicity_test.go`):** Proves that failures during downstream event publishing or persistence cause an atomic transaction rollback.

---

## 8. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Standardized single delivery_attempts table DDL, decoupled storage health checks, and ADR-004 failure reason codes | Member 3 & Review Agent |
| 2026-11-02 | Implemented Phase 1 DDL migration `20261021000007_create_dispatch_and_delivery.up.sql` with symmetrical down rollback, ADR-004 check constraints, outcome consistency guards, and signature recipient validation | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 2 pure domain layer: DeliveryAttempt entity, ProofOfDelivery VO, FailureReasonCode enum, GeoCoordinate VO, domain errors, and in-memory unit tests | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 3 application layer: StartDeliveryRunUseCase, CompleteDeliveryUseCase (AT-006 IDOR), RecordFailedAttemptUseCase (ADR-004), GetCourierManifestQueryHandler, GetDeliveryHistoryQueryHandler, UploadProofPhotoUseCase, collaborator ports, and mock unit tests | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 4 infrastructure persistence & storage layer: PostgresDeliveryRepository, PostgresAssignmentFinder, PostgresManifestReader, LocalStorageClient (with directory traversal guards), S3StorageClient (SigV4 presigning), and StorageClient factory | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 5 transport layer: DeliveryHandler, ManifestHandler, ErrorMapper, Request/Response DTOs, Route registration with RBAC middleware, co-located living API.md, and httptest unit tests | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 6 system wiring: registered in cmd/api/wiring_dispatch_delivery.go, hooked into BuildAppContainer(), wired fallback mocks for local dev, and added container tests | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 7 acceptance tests: AT-006 Courier IDOR defense, ADR-004 3-attempt ceiling and immediate return on refusal, and transaction atomicity verification | Member 3 (Antigravity) |
| 2026-11-03 | Phase 8 living documentation alignment: zero-drift contract verification against API.md, synchronized tests/README.md, and finalized roadmap sign-off | Member 3 (Antigravity) |
| 2026-11-04 | Comprehensive test gap analysis & hardening: Added BVA tests for GeoCoordinates (-90..90, -180..180 boundaries), DeliveryAttempt entity cross-field consistency, Signature stroke validation, Manifest batch run limits (1..100) & malformed JSON checks, and manifest data minimization assertions (redaction of wholesale pricing and customer financial PII). | Member 3 (Antigravity) |
| 2026-11-05 | Hardened storage factory environment resolution: Added Cloudflare R2 automatic endpoint resolution via Account ID, dual support for OBJECT_STORAGE_* and S3_* aliases, and updated .env and documentation. | Member 3 (Antigravity) |
| 2026-11-10 | Resequenced delivery migration to 20261110000004_create_dispatch_and_delivery, removed stub parcels table to sequence cleanly after Member 2 parcels | Member 3 (Antigravity) |
