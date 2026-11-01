# Member 06 — Customer & Merchant Frontend Alignment Guide

> **Author / Domain Owner:** Member 6 — Customer & Merchant Frontend (Sajini)  
> **Target Directory:** `web/src/features/customer-merchant/` (Inside Unified SPA)  
> **Status:** Authoritative Architectural Alignment & Action Sheet for Team Meeting 2  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Domain Overview & Scope of Work

Member 6 owns the primary customer-facing and merchant-facing web experiences within the unified React Single-Page Application (SPA): **The Customer Booking Wizard**, **The Live Shipment Tracker**, and **The Merchant Self-Service Portal**.

### 1.1 In-Scope Responsibilities
- Customer self-registration and login forms (`/auth/register`, `/auth/login`).
- Multi-step parcel booking wizard (`/portal/book`) with live fee quotation.
- Interactive public and authenticated tracking views (`/track/:trackingNumber` and `/portal/tracking`).
- Real-time milestone updates using a custom Server-Sent Events hook (`useSSE`).
- Merchant portal views (`/portal/merchants/orders`, `/portal/merchants/api-keys`, `/portal/merchants/webhooks`).
- CSV batch order upload UI for merchants.
- Display modal for newly generated API keys (`sd_live_*`) and webhook secrets (`whsec_*`) that warns users they will never be displayed again.
- Setting explicit `data-testid` attributes on all form inputs, buttons, and status badges for Member 10's Playwright E2E test suite.

### 1.2 Out-of-Scope Responsibilities
- Dispatcher board, fleet management, and admin screens (Owned by Member 7).
- Courier mobile task list and POD photo capture (Owned by Member 7).
- Traefik API Gateway and Nginx container configuration (Owned by Member 9).
- Calculating delivery fees on the client side (Forbidden by Engineering Standards).

---

## 2. Document Audit & Current State

- **Current Status in `docs/drafts/`:** Only a basic checklist `README.md` was submitted. Full 8-document suite is pending.
- **Root Codebase Baseline:** Architectural blueprint established in `web/README.md`.
- **Immediate Action:** Member 6 must author `doc-01` through `doc-08` using the contracts and UI workflows defined below.

---

## 3. Core UI Workflows & State Machine

### 3.1 Multi-Step Booking Wizard (`/portal/book`)
The booking wizard is structured into 4 sequential steps with client-side form validation using `react-hook-form` and `zod`:
1. **Step 1: Addresses:** Sender address (defaults to saved profile) and Recipient address + phone number.
2. **Step 2: Parcel Dimensions:** Weight (kg), Length, Width, Height (cm), and pickup/dropoff zone selection.
3. **Step 3: Instant Fee Quote:** Invokes `POST /api/v1/pricing/quotes`. Displays breakdown: base fee, chargeable weight, and total amount in LKR. Caches `quote_id`.
4. **Step 4: Confirmation & Idempotent Submission:**
   - Client generates a UUIDv4 idempotency key.
   - Dispatches `POST /api/v1/parcels` with `Idempotency-Key` header and `{ "quote_id": "...", ... }`.
   - On success, displays confirmation screen with generated tracking number (`SD-YYYYMMDD-XXXXX`) and instant link to tracker.

### 3.2 Real-Time Live Tracking Hook (`useSSE`)
```typescript
export function useSSE(trackingNumber: string) {
  const [status, setStatus] = useState<string | null>(null);
  const [milestones, setMilestones] = useState<Milestone[]>([]);

  useEffect(() => {
    if (!trackingNumber) return;
    const es = new EventSource(`/api/v1/tracking/${trackingNumber}/stream`);

    es.addEventListener('status_update', (event) => {
      const data = JSON.parse(event.data);
      setStatus(data.current_status);
      setMilestones((prev) => [...prev, data]);
    });

    es.onerror = () => {
      // Automatic browser reconnect handled by EventSource
    };

    return () => es.close();
  }, [trackingNumber]);

  return { status, milestones };
}
```

---

## 4. Playwright E2E Integration Standards (`data-testid`)

To ensure Member 10's automated E2E tests pass reliably without brittle CSS selectors, Member 6 must attach the following standard attributes:

| Element / Component | Required `data-testid` | Description |
| :--- | :--- | :--- |
| Login Email Input | `data-testid="login-email-input"` | User login form |
| Login Password Input | `data-testid="login-password-input"`| User login form |
| Login Submit Button | `data-testid="login-submit-btn"` | Trigger auth |
| Booking Weight Input | `data-testid="booking-weight-input"`| Weight in kg |
| Booking Dimensions | `data-testid="booking-dims-input"` | L x W x H inputs |
| Quote Price Display | `data-testid="quote-price-cents"` | Displays calculated fee |
| Booking Submit Button | `data-testid="booking-submit-btn"` | Confirm booking |
| Tracking Input Field | `data-testid="tracking-search-input"`| Public tracker input |
| Tracking Status Badge | `data-testid="tracking-status-badge"`| Current status display |
| Milestone Timeline List| `data-testid="tracking-milestone-list"`| Milestone items |

---

## 5. Member 9 DevOps & Infrastructure Configuration

| Config Parameter | Required Specification | Alignment Rule |
| :--- | :--- | :--- |
| **Container & Port** | Static assets served by Nginx on port `:3000` | Agreement A1 & A12 |
| **Dockerfile & Nginx** | Owned and maintained by Member 9 | Agreement A3 |
| **API Base URL** | Driven by environment variable `VITE_API_BASE_URL` (default `/api/v1`) | Agreement A4 & A12 |
| **Build Command** | `npm run build` (Outputs optimized bundle to `dist/`) | Agreement A12 |
| **Traefik Route** | Traefik routes `/`, `/portal/*` to `web:3000` | Agreement A9 |

---
*End of Member 06 Specification & Alignment Guide.*
