# SwiftDrop Web Application — Consumed API Specifications & Client Contracts

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

## 3. Standard Error Envelope

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
