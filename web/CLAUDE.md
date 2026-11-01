# SwiftDrop — Claude Code Frontend Instructions (`web/`)

> **Primary Directive:** Always read and strictly adhere to `AGENTS.md` at the repository root and `web/README.md` before making any frontend changes.

---

## 1. Quick Reference & Commands

```bash
# Navigate to frontend workspace
cd web

# Install dependencies
npm install

# Start development server (HMR on http://localhost:3000)
npm run dev

# Run TypeScript compilation check and ESLint
npm run lint

# Run unit and component test suite (Vitest + React Testing Library)
npm run test:unit

# Run unit tests with code coverage report
npm run test:unit:coverage

# Run End-to-End browser tests (Playwright)
npm run test:e2e

# Run Playwright tests with interactive UI mode
npx playwright test --ui

# Build production bundle with minification
npm run build

# Preview production build locally
npm run preview
```

---

## 2. Capability Ownership & Feature Boundaries

- **Member 6 (Customer & Merchant Portal):** `src/features/customer-merchant/`
  - Customer registration/login & Google OIDC.
  - Multi-step parcel booking wizard & fee quote countdown preview.
  - Customer parcel history, tracking timeline, and printable receipts.
  - Real-time SSE live milestone updates (`useSSE`) with polling fallback.
  - Merchant dashboard, batch CSV parcel creation, API key modal, and webhook settings.
- **Member 7 (Dispatcher/Admin Backoffice & Courier Mobile):** `src/features/dispatcher-admin/`
  - Interactive dispatch board: unassigned parcel queue, courier map/roster, assignment.
  - Active courier fleet management, vehicle load capacity, and shift toggle.
  - Mobile-first courier task run (`/courier/tasks`), barcode scanner, and POD capture (photo/signature).
  - Failed delivery reporting with standardized reason codes (`RECIPIENT_UNAVAILABLE`, `INVALID_ADDRESS`, etc.).
  - Admin management: delivery zones, base tariffs, pricing rules, user/merchant approval, and SLA reports.
- **Shared Shell & UI System:** `src/app/` (routes, guards, layouts, providers) and `src/shared/` (components, API client, hooks, types, utils).
  - **Strict Rule:** Never import across feature directories (`customer-merchant` vs. `dispatcher-admin`). Shared elements belong exclusively in `src/shared/`.

---

## 3. Inviolable Domain Invariants & Rules

1. **Zero Client-Side Fee Calculations:**
   - Frontend **NEVER** computes delivery fees, base charges, weight surcharges, or volumetric weight independently.
   - All quotes originate authoritatively from `POST /api/v1/pricing/quote`.
   - The returned `quote_id` (15-min validity) must be submitted with `POST /api/v1/parcels`.
2. **Zero Client-Side State Transitions:**
   - Frontend **NEVER** decides or enforces valid parcel lifecycle transitions. The backend `Parcel` aggregate root is the single source of truth.
3. **Dual-Token Authentication Security:**
   - Access token (JWT, 15m lifetime) kept **in-memory only** within `AuthContext`.
   - **Prohibition:** NEVER store access tokens or refresh tokens in `localStorage` or `sessionStorage`.
   - Refresh token stored in secure `HttpOnly`, `SameSite=Strict`, `Secure` cookie.
   - Transparent mutex-locked token refresh on `401 Unauthorized` responses.
4. **Public Tracking Privacy Masking (SEC-016):**
   - The public tracking view (`/track/:trackingNumber`) must **NEVER** display recipient phone numbers, full street addresses, sender details, proof images, or internal notes.
   - Only milestone statuses, timestamps, and city/province locations may be displayed publicly.
5. **Idempotency Guarantee:**
   - All non-idempotent mutation requests (parcel booking, batch creation) must inject `Idempotency-Key: <uuid-v4>`.
6. **Financial Minor Units:**
   - All money is represented in Sri Lankan Rupees (LKR) integer cents (`bigint` / minor units).
   - Format cents into decimal display using the shared utility `formatLKR(cents)` (e.g. `45000` &rarr; `LKR 450.00`). Never perform floating-point arithmetic on currency.

---

## 4. Unit & Component Testing Rules (Vitest + React Testing Library)

- **Test File Co-location:** Unit and component tests must be co-located with their subject (`<Component>.test.tsx` or `__tests__/`).
- **Framework:** Use Vitest with `@testing-library/react` and `@testing-library/jest-dom`.
- **Mandatory Test Coverage:**
  1. **Domain Utilities:** Financial formatters (`formatLKR`), dimension/date helpers, and Zod validation schemas.
  2. **Custom Hooks:** `useAuth` (login, logout, token refresh), `useSSE` (event parsing, reconnect handling), `useDebounce`.
  3. **Route Guards:** Verify `RequireAuth`, `RoleGuard`, and `GuestOnly` allow or redirect appropriate roles.
  4. **Component States:** Explicitly test `Loading` skeleton, `Empty` state, `Validation Error` messages, and `Server Error` banners.
- **Assertion Integrity:** Never delete, weaken, or mock away invariant assertions to force passing builds.

---

## 5. End-to-End (E2E) Testing Rules (Playwright)

- **Directory Structure:** All E2E tests reside in `web/tests/e2e/`.
  ```
  web/tests/e2e/
  ├── fixtures/                       # Authentication and role setup fixtures
  ├── pages/                          # Page Object Models (POM)
  │   ├── LoginPage.ts
  │   ├── BookingWizardPage.ts
  │   ├── TrackingPage.ts
  │   ├── DispatchBoardPage.ts
  │   └── CourierTasksPage.ts
  └── specs/                          # Journey spec files
      ├── auth.spec.ts
      ├── customer-booking.spec.ts
      ├── public-tracking.spec.ts
      ├── dispatch-assignment.spec.ts
      ├── courier-pod.spec.ts
      └── delivery-failure.spec.ts
  ```
- **Page Object Model (POM):** All UI interactions must be encapsulated inside dedicated Page Object classes in `tests/e2e/pages/`. Never write raw, repetitive page selector logic inside spec files.
- **Resilient Selectors:** Use semantic user-facing locators (`getByRole`, `getByLabelText`, `getByText`) or explicit test identifiers (`getByTestId('quote-price-cents')`). Avoid brittle CSS class hierarchies.
- **Mandatory Critical Journeys:**
  - **Journey 1: Customer Booking:** Login &rarr; Enter dimensions/weight &rarr; Verify live quote preview from API (15m countdown) &rarr; Submit booking &rarr; Verify generated tracking number & receipt view.
  - **Journey 2: Public Tracking Redaction:** Visit `/track/:trackingNumber` without logging in &rarr; Verify milestone timeline renders &rarr; Assert recipient phone, full street address, and proof images are completely redacted.
  - **Journey 3: Dispatch & Courier POD Run:** Dispatcher assigns parcel to courier &rarr; Courier logs in on mobile viewport &rarr; Marks picked up &rarr; Captures signature & photo &rarr; Verify status moves to `DELIVERED`.
  - **Journey 4: Delivery Failure & Reattempt:** Courier reports failed attempt with reason code (`RECIPIENT_UNAVAILABLE`) &rarr; Verify status transitions to `ATTEMPT_FAILED` &rarr; Dispatcher reviews attempt count (1 of 3) &rarr; Authorizes reattempt.
  - **Journey 5: Security & Role Boundaries:** Verify `ROLE_CUSTOMER` accessing `/ops/dispatch` or `/ops/zones` is redirected to 403 Forbidden.

---

## 6. Mandatory Workflow & Completion Report

1. Inspect `AGENTS.md`, `web/README.md`, and relevant contracts before writing code.
2. Formulate and state an implementation plan.
3. Make the smallest complete change without crossing domain boundaries.
4. Run tests (`npm run lint`, `npm run test:unit`, `npm run test:e2e`).
5. Synchronously update `web/README.md` if routes or contracts change.
6. Provide the structured Task Completion Report defined in `AGENTS.md`.
