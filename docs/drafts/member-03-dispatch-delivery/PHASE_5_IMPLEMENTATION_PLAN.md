# Phase 5 Implementation Plan: Transport Layer & HTTP Handlers

> **Module:** Dispatch & Delivery (`internal/dispatch/transport/` & `internal/delivery/transport/`)  
> **Author:** Member 3 — Dispatch, Couriers & Proof of Delivery  
> **Governing Standards:** SRS v1.0 (§4.6, §4.7), ADR-004, `AGENTS.md` (§6, §8, §9), `api-conventions.md`  
> **Status:** Pending Review & Execution

---

## 1. Executive Summary & Architectural Scope

Phase 5 implements the **Transport Layer** (HTTP Handlers, Request/Response DTOs, Input Validation, and Error Mappers) for Member 3's domains:
1. **Fleet & Dispatch API (`internal/dispatch/transport/`):**
   - Courier fleet onboarding and zone filtering
   - Live dispatch board overview (unassigned parcels and courier workloads)
   - Concurrency-safe parcel assignment and assignment cancellation (AT-013)
   - Dispatcher-supervised delivery reattempt scheduling (ADR-004)
2. **Delivery Execution & POD API (`internal/delivery/transport/`):**
   - Courier shift status toggling (self-service vs administrative)
   - Courier daily manifest retrieval with strict operational data minimization
   - Delivery run commencement (individual and batch start-run)
   - Proof of Delivery (POD) photo binary/multipart upload
   - Doorstep delivery completion with mandatory proof verification and AT-006 IDOR defense
   - Failed delivery attempt recording with ADR-004 reason codes and 3-attempt ceiling
   - Historical delivery attempt audit logs with presigned proof URLs

---

## 2. API Route Specifications & RBAC Matrix

All endpoints are registered on standard Go 1.22 `http.ServeMux` using enhanced route patterns with HTTP method guards and `{id}` path parameter extraction.

### 2.1 Dispatch & Courier Operations (`/api/v1/dispatch/*`, `/api/v1/couriers/*`)

| Method & Route Pattern | Auth Role | Description | Rate Limit |
| :--- | :--- | :--- | :--- |
| `POST /api/v1/dispatch/couriers` | `ADMIN`, `DISPATCHER` | Register courier fleet profile. | 60/min |
| `GET /api/v1/dispatch/couriers` | `DISPATCHER`, `ADMIN` | List/filter couriers by `zone_id`. | 120/min |
| `GET /api/v1/dispatch/board` | `DISPATCHER`, `ADMIN` | Live dispatch board (unassigned parcels & courier loads). | 120/min |
| `POST /api/v1/dispatch/assignments` | `DISPATCHER`, `ADMIN` | Assign parcel to courier with concurrency serialization. | 120/min |
| `POST /api/v1/dispatch/assignments/{id}/cancel` | `DISPATCHER`, `ADMIN` | Cancel active assignment and restore courier capacity. | 60/min |
| `POST /api/v1/dispatch/parcels/{id}/reschedule` | `DISPATCHER`, `ADMIN` | Authorize reattempt for failed parcel to assigned courier. | 60/min |

### 2.2 Doorstep Delivery & Courier Handover (`/api/v1/courier/*`, `/api/v1/deliveries/*`)

| Method & Route Pattern | Auth Role | Description | Rate Limit |
| :--- | :--- | :--- | :--- |
| `POST /api/v1/courier/shift` | `COURIER`, `DISPATCHER`, `ADMIN` | Toggle shift status (`ON_SHIFT`, `OFF_SHIFT`, `ON_BREAK`). | 30/min |
| `GET /api/v1/courier/parcels` | `COURIER` | Active delivery manifest (strictly scoped to `auth.UserID`). | 120/min |
| `POST /api/v1/courier/parcels/{id}/start-run` | `COURIER` | Mark parcel `OUT_FOR_DELIVERY` from depot. | 60/min |
| `POST /api/v1/courier/runs/start-batch` | `COURIER` | Batch start delivery run for courier manifest. | 30/min |
| `POST /api/v1/deliveries/{id}/proof-photo` | `COURIER` | Upload POD photo (multipart/form-data) to object storage. | 30/min |
| `POST /api/v1/deliveries/{id}/complete` | `COURIER` | Complete delivery with verified POD (`DELIVERED`). | 60/min |
| `POST /api/v1/deliveries/{id}/failed` | `COURIER` | Record failed attempt & reason code (`ATTEMPT_FAILED`/`RETURNED`). | 60/min |
| `GET /api/v1/deliveries/parcels/{id}` | `DISPATCHER`, `ADMIN`, `COURIER` | Retrieve delivery attempt logs and proof URLs. | 120/min |

---

## 3. Data Transfer Objects (DTOs) & JSON Payloads

### 3.1 Dispatch Request & Response Payloads

#### `POST /api/v1/dispatch/couriers`
- **Request Body:**
  ```json
  {
    "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "employee_code": "CR-7001",
    "vehicle_type": "MOTORCYCLE",
    "max_capacity": 25,
    "current_zone_id": "ZONE_COLOMBO_01"
  }
  ```
- **Response (`201 Created`):**
  ```json
  {
    "data": {
      "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "employee_code": "CR-7001",
      "vehicle_type": "MOTORCYCLE",
      "max_capacity": 25,
      "status": "OFF_SHIFT",
      "current_zone_id": "ZONE_COLOMBO_01",
      "is_active": true,
      "created_at": "2026-11-02T10:00:00Z"
    }
  }
  ```

#### `POST /api/v1/dispatch/assignments`
- **Headers:** `Idempotency-Key: <uuid>` (optional)
- **Request Body:**
  ```json
  {
    "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
    "courier_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "notes": "Fragile electronics delivery"
  }
  ```
- **Response (`201 Created`):**
  ```json
  {
    "data": {
      "assignment_id": "1fa85f64-5717-4562-b3fc-2c963f66afa1",
      "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
      "courier_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "assigned_by": "0fa85f64-5717-4562-b3fc-2c963f66afa0",
      "status": "ACTIVE",
      "assigned_at": "2026-11-02T10:15:00Z"
    }
  }
  ```

### 3.2 Delivery Request & Response Payloads

#### `POST /api/v1/deliveries/{id}/proof-photo`
- **Headers:** `Content-Type: multipart/form-data`
- **Form Part:** `file` (JPEG/PNG, $\le 5$ MB)
- **Response (`201 Created`):**
  ```json
  {
    "data": {
      "photo_url": "/api/v1/deliveries/proofs/proofs/2026/11/02/7fa85f64.jpg",
      "storage_key": "proofs/2026/11/02/7fa85f64.jpg",
      "content_type": "image/jpeg",
      "uploaded_at": "2026-11-02T14:30:00Z"
    }
  }
  ```

#### `POST /api/v1/deliveries/{id}/complete`
- **Request Body:**
  ```json
  {
    "proof_type": "PHOTO",
    "proof_url": "https://cdn.swiftdrop.io/proofs/2026/11/02/7fa85f64.jpg",
    "received_by_name": "Kasun Perera",
    "latitude": 6.927079,
    "longitude": 79.861244,
    "notes": "Handed to recipient in person at front gate"
  }
  ```
- **Response (`200 OK`):**
  ```json
  {
    "data": {
      "attempt_id": "8fa85f64-5717-4562-b3fc-2c963f66afa8",
      "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
      "status": "DELIVERED",
      "outcome": "SUCCESS",
      "completed_at": "2026-11-02T14:32:00Z"
    }
  }
  ```

#### `POST /api/v1/deliveries/{id}/failed`
- **Request Body:**
  ```json
  {
    "failure_reason_code": "RECIPIENT_UNAVAILABLE",
    "notes": "Called customer twice; gate locked with no answer.",
    "latitude": 6.927079,
    "longitude": 79.861244
  }
  ```
- **Response (`200 OK`):**
  ```json
  {
    "data": {
      "attempt_id": "9fa85f64-5717-4562-b3fc-2c963f66afa9",
      "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
      "attempt_number": 2,
      "max_attempts": 3,
      "outcome": "FAILED",
      "failure_reason_code": "RECIPIENT_UNAVAILABLE",
      "parcel_status": "ATTEMPT_FAILED",
      "is_terminal": false,
      "attempted_at": "2026-11-02T14:35:00Z"
    }
  }
  ```

---

## 4. Error Mapping Matrix (RFC 7807 / SwiftDrop Canonical Envelopes)

All HTTP handlers map domain, application, and persistence errors to standard `httpx.WriteError` payloads:

| Domain / Application Error | HTTP Status | Error Code | Client Message |
| :--- | :---: | :--- | :--- |
| `domain.ErrNotAssignedCourier` (AT-006) | `403 Forbidden` | `COURIER_NOT_ASSIGNED` | Courier is not authorized to deliver this parcel. |
| `domain.ErrCourierOverloaded` | `409 Conflict` | `COURIER_CAPACITY_EXCEEDED` | Courier has reached maximum active package capacity. |
| `domain.ErrCourierNotOnShift` | `409 Conflict` | `COURIER_NOT_ON_SHIFT` | Courier must be ON_SHIFT to accept delivery tasks. |
| `domain.ErrInvalidShiftTransition` | `400 Bad Request` | `INVALID_SHIFT_TRANSITION` | Requested shift status transition is not permitted. |
| `application.ErrDuplicateActiveAssignment` (AT-013)| `409 Conflict` | `PARCEL_ALREADY_ASSIGNED` | Parcel currently has another active courier assignment. |
| `application.ErrInvalidParcelStatusForAssignment` | `409 Conflict` | `INVALID_PARCEL_STATE` | Parcel must be in CREATED or ATTEMPT_FAILED status. |
| `application.ErrParcelNotOutForDelivery` | `409 Conflict` | `PARCEL_NOT_OUT_FOR_DELIVERY` | Parcel must be in OUT_FOR_DELIVERY status to complete. |
| `domain.ErrMissingDeliveryProof` | `422 Unprocessable` | `DELIVERY_PROOF_REQUIRED` | Valid proof (photo, signature, or confirmation) is required. |
| `domain.ErrInvalidProofRecipient` | `422 Unprocessable` | `INVALID_RECIPIENT_NAME` | Recipient name must be between 2 and 150 characters. |
| `domain.ErrInvalidSignatureData` | `422 Unprocessable` | `INVALID_SIGNATURE_DATA` | Signature stroke data must contain at least 50 characters. |
| `domain.ErrInvalidFailureReason` | `400 Bad Request` | `INVALID_FAILURE_REASON` | Invalid ADR-004 failure reason code provided. |
| `domain.ErrDetailedNoteRequired` | `422 Unprocessable` | `DETAILED_NOTE_REQUIRED` | Reason code OTHER requires a detailed note of >= 15 characters. |
| `application.ErrCourierNotFound` | `404 Not Found` | `COURIER_NOT_FOUND` | Courier profile does not exist. |
| `application.ErrAssignmentNotFound` | `404 Not Found` | `ASSIGNMENT_NOT_FOUND` | Active courier assignment not found. |
| `application.ErrParcelNotAssigned` | `404 Not Found` | `PARCEL_NOT_ASSIGNED` | No active courier assignment found for target parcel. |

---

## 5. Directory & File Implementation Structure

```
core-api/internal/
├── dispatch/
│   ├── README.md               # Living Module Documentation (Update synchronously)
│   ├── API.md                  # Complete OpenAPI/REST API Contract (New)
│   └── transport/              # New Transport Subpackage
│       ├── doc.go              # Transport package documentation
│       ├── requests.go         # Inbound JSON request structures & input validators
│       ├── responses.go        # Outbound JSON response DTO models
│       ├── dispatch_handlers.go# HTTP handlers for /api/v1/dispatch/*
│       ├── courier_handlers.go # HTTP handlers for /api/v1/courier/* (shift toggling)
│       ├── error_mapper.go     # Error to HTTP status & code mapping helper
│       ├── routes.go           # Route registration onto http.ServeMux
│       └── transport_test.go   # HTTP unit & integration tests with httptest.ResponseRecorder
│
└── delivery/
    ├── README.md               # Living Module Documentation (Update synchronously)
    ├── API.md                  # Complete OpenAPI/REST API Contract (New)
    └── transport/              # New Transport Subpackage
        ├── doc.go              # Transport package documentation
        ├── requests.go         # Inbound JSON request structures & input validators
        ├── responses.go        # Outbound JSON response DTO models
        ├── delivery_handlers.go# HTTP handlers for /api/v1/deliveries/* & proof upload
        ├── manifest_handlers.go# HTTP handlers for /api/v1/courier/parcels & runs
        ├── error_mapper.go     # Error to HTTP status & code mapping helper
        ├── routes.go           # Route registration onto http.ServeMux
        └── transport_test.go   # HTTP unit & integration tests with httptest.ResponseRecorder
```

---

## 6. Implementation Steps Checklist

- [ ] **Step 1:** Create `core-api/internal/dispatch/transport/doc.go`, `requests.go`, and `responses.go`.
- [ ] **Step 2:** Implement `core-api/internal/dispatch/transport/error_mapper.go` for RFC 7807 / `httpx.WriteError` mapping.
- [ ] **Step 3:** Implement `core-api/internal/dispatch/transport/dispatch_handlers.go` (`RegisterCourier`, `ListCouriers`, `GetBoard`, `AssignCourier`, `CancelAssignment`, `RescheduleParcel`).
- [ ] **Step 4:** Implement `core-api/internal/dispatch/transport/courier_handlers.go` (`ToggleShift`).
- [ ] **Step 5:** Implement `core-api/internal/dispatch/transport/routes.go` registering all dispatch routes with RBAC middleware.
- [ ] **Step 6:** Implement `core-api/internal/delivery/transport/doc.go`, `requests.go`, and `responses.go`.
- [ ] **Step 7:** Implement `core-api/internal/delivery/transport/error_mapper.go` for delivery domain errors.
- [ ] **Step 8:** Implement `core-api/internal/delivery/transport/delivery_handlers.go` (`UploadProofPhoto`, `CompleteDelivery`, `RecordFailedAttempt`, `GetDeliveryHistory`).
- [ ] **Step 9:** Implement `core-api/internal/delivery/transport/manifest_handlers.go` (`GetManifest`, `StartRun`, `StartBatchRun`).
- [ ] **Step 10:** Implement `core-api/internal/delivery/transport/routes.go` registering all delivery routes with RBAC middleware.
- [ ] **Step 11:** Implement comprehensive transport unit tests using `net/http/httptest` covering:
  - Happy path HTTP 200/201 responses.
  - Validation failures (HTTP 400/422).
  - RBAC unauthorized & forbidden cases (HTTP 401/403).
  - Courier IDOR protection (HTTP 403).
  - Capacity conflict and state transitions (HTTP 409).
- [ ] **Step 12:** Author co-located living API specifications:
  - `core-api/internal/dispatch/API.md`
  - `core-api/internal/delivery/API.md`
- [ ] **Step 13:** Update co-located `README.md` in `internal/dispatch/` and `internal/delivery/`.
- [ ] **Step 14:** Deliver Task Completion Report per `AGENTS.md` §12.

---

## 7. Definition of Done (DoD) for Phase 5

Phase 5 is complete and approved for advancement to **Phase 6 (System Wiring & Dependency Injection)** only when:
1. All 14 endpoints listed in Section 2 are implemented with input validation and HTTP status code mappings.
2. Standard response wrappers (`{"data": ...}`) and structured error envelopes per `api-conventions.md` are strictly utilized.
3. Courier IDOR defense (AT-006) and assignment serialization (AT-013) return proper 403 and 409 status codes.
4. Co-located living API specifications (`API.md`) are created for both modules with realistic JSON payloads.
5. Co-located module `README.md` files are updated to reflect the transport layer.
6. All tests pass with zero compiler or linter errors.

