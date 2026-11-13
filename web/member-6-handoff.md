# SwiftDrop Member 6 frontend handoff

Branch `feat/m6-role-dashboard-ui`. Review target `dev`.

| Area | Route or entry point | Features |
| --- | --- | --- |
| Customer account | `/auth/register`, `/auth/login` | Email verification, registration and sign-in |
| Customer portal | `/dashboard`, `/portal/book` | Recent parcels, postal-code lookup, quote, booking, tracking link and receipt |
| Tracking | `/track`, `/portal/tracking` | Public and signed-in tracking |
| Merchant | `/portal/merchants/apply`, `/portal/merchants/orders`, `/portal/merchants/api-keys`, `/portal/merchants/webhooks` | Application, orders, CSV import, keys and webhooks |
| Assistant | Customer dashboard chat icon | Prompt shortcuts, chat and booking handoff |

Use the [selector register](test-ids.md) for exact `data-testid` values and the [design system](design.mdx) for look and feel. The current feature code lives under `src/features/customer-merchant/`, with account screens under `src/features/auth/` and the customer and merchant dashboard in `src/features/dashboards/RoleHome.tsx`.

## Key test IDs

| Screen | Exact `data-testid` values | What to check |
| --- | --- | --- |
| Sign-in | `login-email-input`, `login-password-input`, `login-submit-btn` | Enter credentials and submit |
| Registration | `register-email-input`, `register-send-otp-btn`, `register-otp-input`, `register-submit-btn` | Request a verification code, enter it, and submit the completed form |
| Booking | `booking-pickup-postal-input`, `booking-dropoff-postal-input`, `booking-service-select`, `quote-price-cents`, `booking-submit-btn` | Enter postal codes, select a service, verify the quote amount, and confirm |
| Booking result | `booking-confirmation-tracking-no`, `booking-receipt-btn`, `booking-receipt` | Read the tracking number and open the official receipt |
| Tracking | `tracking-search-input`, `tracking-search-btn`, `tracking-status-badge`, `tracking-timeline` | Look up a parcel and verify its status and timeline |
| Merchant | `merchant-application-submit-btn`, `merchant-application-status`, `merchant-order-table`, `batch-csv-file-input`, `batch-csv-submit-btn`, `merchant-credential-create-btn` | Submit an application, inspect its status, check orders, upload a CSV, and create an API key or webhook |
| Assistant | `assistant-open-btn`, `assistant-prompt-input`, `assistant-send-btn` | Open chat, enter a message, and send it |

## Booking test data

| Field | Value |
| --- | --- |
| Sender postal code | `00100` → `ZONE_COLOMBO_01` |
| Recipient postal code | `00400` → `ZONE_COLOMBO_02` |
| Service | `STANDARD` |
| Weight and dimensions | 2.5 kg, 20 × 15 × 10 cm |
| Local pricing fixture | `tariff_colombo_intra_v1`, version 1, with `is_active = TRUE` |
| Expected result | Quote and receipt both show LKR 425.00. The quote amount is 42,500 cents. |

## Verification

The frontend build and lint passed. A focused client check passed with simulated receipt and zone-error responses. In the browser preview, registration showed the unavailable-email feedback, demo sign-in reached the customer dashboard, the assistant opened and filled the delivery estimate prompt, and the booking page showed sender and recipient postal-code fields.

Live verification is pending. The deployed zone endpoint returned HTTP 525 and the local Core API readiness URL returned HTTP 404. Once the test backend is available, repeat registration with a real OTP and verify quote through receipt using the fixture above. Verify tracking, merchant workflows, and assistant replies on the same test backend.
