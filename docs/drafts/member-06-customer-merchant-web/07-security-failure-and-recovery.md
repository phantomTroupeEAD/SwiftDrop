# Customer and Merchant Frontend: Security, Failure and Recovery

Owner: Sajini, Member 6  
Date: 24 October 2026  
Status: Planned controls; supplements the shared security policy

## Access rules

Customers use their own booking/history screens. Merchants use their authorized merchant records. Operations screens belong to M7. Frontend guards support navigation, while backend authorization remains mandatory for every protected request. I will not assume a role from an editable browser value or trust an arbitrary tenant header.

## Sensitive data

My forms handle names, addresses and phone numbers. Authenticated pages may show private shipment details returned by the API. Public tracking must receive and display only approved public fields. Hiding private fields with CSS is insufficient; M5 must omit them from public responses.

Access tokens stay in memory. Refresh cookies remain inaccessible to JavaScript. Merchant API secrets and provider credentials must not be embedded in the bundle. Anything prefixed `VITE_` is public build configuration, not a secret.

## Threats and controls

| Risk | My control | Backend/shared dependency |
| --- | --- | --- |
| Cross-user or cross-merchant access | Clear private cache on identity changes; show access denied | Object/tenant authorization on every endpoint |
| Script injection in notes/names | Render text safely; do not use untrusted raw HTML | Validate input and safe response handling |
| Token exposure | No browser persistent storage, URLs or debug logs for tokens | Secure session issuance/revocation |
| Duplicate booking | Stable request key and disabled pending submit | Server idempotency storage/enforcement |
| Public tracking disclosure | Restrict UI to approved fields; test responses | Public API data minimization |
| Forged role/price | Never submit client values as authority | Backend checks and server-side pricing |
| Excessive retries | Bounded retry behavior; respect rate limits | API rate limiting |
| Cookie-based request abuse | Follow M1's agreed cookie/CSRF flow | CSRF protection and origin validation where required |

Google redirect behavior and SameSite cookie settings need end-to-end testing with M1. I will not weaken the shared cookie policy independently to make a login test pass.

## Failure-mode matrix

| Failure | User message/action | Recovery |
| --- | --- | --- |
| Quote API unavailable | Fee unavailable; retain draft | Explicit retry without inventing a fee |
| Booking timeout | Outcome could not be confirmed | Retry original payload with original key |
| Expired quote | Request updated quote | Ask user to confirm changed amount |
| Session expiry | Refresh once through shared client | Clear state and sign in if refresh fails |
| Forbidden record | Access denied | Do not keep retrying |
| Missing record | Not found | Allow return/search |
| Rate limited | Ask user to wait | Respect provided retry interval |
| SSE disconnected | Show connection state | Reconnect, re-fetch and bounded polling fallback |
| Receipt download fails | Download unavailable | Retry authorized endpoint |
| Webhook settings rejected | Show safe field/summary errors | Preserve editable fields; backend validates target |

The shared client needs configurable finite timeouts. Exact HTTP timeout, read retry count, polling interval and refresh coordination behavior still need team agreement. I will not enable automatic mutation retries unless their contract makes that safe.

## Partial results and cleanup

I will invalidate list/detail queries after confirmed writes. A stale quote response must be ignored when inputs have changed. Logout/account switch removes private data. Route changes close SSE subscriptions and timers. A frontend retry must never create a new booking simply because the first response was lost.

## Logging and audit

I may show a safe correlation reference so a teammate can find a failed request. I will not log request bodies containing passwords, addresses, tokens or merchant secrets. Server audit records belong to the backend; browser logging is not a substitute.

## Recovery ownership

There is no frontend-owned DLQ, reconciliation job or database compensation. M8/M9 handle asynchronous recovery and operational investigation. The UI must report available status honestly and must not mark a notification, webhook or delivery successful without a backend result.

## Checks before completion

- [ ] Private API authorization tested with another user's/merchant's identifiers.
- [ ] Public response and rendered page checked for sensitive data.
- [ ] Tokens absent from persistent storage, URLs, logs and bundle configuration.
- [ ] Refresh failure clears private cached data.
- [ ] Retry uses a stable key for the same booking.
- [ ] Session and Google flows checked with M1.
- [ ] Failure states and stream cleanup tested.

These checks are pending; unchecked items are not completed controls.
