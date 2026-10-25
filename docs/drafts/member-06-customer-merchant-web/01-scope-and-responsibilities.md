# Customer and Merchant Frontend: Scope and Responsibilities

Project: SwiftDrop  
Owner: Sajini, Member 6  
Date: 24 October 2026  
Status: Design proposal; implementation and integration are pending

## Purpose

My part of SwiftDrop is the customer and merchant portal. I am responsible for the screens that let a customer book and track a parcel and let a merchant view and create delivery orders. I will connect these screens to the backend APIs provided by the other members.

## My scope

| Area | My responsibility | Backend owner |
| --- | --- | --- |
| Registration | Customer registration and merchant application screens | M1 / M4 |
| Login | Contribute to the shared login and Google sign-in interface | M1; shared with M7 |
| Booking | Pickup/drop-off details, parcel details, quote display and confirmation | M2 |
| History | Customer parcel list, parcel details and status history | M2 / M5 |
| Receipts | Show receipt information and provide the authorized download action | M2 |
| Tracking | Public tracking page, timeline and live status display | M5 |
| Merchant portal | Merchant order list, order creation and approval-state display | M4 |
| Merchant settings | Webhook/profile settings where supported by the agreed API | M4 |
| UI states | Loading, empty, validation error, server error and success states | My responsibility |

## Boundaries

I will not implement pricing rules, decide valid parcel-state transitions, issue tokens, approve merchants or update database tables. I will display the backend result rather than reproduce these rules in the browser. I will not implement the dispatcher, admin or courier screens; these belong to M7.

The portal does not own authoritative business data. PostgreSQL and the backend modules own that data. My forms and query cache hold temporary copies needed to display a screen.

## Ownership matrix

| Resource | Ownership |
| --- | --- |
| `web/src/features/customer-merchant/` | My feature code |
| `web/src/app/` and `web/src/shared/` | Shared with M7; coordinate edits |
| `web/README.md` | Shared frontend documentation |
| Auth endpoints and session contract | M1 |
| Parcel and pricing endpoints | M2 |
| Merchant endpoints | M4 |
| Tracking endpoint and SSE stream | M5 |
| Database tables and migrations | Backend module owners |
| Queues, relay and webhook delivery | M8 |
| Web Dockerfile and Nginx configuration | Proposed owner M9; confirm with M7/M9 |

## Dependencies

I need the authentication contract from M1, parcel/quote/receipt contracts from M2, merchant contracts from M4 and tracking/SSE contracts from M5. M7 and I need to agree on shared layouts, route guards, components and API-client changes. M9 needs the build command, output directory, port and API configuration for deployment.

The frontend calls versioned HTTP APIs. It does not access the database or message broker. Other modules supply data and accept validated requests through their APIs; they should not depend on frontend component internals.

## Rules I need to preserve

- A public tracking page must not reveal private contact details, street addresses or proof attachments.
- A customer or merchant must only see records returned for their authorized account.
- I will not treat a hidden button or route guard as backend authorization.
- A retry of the same booking must reuse its idempotency key.
- Prices come from the backend and are displayed with their currency.
- Access tokens stay in memory; refresh-token handling follows the agreed backend cookie flow.

## Scope still to confirm

CSV imports, bulk orders, address books and label generation appear in the frontend plan but are not part of my minimum delivery until the team agrees on their APIs and priority. Live GPS maps and online payments are outside the current baseline.

I also need confirmation of the exact quote path, role strings, merchant approval values, receipt format and Google callback behavior. The feature folder can contain several workflows; this document covers my frontend area rather than creating separate backend modules for each page.

## Completion criteria

My part is complete when the agreed customer and merchant journeys work with real APIs, required error and permission cases are handled, tests have recorded results, the frontend builds and the living documentation matches the implementation.
