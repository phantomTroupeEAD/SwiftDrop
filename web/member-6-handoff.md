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

The frontend build and lint passed. A focused client check passed with simulated receipt and zone-error responses. In the browser preview, registration showed the unavailable-email feedback, demo sign-in reached the customer dashboard, the assistant opened and filled the delivery estimate prompt, and the booking page showed sender and recipient postal-code fields.

Live verification is pending. The deployed zone endpoint returned HTTP 525 and the local Core API readiness URL returned HTTP 404. Repeat registration with a real OTP and verify quote through receipt, tracking, merchant workflows, and assistant replies once the test backend is available. For booking, use Standard, `00100` to `00400`, 2.5 kg, and 20 × 15 × 10 cm. The expected quote and receipt total is LKR 425.00 with Hansi’s tariff loaded.
