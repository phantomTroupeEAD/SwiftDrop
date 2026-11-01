# Dispatch Module API Specifications (`/api/v1/dispatch`, `/api/v1/couriers`)

> **Module:** `internal/dispatch` | **Owner:** Member 3 | **Status:** Baseline

---

## 1. Assign Courier to Parcel
Assigns an unassigned parcel to a specific active courier.

- **Method / Path:** `POST /api/v1/dispatch/assignments`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Headers:** 
  - `Authorization: Bearer <jwt>`
  - `Content-Type: application/json`

### Request Body
```json
{
  "parcel_id": "prcl_01HXYZ1234567890",
  "courier_id": "usr_courier_99"
}
```

### Success Response (`200 OK`)
```json
{
  "data": {
    "assignment_id": "asgn_01HJ0987654321",
    "parcel_id": "prcl_01HXYZ1234567890",
    "courier_id": "usr_courier_99",
    "assigned_at": "2026-10-21T09:30:00Z",
    "assigned_by": "usr_dispatcher_01"
  }
}
```

### Error Responses
- `400 Bad Request`: `PARCEL_NOT_ELIGIBLE_FOR_ASSIGNMENT` — Parcel is already in transit or delivered.
- `409 Conflict`: `COURIER_CAPACITY_EXCEEDED` — Courier has reached maximum concurrent capacity.
- `404 Not Found`: Parcel or Courier does not exist.

---

## 2. Get Dispatcher Board
Returns unassigned parcels and current courier loads for a specific zone.

- **Method / Path:** `GET /api/v1/dispatch/board`
- **Auth Required:** Bearer JWT (`DISPATCHER`, `ADMIN`)
- **Headers:** `Authorization: Bearer <jwt>`
- **Query Params:** `?zone_id=ZONE_COLOMBO_01&status=ELIGIBLE` (also supports `zone_code`)

### Success Response (`200 OK`)
```json
{
  "data": {
    "zone_id": "ZONE_COLOMBO_01",
    "unassigned_parcels": [
      {
        "id": "prcl_01HXYZ1234567890",
        "tracking_number": "SD-20261021-04921",
        "weight_kg": 2.5,
        "recipient_city": "Colombo",
        "created_at": "2026-10-21T09:00:00Z"
      }
    ],
    "active_couriers": [
      {
        "courier_id": "usr_courier_99",
        "name": "Bob Miller",
        "current_zone_id": "ZONE_COLOMBO_01",
        "active_assignments": 8,
        "max_capacity": 25,
        "status": "ON_SHIFT"
      }
    ]
  }
}
```
