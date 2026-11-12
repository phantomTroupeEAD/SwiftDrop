# Delivery Records Module API Specifications (`/api/v1/courier`, `/api/v1/deliveries`)

> **Module:** `internal/delivery` | **Owner:** Member 3 — Dispatch, Couriers & Proof of Delivery | **Status:** Active (Phase 5)

---

## 1. Get Courier Active Manifest

Retrieves the active assigned parcel manifest for the currently authenticated courier with strict operational data minimization (customer PII and wholesale rates redacted).

- **Method / Path:** `GET /api/v1/courier/parcels`
- **Auth Required:** Bearer JWT (`COURIER`)
- **Headers:** `Authorization: Bearer <token>`

### Success Response (`200 OK`)
```json
{
  "data": [
    {
      "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
      "tracking_number": "SD-20261102-1001",
      "current_status": "ASSIGNED",
      "destination_zone": "ZONE_COLOMBO_01",
      "recipient_name": "Kasun Perera",
      "recipient_phone": "+94771234567",
      "delivery_street": "123 Galle Road, Bambalapitiya",
      "delivery_city": "Colombo",
      "weight_kg": 2.5,
      "handling_instructions": "Fragile glassware, handle with care",
      "attempt_count": 0,
      "assigned_at": "2026-11-02T10:15:00Z"
    }
  ]
}
```

---

## 2. Start Single Parcel Delivery Run

Transitions a parcel from `ASSIGNED` to `OUT_FOR_DELIVERY` when the courier departs the hub.

- **Method / Path:** `POST /api/v1/courier/parcels/{id}/start-run`
- **Auth Required:** Bearer JWT (`COURIER`)

### Success Response (`200 OK`)
```json
{
  "data": {
    "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
    "status": "OUT_FOR_DELIVERY",
    "started_at": "2026-11-02T10:30:00Z"
  }
}
```

### Error Responses
- `403 Forbidden`: `COURIER_NOT_ASSIGNED` — Courier is not assigned to this parcel (**AT-006 IDOR Defense**).
- `404 Not Found`: `PARCEL_NOT_ASSIGNED` — Parcel does not have an active assignment.

---

## 3. Batch Start Delivery Run

Transitions multiple parcels to `OUT_FOR_DELIVERY` simultaneously upon departing depot on morning route.

- **Method / Path:** `POST /api/v1/courier/runs/start-batch`
- **Auth Required:** Bearer JWT (`COURIER`)

### Request Body
```json
{
  "parcel_ids": [
    "7fa85f64-5717-4562-b3fc-2c963f66afa7",
    "8fa85f64-5717-4562-b3fc-2c963f66afa8"
  ]
}
```

### Success Response (`200 OK`)
```json
{
  "data": {
    "started_parcels": [
      "7fa85f64-5717-4562-b3fc-2c963f66afa7",
      "8fa85f64-5717-4562-b3fc-2c963f66afa8"
    ],
    "failed_parcels": [],
    "total_started": 2
  }
}
```

---

## 4. Upload Proof-of-Delivery Photo

Streams a doorstep handover photo (JPEG/PNG, $\le 5$ MB) to Cloudflare R2 / AWS S3 or local storage.

- **Method / Path:** `POST /api/v1/deliveries/{id}/proof-photo`
- **Auth Required:** Bearer JWT (`COURIER`)
- **Headers:** `Content-Type: multipart/form-data`
- **Form Part:** `file` (binary stream)

### Success Response (`201 Created`)
```json
{
  "data": {
    "photo_url": "https://cdn.swiftdrop.io/proofs/2026/11/02/7fa85f64.jpg",
    "storage_key": "proofs/2026/11/02/7fa85f64.jpg",
    "content_type": "image/jpeg",
    "uploaded_at": "2026-11-02T14:30:00Z"
  }
}
```

### Error Responses
- `400 Bad Request`: `MISSING_FILE` or `INVALID_PHOTO_FORMAT` — File part missing or unsupported image type.
- `403 Forbidden`: `COURIER_NOT_ASSIGNED` — Only the assigned courier may upload delivery proof (**AT-006**).
- `413 Payload Too Large`: `PHOTO_TOO_LARGE` — Upload exceeds 5 MB limit.

---

## 5. Complete Delivery with Verified Proof

Records successful doorstep parcel handover along with validated Proof of Delivery (Photo, Signature, or Confirmation) and transitions parcel to `DELIVERED`.

- **Method / Path:** `POST /api/v1/deliveries/{id}/complete`
- **Auth Required:** Bearer JWT (`COURIER`)

### Request Body (Photo Proof)
```json
{
  "proof_type": "PHOTO",
  "proof_url": "https://cdn.swiftdrop.io/proofs/2026/11/02/7fa85f64.jpg",
  "received_by_name": "Kasun Perera",
  "latitude": 6.927079,
  "longitude": 79.861244,
  "notes": "Handed to recipient in person at front door"
}
```

### Request Body (Signature Proof)
```json
{
  "proof_type": "SIGNATURE",
  "signature_data": "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDAiIGhlaWdodD0iMTAwIj48cGF0aCBkPSJNMTAsMTAgTDIwLDIwIiBzdHJva2U9ImJsYWNrIi8+PC9zdmc+",
  "received_by_name": "Kasun Perera"
}
```

### Success Response (`200 OK`)
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

### Error Responses
- `403 Forbidden`: `COURIER_NOT_ASSIGNED` — Caller is not the assigned courier (**AT-006**).
- `409 Conflict`: `PARCEL_NOT_OUT_FOR_DELIVERY` — Parcel status must be `OUT_FOR_DELIVERY`.
- `422 Unprocessable Entity`: `DELIVERY_PROOF_REQUIRED` — Missing proof payload or invalid proof type.
- `422 Unprocessable Entity`: `INVALID_RECIPIENT_NAME` — Recipient name must be 2-150 characters.
- `422 Unprocessable Entity`: `INVALID_SIGNATURE_DATA` — Stroke data must contain $\ge 50$ characters.

---

## 6. Record Failed Delivery Attempt

Logs an unsuccessful doorstep delivery attempt with an approved ADR-004 reason code. Enforces the 3-attempt ceiling (transitioning to `RETURNED` on the 3rd failed attempt or upon `REFUSED_BY_RECIPIENT`).

- **Method / Path:** `POST /api/v1/deliveries/{id}/failed`
- **Auth Required:** Bearer JWT (`COURIER`)

### Request Body
```json
{
  "failure_reason_code": "RECIPIENT_UNAVAILABLE",
  "notes": "Called recipient phone twice; premises closed with security gate locked",
  "latitude": 6.927079,
  "longitude": 79.861244
}
```

### Success Response (`200 OK` - Non-terminal Attempt)
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

### Success Response (`200 OK` - Terminal Return to Hub)
```json
{
  "data": {
    "attempt_id": "9fa85f64-5717-4562-b3fc-2c963f66afb0",
    "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
    "attempt_number": 3,
    "max_attempts": 3,
    "outcome": "FAILED",
    "failure_reason_code": "RECIPIENT_UNAVAILABLE",
    "parcel_status": "RETURNED",
    "is_terminal": true,
    "attempted_at": "2026-11-02T15:00:00Z"
  }
}
```

### Error Responses
- `400 Bad Request`: `INVALID_FAILURE_REASON` — Must be one of 7 approved ADR-004 codes (`RECIPIENT_UNAVAILABLE`, `CUSTOMER_REQUESTED_RESCHEDULE`, `ACCESS_RESTRICTED`, `INVALID_ADDRESS`, `REFUSED_BY_RECIPIENT`, `DAMAGED_IN_TRANSIT`, `OTHER`).
- `403 Forbidden`: `COURIER_NOT_ASSIGNED` — Caller is not the assigned courier (**AT-006**).
- `422 Unprocessable Entity`: `DETAILED_NOTE_REQUIRED` — Reason `OTHER` requires a note $\ge 15$ characters.

---

## 7. Get Parcel Delivery Attempt History

Retrieves historical delivery attempt logs, failure reasons, and presigned POD photo URLs for a parcel.

- **Method / Path:** `GET /api/v1/deliveries/parcels/{id}`
- **Auth Required:** Bearer JWT (`COURIER`, `DISPATCHER`, `ADMIN`)

### Success Response (`200 OK`)
```json
{
  "data": {
    "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
    "current_status": "DELIVERED",
    "attempt_count": 2,
    "attempts": [
      {
        "id": "9fa85f64-5717-4562-b3fc-2c963f66afa9",
        "attempt_number": 1,
        "outcome": "FAILED",
        "failure_reason_code": "ACCESS_RESTRICTED",
        "notes": "Condo security denied entry without resident gate pass",
        "attempted_at": "2026-11-01T14:30:00Z"
      },
      {
        "id": "8fa85f64-5717-4562-b3fc-2c963f66afa8",
        "attempt_number": 2,
        "outcome": "SUCCESS",
        "proof_type": "PHOTO",
        "proof_attachment_url": "https://cdn.swiftdrop.io/proofs/2026/11/02/7fa85f64.jpg",
        "received_by_name": "Kasun Perera",
        "attempted_at": "2026-11-02T14:32:00Z"
      }
    ]
  }
}
```
