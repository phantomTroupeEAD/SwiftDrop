# SwiftDrop Web Application — Consumed API Specifications & Client Contracts

## Apple-style portal integration checkpoint

Before requesting a booking quote, the web client resolves both five-digit postal codes through `GET /api/v1/zones?postal_code=...`. It uses the returned `data.id` values for pricing and reuses the same zones when submitting that quote's booking.

| Role | Current pages and API integration | Remaining integration |
| --- | --- | --- |
| Public | Landing, sign-in, customer registration, public tracking | Verify OTP delivery and tracking updates against deployed services. |
| Customer | Dashboard, booking, receipts, signed-in tracking, merchant application, AI chat | Verify quote, booking and receipt together with seeded pricing. Verify AI replies and booking prefill against the assistant service. |
| Merchant | Dashboard, profile, orders and CSV import, API keys, webhooks | Verify tenant isolation and that CSV imports create parcels through the live booking adapter. |
| Dispatcher | Dispatcher dashboard route | Live dispatch board (`GET /dispatch/board`), courier assignment (`POST /dispatch/assignments`), and courier fleet onboarding modal (`POST /dispatch/couriers`). |
| Courier | Courier dashboard route | Replace local task fixtures and status changes with delivery and proof-of-delivery operations. |
| Admin | Admin dashboard with user management requests | Connect delivery zones, settings, audit logs, delivery reports, and merchant approvals. |

Background event processing, internal service interfaces, and health probes support these workflows through the backend.

SwiftDrop Assistant sends `POST /api/v1/ai/chat` with the customer Bearer token and JSON fields `prompt` and optional `conversation_id`. The service returns `reply`, `conversation_id`, and optional `actions`. The chat renders replies as text and accepts only a recognized booking action. It prefills supported postal codes, weight, and service level. Booking still requests an authoritative quote and requires confirmation.

Retrying the merchant profile repeats only `GET /api/v1/merchants/me` with the current session token. Order and integration requests use their existing endpoints.

The routed portal uses real requests and never substitutes sample quotes, credentials, orders or tracking data. The older design preview and prototype dashboard notes below describe separate pages.

- Booking sends `origin_zone_id`, `destination_zone_id`, `actual_weight_kg`, `dimensions` and `service_level` to the pricing handler. It renders the returned `fee_cents`. The parcel transport on `feature/member-02-http-transport` takes the quote ID, top-level `recipient_name` and `recipient_phone`, parcel dimensions, weight, and pickup and dropoff addresses. The service level belongs to the quote. The client keeps the same idempotency key when retrying an uncertain submission. The pricing and parcel routers still need to be available in the running Core API for live verification. The older examples below describe the original design.
- Public tracking uses the two URLs in §2.3 and reads the city from `destination.city` in the tracking service response. A `status_update` event triggers a REST refresh. EventSource reconnects automatically, and the page refreshes via REST every 30 seconds while it reconnects.
- Signed-in customer and merchant tracking uses `GET /api/v1/tracking/{tracking_number}` with the session Bearer token. It displays only the authorized destination and milestone fields and refreshes through REST every 30 seconds after a successful lookup.
- The customer home requests `GET /api/v1/parcels?page=1&page_size=20` and reads the documented `items` collection. This list route must be enabled for recent customer parcels to appear. The merchant home uses `GET /api/v1/merchant/orders`. Both send the signed-in session token and summarize only the returned records.
- After booking, the frontend uses the returned parcel ID to request `GET /api/v1/parcels/{id}/receipt` with a Bearer token. It displays the server-issued receipt number, formatted total and payment status. The route is documented in the [parcel contract](../core-api/internal/parcels/API.md) and is pending registration by the parcels module. A missing receipt leaves the booking confirmation and tracking link available.
- Merchant orders are paginated in the browser after filtering. The current orders endpoint returns one collection and does not implement server-side pagination. Browser address autofill does not require a saved-address endpoint.
- Merchant requests follow the handler on `dev`. Application submission sends `business_name`, `contact_email`, `contact_phone` and `address: { line1, city, country }`. `GET /merchants/me` retrieves the application status or the signed-in merchant's profile, including `merchant_id`. API key creation sends `label` and `scopes` and displays the returned `api_key` once. The client also accepts `plaintext_key` if returned by the test deployment. Webhook creation sends `url` and `events` from `GET /merchants/webhook-event-types` and displays `signing_secret` once. The portal lists and removes credentials using the matching `/merchants/me/api-keys` and `/merchants/me/webhooks` routes.
- `GET /merchant/orders` lists tenant orders. CSV import posts individual records to `/merchant/orders` using one idempotency key per record and reuses those keys on retry while the page remains open. The current merchant service uses a booking adapter stub. Verify the real adapter before relying on order creation.
- On page load, the frontend restores a real session with `POST /api/v1/auth/refresh`. The browser sends the HttpOnly refresh cookie and receives its rotated replacement. The access token stays in memory. Protected routes wait for restoration before redirecting to sign-in.
- Webhook secret rotation still needs coordinated implementation and verification.

Clean UI update (6 November 2026): presentation cleanup introduces no HTTP contract changes. Booking confirmation, order import and tracking lookup remain unconnected and report that outcome on activation. Quote values are fixed sample data; tracking samples do not mirror typed lookup numbers or private booking addresses. No simulated live-connection state is presented as a real service connection.

Theme-preview update (6 November 2026): the Option 3 signature showcase makes no network requests and introduces no API changes. Its sample booking and tracking forms only update a local status message. It is not evidence of backend integration.

> **Workspace:** `web/`  
> **Target Audience:** Frontend Developers (Members 6 & 7), QA Engineers (Member 10), and Backend Service Owners  
> **Base Path Variable:** `VITE_API_BASE_URL` (defaults to `/api/v1` via Traefik Gateway)  
> **Specification Version:** `2.0.0` (Authoritative System Baseline)

---

## 1. Overview & Transport Protocol

All frontend network requests are routed via the Traefik API Gateway (`http://gateway:80` / `http://localhost`). The frontend Single-Page Application (SPA) never connects directly to internal container ports (`8080`, `8081`, `8000`), ensuring strict TLS, rate-limiting, and CORS policy enforcement.

### 1.1 Shared Request Headers
- `Content-Type: application/json` (for all JSON mutations)
- `Authorization: Bearer <accessToken>` (injected automatically from in-memory `AuthContext` for protected endpoints)
- `Idempotency-Key: <uuidv4>` (mandatory for parcel booking and order creation)
- `Accept: text/event-stream` (for real-time SSE streams)

---

## 2. Consumed API Contracts — Customer & Merchant Domain (Member 6)

### 2.1 Identity & Authentication (Owned by Member 1)

#### `POST /api/v1/auth/register`
- **Description:** Customer self-registration.
- **Auth:** Public / Anonymous.
- **Request Body:**
```json
{
  "full_name": "Sajini Perera",
  "email": "sajini@example.com",
  "password": "Password123!",
  "phone": "+94771234567"
}
```
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
    "email": "sajini@example.com",
    "full_name": "Sajini Perera",
    "role": "CUSTOMER"
  }
}
```

#### `POST /api/v1/auth/login`
- **Description:** User login. Sets HttpOnly refresh cookie and returns short-lived JWT.
- **Auth:** Public / Anonymous.
- **Request Body:**
```json
{
  "email": "sajini@example.com",
  "password": "Password123!"
}
```
- **Success Response (`200 OK`):** Sets cookie `swiftdrop_refresh_token` (`HttpOnly; Secure; SameSite=Strict`).
```json
{
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
      "email": "sajini@example.com",
      "full_name": "Sajini Perera",
      "role": "CUSTOMER",
      "tenant_id": null
    }
  }
}
```

#### `POST /api/v1/auth/refresh`
- **Description:** Transparent background token rotation.
- **Auth:** Cookie (`swiftdrop_refresh_token`).
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

---

### 2.2 Parcels & Pricing Engine (Owned by Member 2)

#### `POST /api/v1/pricing/quotes`
- **Description:** Computes delivery fee estimate based on weight and dimensions.
- **Auth:** Public or Bearer JWT.
- **Request Body:**
```json
{
  "pickup_zone_id": "ZONE_COLOMBO_01",
  "dropoff_zone_id": "ZONE_KANDY_01",
  "weight_kg": 2.5,
  "length_cm": 25.0,
  "width_cm": 15.0,
  "height_cm": 10.0,
  "service_level": "STANDARD"
}
```
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "quote_id": "0192e4b0-7a1b-7c82-9f34-1234567890ab",
    "base_fee_cents": 50000,
    "weight_surcharge_cents": 15000,
    "service_surcharge_cents": 0,
    "total_fee_cents": 65000,
    "currency": "LKR",
    "expires_at": "2026-10-26T12:15:00Z"
  }
}
```

#### `POST /api/v1/parcels`
- **Description:** Customer parcel booking submission.
- **Auth:** Bearer JWT (`CUSTOMER` or `MERCHANT`).
- **Headers:** `Idempotency-Key: <uuidv4>`
- **Request Body:**
```json
{
  "quote_id": "0192e4b0-7a1b-7c82-9f34-1234567890ab",
  "pickup": {
    "name": "Sajini Perera",
    "phone": "+94771234567",
    "address_lines": ["12 Flower Road"],
    "city": "Colombo",
    "zone_id": "ZONE_COLOMBO_01"
  },
  "dropoff": {
    "name": "Kamal Gunaratne",
    "phone": "+94719876543",
    "address_lines": ["45 Peradeniya Road"],
    "city": "Kandy",
    "zone_id": "ZONE_KANDY_01"
  },
  "parcel": {
    "weight_kg": 2.5,
    "dimensions_cm": {
      "length": 25.0,
      "width": 15.0,
      "height": 10.0
    },
    "description": "Textbooks and Stationery"
  }
}
```
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "id": "0192e4b5-8c2d-7e93-8a45-6789012345ef",
    "tracking_number": "SD-20261026-00142",
    "status": "CREATED",
    "total_fee_cents": 65000,
    "currency": "LKR",
    "created_at": "2026-10-26T12:00:00Z"
  }
}
```

---

### 2.3 Live Tracking & Real-Time Updates (Owned by Member 5)

#### `GET /api/v1/tracking/public/{trackingNumber}`
- **Description:** Public shipment tracking snapshot with privacy masking (**SEC-009**).
- **Auth:** Public / Anonymous.
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "tracking_number": "SD-20261026-00142",
    "status": "IN_TRANSIT",
    "destination_city": "Kandy",
    "last_updated_at": "2026-10-26T12:30:00Z",
    "milestones": [
      {
        "status": "CREATED",
        "timestamp": "2026-10-26T12:00:00Z"
      },
      {
        "status": "PICKED_UP",
        "timestamp": "2026-10-26T12:15:00Z"
      },
      {
        "status": "IN_TRANSIT",
        "timestamp": "2026-10-26T12:30:00Z"
      }
    ]
  }
}
```

#### `GET /api/v1/tracking/public/{trackingNumber}/stream`
- **Description:** Server-Sent Events (SSE) live milestone stream.
- **Auth:** Public / Anonymous.
- **Headers:** `Accept: text/event-stream`
- **Stream Event Format:**
```
:keepalive

event: status_update
id: evt-0192e4bf-1234
data: {"tracking_number":"SD-20261026-00142","status":"OUT_FOR_DELIVERY","occurred_at":"2026-10-26T12:45:00Z"}

```

---

### 2.4 Merchant Integration & Credentials (Owned by Member 4)

#### `POST /api/v1/merchants/applications`
- **Description:** Business onboarding registration.
- **Auth:** Bearer JWT.
- **Request Body:**
```json
{
  "business_name": "Lanka Retail Group",
  "brn": "PV-123456",
  "contact_email": "operations@lankaretail.lk",
  "contact_phone": "+94112345678",
  "registered_address": "88 Galle Road, Colombo 03"
}
```
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "merchant_id": "0192e4c0-9a2b-7d83-9b12-567890abcdef",
    "business_name": "Lanka Retail Group",
    "status": "PENDING",
    "created_at": "2026-10-26T12:00:00Z"
  }
}
```

#### `POST /api/v1/merchants/me/api-keys`
- **Description:** Generates machine-to-machine API key.
- **Auth:** Bearer JWT (`MERCHANT`).
- **Request Body:**
```json
{
  "label": "Production-ERP"
}
```
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "key_id": "key_0192e4c5",
    "key_prefix": "sd_live_a8f9",
    "label": "Production-ERP",
    "key": "sd_live_a8f9021b47c928e192837465b8c9d0e1f2a3b4c5",
    "created_at": "2026-10-26T12:00:00Z"
  }
}
```
> [!IMPORTANT]
> The `key` field is returned **exactly once**. Frontend displays this in an emergency modal with zero browser persistence.

#### `POST /api/v1/merchants/me/webhooks`
- **Description:** Registers outbound webhook destination.
- **Auth:** Bearer JWT (`MERCHANT`).
- **Request Body:**
```json
{
  "url": "https://api.lankaretail.lk/webhooks/swiftdrop",
  "subscribed_event_types": [
    "parcel.booked.v1",
    "parcel.picked_up.v1",
    "parcel.delivered.v1",
    "parcel.delivery_failed.v1"
  ]
}
```
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "id": "wh_0192e4ca",
    "url": "https://api.lankaretail.lk/webhooks/swiftdrop",
    "subscribed_event_types": [
      "parcel.booked.v1",
      "parcel.picked_up.v1",
      "parcel.delivered.v1",
      "parcel.delivery_failed.v1"
    ],
    "secret": "whsec_98f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9",
    "enabled": true
  }
}
```
> [!IMPORTANT]
> The HMAC-SHA256 signing `secret` is displayed **exactly once** in a one-time copy modal with zero browser persistence.

#### `POST /api/v1/merchant/orders`
- **Description:** Single or batch CSV parcel creation for merchants.
- **Auth:** Bearer JWT (`MERCHANT`).
- **Headers:** `Idempotency-Key: <uuidv4>`
- **Request Body:**
```json
{
  "pickup": {
    "name": "Lanka Retail Warehouse",
    "phone": "+94112345678",
    "address_lines": ["Zone B, Logistics Park"],
    "city": "Colombo",
    "zone_id": "ZONE_COLOMBO_01"
  },
  "dropoff": {
    "name": "Sunil Perera",
    "phone": "+94771112233",
    "address_lines": ["10 Main Street"],
    "city": "Colombo",
    "zone_id": "ZONE_COLOMBO_01"
  },
  "parcel": {
    "weight_kg": 1.5,
    "dimensions_cm": { "length": 20.0, "width": 15.0, "height": 10.0 },
    "description": "Apparel and Accessories"
  }
}
```
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "order_id": "0192e4cf-3a1b-7e92-8f12-9012345678ab",
    "tracking_number": "SD-20261026-00892",
    "status": "CREATED"
  }
}
```

---

### 2.8 Dispatcher Operations (Owned by Member 3 & Member 7)

#### `POST /api/v1/dispatch/couriers` — Register Courier Profile
- **Screen:** Dispatcher Dashboard (`/dispatcher/dashboard` -> `+ Add Courier` Modal)
- **Role:** `DISPATCHER`, `ADMIN`
- **Request Headers:**
  - `Authorization: Bearer <accessToken>`
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "user_id": "018f4a7c-1003-7000-8000-000000000003",
  "employee_code": "CR-7001",
  "vehicle_type": "MOTORCYCLE",
  "max_capacity": 25,
  "current_zone_id": "ZONE_COLOMBO_01"
}
```
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "user_id": "018f4a7c-1003-7000-8000-000000000003",
    "employee_code": "CR-7001",
    "vehicle_type": "MOTORCYCLE",
    "max_capacity": 25,
    "status": "OFF_SHIFT",
    "current_zone_id": "ZONE_COLOMBO_01",
    "is_active": true,
    "created_at": "2026-11-17T20:00:00Z"
  }
}
```
- **Error Responses:**
  - `400 Bad Request`: `VALIDATION_FAILED` (missing fields, invalid UUID, capacity out of range)
  - `409 Conflict`: `COURIER_PROFILE_EXISTS` (courier profile already registered for this user)
  - `422 Unprocessable Entity`: `USER_NOT_A_COURIER` (user exists but does not have the COURIER role)

#### `GET /api/v1/dispatch/couriers/candidates` — List Eligible Courier Accounts
- **Screen:** Dispatcher Dashboard (`/dispatcher/dashboard` -> `+ Add Courier` Modal account selector)
- **Role:** `DISPATCHER`, `ADMIN`
- **Request Headers:**
  - `Authorization: Bearer <accessToken>`
- **Success Response (`200 OK`):**
```json
{
  "data": [
    {
      "user_id": "018f4a7c-1003-7000-8000-000000000003",
      "display_name": "Kamal Perera",
      "email": "courier@swiftdrop.local",
      "phone": "+94770000003"
    }
  ]
}
```

#### `GET /api/v1/dispatch/board` — Query Live Dispatch Board
- **Screen:** Dispatcher Dashboard (`/dispatcher/dashboard` -> Unassigned & Assigned Parcels)
- **Role:** `DISPATCHER`, `ADMIN`
- **Query Parameters:**
  - `zone_id` (string, optional): Target delivery zone code (e.g. `ZONE_COLOMBO_01`).
- **Request Headers:**
  - `Authorization: Bearer <accessToken>`
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "zone_id": "ZONE_COLOMBO_01",
    "unassigned_parcels": [
      {
        "id": "018f4a7c-8004-7000-8000-000000000004",
        "tracking_number": "SD-20261117-CMB004",
        "weight_kg": 5.5,
        "recipient_city": "Colombo",
        "status": "CREATED",
        "created_at": "2026-11-17T15:59:33.35314Z"
      }
    ],
    "assigned_parcels": [
      {
        "id": "018f4a7c-8002-7000-8000-000000000002",
        "tracking_number": "SD-20261117-CMB002",
        "weight_kg": 4.0,
        "recipient_city": "Colombo",
        "status": "ASSIGNED",
        "courier_id": "018f4a7c-1003-7000-8000-000000000008"
      }
    ],
    "courier_loads": [
      {
        "courier_id": "018f4a7c-1003-7000-8000-000000000003",
        "name": "Kamal Perera (Courier)",
        "employee_code": "CR-0012",
        "current_zone_id": "ZONE_COLOMBO_01",
        "active_assignments": 1,
        "max_capacity": 25,
        "status": "ON_SHIFT",
        "vehicle_type": "MOTORCYCLE"
      }
    ]
  }
}
```

#### `GET /api/v1/dispatch/couriers` — List Couriers by Zone
- **Screen:** Dispatcher Dashboard (`/dispatcher/dashboard` -> Zone Filter & Courier Roster)
- **Role:** `DISPATCHER`, `ADMIN`
- **Query Parameters:**
  - `zone_id` (string, optional): Target delivery zone code (e.g. `ZONE_COLOMBO_01`).
- **Request Headers:**
  - `Authorization: Bearer <accessToken>`
- **Success Response (`200 OK`):**
```json
{
  "data": [
    {
      "user_id": "018f4a7c-1003-7000-8000-000000000003",
      "courier_name": "Kamal Perera (Courier)",
      "employee_code": "CR-0012",
      "current_zone_id": "ZONE_COLOMBO_01",
      "active_count": 0,
      "max_capacity": 25,
      "status": "ON_SHIFT",
      "vehicle_type": "MOTORCYCLE"
    }
  ]
}
```

#### `POST /api/v1/dispatch/assignments/{id}/cancel` — Cancel Courier Assignment
- **Screen:** Dispatcher Dashboard (`/dispatcher/dashboard` -> Assigned Parcels card -> Unassign button & Cancel Modal)
- **Role:** `DISPATCHER`, `ADMIN`
- **Path Parameter:**
  - `id` (string, UUID): The parcel UUID whose active assignment is being cancelled.
- **Request Headers:**
  - `Authorization: Bearer <accessToken>`
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "reason": "Courier vehicle flat tire on Galle Road"
}
```
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "assignment_id": "018f4a7c-8003-7000-8000-000000000003",
    "parcel_id": "018f4a7c-8003-7000-8000-000000000003",
    "status": "CANCELLED",
    "ended_at": "2026-11-17T16:21:26.9176536Z",
    "reason": "Courier vehicle flat tire on Galle Road"
  }
}
```
- **Operational Effect:** Courier active workload decreases by 1; parcel reverts to `CREATED` and reappears in Unassigned Parcels.

#### `POST /api/v1/dispatch/parcels/{id}/reschedule` — Reschedule Failed Delivery Reattempt
- **Screen:** Dispatcher Dashboard (`/dispatcher/dashboard` -> Unassigned Parcels queue -> `Attempt Failed` badge -> **Reschedule** button & Reschedule Modal)
- **Role:** `DISPATCHER`, `ADMIN`
- **Path Parameter:**
  - `id` (string, UUID): The parcel UUID in `ATTEMPT_FAILED` status.
- **Request Headers:**
  - `Authorization: Bearer <accessToken>`
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "courier_id": "018f4a7c-1003-7000-8000-000000000003",
  "notes": "Customer confirmed home between 2 PM and 5 PM"
}
```
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "assignment_id": "df9c0fbd-e747-4f69-8d04-a2a63a853832",
    "parcel_id": "018f4a7c-8003-7000-8000-000000000003",
    "courier_id": "018f4a7c-1003-7000-8000-000000000003",
    "assigned_by": "018f4a7c-1002-7000-8000-000000000002",
    "status": "ACTIVE",
    "assigned_at": "2026-11-17T16:35:30.7625662Z"
  }
}
```
- **Operational Effect:** Marks previous open assignment `REASSIGNED`, creates new `ACTIVE` assignment for target courier, transitions parcel status from `ATTEMPT_FAILED` to `ASSIGNED`, and decrements remaining driver capacity.

---

### 2.7 Courier Delivery Verification & Proof of Delivery (POD)

#### `POST /api/v1/deliveries/{id}/proof-photo` — Stream Handover Proof Photo
- **Role Required:** `COURIER`
- **Content-Type:** `multipart/form-data`
- **Part:** `file` (Binary image JPEG/PNG/WebP, $\le 5$ MB)
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "photo_url": "/api/v1/deliveries/proofs/proofs/018f4a7c-8004-7000-8000-000000000004_1794970609.jpg",
    "storage_key": "proofs/018f4a7c-8004-7000-8000-000000000004_1794970609.jpg",
    "content_type": "image/jpeg",
    "uploaded_at": "2026-11-18T08:26:49.1029Z"
  }
}
```

#### `POST /api/v1/deliveries/{id}/complete` — Complete Delivery with Verified Proof
- **Role Required:** `COURIER`
- **Content-Type:** `application/json`
- **Header:** `Idempotency-Key: <UUID>`

**Option A: Photo Proof Payload:**
```json
{
  "proof_type": "PHOTO",
  "proof_url": "/api/v1/deliveries/proofs/proofs/018f4a7c-8004-7000-8000-000000000004_1794970609.jpg",
  "received_by_name": "Kasun Perera"
}
```

**Option B: Vector Signature Proof Payload:**
```json
{
  "proof_type": "SIGNATURE",
  "signature_data": "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIzNjAiIGhlaWdodD0iMTYwIiB2aWV3Qm94PSIwIDAgMzYwIDE2MCI+PHBhdGggZD0iTSAxMCAxMCBMIDEwMCAxMDAiIHN0cm9rZT0iY3VycmVudENvbG9yIiBzdHJva2Utd2lkdGg9IjIuNSIgZmlsbD0ibm9uZSIvPjwvc3ZnPg==",
  "received_by_name": "Kasun Perera"
}
```

- **Invariants Enforced:**
  - AT-006 IDOR protection: Authenticated courier must be assigned to parcel.
  - Parcel must be in `OUT_FOR_DELIVERY` status.
  - If `proof_type == "SIGNATURE"`, `signature_data` must contain $\ge 50$ characters of vector stroke data.
  - Recipient name must be 2–150 characters.
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "attempt_id": "aab9c8ca-e948-4304-b675-806bdd80db7e",
    "parcel_id": "018f4a7c-8004-7000-8000-000000000004",
    "status": "DELIVERED",
    "outcome": "SUCCESS",
    "completed_at": "2026-11-18T08:26:49.116Z"
  }
}
```
- **Operational Effect:** Parcel moves to `DELIVERED`, courier assignment marked `COMPLETED`, courier load restored, and `parcel.delivered.v1` outbox event published.

#### `POST /api/v1/deliveries/{id}/failed` — Record Failed Delivery Attempt
- **Role Required:** `COURIER`
- **Content-Type:** `application/json`
- **Request Body:**
```json
{
  "failure_reason_code": "RECIPIENT_UNAVAILABLE",
  "notes": "Gate locked, called recipient twice with no answer"
}
```
- **Approved ADR-004 Failure Reason Codes:**
  - `RECIPIENT_UNAVAILABLE`
  - `CUSTOMER_REQUESTED_RESCHEDULE`
  - `ACCESS_RESTRICTED`
  - `INVALID_ADDRESS`
  - `REFUSED_BY_RECIPIENT` (Immediate terminal return)
  - `DAMAGED_IN_TRANSIT`
  - `OTHER` (Requires note $\ge 15$ characters)
- **Invariants Enforced:**
  - Caller must be the assigned courier (AT-006 IDOR protection).
  - Parcel must be in `OUT_FOR_DELIVERY` status.
  - Failure reason code must match an approved ADR-004 enum value.
  - Reason `OTHER` mandates `notes` $\ge 15$ characters.
  - 3-attempt ceiling: attempt 1 or 2 transitions parcel to `ATTEMPT_FAILED`. Attempt 3 or `REFUSED_BY_RECIPIENT` transitions parcel to `RETURNED`.
- **Success Response (`200 OK` - Non-Terminal):**
```json
{
  "data": {
    "attempt_id": "9fa85f64-5717-4562-b3fc-2c963f66afa9",
    "parcel_id": "7fa85f64-5717-4562-b3fc-2c963f66afa7",
    "attempt_number": 1,
    "max_attempts": 3,
    "outcome": "FAILED",
    "failure_reason_code": "RECIPIENT_UNAVAILABLE",
    "parcel_status": "ATTEMPT_FAILED",
    "is_terminal": false,
    "attempted_at": "2026-11-18T21:55:00Z"
  }
}
```
- **Success Response (`200 OK` - Terminal):**
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
    "attempted_at": "2026-11-18T22:00:00Z"
  }
}
```

#### `GET /api/v1/deliveries/parcels/{id}` — Get Parcel Delivery Attempt History
- **Role Required:** `COURIER`, `DISPATCHER`, `ADMIN`
- **Use Case:** Audit trail of all attempts made on a parcel, including failure reasons, notes, timestamps, recipient names, and proof attachments.
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "parcel_id": "018f4a7c-8004-7000-8000-000000000004",
    "current_status": "DELIVERED",
    "attempt_count": 1,
    "attempts": [
      {
        "attempt_id": "aab9c8ca-e948-4304-b675-806bdd80db7e",
        "attempt_number": 1,
        "outcome": "SUCCESS",
        "proof_type": "PHOTO",
        "proof_attachment_url": "/api/v1/deliveries/proofs/proofs/018f4a7c-8004-7000-8000-000000000004_1794970609.jpg",
        "received_by_name": "kavinda",
        "notes": "",
        "courier_id": "018f4a7c-1003-7000-8000-000000000003",
        "attempted_at": "2026-11-18T02:56:49.11666Z"
      }
    ]
  }
}
```
- **Error Responses:**
  - `404 Not Found`: `PARCEL_NOT_FOUND` (no attempts recorded yet for this parcel).

#### `POST /api/v1/courier/shift` — Administrative Courier Shift Management
- **Screen:** Dispatcher Dashboard (`/dispatcher/dashboard` -> Courier Roster card -> Start shift / Break / Resume shift / End shift buttons)
- **Role:** `DISPATCHER`, `ADMIN` (also `COURIER` for self-service)
- **Request Headers:**
  - `Authorization: Bearer <accessToken>`
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "courier_id": "018f4a7c-1003-7000-8000-000000000008",
  "target_status": "ON_BREAK"
}
```
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "courier_id": "018f4a7c-1003-7000-8000-000000000008",
    "new_status": "ON_BREAK",
    "updated_at": "2026-11-17T16:50:11Z"
  }
}
```
- **Operational Effect:** Courier shift status updates in real time. Couriers in `ON_BREAK` or `OFF_SHIFT` are immediately excluded from the dispatcher's available assignment dropdown.

#### `GET /api/v1/deliveries/parcels/{id}` — Get Parcel Delivery Attempt History
- **Role Required:** `COURIER`, `DISPATCHER`, `ADMIN`
- **Use Case:** Audit trail of all attempts made on a parcel, including failure reasons, notes, timestamps, recipient names, and proof attachments.
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "parcel_id": "018f4a7c-8004-7000-8000-000000000004",
    "current_status": "DELIVERED",
    "attempt_count": 1,
    "attempts": [
      {
        "attempt_id": "aab9c8ca-e948-4304-b675-806bdd80db7e",
        "attempt_number": 1,
        "outcome": "SUCCESS",
        "proof_type": "PHOTO",
        "proof_attachment_url": "/api/v1/deliveries/proofs/proofs/018f4a7c-8004-7000-8000-000000000004_1794970609.jpg",
        "received_by_name": "kavinda",
        "notes": "",
        "courier_id": "018f4a7c-1003-7000-8000-000000000003",
        "attempted_at": "2026-11-18T02:56:49.11666Z"
      }
    ]
  }
}
```
- **Error Responses:**
  - `404 Not Found`: `PARCEL_NOT_FOUND` (no attempts recorded yet for this parcel).

---

## 3. Standard Error Envelope

Authentication, registration, and portal clients display the error's `correlation_id` as a request reference. If the error body omits it, the client uses the `X-Request-ID` response header when available.

When any backend API request fails, the shared client parses the standard error response:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "The provided parcel weight exceeds maximum single-parcel limit.",
    "correlation_id": "req-98f2a1b7-4c3e-4d81",
    "timestamp": "2026-10-26T12:00:10Z",
    "details": [
      {
        "field": "parcel.weight_kg",
        "issue": "must not exceed 50.0 kg"
      }
    ]
  }
}
```

---

## 4. Playwright Automated Test Attributes (`data-testid`)

Every interactive form element, trigger button, and status indicator maintains an explicit `data-testid` attribute for Member 10's E2E test matrix:

| Component | `data-testid` | Description |
| :--- | :--- | :--- |
| Email Input | `data-testid="login-email-input"` | Login form email |
| Password Input | `data-testid="login-password-input"` | Login form password |
| Login Button | `data-testid="login-submit-btn"` | Triggers login |
| Booking Weight | `data-testid="booking-weight-input"` | Step 2 parcel weight |
| Booking Dims | `data-testid="booking-dims-input"` | Step 2 dimensions |
| Quote Price | `data-testid="quote-price-cents"` | Step 3 price badge in LKR |
| Booking Submit | `data-testid="booking-submit-btn"` | Step 4 idempotent submission |
| Tracking Input | `data-testid="tracking-search-input"` | Public tracking search |
| Tracking Status | `data-testid="tracking-status-badge"` | Current lifecycle badge |
| Batch CSV Input | `data-testid="batch-csv-file-input"` | Merchant CSV file input |
| One-Time Key Copy | `data-testid="one-time-key-copy-btn"` | Copies raw API key once |

## Implementation checkpoint — 5 November 2026

This document describes consumed API contracts, not proof that those workflows are integrated. The M6 customer and merchant dashboards currently render prototype data: quotes are calculated locally, booking/receipt actions show alerts, tracking is static, CSV browsing is unwired, and the example key is not an issued one-time credential. These must be replaced with approved service calls before acceptance.

The testability foundation adds DOM selectors and accessible field labels only; it changes no HTTP request, response, authentication or backend contract. See [test-ids.md](test-ids.md) for implemented versus reserved selectors and [design.mdx](design.mdx) for the visual baseline. Shared login integration remains a coordinated M6/M7 task.
