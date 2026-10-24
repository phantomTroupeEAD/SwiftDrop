# Document 4 — Interface & Integration Contract Specification

> **Module:** Dispatch, Couriers & Proof of Delivery  
> **Author / Owner:** Member 03  
> **Status:** Draft / Baseline (For Review in Team Meeting 2)  
> **Target Production Code Paths:** `core-api/internal/dispatch/` and `core-api/internal/delivery/`  
> **Clean Architecture Boundary:** Modular Monolith Internal Contexts  
> **Key Upstream Dependencies:** Member 1 (Auth & RBAC), Member 2 (Parcel Lifecycle & Schema), Member 8 (Transactional Outbox Interface)  
> **Key Downstream Consumers:** Member 5 (Tracking API), Member 7 (Dispatcher & Courier Frontend Web UI), Member 8 (Messaging & Notification Worker)

---

## 1. Architectural Interface Overview

This document specifies the complete, binding external and internal interface contracts for Member 03:
1. **Synchronous REST / JSON HTTP Endpoints:** Exposing operational capabilities for Dispatchers, Administrators, and Couriers.
2. **Asynchronous Outbox Event Schemas:** Versioned event contracts published to `outbox_events` and consumed via RabbitMQ by Member 5 (Tracking API) and Member 8 (Notification & Webhook Workers).
3. **Internal Modular Monolith Interface Contracts:** Go interfaces governing cross-context communication with Member 2 (Parcels) and Member 8 (Transactional Outbox).

### 1.1 Global API Conventions
- **Base URI:** `/api/v1`
- **Transport Security:** TLS 1.3 enforced at Traefik API Gateway.
- **Authentication:** Bearer JWT required in the `Authorization: Bearer <token>` header for all endpoints.
- **Content-Type:** `application/json` (except photo uploads which use `multipart/form-data`).
- **Standard Date/Time:** ISO 8601 in UTC (`YYYY-MM-DDTHH:MM:SSZ`).
- **Standard Error Format:** RFC 7807 problem details JSON:
  ```json
  {
    "error": {
      "code": "COURIER_CAPACITY_EXCEEDED",
      "message": "Assigned courier has reached maximum capacity of 25 active parcels.",
      "request_id": "req_01HJ99223344",
      "timestamp": "2026-10-24T14:32:00Z"
    }
  }
  ```

---

## 2. HTTP Endpoint Inventory

| Method & Path | Primary Capability | Authorized Roles | Idempotent |
| :--- | :--- | :--- | :---: |
| `POST /api/v1/dispatch/couriers` | Provision courier fleet profile | `ADMIN`, `DISPATCHER` | No |
| `GET /api/v1/dispatch/couriers` | List & filter couriers across zones | `DISPATCHER`, `ADMIN` | Yes |
| `GET /api/v1/dispatch/couriers/{id}` | Get individual courier profile & load | `DISPATCHER`, `ADMIN` | Yes |
| `GET /api/v1/dispatch/board` | Query unassigned parcels & courier loads | `DISPATCHER`, `ADMIN` | Yes |
| `POST /api/v1/dispatch/assignments` | Assign parcel to active courier | `DISPATCHER`, `ADMIN` | Yes (`Idempotency-Key`) |
| `POST /api/v1/dispatch/assignments/{id}/cancel` | Cancel assignment / unassign driver | `DISPATCHER`, `ADMIN` | Yes |
| `POST /api/v1/dispatch/parcels/{id}/reschedule` | Dispatcher authorizes reattempt | `DISPATCHER`, `ADMIN` | Yes |
| `POST /api/v1/courier/shift` | Courier toggles shift status | `COURIER` | No |
| `GET /api/v1/courier/parcels` | Courier daily active manifest | `COURIER` | Yes |
| `POST /api/v1/courier/parcels/{id}/start-run` | Confirm pickup & start delivery run (`OUT_FOR_DELIVERY`) | `COURIER` | Yes |
| `POST /api/v1/courier/runs/start-batch` | Batch pickup & start run for multiple parcels | `COURIER` | Yes |
| `POST /api/v1/deliveries/{id}/proof-photo` | Upload POD photo to Cloudflare R2 | `COURIER` | No |
| `POST /api/v1/deliveries/{id}/complete` | Complete delivery with verified POD | `COURIER` | Yes (`Idempotency-Key`) |
| `POST /api/v1/deliveries/{id}/failed` | Record failed attempt & reason code | `COURIER` | Yes |
| `GET /api/v1/deliveries/parcels/{id}` | Get delivery attempt history & proof | `DISPATCHER`, `ADMIN`, `COURIER` | Yes |

---

## 3. Synchronous REST Endpoint Contracts

### 3.1 Fleet Management: Provision Courier Profile
- **Method / Path:** `POST /api/v1/dispatch/couriers`
- **Auth Required:** Bearer JWT (`ADMIN`, `DISPATCHER`)
- **Headers:** `Authorization: Bearer <jwt>`, `Content-Type: application/json`

#### Request Body
```json
{
  "user_id": "usr_99887766-aaaa-bbbb-cccc-112233445566",
  "employee_code": "CR-0042",
  "vehicle_type": "MOTORCYCLE",
  "max_capacity": 25,
  "zone_code": "ZONE_COLOMBO_METRO"
}
```

#### Success Response (`201 Created`)
```json
{
  "data": {
    "user_id": "usr_99887766-aaaa-bbbb-cccc-112233445566",
    "employee_code": "CR-0042",
    "vehicle_type": "MOTORCYCLE",
    "max_capacity": 25,
    "status": "OFF_SHIFT",
    "zone_code": "ZONE_COLOMBO_METRO",
    "is_active": true,
    "created_at": "2026-10-24T08:00:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request`: `INVALID_VEHICLE_TYPE` — Vehicle type must be `MOTORCYCLE`, `VAN`, or `TRUCK`.
- `409 Conflict`: `COURIER_PROFILE_EXISTS` — Operational profile already exists for this user ID.

---

### 3.2 List & Filter Couriers Across Zones
- **Method / Path:** `GET /api/v1/dispatch/couriers`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Query Parameters:**
  - `zone_code` (string, optional): Filter by `ZONE_COLOMBO_METRO`, `ZONE_GREATER_WESTERN`, `ZONE_OUTSTATION`.
  - `status` (string, optional): Filter by `ON_SHIFT`, `OFF_SHIFT`, `ON_BREAK`, `SUSPENDED`.
  - `vehicle_type` (string, optional): Filter by `MOTORCYCLE`, `VAN`, `TRUCK`.

#### Success Response (`200 OK`)
```json
{
  "data": {
    "total_couriers": 2,
    "couriers": [
      {
        "user_id": "usr_courier_bob",
        "employee_code": "CR-0012",
        "name": "Bob Miller",
        "phone_number": "+94771112233",
        "vehicle_type": "MOTORCYCLE",
        "max_capacity": 25,
        "status": "ON_SHIFT",
        "zone_code": "ZONE_COLOMBO_METRO",
        "active_assignments": 3,
        "is_active": true
      },
      {
        "user_id": "usr_courier_alice",
        "employee_code": "CR-0015",
        "name": "Alice Silva",
        "phone_number": "+94774445566",
        "vehicle_type": "VAN",
        "max_capacity": 60,
        "status": "ON_SHIFT",
        "zone_code": "ZONE_GREATER_WESTERN",
        "active_assignments": 1,
        "is_active": true
      }
    ]
  }
}
```

---

### 3.3 Get Individual Courier Profile & Workload
- **Method / Path:** `GET /api/v1/dispatch/couriers/{id}`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)

#### Request Parameters (Path)
- `id` (string, required): User UUID of the courier (`couriers.user_id`).

#### Success Response (`200 OK`)
```json
{
  "data": {
    "user_id": "usr_courier_bob",
    "employee_code": "CR-0012",
    "name": "Bob Miller",
    "phone_number": "+94771112233",
    "vehicle_type": "MOTORCYCLE",
    "max_capacity": 25,
    "status": "ON_SHIFT",
    "zone_code": "ZONE_COLOMBO_METRO",
    "active_assignments": 3,
    "capacity_available": 22,
    "is_active": true,
    "created_at": "2026-10-21T08:00:00Z"
  }
}
```

#### Error Responses
- `404 Not Found`: `COURIER_NOT_FOUND` — Specified courier profile does not exist.

---

### 3.4 Dispatch Board: Query Unassigned Parcels & Courier Workloads
- **Method / Path:** `GET /api/v1/dispatch/board`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Query Parameters:**
  - `zone_code` (string, required): Canonical zone identifier (`ZONE_COLOMBO_METRO`, `ZONE_GREATER_WESTERN`, `ZONE_OUTSTATION`).
  - `status` (string, optional, default: `ELIGIBLE`): Filter by `CREATED` or `ATTEMPT_FAILED`.

#### Success Response (`200 OK`)
```json
{
  "data": {
    "zone_code": "ZONE_COLOMBO_METRO",
    "summary": {
      "total_unassigned_parcels": 2,
      "total_active_couriers": 2,
      "total_fleet_capacity": 85,
      "utilized_fleet_capacity": 4
    },
    "unassigned_parcels": [
      {
        "id": "prcl_01HXYZ1234567890",
        "tracking_number": "SD-20261024-04921",
        "current_status": "CREATED",
        "destination_zone": "ZONE_COLOMBO_METRO",
        "estimated_transit_days": 1,
        "weight_kg": 2.50,
        "recipient_city": "Colombo 03",
        "created_at": "2026-10-24T09:00:00Z"
      },
      {
        "id": "prcl_01HXYZ9988776655",
        "tracking_number": "SD-20261024-09812",
        "current_status": "ATTEMPT_FAILED",
        "destination_zone": "ZONE_COLOMBO_METRO",
        "estimated_transit_days": 1,
        "attempt_count": 1,
        "weight_kg": 4.10,
        "recipient_city": "Colombo 03",
        "created_at": "2026-10-23T14:00:00Z"
      }
    ],
    "active_couriers": [
      {
        "courier_id": "usr_courier_bob",
        "name": "Bob Miller",
        "employee_code": "CR-0012",
        "vehicle_type": "MOTORCYCLE",
        "active_assignments": 3,
        "max_capacity": 25,
        "shift_status": "ON_SHIFT",
        "capacity_available": 22
      },
      {
        "courier_id": "usr_courier_alice",
        "name": "Alice Silva",
        "employee_code": "CR-0015",
        "vehicle_type": "VAN",
        "active_assignments": 1,
        "max_capacity": 60,
        "shift_status": "ON_SHIFT",
        "capacity_available": 59
      }
    ]
  }
}
```

---

### 3.5 Parcel Assignment: Assign Courier to Parcel
- **Method / Path:** `POST /api/v1/dispatch/assignments`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Headers:** `Authorization: Bearer <jwt>`, `Idempotency-Key: <uuid>`

#### Request Body
```json
{
  "parcel_id": "prcl_01HXYZ1234567890",
  "courier_id": "usr_courier_bob"
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "assignment_id": "asgn_01HJ0987654321",
    "parcel_id": "prcl_01HXYZ1234567890",
    "courier_id": "usr_courier_bob",
    "status": "ASSIGNED",
    "assigned_by": "usr_dispatcher_dan",
    "assigned_at": "2026-10-24T09:30:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request`: `PARCEL_NOT_ELIGIBLE_FOR_ASSIGNMENT` — Parcel is already in transit or terminal state.
- `400 Bad Request`: `COURIER_NOT_ON_SHIFT` — Selected courier is currently `OFF_SHIFT` or `ON_BREAK`.
- `409 Conflict`: `COURIER_CAPACITY_EXCEEDED` — Courier active parcel workload equals or exceeds vehicle capacity.
- `409 Conflict`: `PARCEL_ALREADY_ASSIGNED` — Parcel was assigned concurrently by another dispatcher.

---

### 3.6 Cancel Assignment / Unassign Courier
- **Method / Path:** `POST /api/v1/dispatch/assignments/{id}/cancel`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Headers:** `Authorization: Bearer <jwt>`

#### Request Body
```json
{
  "cancellation_reason": "Courier vehicle flat tire; reassigning to relief driver."
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "assignment_id": "asgn_01HJ0987654321",
    "parcel_id": "prcl_01HXYZ1234567890",
    "courier_id": "usr_courier_bob",
    "status": "CANCELLED",
    "ended_at": "2026-10-24T09:45:00Z",
    "notes": "Courier vehicle flat tire; reassigning to relief driver."
  }
}
```

#### Error Responses
- `404 Not Found`: `ASSIGNMENT_NOT_FOUND` — Assignment record does not exist.
- `409 Conflict`: `ASSIGNMENT_ALREADY_CLOSED` — Assignment is already completed or cancelled.

---

### 3.7 Dispatcher Reattempt Authorization
- **Method / Path:** `POST /api/v1/dispatch/parcels/{id}/reschedule`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)

#### Request Body
```json
{
  "target_courier_id": "usr_courier_bob",
  "scheduled_date": "2026-10-25",
  "preferred_time_window": "AFTERNOON_14_18",
  "dispatch_notes": "Customer confirmed home after 2:00 PM via support phone call."
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "parcel_id": "prcl_01HXYZ9988776655",
    "status": "ASSIGNED",
    "attempt_count": 1,
    "max_attempts": 3,
    "scheduled_date": "2026-10-25",
    "preferred_time_window": "AFTERNOON_14_18",
    "assigned_courier_id": "usr_courier_bob",
    "rescheduled_at": "2026-10-24T10:15:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request`: `MAX_ATTEMPTS_EXCEEDED` — Parcel has already failed 3 attempts and cannot be rescheduled.
- `409 Conflict`: `INVALID_STATE_FOR_RESCHEDULE` — Parcel is not currently in `ATTEMPT_FAILED` status.

---

### 3.8 Courier Shift: Self-Service Shift Toggle
- **Method / Path:** `POST /api/v1/courier/shift`
- **Auth Required:** Bearer JWT (`COURIER`)

#### Request Body
```json
{
  "status": "ON_SHIFT"
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "courier_id": "usr_courier_bob",
    "status": "ON_SHIFT",
    "max_capacity": 25,
    "active_assignments": 3,
    "updated_at": "2026-10-24T07:45:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request`: `INVALID_SHIFT_STATE` — Allowed values are `ON_SHIFT`, `OFF_SHIFT`, `ON_BREAK`.
- `403 Forbidden`: `COURIER_SUSPENDED` — Courier has been suspended by management and cannot toggle shift.

---

### 3.9 Courier Manifest: Daily Active Deliveries
- **Method / Path:** `GET /api/v1/courier/parcels`
- **Auth Required:** Bearer JWT (`COURIER`)
- **Data Minimization:** Financial pricing, billing margins, and merchant API keys are strictly redacted.

#### Request Parameters
- None. The query manifest is strictly and automatically scoped to the calling courier extracted from `Authorization: Bearer <jwt>`.

#### Success Response (`200 OK`)
```json
{
  "data": {
    "courier_id": "usr_courier_bob",
    "shift_status": "ON_SHIFT",
    "active_count": 1,
    "parcels": [
      {
        "parcel_id": "prcl_01HXYZ1234567890",
        "tracking_number": "SD-20261024-04921",
        "current_status": "OUT_FOR_DELIVERY",
        "destination_zone": "ZONE_COLOMBO_METRO",
        "recipient_name": "Jane Smith",
        "recipient_phone": "+94771234567",
        "delivery_street": "No 42, Galle Road",
        "delivery_city": "Colombo 03",
        "weight_kg": 2.50,
        "handling_instructions": "Fragile: glassware inside. Leave at door if gate locked.",
        "attempt_count": 0,
        "assigned_at": "2026-10-24T09:30:00Z"
      }
    ]
  }
}
```

---

### 3.10 Courier Run Start: Confirm Pickup & Begin Run (Single Parcel)
Consolidates package collection and delivery run start in a single operational step, moving the parcel directly from `ASSIGNED` to `OUT_FOR_DELIVERY`.

- **Method / Path:** `POST /api/v1/courier/parcels/{id}/start-run`
- **Auth Required:** Bearer JWT (`COURIER`)
- **Headers:** `Authorization: Bearer <jwt>`, `Content-Type: application/json`

#### Request Body
```json
{
  "depot_hub_code": "HUB_COLOMBO_CENTRAL",
  "notes": "Package collected from depot shelf; starting morning delivery run."
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "parcel_id": "prcl_01HXYZ1234567890",
    "tracking_number": "SD-20261024-04921",
    "status": "OUT_FOR_DELIVERY",
    "started_at": "2026-10-24T10:15:00Z"
  }
}
```

#### Error Responses
- `403 Forbidden`: `NOT_ASSIGNED_COURIER` — Caller is not the assigned driver for this parcel.
- `409 Conflict`: `INVALID_STATE_TRANSITION` — Parcel is not currently in `ASSIGNED` status.

---

### 3.11 Courier Batch Run Start: Begin Run for Multiple Parcels
Enables a courier to accept and start the delivery run for their entire assigned morning manifest with a single action.

- **Method / Path:** `POST /api/v1/courier/runs/start-batch`
- **Auth Required:** Bearer JWT (`COURIER`)
- **Headers:** `Authorization: Bearer <jwt>`, `Content-Type: application/json`

#### Request Body
```json
{
  "parcel_ids": [
    "prcl_01HXYZ1234567890",
    "prcl_01HXYZ9988776655"
  ],
  "depot_hub_code": "HUB_COLOMBO_CENTRAL"
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "total_started": 2,
    "started_at": "2026-10-24T10:15:00Z",
    "parcels": [
      {
        "parcel_id": "prcl_01HXYZ1234567890",
        "status": "OUT_FOR_DELIVERY"
      },
      {
        "parcel_id": "prcl_01HXYZ9988776655",
        "status": "OUT_FOR_DELIVERY"
      }
    ]
  }
}
```

#### Error Responses
- `400 Bad Request`: `EMPTY_PARCEL_LIST` — Request array contains zero parcel IDs.
- `403 Forbidden`: `NOT_ASSIGNED_COURIER` — One or more parcels are not assigned to caller.

---

### 3.12 Proof of Delivery Photo Upload (Cloudflare R2 / S3)
- **Method / Path:** `POST /api/v1/deliveries/{id}/proof-photo`
- **Auth Required:** Bearer JWT (`COURIER`)
- **Content-Type:** `multipart/form-data`

#### Multipart Form Fields
- `photo`: Binary image file (`image/jpeg`, `image/png`, `image/webp`, max size 10 MB).

#### Success Response (`200 OK`)
```json
{
  "data": {
    "parcel_id": "prcl_01HXYZ1234567890",
    "proof_attachment_url": "proofs/2026/10/prcl_01HXYZ1234567890_pod.jpg",
    "file_size_bytes": 1420850,
    "uploaded_at": "2026-10-24T11:14:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request`: `INVALID_FILE_TYPE_OR_SIZE` — Non-image MIME type or file exceeds 10 MB.
- `403 Forbidden`: `NOT_ASSIGNED_COURIER` — Caller is not the assigned driver for this parcel.

---

### 3.13 Delivery Completion: Complete Delivery with Proof
- **Method / Path:** `POST /api/v1/deliveries/{id}/complete`
- **Auth Required:** Bearer JWT (`COURIER`)
- **Headers:** `Authorization: Bearer <jwt>`, `Idempotency-Key: <uuid>`

#### Request Body
```json
{
  "proof_type": "SIGNATURE",
  "signature_data": "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDAiIGhlaWdodD0iMTAwIj48cGF0aCBkPSJNIDEwIDEwIEwgOTAgOTAgWiIgc3Ryb2tlPSJibGFjayIgZmlsbD0ibm9uZSIvPjwvc3ZnPg==",
  "received_by_name": "Jane Smith (Self)",
  "notes": "Handed directly to recipient at front entrance",
  "latitude": 6.9271,
  "longitude": 79.8612
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "delivery_attempt_id": "atm_01HK9988776655",
    "parcel_id": "prcl_01HXYZ1234567890",
    "status": "DELIVERED",
    "delivered_at": "2026-10-24T11:15:00Z",
    "proof_type": "SIGNATURE",
    "received_by_name": "Jane Smith (Self)"
  }
}
```

#### Error Responses
- `400 Bad Request`: `MISSING_DELIVERY_PROOF` — Proof payload missing or invalid.
- `403 Forbidden`: `NOT_ASSIGNED_COURIER` — Caller is not the courier assigned to this parcel.
- `409 Conflict`: `INVALID_STATE_FOR_DELIVERY` — Parcel is not in `OUT_FOR_DELIVERY` status.
- `422 Unprocessable Entity`: `RECIPIENT_NAME_REQUIRED` — Recipient name missing when `proof_type == 'SIGNATURE'`.

---

### 3.14 Delivery Attempt Failure: Log Failed Attempt
- **Method / Path:** `POST /api/v1/deliveries/{id}/failed`
- **Auth Required:** Bearer JWT (`COURIER`)

#### Request Body
```json
{
  "reason_code": "RECIPIENT_UNAVAILABLE",
  "notes": "Arrived at location at 11:30 AM. Called phone twice; no response at intercom.",
  "latitude": 6.9271,
  "longitude": 79.8612
}
```

#### Success Response (Branch A: Recoverable Attempt $< 3$, `200 OK`)
```json
{
  "data": {
    "attempt_id": "atm_01HK9988776699",
    "parcel_id": "prcl_01HXYZ1234567890",
    "attempt_number": 1,
    "max_attempts": 3,
    "status": "ATTEMPT_FAILED",
    "reason_code": "RECIPIENT_UNAVAILABLE",
    "next_action": "RETURN_TO_HUB_FOR_REVIEW",
    "recorded_at": "2026-10-24T11:35:00Z"
  }
}
```

#### Success Response (Branch B: Terminal 3rd Attempt or Refusal, `200 OK`)
```json
{
  "data": {
    "attempt_id": "atm_01HK9988776700",
    "parcel_id": "prcl_01HXYZ1234567890",
    "attempt_number": 3,
    "max_attempts": 3,
    "status": "RETURNED",
    "reason_code": "RECIPIENT_UNAVAILABLE",
    "next_action": "RETURN_TO_SENDER",
    "recorded_at": "2026-10-24T11:35:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request`: `INVALID_FAILURE_REASON` — Reason code not in approved whitelist.
- `422 Unprocessable Entity`: `FAILURE_NOTE_TOO_SHORT` — Reason is `OTHER` but note length $< 15$ characters.

---

### 3.15 Delivery History: Get Attempt History & Proof for Parcel
- **Method / Path:** `GET /api/v1/deliveries/parcels/{id}`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`, `COURIER`)
- **Headers:** `Authorization: Bearer <jwt>`

#### Request Parameters (Path)
- `id` (string, required): Canonical UUID of the parcel (`parcels.id`).

#### Success Response (`200 OK`)
```json
{
  "data": {
    "parcel_id": "prcl_01HXYZ1234567890",
    "tracking_number": "SD-20261024-04921",
    "current_status": "DELIVERED",
    "attempt_count": 2,
    "attempts": [
      {
        "attempt_id": "atm_01HK9988776699",
        "attempt_number": 1,
        "outcome": "FAILED",
        "reason_code": "RECIPIENT_UNAVAILABLE",
        "notes": "Arrived at location at 11:30 AM. Called phone twice; no response at intercom.",
        "courier_id": "usr_courier_bob",
        "attempted_at": "2026-10-24T11:35:00Z"
      },
      {
        "attempt_id": "atm_01HK9988776655",
        "attempt_number": 2,
        "outcome": "SUCCESS",
        "proof_type": "SIGNATURE",
        "proof_attachment_url": null,
        "received_by_name": "Jane Smith (Self)",
        "notes": "Handed directly to recipient at front entrance",
        "courier_id": "usr_courier_bob",
        "attempted_at": "2026-10-25T14:15:00Z"
      }
    ]
  }
}
```

#### Error Responses
- `404 Not Found`: `PARCEL_NOT_FOUND` — Parcel ID does not exist or has zero delivery attempts recorded.

---

## 4. Asynchronous Outbox Event Contracts

Member 03 commits events to the `outbox_events` table within the same transaction as state changes. The Outbox Relay polls and publishes these to RabbitMQ exchange `swiftdrop.events`.

### 4.1 Canonical Event Envelope
```json
{
  "event_id": "evt_01HK8877665544",
  "event_type": "parcel.delivered.v1",
  "schema_version": 1,
  "aggregate_type": "PARCEL",
  "aggregate_id": "prcl_01HXYZ1234567890",
  "occurred_at": "2026-10-24T11:15:00Z",
  "payload": {}
}
```

---

### 4.2 Event 1: `parcel.assigned.v1`
- **Topic Key:** `parcel.assigned.v1`
- **Trigger:** Dispatcher assigns parcel to an on-shift courier.
- **Consumers:** Member 5 (Tracking API updates assignment status), Member 8 (Notification Worker alerts courier).

#### Payload Schema
```json
{
  "parcel_id": "prcl_01HXYZ1234567890",
  "tracking_number": "SD-20261024-04921",
  "courier_id": "usr_courier_bob",
  "assigned_by": "usr_dispatcher_dan",
  "assigned_at": "2026-10-24T09:30:00Z"
}
```

---

### 4.3 Event 2: `courier.unassigned.v1`
- **Topic Key:** `courier.unassigned.v1`
- **Trigger:** Dispatcher or admin cancels an active assignment prior to run departure.
- **Consumers:** Member 5 (Tracking API removes courier assignment display), Member 8 (Notification Worker alerts courier of cancellation).

#### Payload Schema
```json
{
  "assignment_id": "asgn_01HJ0987654321",
  "parcel_id": "prcl_01HXYZ1234567890",
  "tracking_number": "SD-20261024-04921",
  "courier_id": "usr_courier_bob",
  "cancelled_by": "usr_dispatcher_dan",
  "reason": "Courier vehicle flat tire; reassigning to relief driver.",
  "unassigned_at": "2026-10-24T09:45:00Z"
}
```

---

### 4.4 Event 3: `parcel.out_for_delivery.v1`
- **Topic Key:** `parcel.out_for_delivery.v1`
- **Trigger:** Courier starts delivery run from depot.
- **Consumers:** Member 5 (Tracking API SSE broadcast), Member 8 (Customer SMS & Merchant Webhook dispatch).

#### Payload Schema
```json
{
  "parcel_id": "prcl_01HXYZ1234567890",
  "tracking_number": "SD-20261024-04921",
  "courier_id": "usr_courier_bob",
  "started_at": "2026-10-24T10:15:00Z"
}
```

---

### 4.5 Event 4: `parcel.delivered.v1`
- **Topic Key:** `parcel.delivered.v1`
- **Trigger:** Delivery completed with verified proof.
- **Consumers:** Member 5 (Tracking API marks milestone `DELIVERED`), Member 8 (Notification Worker sends delivery receipt SMS; Webhook Worker dispatches signed `parcel.delivered` payload to merchant).

#### Payload Schema
```json
{
  "parcel_id": "prcl_01HXYZ1234567890",
  "tracking_number": "SD-20261024-04921",
  "courier_id": "usr_courier_bob",
  "proof_type": "SIGNATURE",
  "proof_url": "",
  "received_by_name": "Jane Smith (Self)",
  "delivered_at": "2026-10-24T11:15:00Z"
}
```

---

### 4.6 Event 5: `parcel.delivery_failed.v1`
- **Topic Key:** `parcel.delivery_failed.v1`
- **Trigger:** Unsuccessful delivery attempt logged ($\text{attempt} < 3$).
- **Consumers:** Member 5 (Tracking API displays masked status: *"Delivery attempt 1 of 3 unsuccessful (Recipient Unavailable)"*), Member 8 (Customer notification to reschedule; Merchant Webhook).

#### Payload Schema
```json
{
  "parcel_id": "prcl_01HXYZ1234567890",
  "tracking_number": "SD-20261024-04921",
  "courier_id": "usr_courier_bob",
  "attempt_number": 1,
  "max_attempts": 3,
  "reason_code": "RECIPIENT_UNAVAILABLE",
  "next_action": "RETURN_TO_HUB_FOR_REVIEW",
  "attempted_at": "2026-10-24T11:35:00Z"
}
```

---

### 4.7 Event 6: `parcel.returned.v1`
- **Topic Key:** `parcel.returned.v1`
- **Trigger:** Terminal transition reached on 3rd failed attempt or recipient refusal.
- **Consumers:** Member 5 (Tracking API marks terminal `RETURNED`), Member 8 (Sends return notice to customer and sender; Webhook Worker triggers merchant ERP return process).

#### Payload Schema
```json
{
  "parcel_id": "prcl_01HXYZ1234567890",
  "tracking_number": "SD-20261024-04921",
  "courier_id": "usr_courier_bob",
  "total_attempts": 3,
  "final_reason_code": "RECIPIENT_UNAVAILABLE",
  "returned_at": "2026-10-24T11:35:00Z"
}
```

---

## 5. Intra-Monolith Go Interface Contracts

Cross-boundary communication inside `core-api` is strictly governed by Go interfaces to preserve decoupling:

```go
package contracts

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
)

// ParcelStatusUpdater is implemented by internal/parcels to enforce state machine transitions
type ParcelStatusUpdater interface {
	UpdateStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, newStatus string, actorID uuid.UUID) error
	RecordAttemptFailure(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, reasonCode string, actorID uuid.UUID) (newStatus string, newAttemptCount int, err error)
}

// OutboxWriter is implemented by internal/events to commit events in the caller's transaction
type OutboxWriter interface {
	WriteEvent(ctx context.Context, tx *sql.Tx, eventType string, aggregateID uuid.UUID, payload any) error
}
```
