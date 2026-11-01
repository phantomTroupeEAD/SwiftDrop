# Document 5 — Data & Persistence Specification

> **Module:** Customer & Merchant Frontend  
> **Owning Member:** Sajini, Member 6 — Customer & Merchant Frontend  
> **Target Workspace:** `web/src/features/customer-merchant/` (Inside Unified React SPA)  
> **Status:** Draft Specifications Complete (Ready for Team Review in Meeting 2)  
> **Governing Standards:** SwiftDrop SRS, ADR-001, ADR-003, `AGENTS.md`, and `.agents/rules/frontend.md`  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Data Ownership & Storage Boundaries

The frontend does not own, manage, or migrate any PostgreSQL tables or server databases. All authoritative business entities—such as users, parcels, quotes, tariffs, courier assignments, and merchant profiles—reside permanently in the PostgreSQL database owned by the backend module owners (Members 1, 2, 3, and 4).

The frontend's role is strictly limited to:
1. Capturing and validating transient user input in local form memory.
2. Caching server responses temporarily in the browser using TanStack Query.
3. Rendering formatted representations of authoritative backend state.
4. Ensuring sensitive credentials (access tokens, API keys, webhook secrets) are never leaked to persistent browser storage.

---

## 2. In-Memory Data Models & TypeScript Interfaces

### 2.1 Authenticated User Principal (`AuthContext`)
```typescript
export interface UserPrincipal {
  id: string; // UUID v4
  email: string;
  fullName: string;
  role: 'CUSTOMER' | 'MERCHANT' | 'DISPATCHER' | 'ADMIN' | 'COURIER';
  tenantId: string | null; // Populated for approved merchants
  phone?: string;
}
```

### 2.2 Server Pricing Quote (`POST /api/v1/pricing/quotes`)
```typescript
export interface ServerQuote {
  quoteId: string; // UUID v4
  amountMinor: number; // Integer cents in LKR (e.g., 65000 = LKR 650.00)
  currency: 'LKR';
  pricingVersion: string;
  validUntil: string; // ISO 8601 UTC timestamp (15-minute guarantee)
  breakdown: {
    baseFareMinor: number;
    weightChargeMinor: number;
    volumetricChargeMinor: number;
    serviceFeeMinor: number;
  };
}
```

### 2.3 Parcel Booking Payload (`POST /api/v1/parcels`)
```typescript
export interface ParcelBookingPayload {
  quote_id: string;
  pickup: {
    name: string;
    phone: string;
    address_lines: string[];
    city: string;
    zone_id: string;
  };
  dropoff: {
    name: string;
    phone: string;
    address_lines: string[];
    city: string;
    zone_id: string;
  };
  parcel: {
    weight_kg: number;
    dimensions_cm: {
      length: number;
      width: number;
      height: number;
    };
    description?: string;
  };
}
```

### 2.4 Tracking Milestone DTOs
```typescript
export interface TrackingMilestone {
  status: 'CREATED' | 'ASSIGNED' | 'PICKED_UP' | 'IN_TRANSIT' | 'OUT_FOR_DELIVERY' | 'DELIVERED' | 'ATTEMPT_FAILED' | 'CANCELLED' | 'RETURNED';
  timestamp: string; // ISO 8601 UTC
  location?: string; // City or district only (masked for public view)
  description?: string;
}

export interface PublicTrackingResponse {
  trackingNumber: string; // SD-YYYYMMDD-XXXXX
  status: string;
  lastUpdatedAt: string;
  history: TrackingMilestone[];
}
```

### 2.5 Merchant Integration DTOs
```typescript
export interface MerchantApiKeyMetadata {
  id: string; // UUID
  prefix: string; // e.g., "sd_live_a8f9..."
  label: string;
  createdAt: string;
  lastUsedAt: string | null;
}

export interface MerchantWebhookConfig {
  id: string; // UUID
  url: string;
  events: string[];
  status: 'ACTIVE' | 'DISABLED';
  createdAt: string;
}
```

---

## 3. Strict Storage Policies & Security Guardrails

| Data Classification | Storage Mechanism | Lifetime | Security Rule / Restriction |
| :--- | :--- | :--- | :--- |
| **JWT Access Token** | React `AuthContext` (In-Memory State) | 15 Minutes | **Zero Persistence.** Never stored in `localStorage`, `sessionStorage`, or cookies. Purged immediately on tab close or `logout`. |
| **Refresh Token** | Browser Cookie (`swiftdrop_refresh_token`) | 7 Days | `HttpOnly`, `Secure`, `SameSite=Strict`. Inaccessible to JavaScript. Handled exclusively by browser network stack. |
| **Merchant API Key (`sd_live_*`)** | Component State (Temporary Modal) | Destroyed on modal close | **One-time view only.** Never stored in browser storage, query cache, or debug logs. |
| **Webhook Secret (`whsec_*`)** | Component State (Temporary Modal) | Destroyed on modal close | **One-time view only.** Never stored in browser storage or transmitted back to frontend. |
| **Booking Form Drafts** | React `react-hook-form` State | In-Memory until submit | Discarded on navigation or page refresh. No PII persisted to browser storage by default. |
| **Server Query Cache** | TanStack Query (`queryClient`) | In-Memory (5 min TTL) | Scoped strictly to authenticated user/tenant ID. Wiped completely on `logout`. |
| **Currency Values** | Integer cents (`number` / `bigint`) | Transient | Always in Sri Lankan Rupees (LKR) integer minor units. Formatted using `formatLKR(cents)`. Zero floating-point arithmetic. |
| **Tracking Number** | Immutable Server String | Transient | Generated authoritatively by Member 2 upon parcel creation. Never generated or altered by frontend. |

---

## 4. Query Caching & Cache Invalidation Strategy

TanStack Query provides responsive navigation while preventing stale or unauthorized data leakage:

1. **Scoped Query Keys:**
   - Parcels: `['parcels', 'list', user.id, { page, status }]`
   - Parcel Details: `['parcels', 'detail', user.id, parcelId]`
   - Merchant Orders: `['merchant', 'orders', user.tenantId, { page }]`
   - Active API Keys: `['merchant', 'api-keys', user.tenantId]`
   - Webhook Configurations: `['merchant', 'webhooks', user.tenantId]`
2. **Mutation Invalidation:**
   - Creating a parcel booking invalidates `['parcels', 'list']`.
   - Creating or canceling a merchant order invalidates `['merchant', 'orders']`.
   - Revoking an API key or deleting a webhook invalidates their respective query caches.
3. **Session Termination Cleanup:**
   - Invoking `logout()` calls `queryClient.clear()`, wiping all cached queries, customer details, and merchant records instantly.

---

## 5. Currency & Financial Representation Standards

In accordance with SwiftDrop Engineering Standards:
- All delivery fees and financial amounts are transmitted and stored in **integer minor units** (Sri Lankan Rupee cents). For example, `LKR 650.00` is represented as integer `65000`.
- The frontend uses the shared utility `formatLKR(cents: number | bigint): string`:
  ```typescript
  export function formatLKR(cents: number | bigint): string {
    const rupees = Number(cents) / 100;
    return new Intl.NumberFormat('en-LK', {
      style: 'currency',
      currency: 'LKR',
      minimumFractionDigits: 2,
    }).format(rupees);
  }
  ```
- **Prohibition:** The frontend never performs floating-point calculations or fee arithmetic.
