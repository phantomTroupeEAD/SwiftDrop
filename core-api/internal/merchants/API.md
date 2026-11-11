# Merchant Module API Specification

> **Base Path:** `/api/v1`  
> **Module Owner:** Member 4 — Merchant Integration  
> **Status:** Production-Ready (Clean Monolith Integration)

---

## Table of Contents
1. [Merchant Onboarding & Profile](#1-merchant-onboarding--profile)
   - [POST /api/v1/merchants/applications](#post-apiv1merchantsapplications)
   - [GET /api/v1/merchants/me](#get-apiv1merchantsme)
2. [API Key Management](#2-api-key-management)
   - [GET /api/v1/merchants/me/api-keys](#get-apiv1merchantsmeapi-keys)
   - [POST /api/v1/merchants/me/api-keys](#post-apiv1merchantsmeapi-keys)
   - [DELETE /api/v1/merchants/me/api-keys/{key_id}](#delete-apiv1merchantsmeapi-keyskey_id)
3. [Webhook Subscription Management](#3-webhook-subscription-management)
   - [GET /api/v1/merchants/webhook-event-types](#get-apiv1merchantswebhook-event-types)
   - [GET /api/v1/merchants/me/webhooks](#get-apiv1merchantsmewebhooks)
   - [POST /api/v1/merchants/me/webhooks](#post-apiv1merchantsmewebhooks)
   - [DELETE /api/v1/merchants/me/webhooks/{webhook_id}](#delete-apiv1merchantsmewebhookswebhook_id)
4. [B2B Merchant Orders](#4-b2b-merchant-orders)
   - [POST /api/v1/merchant/orders](#post-apiv1merchantorders)
   - [GET /api/v1/merchant/orders](#get-apiv1merchantorders)
5. [Administrative Merchant Operations](#5-administrative-merchant-operations)
   - [GET /api/v1/admin/merchants](#get-apiv1adminmerchants)
   - [GET /api/v1/admin/merchants/{merchant_id}](#get-apiv1adminmerchantsmerchant_id)
   - [POST /api/v1/admin/merchants/{merchant_id}/{action}](#post-apiv1adminmerchantsmerchant_idaction)

---

## 1. Merchant Onboarding & Profile

### POST `/api/v1/merchants/applications`
Submit a new merchant onboarding application.

- **Auth Required:** Bearer JWT (`CUSTOMER`)
- **Headers:** `Authorization: Bearer <jwt>`, `Content-Type: application/json`

#### Request Body
```json
{
  "business_name": "Apex Electronics Pvt Ltd",
  "contact_email": "operations@apexelectronics.lk",
  "contact_phone": "+94771234567",
  "address": {
    "line1": "45 Galle Road",
    "line2": "Level 3",
    "city": "Colombo",
    "postal_code": "00300",
    "country": "LK"
  }
}
```

#### Success Response (`201 Created`)
```json
{
  "data": {
    "merchant_id": "c1f72a44-8c88-4c31-9562-b98a0b0d62fe",
    "business_name": "Apex Electronics Pvt Ltd",
    "status": "PENDING",
    "contact_email": "operations@apexelectronics.lk",
    "contact_phone": "+94771234567",
    "address": {
      "line1": "45 Galle Road",
      "line2": "Level 3",
      "city": "Colombo",
      "postal_code": "00300",
      "country": "LK"
    },
    "approved_at": null,
    "version": 1,
    "created_at": "2026-11-07T12:00:00Z",
    "updated_at": "2026-11-07T12:00:00Z"
  }
}
```

#### Error Responses
- `401 Unauthorized`: `UNAUTHENTICATED` — Missing or invalid Bearer token.
- `409 Conflict`: `MERCHANT_ALREADY_EXISTS` — You already have an active merchant application.
- `422 Unprocessable Entity`: `VALIDATION_FAILED` — Invalid phone (must be E.164), invalid country code, or missing address.line1.

---

### GET `/api/v1/merchants/me`
Retrieve the authenticated merchant's business profile.

- **Auth Required:** Bearer JWT (`MERCHANT` or applicant `CUSTOMER`)
- **Headers:** `Authorization: Bearer <jwt>`

#### Success Response (`200 OK`)
```json
{
  "data": {
    "merchant_id": "c1f72a44-8c88-4c31-9562-b98a0b0d62fe",
    "business_name": "Apex Electronics Pvt Ltd",
    "status": "APPROVED",
    "contact_email": "operations@apexelectronics.lk",
    "contact_phone": "+94771234567",
    "address": {
      "line1": "45 Galle Road",
      "line2": "Level 3",
      "city": "Colombo",
      "postal_code": "00300",
      "country": "LK"
    },
    "approved_at": "2026-11-07T12:30:00Z",
    "version": 2,
    "created_at": "2026-11-07T12:00:00Z",
    "updated_at": "2026-11-07T12:30:00Z"
  }
}
```

---

## 2. API Key Management

### GET `/api/v1/merchants/me/api-keys`
List all API keys belonging to the authenticated merchant.

- **Auth Required:** Bearer JWT (`MERCHANT`)
- **Headers:** `Authorization: Bearer <jwt>`

#### Success Response (`200 OK`)
```json
{
  "data": [
    {
      "key_id": "0192e21b-e5fc-7301-a185-1cb0ef695711",
      "label": "Production Server Key",
      "key_prefix": "sd_live_K4m9Zp2L",
      "scopes": ["parcels:create", "parcels:read"],
      "status": "ACTIVE",
      "created_at": "2026-11-07T12:45:00Z",
      "expires_at": null,
      "revoked_at": null,
      "last_used_at": "2026-11-07T13:10:00Z"
    }
  ]
}
```

---

### POST `/api/v1/merchants/me/api-keys`
Generate a new high-entropy programmatic API key.

- **Auth Required:** Bearer JWT (`MERCHANT`)
- **Headers:** `Authorization: Bearer <jwt>`, `Content-Type: application/json`

#### Request Body
```json
{
  "label": "ERP Integration Key",
  "scopes": ["parcels:create", "parcels:read"],
  "expires_at": "2027-11-07T00:00:00Z"
}
```

#### Success Response (`201 Created`)
> **Important:** The `api_key` plaintext is revealed only once in this response and cannot be retrieved again.
```json
{
  "data": {
    "key_id": "0192e21b-e5fc-7301-a185-1cb0ef695711",
    "label": "ERP Integration Key",
    "key_prefix": "sd_live_K4m9Zp2L",
    "scopes": ["parcels:create", "parcels:read"],
    "status": "ACTIVE",
    "api_key": "sd_live_K4m9Zp2L_1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p",
    "note": "Save this key now. It will not be shown again.",
    "created_at": "2026-11-07T12:45:00Z",
    "expires_at": "2027-11-07T00:00:00Z",
    "revoked_at": null,
    "last_used_at": null
  }
}
```

#### Error Responses
- `403 Forbidden`: `MERCHANT_NOT_APPROVED` — Only approved merchants can create API keys.
- `409 Conflict`: `API_KEY_LIMIT_REACHED` — Maximum of 5 active keys allowed.
- `409 Conflict`: `API_KEY_LABEL_DUPLICATE` — An active key already uses this label.

---

### DELETE `/api/v1/merchants/me/api-keys/{key_id}`
Revoke an active API key immediately.

- **Auth Required:** Bearer JWT (`MERCHANT`)
- **Headers:** `Authorization: Bearer <jwt>`
- **Response:** `204 No Content`

---

## 3. Webhook Subscription Management

### GET `/api/v1/merchants/webhook-event-types`
Retrieve the catalog of subscribable webhook event types.

- **Auth Required:** Bearer JWT
- **Success Response (`200 OK`)**
```json
{
  "data": [
    {"event_type": "parcel.booked.v1", "description": "Parcel created"},
    {"event_type": "parcel.assigned.v1", "description": "Courier assigned"},
    {"event_type": "parcel.picked_up.v1", "description": "Parcel collected from sender"},
    {"event_type": "parcel.out_for_delivery.v1", "description": "Out for final-mile delivery"},
    {"event_type": "parcel.delivered.v1", "description": "Delivered with proof"},
    {"event_type": "parcel.delivery_failed.v1", "description": "Delivery attempt failed"}
  ]
}
```

---

### GET `/api/v1/merchants/me/webhooks`
List all registered webhook subscriptions.

- **Auth Required:** Bearer JWT (`MERCHANT`)
- **Success Response (`200 OK`)**
```json
{
  "data": [
    {
      "webhook_id": "0192e21c-3b9f-7402-9988-5dc1fa284102",
      "url": "https://api.apexelectronics.lk/webhooks/swiftdrop",
      "events": ["parcel.delivered.v1", "parcel.delivery_failed.v1"],
      "enabled": true,
      "secret_last4": "8a9F",
      "created_at": "2026-11-07T12:50:00Z",
      "updated_at": "2026-11-07T12:50:00Z"
    }
  ]
}
```

---

### POST `/api/v1/merchants/me/webhooks`
Register a new HTTPS callback endpoint.

- **Auth Required:** Bearer JWT (`MERCHANT`)
- **Headers:** `Authorization: Bearer <jwt>`, `Content-Type: application/json`

#### Request Body
```json
{
  "url": "https://api.apexelectronics.lk/webhooks/swiftdrop",
  "events": ["parcel.delivered.v1", "parcel.delivery_failed.v1"]
}
```

#### Success Response (`201 Created`)
```json
{
  "data": {
    "webhook_id": "0192e21c-3b9f-7402-9988-5dc1fa284102",
    "url": "https://api.apexelectronics.lk/webhooks/swiftdrop",
    "events": ["parcel.delivered.v1", "parcel.delivery_failed.v1"],
    "enabled": true,
    "secret_last4": "8a9F",
    "signing_secret": "whsec_0192e21c3b9f740288112233445566778899aabbccddeeff",
    "note": "Save this signing secret now. It will not be shown again.",
    "created_at": "2026-11-07T12:50:00Z",
    "updated_at": "2026-11-07T12:50:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request`: `WEBHOOK_SECRET_NOT_ALLOWED` — Signing secrets are generated by SwiftDrop; do not provide a secret.
- `400 Bad Request`: `SSRF_PROHIBITED_URL` — Target URL resolves to an internal or private network IP.
- `409 Conflict`: `WEBHOOK_LIMIT_REACHED` — Maximum of 10 active endpoints allowed.
- `422 Unprocessable Entity`: `WEBHOOK_URL_SCHEME_NOT_ALLOWED` — Only HTTPS is allowed.

---

### DELETE `/api/v1/merchants/me/webhooks/{webhook_id}`
Soft-delete a webhook subscription.

- **Auth Required:** Bearer JWT (`MERCHANT`)
- **Response:** `204 No Content`

---

## 4. B2B Merchant Orders

### POST `/api/v1/merchant/orders`
Ingest an order directly into SwiftDrop for dispatch.

- **Auth Required:** Either `X-API-Key: sd_live_*` OR `Authorization: Bearer <jwt>` (never both)
- **Required Header:** `Idempotency-Key: <uuid>`

#### Request Body
```json
{
  "merchant_reference": "ORD-2026-9988",
  "recipient": {
    "name": "Nimal Perera",
    "phone": "+94779876543",
    "address": {
      "line1": "12 Temple Road",
      "city": "Kandy",
      "country": "LK"
    }
  },
  "package": {
    "weight_grams": 1500,
    "dimensions_cm": {
      "length": 25,
      "width": 15,
      "height": 10
    },
    "description": "Electronic components"
  }
}
```

#### Success Response (`201 Created`)
```json
{
  "data": {
    "parcel_id": "550e8400-e29b-41d4-a716-446655440000",
    "tracking_number": "SD-20261107-9988",
    "merchant_reference": "ORD-2026-9988",
    "status": "BOOKED",
    "fee": {
      "currency": "LKR",
      "amount_minor": 150000
    },
    "created_at": "2026-11-07T13:00:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request`: `AMBIGUOUS_CREDENTIALS` — Send either X-API-Key or Authorization, not both.
- `400 Bad Request`: `IDEMPOTENCY_KEY_REQUIRED` / `IDEMPOTENCY_KEY_INVALID`
- `409 Conflict`: `IDEMPOTENCY_KEY_PAYLOAD_MISMATCH` — Reusing idempotency key with different payload.

---

### GET `/api/v1/merchant/orders`
List recent orders booked by this merchant.

- **Auth Required:** `X-API-Key` or Bearer JWT (`MERCHANT`)
- **Success Response (`200 OK`)**
```json
{
  "data": [
    {
      "parcel_id": "550e8400-e29b-41d4-a716-446655440000",
      "tracking_number": "SD-20261107-9988",
      "merchant_reference": "ORD-2026-9988",
      "status": "BOOKED",
      "fee": {
        "currency": "LKR",
        "amount_minor": 150000
      },
      "created_at": "2026-11-07T13:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 1,
    "total_items": 1,
    "total_pages": 1
  }
}
```

---

## 5. Administrative Merchant Operations

### GET `/api/v1/admin/merchants`
Paginated search and filter of merchant accounts.

- **Auth Required:** Bearer JWT (`ADMIN`)
- **Query Parameters:** `status` (`PENDING`, `APPROVED`, etc.), `page` (default 1), `per_page` (default 20, max 100)

---

### GET `/api/v1/admin/merchants/{merchant_id}`
Retrieve complete merchant details by ID for review.

- **Auth Required:** Bearer JWT (`ADMIN`)

---

### POST `/api/v1/admin/merchants/{merchant_id}/{action}`
Execute an administrative state transition (`approve`, `reject`, `suspend`, `reinstate`, `disable`).

- **Auth Required:** Bearer JWT (`ADMIN`)
- **Headers:** `Authorization: Bearer <jwt>`, `Content-Type: application/json`

#### Request Body
```json
{
  "expected_version": 1,
  "reason": "Business registration documents verified against the registry."
}
```

#### Success Response (`200 OK`)
Returns the updated merchant entity with incremented `version`.

#### Error Responses
- `409 Conflict`: `MERCHANT_VERSION_CONFLICT` — Concurrent modification; reload and retry.
- `409 Conflict`: `MERCHANT_INVALID_TRANSITION` — Transition not allowed from current status.
- `422 Unprocessable Entity`: `REASON_REQUIRED` — Reason of 5–500 characters required.
