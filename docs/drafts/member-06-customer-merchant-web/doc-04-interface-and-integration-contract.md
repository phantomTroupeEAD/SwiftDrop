# Document 4 — Interface & Integration Contract

> **Module:** Customer & Merchant Frontend  
> **Owning Member:** Sajini, Member 6 — Customer & Merchant Frontend  
> **Target Workspace:** `web/src/features/customer-merchant/` (Inside Unified React SPA)  
> **Status:** Draft Specifications Complete (Ready for Team Review in Meeting 2)  
> **Governing Standards:** SwiftDrop SRS, ADR-001, ADR-003, `AGENTS.md`, and Team Agreements with Member 9  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. HTTP Configuration & Gateway Conventions

The customer and merchant portal communicates with backend microservices exclusively via the Traefik API Gateway.

### 1.1 Base URL & Environment Variable
- **Vite Configuration Variable:** `VITE_API_BASE_URL`
- **Default Production / Docker Value:** `/api/v1` (Agreement A4 & A12 with Member 9).
- **Client Request Prefix:** All client HTTP requests prepend `VITE_API_BASE_URL`. Standard endpoints are referenced relative to this base path (e.g. `POST /api/v1/pricing/quotes`), preventing accidental path duplication.
- **Gateway Path Routing (ADR-001 / Agreement A9):**
  - `/api/v1/tracking/**` &rarr; Routed to `tracking-api:8081` (Member 5)
  - `/api/v1/ai/**` &rarr; Routed to `ai-assistant:8000` (Member 10)
  - `/api/v1/**` &rarr; Routed to `core-api:8080` (Members 1, 2, 4)
  - `/**` & `/portal/**` &rarr; Routed to `web:3000` (Members 6 & 7 SPA)

---

## 2. Consumed API Inventory & Cross-Module Contracts

The table below provides the authoritative inventory of all backend endpoints consumed by Member 6, their owning members, authentication mechanisms, and resolved path agreements.

| Method | Relative Path | Owning Module | Auth Required | Request Header Requirements | Description & Payload Contract |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **POST** | `/auth/register` | **Member 1** (Identity) | Public | `Content-Type: application/json` | Customer self-registration (`fullName`, `email`, `password`, `phone`) |
| **POST** | `/auth/login` | **Member 1** (Identity) | Public | `Content-Type: application/json` | Local login; returns JWT access token in body, sets HttpOnly refresh cookie |
| **POST** | `/auth/refresh` | **Member 1** (Identity) | Cookie | `Cookie: swiftdrop_refresh_token` | Silent refresh token rotation; returns new 15-min JWT |
| **POST** | `/auth/logout` | **Member 1** (Identity) | Bearer JWT | `Authorization: Bearer <token>` | Revokes session family and clears refresh cookie |
| **GET** | `/auth/me` | **Member 1** (Identity) | Bearer JWT | `Authorization: Bearer <token>` | Fetches authenticated caller profile and role claims |
| **GET** | `/auth/google` | **Member 1** (Identity) | Public | None | Initiates Google OIDC authorization code flow |
| **GET** | `/auth/google/callback` | **Member 1** (Identity) | Public | Query: `code`, `state` | Google OAuth redirect callback handled by backend |
| **POST** | `/pricing/quotes` | **Member 2** (Pricing) | Public / Bearer | `Content-Type: application/json` | Calculates delivery fee; returns `quote_id`, `amount_minor`, `valid_until` |
| **POST** | `/parcels` | **Member 2** (Parcels) | Bearer JWT | `Idempotency-Key: <uuid>` | Submits parcel booking with `quote_id`; returns `trackingNumber` |
| **GET** | `/parcels` | **Member 2** (Parcels) | Bearer JWT | `Authorization: Bearer <token>` | Paginated list of parcels visible to authenticated caller |
| **GET** | `/parcels/{id}` | **Member 2** (Parcels) | Bearer JWT | `Authorization: Bearer <token>` | Detailed parcel record for authorized owner |
| **GET** | `/parcels/{id}/history` | **Member 2** (Parcels) | Bearer JWT | `Authorization: Bearer <token>` | Complete chronological audit history for authorized owner |
| **GET** | `/parcels/{id}/receipt` | **Member 2** (Parcels) | Bearer JWT | `Authorization: Bearer <token>` | Download official delivery receipt (PDF / JSON) |
| **GET** | `/tracking/public/{trackingNumber}` | **Member 5** (Tracking) | Public | `Accept: application/json` | Privacy-safe current tracking snapshot and milestone history |
| **GET** | `/tracking/public/{trackingNumber}/stream` | **Member 5** (Tracking) | Public | `Accept: text/event-stream` | Live SSE tracking event stream emitting `tracking-update` events |
| **GET** | `/tracking/{trackingNumber}/history` | **Member 5** (Tracking) | Bearer JWT | `Authorization: Bearer <token>` | Authenticated full tracking history |
| **POST** | `/merchants/applications` | **Member 4** (Merchants)| Bearer JWT | `Content-Type: application/json` | Submits merchant onboarding application |
| **GET** | `/merchants/me` | **Member 4** (Merchants)| Bearer JWT | `Authorization: Bearer <token>` | Checks merchant onboarding and approval status |
| **GET** | `/merchants/me/api-keys` | **Member 4** (Merchants)| Bearer JWT | `Authorization: Bearer <token>` | Lists active API keys (metadata and masked prefix only) |
| **POST** | `/merchants/me/api-keys` | **Member 4** (Merchants)| Bearer JWT | `Content-Type: application/json` | Generates machine-to-machine API key (raw `sd_live_*` returned once) |
| **DELETE**| `/merchants/me/api-keys/{id}` | **Member 4** (Merchants)| Bearer JWT | `Authorization: Bearer <token>` | Revokes an existing API key |
| **GET** | `/merchants/me/webhooks` | **Member 4** (Merchants)| Bearer JWT | `Authorization: Bearer <token>` | Lists registered merchant webhook endpoints |
| **POST** | `/merchants/me/webhooks` | **Member 4** (Merchants)| Bearer JWT | `Content-Type: application/json` | Registers webhook endpoint (raw signing secret `whsec_*` returned once) |
| **DELETE**| `/merchants/me/webhooks/{id}` | **Member 4** (Merchants)| Bearer JWT | `Authorization: Bearer <token>` | Deletes registered webhook endpoint |
| **POST** | `/merchants/me/webhooks/{id}/rotate-secret` | **Member 4** (Merchants)| Bearer JWT | None | Rotates webhook secret (new raw secret returned once) |
| **GET** | `/merchant/orders` | **Member 4** (Merchants)| Bearer JWT | `Authorization: Bearer <token>` | Lists merchant orders scoped to authenticated tenant |
| **POST** | `/merchant/orders` | **Member 4** (Merchants)| Bearer JWT | `Idempotency-Key: <uuid>` | Creates merchant order (used by single creation & batch CSV upload) |

---

## 3. Critical Contract Alignments & Clarifications

### 3.1 Fee Quotation Endpoint Path Resolution (`/pricing/quotes`)
- **Background:** Earlier draft notes and the Alignment Guide referenced `POST /api/v1/pricing/quote` (singular), while Member 2's formal specification (Doc 4 §3 & §4.1) establishes `POST /api/v1/pricing/quotes` (plural).
- **Resolution:** Member 6 adopts the plural form `POST /api/v1/pricing/quotes` conforming strictly to Member 2's authoritative contract. The shared HTTP client is configured accordingly.

### 3.2 Live Tracking Stream Endpoint Path Resolution (`/tracking/public/{trackingNumber}/stream`)
- **Background:** The Alignment Guide proposed `/api/v1/tracking/{trackingNumber}/stream`, whereas Member 5's formal contract (Doc 4 §6.1) specifies `/api/v1/tracking/public/{trackingNumber}/stream` for anonymous public visitors.
- **Resolution:** Member 6's `useSSE` hook targets `/api/v1/tracking/public/{trackingNumber}/stream` for public tracking. If Traefik or Member 5 aliases `/api/v1/tracking/{trackingNumber}/stream` as a fallback, the hook accepts configurable path templates.

### 3.3 Merchant Browser Authentication vs. M2M API Keys (Strict Rule)
- **Clarification:** The merchant portal in the web browser **never** uses machine-to-machine `X-API-Key` headers for ordinary dashboard navigation, batch CSV uploads, or order creation.
- **Enforcement:** All browser interactions with `/merchants/*` and `/merchant/orders` are authenticated using the human user's **JWT Bearer token** carrying the `tenant_id` claim.
- **Security Guardrail:** As defined in Member 4's Doc 4 §3.1, sending `X-API-Key` to `/merchants/me/**` results in `401 Unauthorized`, and sending both `Authorization` and `X-API-Key` headers simultaneously is rejected with `400 VALIDATION_FAILED` (ambiguous principal). Therefore, `X-API-Key` is strictly reserved for external backend servers.

---

## 4. Standard Request & Response Envelopes

### 4.1 Success Response Envelope
All Core API endpoints return JSON payloads wrapped in the standard envelope:
```json
{
  "data": {
    "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
    "trackingNumber": "SD-20261025-00481",
    "status": "CREATED",
    "amountMinor": 65000,
    "currency": "LKR",
    "createdAt": "2026-10-25T14:45:00Z"
  }
}
```

Paginated collections include a `meta` block:
```json
{
  "data": [ /* records */ ],
  "meta": {
    "page": 1,
    "perPage": 20,
    "totalRecords": 85,
    "totalPages": 5
  }
}
```

### 4.2 Standard Error Response Envelope
When any request fails, the shared client expects the standardized SwiftDrop error envelope:
```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "The provided parcel weight exceeds maximum single-parcel limit.",
    "correlation_id": "req-98f2a1b7-4c3e-4d81",
    "timestamp": "2026-10-25T14:45:10Z",
    "details": [
      {
        "field": "parcel.weight_kg",
        "reason": "must not exceed 50.0 kg"
      }
    ]
  }
}
```

The frontend extracts `correlation_id` to render actionable error banners for customer support inquiry.

---

## 5. Open Handoff Questions & Assigned Dependencies

The following table documents all open cross-module questions, the responsible teammate, and their current impact on Member 6:

| # | Item Description | Responsible Member | Impact on Member 6 | Current Mitigation in Specifications |
| :---: | :--- | :--- | :--- | :--- |
| **Q1** | **Receipt Format (PDF vs. JSON):** Does `GET /parcels/{id}/receipt` return a binary PDF (`application/pdf`) or a structured JSON summary? | **Member 2** | Receipt download button logic | Frontend inspects `Content-Type`: downloads directly if PDF, or renders print stylesheet if JSON. |
| **Q2** | **Authenticated Private SSE Stream:** If authenticated customers stream live internal events, is auth passed via cookie or WebSocket? | **Member 5** | Authenticated tracking view | Public view uses `EventSource` on `/tracking/public/{id}/stream`. Private view uses REST polling fallback until agreed. |
| **Q3** | **Zone List Endpoint:** What is the authoritative endpoint to list active delivery zones (`GET /api/v1/admin/zones` or public zones)? | **Member 7 / Member 2**| Populating Step 2 zone dropdowns | Cached client-side zone list fallback until public zone API is exposed. |
| **Q4** | **Google OAuth Token Delivery:** Does `GET /auth/google/callback` return a redirect with cookie, or does it require an exchange token? | **Member 1** | Post-OAuth redirect landing | Handled via background `GET /auth/me` on session cookie. |
