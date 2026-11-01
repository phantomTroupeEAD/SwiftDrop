# Document 7 — Security, Failure & Recovery Specification

> **Module:** Customer & Merchant Frontend  
> **Owning Member:** Sajini, Member 6 — Customer & Merchant Frontend  
> **Target Workspace:** `web/src/features/customer-merchant/` (Inside Unified React SPA)  
> **Status:** Draft Specifications Complete (Ready for Team Review in Meeting 2)  
> **Governing Standards:** SwiftDrop SRS, ADR-001, ADR-003, `AGENTS.md`, and `.agents/rules/frontend.md`  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Security Architecture & Threat Mitigations

Security in the SwiftDrop web application operates on the principle of **zero client trust**: while the frontend enforces thorough UX-level validations and privacy safeguards, authoritative enforcement resides strictly within the backend API and database layer.

### 1.1 Dual-Token Authentication Security (ADR-003)
- **In-Memory JWT Access Token:** The 15-minute access token is stored **exclusively in transient React memory** (`AuthContext`). It is never written to `localStorage`, `sessionStorage`, or custom browser cookies. This eliminates Cross-Site Scripting (XSS) token harvesting attacks.
- **HttpOnly Refresh Cookie:** The refresh token is transmitted in an `HttpOnly`, `SameSite=Strict`, `Secure` cookie named `swiftdrop_refresh_token`. JavaScript has zero access to this cookie, mitigating token exfiltration.
- **Transparent Refresh Coordination:** When the HTTP client encounters `401 Unauthorized`, it queues pending requests and dispatches a single call to `POST /api/v1/auth/refresh`. If refresh succeeds, the queued requests are retried with the new in-memory token. If refresh fails (token expired, reused, or revoked), the frontend purges all memory caches and redirects the user to `/auth/login?session_expired=true`.

### 1.2 Public Tracking Privacy Masking (SEC-016)
- **Public Disclosure Rule:** Anyone possessing a valid tracking number can access `/track/:trackingNumber`.
- **Masking Enforcement:** The frontend renders only approved high-level fields: current status, event timestamps, origin city, and destination city/province.
- **Strict Redaction:** Recipient full street addresses, recipient phone numbers, sender details, courier identities, internal operational notes, and proof-of-delivery attachments are **strictly excluded** from the public DOM. Even if a user inspects network responses, Member 5's Tracking API omits these fields server-side.

### 1.3 Merchant Credential & Secret Protection
- **One-Time Display Modal:** When an approved merchant generates an API key (`sd_live_*`) or registers a webhook secret (`whsec_*`), the backend transmits the plaintext secret exactly once.
- **Zero Client Persistence:** The frontend renders a dedicated modal dialog warning the user that the secret will never be displayed again. The secret is kept in transient local component state and is destroyed when the modal closes. It is **never** saved to `localStorage`, `sessionStorage`, cookies, query caches, or debug logs.
- **Separation of Concerns:** The browser never sends `X-API-Key` headers. Human merchant users authenticate using standard JWT Bearer sessions (`tenant_id` claim), leaving API keys strictly for backend automation.

### 1.4 Protection Against Fee & Transition Tampering
- **No Client Fee Calculations:** Delivery fees are never calculated in the browser. The frontend dispatches package dimensions and zones to `POST /api/v1/pricing/quotes` and submits the resulting `quote_id` with `POST /api/v1/parcels`. The backend re-evaluates pricing authoritatively, preventing fee manipulation via DOM editing or URL tampering.
- **No Client State Transitions:** The frontend never decides or submits parcel status transitions; lifecycle state changes are evaluated exclusively by Member 2's Go aggregate root.

---

## 2. Comprehensive Failure-Mode Matrix

| Failure Mode | Root Cause / Trigger | Frontend User Experience | Recovery / Mitigation Strategy |
| :--- | :--- | :--- | :--- |
| **Pricing API Down** | Core API pricing service unavailable (503) | Step 3 displays warning banner: *"Live fee quote unavailable. Please retry in a few moments."* | User clicks *"Retry Quote"*. Form draft is preserved in memory; no fake fee is generated. |
| **Booking Request Timeout** | Network drop or slow gateway during `POST /parcels` | Alert banner: *"We could not confirm your booking. Please click retry."* | User clicks retry: the request is re-dispatched with the **exact same UUIDv4 `Idempotency-Key`**, ensuring zero duplicate charges. |
| **Quote Expiry (15 min)** | User delays on confirmation screen past quote TTL | Modal prompt: *"Your guaranteed fee quote has expired. Click to refresh quote."* | Wizard invalidates stale `quote_id`, invokes `POST /pricing/quotes`, updates countdown, and requires re-confirmation. |
| **Session Expired (401)** | 15-min JWT expires; refresh token invalid/expired | User is gently redirected to `/auth/login?session_expired=true` with toast notification | Transparent refresh attempted once. On failure, query cache is cleared to prevent data leaks. |
| **Forbidden Access (403)** | Customer attempts to view another user's parcel or `/ops/*` | Displays `403 Access Denied` illustration with *"Return to Portal"* button | Router blocks access; API enforces tenant/ownership isolation. No repeated retries. |
| **Tracking Not Found (404)** | Visitor enters non-existent or invalid tracking number | Clear *"Shipment Not Found"* screen with search bar to verify input | Distinguishes cleanly from network outage. Does not leak whether a private order exists. |
| **Rate Limit Exceeded (429)**| Excessive quote requests or search queries | Toast / banner: *"Too many requests. Please wait X seconds before trying again."* | Inspects `Retry-After` header and temporarily disables submit button until interval elapses. |
| **SSE Stream Disconnected** | Network interruption or pod restart on Tracking API | Live indicator turns amber (`"Reconnecting..."`) | EventSource automatically reconnects. Hook triggers background REST fetch to resync authoritative state. |
| **Repeated SSE Failure** | Persistent WebSocket/SSE gateway error (>3 drops) | Live indicator displays `"Updates paused"`; manual refresh button appears | `useSSE` seamlessly falls back to bounded polling (every 15s, max 20 cycles) to protect server. |
| **Receipt Download Fails** | Receipt service temporarily unavailable | Error toast: *"Receipt download failed. Please try again."* | User can retry authorized `GET /parcels/{id}/receipt` without re-booking. |
| **Batch CSV Syntax Errors** | Merchant uploads CSV with missing columns or invalid phones | Preview table flags invalid rows in red with specific cell tooltips | Disables batch submit until merchant uploads a corrected file. |

---

## 3. Data Sanitization & Logging Guardrails

1. **Zero Sensitive Data in Logs:** Form state, user passwords, credit card details, merchant API keys, webhook secrets, and customer PII are strictly barred from `console.log`, browser error tracking, and analytics beacons.
2. **Support Correlation ID:** When displaying server errors, the frontend extracts `error.correlation_id` from the backend error envelope and renders: *"Reference ID: req-98f2a1b7 (Quote this to customer support)"*. This enables instant tracing in Jaeger/Loki logs by Members 7 and 9 without exposing internal stack traces.
3. **DOM Content Security:** All dynamic strings (names, addresses, parcel notes) are rendered as plain text nodes or sanitized React JSX elements, preventing Cross-Site Scripting (XSS) via injected HTML tags.
