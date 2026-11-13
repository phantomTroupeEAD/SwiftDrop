# M6 test-ID register

## Apple-style portal routes

| IDs | Purpose |
| --- | --- |
| `register-full_name-input`, `register-email-input`, `register-phone-input`, `register-password-input`, `register-submit-btn` | Customer registration fields and submit |
| `booking-wizard`, `booking-{pickup,dropoff}-{name,phone,address,city,postal,zone}-input` | Booking form and address fields |
| `booking-{weight,length,width,height,description}-input`, `booking-service-select` | Parcel details |
| `booking-next-btn`, `booking-back-btn`, `quote-price-cents`, `booking-submit-btn`, `booking-confirmation`, `booking-receipt-btn`, `booking-receipt` | Booking navigation, authoritative quote, submission, result and official receipt |
| `tracking-search-input`, `tracking-search-btn`, `tracking-status-badge`, `tracking-timeline` | Public tracking search and results |
| `merchant-application-{business_name,contact_email,contact_phone,line1,city,country}-input`, `merchant-application-submit-btn`, `merchant-application-status` | Merchant application and status |
| `merchant-order-search`, `merchant-order-filter`, `merchant-order-table`, `merchant-order-empty`, `merchant-order-reset-btn`, `merchant-retry-btn` | Tenant order list, filtering, empty state, filter reset and load retry |
| `batch-csv-file-input`, `batch-csv-submit-btn` | CSV selection and order submission or retry |
| `merchant-key-label-input`, `merchant-key-read-scope`, `merchant-key-create-scope`, `merchant-key-revoke-btn` | API key creation fields and revoke actions |
| `merchant-profile-retry-btn` | Retries a failed merchant profile request on the dashboard or orders page |
| `merchant-webhook-url-input`, `merchant-webhook-event-{eventType}`, `merchant-webhook-delete-btn` | Webhook registration fields and remove actions |
| `merchant-credential-create-btn`, `merchant-credential-confirm-btn`, `one-time-key-copy-btn`, `merchant-secret-close-btn` | Credential mutations, one-time copy and dismissal |

Owner: Sajini (M6). Update this file whenever a selector is added, renamed or removed.

## Selector rules

- Use descriptive, stable, kebab-case values. Prefer the canonical IDs already published in `docs/drafts/member-06-customer-merchant-web/MEMBER_06_ALIGNMENT_AND_SPECIFICATION_GUIDE.md` for the corresponding real features.
- Use semantic role/label locators for ordinary accessible controls and `data-testid` for stable state/data hooks. IDs do not replace input labels or prove that a backend workflow works.
- IDs are unique in a mounted view except explicitly listed repeated row/action selectors. Locate a repeated row first, then the control inside it.
- Never embed names, phone numbers, addresses, tokens or secrets in test IDs. Do not log/capture merchant secrets in test artefacts.
- `*-demo-*` and `preview-*` selectors identify prototype or visual preview content only. Do not use them as evidence for real booking, public tracking, receipt or secret-lifecycle acceptance.
- Do not rename canonical selectors without coordinating with M10's E2E owner. Shared login IDs also require M7 coordination.

## Implemented: clean Teal & Warm Ivory shipping application UI

Source: `src/features/customer-merchant/preview/SwiftSignatureTheme.tsx`, `src/features/customer-merchant/preview/DesignPreviewPage.tsx`, `src/features/auth/LoginPage.tsx`.
Routes: `/design-preview` and `/preview/design`.
*Notice: Preview interactions mutate local component state only; no network requests, pricing tariff calculations, or real parcel creation.*

| Test ID | Element / purpose | Availability |
| --- | --- | --- |
| `signature-theme-root` | Root container for the clean shipping UI | Always mounted on `/design-preview` |
| `signature-night-toggle` | Appearance toggle: light daylight ivory vs dark evening palette (`aria-pressed`) | Topbar header control |
| `login-theme-preview-link` | Direct link on `/auth/login` to preview customer/merchant UI | Always mounted on `/auth/login` |
| `signature-booking-step-1` | Stepper item: Step 1 (Addresses) | Customer booking wizard |
| `signature-booking-step-2` | Stepper item: Step 2 (Parcel details) | Customer booking wizard |
| `signature-booking-step-3` | Stepper item: Step 3 (Quote) | Customer booking wizard |
| `signature-booking-step-4` | Stepper item: Step 4 (Review) | Customer booking wizard |
| `signature-booking-sender-name` | Text input: Sender Full Name | Step 1 (Addresses) |
| `signature-booking-sender-phone` | Text input: Sender Phone (Mobile) | Step 1 (Addresses) |
| `signature-pickup-input` | Text input: Pickup Address & City | Step 1 (Addresses) |
| `signature-booking-recipient-name`| Text input: Recipient Full Name | Step 1 (Addresses) |
| `signature-booking-recipient-phone`| Text input: Recipient Phone (Mobile) | Step 1 (Addresses) |
| `signature-delivery-input` | Text input: Delivery Destination & City | Step 1 (Addresses) |
| `signature-booking-next-btn` | Primary CTA: Advance to next wizard step | Step 1 & Step 2 |
| `signature-booking-back-btn` | Secondary CTA: Return to previous wizard step | Step 2, Step 3 & Step 4 |
| `signature-booking-weight` | Number input: Package Weight in kg | Step 2 (Parcel details) |
| `signature-booking-dims` | Text input: Dimensions (L x W x H cm) | Step 2 (Parcel details) |
| `signature-booking-zone-select` | Select dropdown: Pickup Zone | Step 2 (Parcel details) |
| `signature-booking-fragile-check` | Checkbox: Fragile item handling | Step 2 (Parcel details) |
| `signature-booking-preview-btn` | Form submit button: View quote | Step 2 (Parcel details) |
| `signature-booking-quote-display` | Total estimated fee amount display in LKR | Step 3 (Quote) |
| `signature-booking-confirm-btn` | Primary CTA: Confirm booking (preview only) | Step 4 (Review) |
| `signature-tracking-input` | Text input: Quick tracking lookup | Tracking card |
| `signature-tracking-search-btn` | Button: Run quick tracking search | Tracking card |
| `signature-demo-feedback` | Action feedback banner (appears when action triggered) | Mounted on user action |
| `signature-merchant-stat-total` | KPI stat counter: Total orders | Merchant orders view |
| `signature-merchant-stat-transit` | KPI stat counter: In transit | Merchant orders view |
| `signature-merchant-stat-delivered`| KPI stat counter: Delivered | Merchant orders view |
| `signature-merchant-stat-warning` | KPI stat counter: Needs attention | Merchant orders view |
| `signature-merchant-search-input`| Text input: Search orders or city (accessible label) | Merchant orders view |
| `signature-merchant-csv-btn` | Button: Import orders placeholder | Merchant orders view |
| `signature-merchant-orders-table`| Data table: Rendered shipment order rows | Merchant orders view |

## Retired / removed selectors (historical design studio controls)

The following selectors were used during initial theme evaluation and design comparison. They have been removed from the visible product interface and should not be asserted as currently mounted controls:

| Test ID | Previous role | Retirement reason |
| --- | --- | --- |
| `theme-preview-root` | Multi-theme comparison root container | Replaced by single clean shipping UI shell |
| `theme-mode-toggle-split` | View mode toggle: side-by-side comparison | Design studio comparison chrome removed |
| `theme-mode-toggle-dark` | View mode toggle: isolate Option 1 | Design studio comparison chrome removed |
| `theme-mode-toggle-light` | View mode toggle: isolate Option 2 | Design studio comparison chrome removed |
| `theme-mode-toggle-signature` | View mode toggle: Option 3 button | Design studio comparison chrome removed |
| `theme-select-dark-btn` | Working preference button: Dark | Reviewer preference selector removed |
| `theme-select-light-btn` | Working preference button: Light | Reviewer preference selector removed |
| `theme-select-signature-btn` | Working preference button: Signature | Reviewer preference selector removed |
| `coordination-checklist-section` | Cross-module coordination checklist container | Moved out of product UI to handoff docs |
| `signature-nav-customer-btn` | Top screen tab: Customer booking | Navigation moved to sidebar |
| `signature-nav-merchant-btn` | Top screen tab: Merchant orders | Navigation moved to sidebar |
| `signature-nav-foundation-btn` | Top screen tab: Design tokens & specs | Design specs removed from product UI |
| `signature-merchant-loading-state` | Reviewer button to force loading skeleton | Reviewer state controls removed |
| `signature-merchant-empty-state` | Reviewer button to force empty box | Reviewer state controls removed |
| `signature-merchant-error-state` | Reviewer button to force error box | Reviewer state controls removed |
| `preview-*` (all variants) | Component showcase cards, badges, buttons | Prototype comparison studio retired |

## Implemented: customer dashboard prototype

Source: `src/features/dashboards/CustomerDashboard.tsx`. Route: `/customer/dashboard`.

| Test ID | Element / purpose | Availability |
| --- | --- | --- |
| `customer-dashboard` | Root of M6 customer content | Always mounted |
| `customer-track-tab` | Select tracking tab | Always mounted |
| `customer-book-tab` | Select booking tab | Always mounted |
| `customer-history-tab` | Select history tab | Always mounted |
| `customer-tracking-panel` | Tracking tab content | TRACK tab |
| `customer-demo-tracking-number` | Hardcoded example tracking number | TRACK tab; demo only |
| `customer-demo-tracking-status` | Hardcoded example status | TRACK tab; no SSE |
| `customer-booking-panel` | Prototype booking form | BOOK tab |
| `booking-pickup-address` | Enter pickup address | BOOK tab |
| `booking-address-input` | Enter destination street address | BOOK tab; canonical field ID |
| `booking-weight-input` | Enter weight in kg | BOOK tab; canonical field ID |
| `booking-service-level` | Choose existing service option | BOOK tab |
| `customer-demo-quote` | Display locally calculated example fee | BOOK tab; not an API quote |
| `customer-demo-recalculate-btn` | Run existing demo calculation | BOOK tab; replace during integration |
| `customer-demo-book-btn` | Existing alert-only booking action | BOOK tab; no parcel created |
| `customer-history-panel` | Sample history table | HISTORY tab |
| `customer-history-row` | Scope a sample shipment row | HISTORY tab; repeated; use row text |
| `customer-demo-receipt-btn` | Sample row's alert-only receipt action | HISTORY tab; repeated within row |

## Implemented: merchant dashboard prototype

Source: `src/features/dashboards/MerchantDashboard.tsx`. Route: `/merchant/dashboard`.

| Test ID | Element / purpose | Availability |
| --- | --- | --- |
| `merchant-dashboard` | Root of M6 merchant content | Always mounted |
| `merchant-batch-tab` | Select batch tab | Always mounted |
| `merchant-api-keys-tab` | Select API-key tab | Always mounted |
| `merchant-webhooks-tab` | Select webhook tab | Always mounted |
| `merchant-batch-panel` | Batch prototype content | BATCH tab |
| `merchant-batch-browse-btn` | Existing browse placeholder | BATCH tab; no file input wired |
| `merchant-batch-row` | Scope sample batch row | BATCH tab; repeated; use row text |
| `merchant-api-keys-panel` | Sample API-key content | API tab |
| `merchant-demo-api-key` | Read-only example key field | API tab; not a newly issued key |
| `merchant-demo-key-copy-btn` | Copy the existing example value | API tab; not one-time secret flow |
| `merchant-webhooks-panel` | Sample webhook metadata | WEBHOOKS tab; no persistence |

## Reserved: real features not yet implemented here

These IDs are required by the M6 alignment guide. They are not promises that the components or flows already exist.

| Test ID | Intended purpose / owner |
| --- | --- |
| `login-email-input` | Shared login email — coordinate M7 |
| `login-password-input` | Shared login password — coordinate M7 |
| `login-submit-btn` | Shared login submission — coordinate M7 |
| `register-submit-btn` | Customer registration |
| `booking-sender-phone` | Sender phone |
| `booking-recipient-name` | Recipient name |
| `booking-recipient-phone` | Recipient phone |
| `booking-city-input` | Destination city |
| `booking-dims-input` | Group containing labelled length/width/height inputs |
| `get-quote-btn` | Request authoritative pricing quote |
| `quote-price-cents` | Display backend-returned fee formatted as LKR |
| `booking-submit-btn` | Submit real booking using stable idempotency key |
| `booking-success-tracking-link` | Link from backend-confirmed booking |
| `tracking-search-input` | Public tracking number entry |
| `tracking-search-btn` | Start public lookup |
| `tracking-status-badge` | Authoritative public tracking status |
| `tracking-milestone-list` | Privacy-safe milestone list |
| `merchant-apply-submit-btn` | Submit merchant application |
| `batch-csv-file-input` | Actual CSV file input |
| `batch-csv-submit-btn` | Submit validated CSV orders |
| `generate-api-key-btn` | Request a new merchant API key |
| `one-time-key-copy-btn` | Copy a newly issued key in transient modal |
| `dismiss-key-modal-checkbox` | Confirm newly issued key is saved |

## Verification status

The clean shipping UI components (`/design-preview`) are fully verified locally without altering prototype dashboards or shared M7 code.

Verification: `npm run build` passed (`tsc && vite build`). All 31 active selector literals across `SwiftSignatureTheme.tsx` and `LoginPage.tsx` are documented in this register. Input controls in the UI are linked to visible labels with `htmlFor` / `id`. Retired design-comparison and forced-state selectors are documented explicitly. Unit/E2E test scripts remain absent from the package manifest, so no journey-test passes are claimed.
