# SwiftDrop Member 6 frontend handoff

Branch `feat/m6-role-dashboard-ui`. Review target `dev`.

| Area | Route or entry point | Features |
| --- | --- | --- |
| Customer account | `/auth/register`, `/auth/login` | Email verification, registration and sign-in |
| Customer portal | `/dashboard`, `/portal/book` | Recent parcels, postal-code lookup, quote, booking, tracking link and receipt |
| Tracking | `/track`, `/portal/tracking` | Public and signed-in tracking |
| Merchant | `/portal/merchants/apply`, `/portal/merchants/orders`, `/portal/merchants/api-keys`, `/portal/merchants/webhooks` | Application, orders, CSV import, keys and webhooks |
| Assistant | Embedded customer dashboard panel | Prompt shortcuts, chat and booking handoff |

Use the [selector register](test-ids.md) for exact `data-testid` values and the [design system](design.mdx) for look and feel. The current feature code lives under `src/features/customer-merchant/`, with account screens under `src/features/auth/` and the customer and merchant dashboard in `src/features/dashboards/RoleHome.tsx`.

## Key test IDs

| Screen | Exact `data-testid` values | What to check |
| --- | --- | --- |
| Sign-in | `login-email-input`, `login-password-input`, `login-submit-btn` | Enter credentials and submit |
| Registration | `register-email-input`, `register-send-otp-btn`, `register-otp-input`, `register-submit-btn` | Request a verification code, enter it, and submit the completed form |
| Booking | `booking-pickup-postal-input`, `booking-dropoff-postal-input`, `booking-service-select`, `quote-price-cents`, `booking-submit-btn` | Enter postal codes, select a service, verify the quote amount, and confirm |
| Booking result | `booking-confirmation-tracking-no`, `booking-receipt-btn`, `booking-receipt` | Read the tracking number and open the official receipt |
| Quote expiry | `booking-quote-countdown` | Watch the quote expire and check that a new booking requires a fresh quote |
| Tracking | `tracking-search-input`, `tracking-search-btn`, `tracking-status-badge`, `tracking-timeline` | Look up a parcel and verify its status and timeline |
| Merchant | `merchant-application-submit-btn`, `merchant-application-status`, `merchant-order-table`, `batch-csv-file-input`, `batch-csv-submit-btn`, `merchant-credential-create-btn` | Submit an application, inspect its status, check orders, upload a CSV, and create an API key or webhook |
| Assistant | `assistant-panel`, `assistant-prompt-input`, `assistant-send-btn` | Find the embedded chat, enter a message, and send it |

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

## Audit implementation notes

- The customer dashboard embeds the assistant in the page. Tests should use `assistant-panel`, `assistant-prompt-input`, and `assistant-send-btn`. There is no launcher control.
- Assistant estimates request a server quote when both postal codes and all package measurements are available. Booking still requires completed sender, recipient, and parcel details.
- Merchant CSV uses sender and recipient fields because the current `POST /api/v1/merchant/orders` handler expects both pickup and dropoff. Dimensions and description are validated during preview. The current order DTO only accepts weight as parcel data.
- Receipt handling supports PDF downloads and printable JSON. The running service must expose the receipt endpoint. The frontend only shows receipt fields the service supplies.
- Verify the merchant role upgrade, booking through receipt, webhook and key creation, and the tracking reconnect in a running integration environment.

### Integration check

- The locally available `origin/dev` revision `b36d559` registers the authorized parcel receipt route. That revision's Member 4 wiring uses `NewBookingStub()`. It holds merchant order records in memory and is not a durable parcel booking integration.
- Persisting CSV dimensions and descriptions requires the Member 4 order handler to pass the fields into a durable Member 2 booking adapter. The CSV preview continues validating them.
- Local endpoint smoke check: the frontend dashboard returned HTTP 200. The process on port 8080 returned HTTP 404 for readiness, zones, and the receipt path. The Tracking API on port 8081 refused connections. These results do not verify booking through receipt, SSE reconnect, merchant approval, or credential creation.
- The receipt transport test source is present in the receipt worktree. The Go toolchain download timed out, so the backend test suites were not run in this check.

### Local merchant API proxy

If port 8080 belongs to another application, run the PostgreSQL-backed SwiftDrop Core API on a free port such as 18080. Set `SWIFTDROP_CORE_API_URL=http://127.0.0.1:18080` in `web/.env.local`, remove any `SWIFTDROP_DEMO_AUTH=1` override, restart Vite, and sign in with a merchant account linked to an approved merchant profile in that database.

### Local merchant smoke test

Core API from `dev` commit `e6c6988` was verified through the frontend proxy on port 3013 with an approved test merchant. Profile, orders, keys, webhooks, and the event catalog returned 200. Creating a key and registering a webhook returned 201 and each supplied its one-time secret. Revoking the key and deleting the webhook returned 204. A test order returned 201 and appeared in the merchant list. The order adapter remains in-memory.
