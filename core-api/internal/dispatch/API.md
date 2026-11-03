# Dispatch & Couriers Module API Specifications (`/api/v1/dispatch`, `/api/v1/courier/shift`)

> **Module:** `internal/dispatch` | **Owner:** Member 3 — Dispatch, Couriers & Proof of Delivery | **Status:** Active (Phase 5)

---

## 1. Register Courier Profile

Onboards an existing authenticated user into the operational courier delivery fleet.

- **Method / Path:** `POST /api/v1/dispatch/couriers`
- **Auth Required:** Bearer JWT (`ADMIN`, `DISPATCHER`)
- **Headers:** 
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`

### Request Body
```json
{
  "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "employee_code": "CR-7001",
  "vehicle_type": "MOTORCYCLE",
  "max_capacity": 25,
  "current_zone_id": "ZONE_COLOMBO_01"
}
```

### Success Response (`201 Created`)
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

### Error Responses
- `400 Bad Request`: `INVALID_REQUEST_BODY` — Malformed JSON or unreadable payload.
- `401 Unauthorized`: `AUTH_UNAUTHORIZED` — Missing or expired bearer token.
- `403 Forbidden`: `AUTH_FORBIDDEN` — Insufficient role permissions.
- `422 Unprocessable Entity`: `VALIDATION_FAILED` — Invalid UUID, unknown vehicle type, or out-of-range capacity.

---

## 2. List Couriers by Zone

Retrieves active couriers and their concurrent parcel workload counts, optionally filtered by delivery zone.

- **Method / Path:** `GET /api/v1/dispatch/couriers`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Query Parameters:**
  - `zone_id` (string, optional): Target delivery zone code (e.g. `ZONE_COLOMBO_01`).

### Success Response (`200 OK`)
```json
{
  "data": [
    {
      "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "courier_name": "Kasun Perera",
      "employee_code": "CR-7001",
      "current_zone_id": "ZONE_COLOMBO_01",
      "active_count": 5,
      "max_capacity": 25,
      "status": "ON_SHIFT",
      "vehicle_type": "MOTORCYCLE"
    }
  ]
}
```

---

## 3. Get Live Dispatch Board

Retrieves an operational snapshot of pending unassigned parcels and current courier loads for a delivery zone.

- **Method / Path:** `GET /api/v1/dispatch/board`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Query Parameters:**
  - `zone_id` (string, optional): Target delivery zone code.

### Success Response (`200 OK`)
```json
{
  "data": {
    "zone_id": "ZONE_COLOMBO_01",
    "unassigned_parcels": [
      {
        "id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
        "tracking_number": "SD-20261102-1001",
        "weight_kg": 2.5,
        "recipient_city": "Colombo",
        "status": "CREATED",
        "created_at": "2026-11-02T09:15:00Z"
      }
    ],
    "courier_loads": [
      {
        "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
        "courier_name": "Kasun Perera",
        "employee_code": "CR-7001",
        "current_zone_id": "ZONE_COLOMBO_01",
        "active_count": 5,
        "max_capacity": 25,
        "status": "ON_SHIFT",
        "vehicle_type": "MOTORCYCLE"
      }
    ]
  }
}
```

---

## 4. Assign Parcel to Courier

Assigns an eligible parcel (`CREATED` or `ATTEMPT_FAILED`) to an on-shift courier with sufficient remaining capacity. Serialized via pessimistic row locking (AT-013).

- **Method / Path:** `POST /api/v1/dispatch/assignments`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Headers:**
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
  - `Idempotency-Key: <uuid>` (optional)

### Request Body
```json
{
  "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
  "courier_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "notes": "Fragile glassware, handle with extra care"
}
```

### Success Response (`201 Created`)
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

### Error Responses
- `400 Bad Request`: `INVALID_COMMAND` — Invalid UUID formatting.
- `404 Not Found`: `COURIER_NOT_FOUND` — Courier profile does not exist.
- `409 Conflict`: `COURIER_CAPACITY_EXCEEDED` — Courier active workload $\ge$ max capacity.
- `409 Conflict`: `COURIER_NOT_ON_SHIFT` — Courier is not currently in `ON_SHIFT` state.
- `409 Conflict`: `PARCEL_ALREADY_ASSIGNED` — Parcel currently has another active assignment (**AT-013**).
- `409 Conflict`: `INVALID_PARCEL_STATE` — Parcel is not in `CREATED` or `ATTEMPT_FAILED`.

---

## 5. Cancel Courier Assignment

Cancels an active assignment, freeing up driver capacity and reverting the parcel to `CREATED`.

- **Method / Path:** `POST /api/v1/dispatch/assignments/{id}/cancel`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)

### Request Body
```json
{
  "reason": "Courier vehicle mechanical breakdown on route"
}
```

### Success Response (`200 OK`)
```json
{
  "data": {
    "assignment_id": "1fa85f64-5717-4562-b3fc-2c963f66afa1",
    "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
    "status": "CANCELLED",
    "ended_at": "2026-11-02T10:45:00Z",
    "reason": "Courier vehicle mechanical breakdown on route"
  }
}
```

### Error Responses
- `404 Not Found`: `ASSIGNMENT_NOT_FOUND` — Assignment ID does not exist.
- `409 Conflict`: `ASSIGNMENT_ALREADY_CLOSED` — Assignment was previously completed or cancelled.

---

## 6. Reschedule Failed Parcel

Dispatcher approves and authorizes a new delivery reattempt for a parcel previously marked `ATTEMPT_FAILED`.

- **Method / Path:** `POST /api/v1/dispatch/parcels/{id}/reschedule`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)

### Request Body
```json
{
  "courier_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "notes": "Customer confirmed availability for reattempt between 2-4 PM"
}
```

### Success Response (`200 OK`)
```json
{
  "data": {
    "assignment_id": "2fa85f64-5717-4562-b3fc-2c963f66afa2",
    "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
    "courier_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "assigned_by": "0fa85f64-5717-4562-b3fc-2c963f66afa0",
    "status": "ACTIVE",
    "assigned_at": "2026-11-02T11:00:00Z"
  }
}
```

---

## 7. Toggle Courier Shift

Transitions a courier between operational shift states (`ON_SHIFT`, `OFF_SHIFT`, `ON_BREAK`).

- **Method / Path:** `POST /api/v1/courier/shift`
- **Auth Required:** Bearer JWT (`COURIER`, `DISPATCHER`, `ADMIN`)

### Request Body
```json
{
  "target_status": "ON_SHIFT"
}
```

### Success Response (`200 OK`)
```json
{
  "data": {
    "courier_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "new_status": "ON_SHIFT",
    "updated_at": "2026-11-02T08:00:00Z"
  }
}
```

### Error Responses
- `400 Bad Request`: `INVALID_SHIFT_TRANSITION` — Transition not allowed for self-service or illegal shift order.
- `403 Forbidden`: `COURIER_SUSPENDED` — Courier is administratively suspended and cannot change status.
