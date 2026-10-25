# Document 3 — Internal Architecture & Design Specification

> **Module:** Customer & Merchant Frontend  
> **Owning Member:** Sajini, Member 6 — Customer & Merchant Frontend  
> **Target Workspace:** `web/src/features/customer-merchant/` (Inside Unified React SPA)  
> **Status:** Draft Specifications Complete (Ready for Team Review in Meeting 2)  
> **Governing Standards:** SwiftDrop SRS, ADR-001, ADR-003, `AGENTS.md`, and `.agents/rules/frontend.md`  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Architectural Approach & Technology Stack

The Customer & Merchant Frontend is implemented as a cohesive, modular feature package inside the unified SwiftDrop web application (`web/`). It adheres strictly to the single-page application (SPA) architecture established in the repository rulebook.

### 1.1 Core Technology Stack
- **Framework & Runtime:** React 18+ with TypeScript in strict mode (`"strict": true` in `tsconfig.json`, zero `any` policy).
- **Build & Development Tooling:** Vite 5+ (fast HMR, ES module bundling, outputting to `dist/`).
- **Styling & Design System:** Tailwind CSS with modern design tokens (Slate neutral base, Indigo/Electric Violet primary accents, semantic status indicators, glassmorphism `backdrop-blur-md`, subtle borders, and 150ms micro-animations).
- **Asynchronous Server State:** TanStack Query v5 (`@tanstack/react-query`) for caching, background refetching, and mutation invalidation.
- **Routing & Navigation:** React Router v6 (`react-router-dom`) with declarative route guards.
- **Form Management & Validation:** `react-hook-form` paired with `zod` for declarative, schema-driven client-side validation.

---

## 2. Directory Layout & Module Boundaries

All code authored by Member 6 resides exclusively within `src/features/customer-merchant/`. Shared application infrastructure is maintained collaboratively with Member 7 in `src/app/` and `src/shared/`. Direct imports between `src/features/customer-merchant/` and `src/features/dispatcher-admin/` are strictly prohibited.

```text
web/src/
├── app/                                    # Shared Application Shell (Member 6 & 7 co-owned)
│   ├── routes/                             # App-wide route definitions
│   ├── layouts/                            # PortalLayout, PublicLayout, AuthLayout
│   ├── guards/                             # RequireAuth, RoleGuard, GuestOnly
│   └── providers/                          # AuthProvider, QueryClientProvider, ToastProvider
├── shared/                                 # Shared Infrastructure (Member 6 & 7 co-owned)
│   ├── api/                                # Axios/fetch client with interceptors
│   ├── components/                         # Button, Input, Modal, Badge, Skeleton, ErrorAlert
│   ├── hooks/                              # useDebounce, useToast, useMediaQuery
│   ├── types/                              # Common API envelope, user principal DTOs
│   └── utils/                              # formatLKR(cents), date helpers
└── features/customer-merchant/             # Member 6 Isolated Domain
    ├── auth/                               # Registration & customer login adapters
    │   ├── components/                     # RegisterForm.tsx, CustomerLoginForm.tsx
    │   └── hooks/                          # useCustomerAuth.ts
    ├── booking/                            # Four-step booking wizard
    │   ├── components/                     # Step1Addresses.tsx, Step2Dimensions.tsx,
    │   │                                   # Step3QuotePreview.tsx, Step4Confirmation.tsx,
    │   │                                   # BookingSuccessCard.tsx
    │   ├── hooks/                          # useBookingWizard.ts, usePricingQuote.ts
    │   └── schemas/                        # bookingValidationSchema.ts
    ├── tracking/                           # Public & authenticated shipment tracking
    │   ├── components/                     # PublicTrackingView.tsx, TrackingTimeline.tsx,
    │   │                                   # MilestoneBadge.tsx, LiveStatusIndicator.tsx
    │   └── hooks/                          # useSSE.ts (Authoritative live stream hook)
    └── merchant/                           # Merchant self-service portal
        ├── components/                     # MerchantDashboard.tsx, OrdersTable.tsx,
        │                                   # CSVBatchUploadModal.tsx, ApiKeyManagement.tsx,
        │                                   # ApiKeyOneTimeModal.tsx, WebhookManagement.tsx,
        │                                   # WebhookSecretOneTimeModal.tsx
        └── hooks/                          # useMerchantOrders.ts, useBatchCSVParser.ts,
                                            # useMerchantCredentials.ts
```

---

## 3. State Management Philosophy

1. **Local Form State:** Managed locally using `react-hook-form`. Forms retain inputs during client-side validation failures and discard temporary drafts upon unmount unless explicitly saved.
2. **Server State (TanStack Query):** All external data (quotes, parcel lists, tracking lookups, merchant orders) is managed by TanStack Query. Query keys are strictly scoped by user ID or tenant ID (e.g., `['parcels', 'list', userId, { page, status }]`). Private caches are wiped completely on `logout`.
3. **Session & Auth State (In-Memory React Context):** The 15-minute JWT access token is retained **exclusively in memory** within `AuthContext`. It is never serialized to `localStorage` or `sessionStorage`. Background token refresh on `401 Unauthorized` is orchestrated transparently by the shared HTTP client.

---

## 4. Robust Real-Time Live Tracking Hook (`useSSE`)

To fulfill the requirements of both the Alignment Guide and Member 5's tracking contract, I have designed a production-grade `useSSE` hook. It avoids the brittleness of standard browser EventSource implementations by providing initial state synchronization, event deduplication, automatic reconnect re-fetching, and bounded polling fallback.

### 4.1 TypeScript Implementation Specification

```typescript
import { useState, useEffect, useRef, useCallback } from 'react';

export type SSEConnectionState = 
  | 'idle' 
  | 'connecting' 
  | 'connected' 
  | 'reconnecting' 
  | 'polling_fallback' 
  | 'error';

export interface Milestone {
  status: string;
  timestamp: string;
  location?: string;
  description?: string;
}

export interface TrackingData {
  trackingNumber: string;
  currentStatus: string;
  lastUpdatedAt: string;
  milestones: Milestone[];
}

export interface UseSSEResult {
  data: TrackingData | null;
  connectionState: SSEConnectionState;
  error: string | null;
  refetchAuthoritative: () => Promise<void>;
}

export function useSSE(trackingNumber: string | null): UseSSEResult {
  const [data, setData] = useState<TrackingData | null>(null);
  const [connectionState, setConnectionState] = useState<SSEConnectionState>('idle');
  const [error, setError] = useState<string | null>(null);

  const eventSourceRef = useRef<EventSource | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const pollingIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const processedEventIdsRef = useRef<Set<string>>(new Set());
  const failureCountRef = useRef<number>(0);

  // 1. Authoritative REST Fetch (Member 5 Contract: GET /api/v1/tracking/public/{trackingNumber})
  const fetchAuthoritativeState = useCallback(async () => {
    if (!trackingNumber) return;
    
    // Abort prior in-flight fetch
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    abortControllerRef.current = new AbortController();

    try {
      const res = await fetch(`/api/v1/tracking/public/${encodeURIComponent(trackingNumber)}`, {
        signal: abortControllerRef.current.signal,
        headers: { 'Accept': 'application/json' },
      });

      if (!res.ok) {
        if (res.status === 404) throw new Error('Shipment tracking number not found');
        throw new Error(`Failed to load tracking data (HTTP ${res.status})`);
      }

      const initialData = await res.json();
      setData({
        trackingNumber: initialData.trackingNumber,
        currentStatus: initialData.status,
        lastUpdatedAt: initialData.lastUpdatedAt,
        milestones: initialData.history || [],
      });
      setError(null);
    } catch (err: any) {
      if (err.name !== 'AbortError') {
        setError(err.message || 'Unable to load shipment information');
      }
    }
  }, [trackingNumber]);

  // 2. Bounded Polling Fallback Mechanism (Every 15 seconds, max 20 cycles)
  const startPollingFallback = useCallback(() => {
    if (pollingIntervalRef.current) clearInterval(pollingIntervalRef.current);
    setConnectionState('polling_fallback');
    
    let cycles = 0;
    pollingIntervalRef.current = setInterval(async () => {
      cycles += 1;
      if (cycles > 20) { // Cease polling after 5 minutes of inactivity to protect server
        if (pollingIntervalRef.current) clearInterval(pollingIntervalRef.current);
        setConnectionState('error');
        setError('Live updates suspended. Click refresh to check status.');
        return;
      }
      await fetchAuthoritativeState();
    }, 15000);
  }, [fetchAuthoritativeState]);

  // 3. Main Stream Lifecycle Management
  useEffect(() => {
    // Reset state when trackingNumber changes
    setData(null);
    setError(null);
    processedEventIdsRef.current.clear();
    failureCountRef.current = 0;

    if (pollingIntervalRef.current) {
      clearInterval(pollingIntervalRef.current);
      pollingIntervalRef.current = null;
    }

    if (!trackingNumber) {
      setConnectionState('idle');
      return;
    }

    // Step A: Load authoritative snapshot immediately
    setConnectionState('connecting');
    fetchAuthoritativeState();

    // Step B: Establish SSE Connection to Member 5's public stream endpoint
    const streamUrl = `/api/v1/tracking/public/${encodeURIComponent(trackingNumber)}/stream`;
    const es = new EventSource(streamUrl);
    eventSourceRef.current = es;

    es.onopen = () => {
      setConnectionState('connected');
      failureCountRef.current = 0;
    };

    // Step C: Listen to status update events
    es.addEventListener('tracking-update', (event: MessageEvent) => {
      try {
        // Event Deduplication: check event ID or construct compound key
        const eventId = event.lastEventId || event.data;
        if (eventId && processedEventIdsRef.current.has(eventId)) {
          return; // Ignore duplicate delivery
        }
        if (eventId) processedEventIdsRef.current.add(eventId);

        const payload = JSON.parse(event.data);
        setData((prev) => {
          if (!prev) return prev;
          const newMilestone: Milestone = {
            status: payload.status,
            timestamp: payload.lastUpdatedAt,
            location: payload.location,
            description: payload.description,
          };
          return {
            ...prev,
            currentStatus: payload.status,
            lastUpdatedAt: payload.lastUpdatedAt,
            milestones: [...prev.milestones, newMilestone],
          };
        });
      } catch (parseErr) {
        console.error('Failed to parse incoming SSE message:', parseErr);
      }
    });

    // Step D: Reconnection & Failure Handler
    es.onerror = () => {
      failureCountRef.current += 1;
      setConnectionState('reconnecting');

      // If reconnecting repeatedly (>3 failures), fall back to bounded REST polling
      if (failureCountRef.current >= 3) {
        es.close();
        eventSourceRef.current = null;
        startPollingFallback();
      } else {
        // Re-fetch authoritative state upon reconnect to prevent lost updates
        fetchAuthoritativeState();
      }
    };

    // Step E: Component Unmount / Navigation Cleanup
    return () => {
      if (es) es.close();
      if (abortControllerRef.current) abortControllerRef.current.abort();
      if (pollingIntervalRef.current) clearInterval(pollingIntervalRef.current);
    };
  }, [trackingNumber, fetchAuthoritativeState, startPollingFallback]);

  return {
    data,
    connectionState,
    error,
    refetchAuthoritative: fetchAuthoritativeState,
  };
}
```

---

## 5. Playwright E2E Integration Standards & Test ID Matrix

To guarantee that Member 10's automated Playwright end-to-end tests run reliably without brittle CSS or XPath selectors, I will embed the required semantic `data-testid` attributes across all customer and merchant components.

### 5.1 Test ID Inventory & Cross-Module Coordination with Member 10

Where the Alignment Guide and Member 10's pre-existing test specifications used slightly different attribute naming or route paths, I have established a **dual-support strategy** (aliasing attributes or supporting both) to ensure 100% interoperability.

| UI Element / Component | Alignment Guide `data-testid` | Member 10 Spec `data-testid` | Implemented Attribute Strategy | Description |
| :--- | :--- | :--- | :--- | :--- |
| **Login Email Input** | `login-email-input` | `login-email-input` | `data-testid="login-email-input"` | Customer & merchant login email input |
| **Login Password Input** | `login-password-input` | `login-password-input` | `data-testid="login-password-input"` | User password input |
| **Login Submit Button** | `login-submit-btn` | `login-submit-btn` | `data-testid="login-submit-btn"` | Primary auth submission button |
| **Booking Weight Input** | `booking-weight-input` | `booking-weight-input` | `data-testid="booking-weight-input"` | Parcel weight input field (kg) |
| **Booking Dimensions** | `booking-dims-input` | `booking-dims-input` | `data-testid="booking-dims-input"` | Container / inputs for L x W x H (cm) |
| **Quote Trigger Button** | *(Step 3 auto-fetch)* | `booking-estimate-btn` | `data-testid="booking-estimate-btn"` | Button triggering quote calculation |
| **Quote Price Display** | `quote-price-cents` | `booking-estimated-fee-display` | Both: `data-testid="quote-price-cents"` and `data-test-alias="booking-estimated-fee-display"` | Rendered delivery fee in LKR |
| **Booking Submit Button** | `booking-submit-btn` | `booking-submit-btn` | `data-testid="booking-submit-btn"` | Final confirmation and order dispatch |
| **Success Tracking Number**| `tracking-number-display` | `booking-success-tracking-number` | `data-testid="booking-success-tracking-number"` | Monospace tracking number on success screen |
| **Tracking Input Field** | `tracking-search-input` | `tracking-search-input` | `data-testid="tracking-search-input"` | Public tracking search text box |
| **Tracking Status Badge** | `tracking-status-badge` | `public-tracking-status-badge` | Both: `data-testid="tracking-status-badge"` and `data-test-alias="public-tracking-status-badge"` | Visual badge rendering lifecycle status |
| **Milestone Timeline** | `tracking-milestone-list`| `tracking-milestone-list`| `data-testid="tracking-milestone-list"` | Ordered container of delivery milestones |
| **Batch CSV Upload File**| `merchant-csv-input` | `merchant-csv-input` | `data-testid="merchant-csv-input"` | File input for merchant batch CSV |
| **API Key One-Time Modal**| `api-key-modal` | `api-key-modal` | `data-testid="api-key-modal"` | Modal displaying raw `sd_live_*` key |
| **API Key Copy Button** | `api-key-copy-btn` | `api-key-copy-btn` | `data-testid="api-key-copy-btn"` | One-click copy button inside modal |
| **Webhook Secret Modal** | `webhook-secret-modal` | `webhook-secret-modal` | `data-testid="webhook-secret-modal"` | Modal displaying raw `whsec_*` secret |

---

## 6. Frontend Architectural Invariants

1. **No External Imports between Features:** `features/customer-merchant/` will never import files from `features/dispatcher-admin/`. Shared interfaces and helpers must be placed in `shared/`.
2. **Form Component Decomposition:** Screens exceeding 250 lines will be decomposed into dedicated step sub-components (`Step1Addresses.tsx`, `Step2Dimensions.tsx`, etc.).
3. **Explicit Loading & Error States:** Every view component must explicitly account for the 5 standard states: `Loading` (skeleton loader), `Empty` (helpful empty state illustration), `Validation Error` (inline field error), `Server Error` (banner with `correlation_id`), and `Success`.
