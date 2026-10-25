# Document 6 — Testing Strategy & Acceptance Specification

> **Module:** Customer & Merchant Frontend  
> **Owning Member:** Sajini, Member 6 — Customer & Merchant Frontend  
> **Target Workspace:** `web/src/features/customer-merchant/` (Inside Unified React SPA)  
> **Status:** Draft Specifications Complete (Ready for Team Review in Meeting 2)  
> **Governing Standards:** SwiftDrop SRS, ADR-001, ADR-003, `AGENTS.md`, and `.agents/rules/frontend.md`  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Testing Philosophy & Honest Disclosure

As Member 6, I am committed to delivering an exceptionally reliable, secure, and thoroughly validated customer and merchant web experience. 

> [!IMPORTANT]
> **Status of Execution:** All test suites, scenarios, and assertions detailed in this document represent **planned specifications, test designs, and fixture contracts**. In accordance with `AGENTS.md` rules, I do not claim that application tests or coverage metrics have passed until the workspace has been implemented, dependencies installed, and test suites executed against live or containerized services.

---

## 2. Multi-Layered Testing Architecture

The frontend testing strategy is structured into three distinct layers:

```
Testing Pyramid:
├── End-to-End (E2E) Browser Journeys (Playwright: Critical User & Security Journeys)
├── Component & State Integration Tests (React Testing Library + Vitest)
└── Unit Tests (Vitest: Zod Schemas, Financial Utilities, Custom Hooks, Route Guards)
```

### 2.1 Unit Testing Layer (Vitest)
- **Financial Utilities:** Boundary testing of `formatLKR(cents)` (e.g., `0` &rarr; `"LKR 0.00"`, `65000` &rarr; `"LKR 650.00"`, very large values, negative checks).
- **Zod Validation Schemas:** Testing parcel dimensions (asserting rejections for `weight_kg > 50`, `length_cm > 200`, negative numbers, non-numeric inputs), phone number validation (Sri Lankan `+94` format and standard international), and email formatting.
- **Custom React Hooks:**
  - `useSSE`: Verifying initial state fetch, handling incoming `tracking-update` messages, asserting deduplication of duplicate event IDs, testing cleanup on unmount, and verifying fallback to bounded polling on repeated failures.
  - `useAuth`: Testing initial in-memory state, login token capture, transparent token refresh interception, and state clearing on `logout`.
  - `useBatchCSVParser`: Testing client-side CSV parsing, header verification, row validation, and syntax error flagging.
- **Route Guards:** Verifying `RequireAuth` and `RoleGuard` prevent unauthorized roles (e.g., redirecting an unauthenticated visitor to `/auth/login` or a `CUSTOMER` to `403 Forbidden` if attempting to access `/ops/dispatch`).

### 2.2 Component State Testing (React Testing Library)
Every data-driven component is tested across all five standard visual states:
1. **Loading State:** Asserting skeleton loaders mimic target layout without jarring layout shifts.
2. **Empty State:** Verifying intuitive iconography, explanatory text, and action buttons (e.g. empty merchant orders table displaying *"No orders yet. Book your first parcel."*).
3. **Validation Error State:** Asserting inline error text and highlighted borders on invalid form inputs.
4. **Server Error State:** Verifying error alert banners display user-friendly messages alongside the backend `correlation_id`.
5. **Success State:** Confirming visual feedback toasts, checkmark indicators, and tracking number display cards.

---

## 3. End-to-End (E2E) Testing with Playwright

Playwright tests are housed in `web/tests/e2e/` utilizing the mandatory Page Object Model (POM) pattern. Spec files contain high-level user actions and assertions, delegating DOM selections to page objects.

### 3.1 Critical Journey Specs

#### Journey 1: Customer Booking & Quote Countdown (`customer-booking.spec.ts`)
1. Customer logs in (`customer@example.com`).
2. Navigates to `/portal/book`.
3. Fills Step 1 addresses (Sender pre-filled, Recipient: Kamal, +94719876543, Kandy).
4. Fills Step 2 dimensions (2.5 kg, 25 x 15 x 10 cm, Zone: Colombo to Kandy).
5. Advances to Step 3: Verifies quote fee displays `"LKR 650.00"` (`data-testid="quote-price-cents"`), asserts 15-minute countdown is ticking.
6. Advances to Step 4: Clicks submit button (`data-testid="booking-submit-btn"`).
7. Asserts confirmation screen renders with tracking number format `SD-YYYYMMDD-XXXXX` (`data-testid="booking-success-tracking-number"`).
8. Clicks *"Track Live Shipment"*: verifies smooth navigation to `/track/SD-...`.

#### Journey 2: Public Tracking Privacy Masking (`public-tracking.spec.ts`)
1. An anonymous browser context (zero cookies, zero JWT) visits `/track/SD-20261025-00481`.
2. Verifies status badge displays `"PICKED_UP"` (`data-testid="tracking-status-badge"`).
3. Verifies milestone timeline is visible (`data-testid="tracking-milestone-list"`).
4. **Security Assertion:** Asserts recipient full street address, recipient phone number, sender contact details, and proof photos are completely absent from the rendered DOM.

#### Journey 3: Merchant API Key & Webhook Modal One-Time View (`merchant-credentials.spec.ts`)
1. Approved merchant logs in (`merchant@example.com`).
2. Navigates to `/portal/merchants/api-keys`.
3. Clicks `"Generate New API Key"`, inputs label `"Production-Shopify"`, and submits.
4. Asserts modal appears (`data-testid="api-key-modal"`) displaying raw key (`sd_live_...`).
5. Asserts warning banner is present. Copies key. Checks *"I have safely copied..."* checkbox.
6. Dismisses modal.
7. **Security Assertion:** Inspects `localStorage`, `sessionStorage`, and query cache—asserts raw key `sd_live_...` is **strictly absent** from all client storage.

#### Journey 4: Merchant CSV Batch Upload (`merchant-batch-upload.spec.ts`)
1. Merchant opens `/portal/merchants/orders`.
2. Uploads mock CSV file (`tests/fixtures/sample_orders.csv`, 5 valid rows) via `data-testid="merchant-csv-input"`.
3. Verifies preview table displays 5 green ready badges.
4. Submits batch. Verifies progress indicator advances from 0% to 100%.
5. Asserts 5 new orders appear in the orders table.

---

## 4. Test ID Cross-Module Coordination Matrix (Member 6 & Member 10)

To eliminate brittle selectors and reconcile any naming differences with Member 10's QA suite, the following dual-attribute strategy is implemented:

| Component / Action | Primary `data-testid` | Member 10 Alias | Description |
| :--- | :--- | :--- | :--- |
| **Login Email** | `login-email-input` | `login-email-input` | User login email |
| **Login Password** | `login-password-input`| `login-password-input`| User login password |
| **Login Button** | `login-submit-btn` | `login-submit-btn` | Auth submit button |
| **Booking Weight** | `booking-weight-input`| `booking-weight-input`| Weight input in kg |
| **Booking Dimensions** | `booking-dims-input` | `booking-dims-input` | Dimensions container |
| **Quote Button** | `booking-estimate-btn`| `booking-estimate-btn`| Triggers quote calculation |
| **Quote Fee Display** | `quote-price-cents` | `booking-estimated-fee-display` | Quoted delivery fee in LKR |
| **Confirm Booking** | `booking-submit-btn` | `booking-submit-btn` | Order submission button |
| **Success Tracking #**| `booking-success-tracking-number` | `booking-success-tracking-number` | Returned tracking code |
| **Tracking Input** | `tracking-search-input`| `tracking-search-input`| Search input field |
| **Tracking Status** | `tracking-status-badge`| `public-tracking-status-badge` | Status badge |
| **Milestone List** | `tracking-milestone-list`| `tracking-milestone-list`| Ordered timeline items |
| **CSV File Input** | `merchant-csv-input` | `merchant-csv-input` | Batch CSV upload input |
| **API Key Modal** | `api-key-modal` | `api-key-modal` | One-time key modal |
| **Webhook Modal** | `webhook-secret-modal`| `webhook-secret-modal`| One-time secret modal |

---

## 5. Planned Verification Scripts & CI Integration

All frontend verification scripts are driven through `npm` from the `web/` directory:

```bash
# Type-checking and code formatting
npm run lint

# Unit and component tests with code coverage
npm run test:unit
npm run test:unit:coverage

# Playwright E2E journeys (headless)
npm run test:e2e

# Production build verification (outputs dist/)
npm run build
```

### 5.1 CI Pipeline Integration (Coordinated with Member 9)
In `.github/workflows/ci.yml`, Member 9 includes a dedicated `web-checks` job executing:
1. `npm ci` (clean install from lockfile).
2. `npm run lint` & `npm run build`.
3. `npm run test:unit`.
4. Playwright E2E execution against containerized backend services.
