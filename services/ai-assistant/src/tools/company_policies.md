# SwiftDrop Courier Services — Authoritative Company Policies & Operating Standards

> **Document Type:** Operational Knowledge Base & Courier Policy Manual  
> **Applicability:** All SwiftDrop hubs, merchant accounts, customer interactions, courier fleet, operations staff, and automated AI assistants  
> **Effective Date:** 2026-01-01 | **Status:** Active Production Baseline  
> **Authority:** Resolves SwiftDrop SRS v1.0, ADR-001 through ADR-004, and Core Domain Contracts  

---

## 1. Parcel Dimension & Packaging Standards

1. **Weight Limitations:**
   - **Minimum Billable Weight:** **0.1 kg** (100 grams).
   - **Maximum Allowable Gross Weight:** **50.0 kg** per standard parcel.
   - **Standard Operational Weight Threshold:** Configurable platform default is **25.0 kg – 30.0 kg** (`parcels.max_weight_kg`). Consignments exceeding 50.0 kg cannot be booked through standard courier channels and must be divided into separate consignments or routed through Dedicated Freight.

2. **Dimension Constraints:**
   - **Maximum Single Dimension (Length, Width, or Height):** **150.0 cm**.
   - **Maximum Parcel Girth:** **300.0 cm** ($\text{Length} + 2 \times (\text{Width} + \text{Height}) \le 300.0\text{ cm}$).
   - **Minimum Parcel Dimensions:** **10.0 cm $\times$ 10.0 cm $\times$ 1.0 cm** (ensures shipping label attachment surface).
   - **Volumetric Weight Conversion:**
     $$\text{Volumetric Weight (kg)} = \frac{\text{Length (cm)} \times \text{Width (cm)} \times \text{Height (cm)}}{5000}$$
     The volumetric divisor is standardized across all domestic routes at **5000 $\text{cm}^3/\text{kg}$**.
   - **Chargeable / Billable Weight:**
     $$\text{Billable Weight} = \max(\text{Actual Gross Weight (kg)}, \text{Volumetric Weight (kg)})$$
     Billing is assessed on the higher of the actual gross scale weight and the cubic volumetric weight, rounded up to the nearest whole kilogram band above the base unit.

3. **Packaging Requirements:**
   - **Outer Enclosure:** All shipments must be packed in rigid, double-wall corrugated cardboard cartons or heavy-duty tear-resistant courier poly mailers.
   - **Fragile Consignments:** Glassware, ceramics, precision electronics, and optical devices must feature a minimum of **5.0 cm of bubble wrap cushioning** on all six sides, internal cardboard partitions, and prominent high-visibility **"FRAGILE"** handling labels.
   - **Liquids & Gels:** Must be double-sealed inside leakproof polyethylene bags, placed upright, and enclosed in an absorbent secondary container capable of containing the entire liquid volume during transit.
   - **Sealing:** Boxes must be securely sealed using heavy-duty pressure-sensitive polymer packing tape (minimum 48 mm width) in an "H-tape" configuration across all center and edge seams.

---

## 2. Prohibited & Restricted Commodities

The following items are strictly **prohibited** from carriage, sortation, or storage across the SwiftDrop network. Any prohibited consignment detected during intake or hub sortation will be quarantined immediately and reported to civil enforcement authorities:

- **Hazardous Materials:** Corrosives (acids, battery fluid), flammable liquids and gases (gasoline, lighter fluid, aerosol sprays), toxic chemicals, infectious biological agents, and radioactive substances.
- **Batteries:** Loose, unpackaged lithium-ion batteries or standalone power banks exceeding 100Wh without certified UN38.3 test documentation and a certified dangerous goods declaration.
- **Controlled Substances:** Narcotics, illegal pharmaceuticals, unauthorized prescription drugs, and synthetic stimulants.
- **Living Organisms:** Live animals, birds, reptiles, fish, insects, plants, seeds, and active microbiological cultures.
- **Perishable Cargo:** Unsealed, unrefrigerated raw foodstuffs, fresh meat, dairy products, and seafood susceptible to spoilage.
- **Weapons & Explosives:** Firearms, firearm replicas, ammunition, fireworks, gunpowder, flares, tactical knives, and martial arts weaponry.
- **Currency & Bullion:** Physical cash (banknotes, coinage), gold/silver bullion, loose precious gems, bearer bonds, and negotiable financial instruments.

---

## 3. Pricing Rules, Tariffs & Cash on Delivery (COD)

1. **Currency & Financial Representation:**
   - **Official Currency:** **Sri Lankan Rupee (LKR)**.
   - **Internal Financial Precision:** All financial figures, tariffs, quotes, and audit totals are stored as **64-bit signed integers (`bigint`) representing minor currency units (cents)**. Floating-point types are strictly forbidden in database persistence and financial business logic (e.g. LKR 450.00 is stored as `45000` cents).
   - **Consumer Invariant:** Customer-facing quotes are formatted in clean integer Sri Lankan Rupees (e.g. `LKR 1,595` or `Rs. 1,595`) without fractional cents, reflecting local commercial practice.

2. **Service Level Multipliers:**
   - **`STANDARD` (1.0x Multiplier):** Standard domestic ground network delivery (2–3 business days).
   - **`EXPRESS` (1.5x Multiplier):** Priority sortation and next-business-day expedited linehaul dispatch.
   - **`SAME_DAY` (2.0x Multiplier):** Intra-zone direct courier dispatch within 6 hours (valid for bookings placed before 12:00 PM).

3. **Baseline Tariff Schedule:**
   - Total delivery fee is computed by adding the Zone Base Fee (first 1.0 kg) to additional weight surcharges ($\max(0, \lceil \text{Billable Weight} - 1.0 \rceil) \times \text{PerKgRate}$), then applying the service level multiplier:
   | Zone Classification | Zone Route Pair | Base Fee (First 1 kg) | Additional Rate (Per Additional kg) |
   | :--- | :--- | :--- | :--- |
   | **Intra-Zone Metro** | Metro $\rightarrow$ Metro (`ZONE_COLOMBO_METRO`) | **LKR 350.00** (`35000` cents) | **LKR 80.00** (`8000` cents) |
   | **Inter-Zone Western**| Metro $\rightarrow$ Greater Western (`ZONE_GREATER_WESTERN`) | **LKR 450.00** (`45000` cents) | **LKR 100.00** (`10000` cents) |
   | **Long Haul** | Metro $\rightarrow$ Outstation (`ZONE_OUTSTATION`) | **LKR 650.00** (`65000` cents) | **LKR 150.00** (`15000` cents) |
   | **Cross-Province** | Outstation $\rightarrow$ Outstation | **LKR 750.00** (`75000` cents) | **LKR 180.00** (`18000` cents) |

4. **Quote Expiration & Price Lock Guarantee:**
   - Quotes generated via the pricing engine remain valid for **15 minutes** from generation (`expires_at = NOW() + 15m`).
   - Once a parcel booking is confirmed with an active quote, the agreed price is immutably frozen onto the parcel record (`parcels.price_cents`). Subsequent tariff changes do not affect confirmed bookings.

5. **Cash on Delivery (COD) Policy:**
   - Maximum allowable COD collection per parcel is **LKR 100,000.00** (`10,000,000` cents), governed by administrative setting `parcels.max_cod_cents`.
   - Couriers must collect the exact COD amount prior to parcel handover. Uncollected or partial COD payments strictly prohibit package release.

---

## 4. Transit Service Level Agreements (SLAs) & Regional Directory

1. **Regional Service Windows:**
   SwiftDrop operates structured dispatch schedules across Sri Lanka's 25 administrative districts:
   - **Express Delivery (Colombo Metropolitan):** **Same-Day / 24 Hours** covering postal codes **10000–10500** (Colombo 01–15 and immediate inner suburbs). Bookings confirmed before 11:00 AM are dispatched on the afternoon courier run.
   - **Standard Delivery (Western & Southern Provinces):** **48 to 72 Hours** covering postal codes **11000–13500** (Gampaha, Negombo, Kalutara) and **80000–82500** (Galle, Matara, Hambantota).
   - **Outstation Delivery (Central, Northern, Eastern, Uva & Sabaragamuwa):** **3 to 5 Business Days** covering postal codes **20000–72500** (Kandy, Matale, Nuwara Eliya, Jaffna, Trincomalee, Batticaloa, Anuradhapura, Polonnaruwa, Kurunegala, Puttalam, Ratnapura, Kegalle, Badulla).

2. **Authoritative Sub-District Postal Directory:**
   The logistics engine maintains automatic resolution for major towns and regional trade hubs:
   - *Western & Colombo:* Colombo Central (`00100`), Gampaha (`11000`), Negombo (`11500`), Kalutara (`12000`).
   - *Central & Hills:* Kandy (`20000`), Kadugannawa (`20430`), Hunnasgiriya (`20948`), Matale (`21000`), Nuwara Eliya (`22200`).
   - *Sabaragamuwa:* Ratnapura (`70000`), Nivitigala (`70450`), Kegalle (`71000`), Mawanella (`71500`).
   - *Southern:* Galle (`80000`), Mirissa (`81740`), Matara (`81000`), Hambantota (`82000`), Walasmulla (`82450`).
   - *North Western & North Central:* Kurunegala (`60000`), Puttalam (`61300`), Anuradhapura (`50000`), Kahatagasdigiliya (`50130`), Polonnaruwa (`51000`).
   - *Northern & Eastern:* Jaffna (`40000`), Batticaloa (`30000`), Trincomalee (`31000`), Badulla (`90000`).

3. **Operating Calendar:**
   Linehaul operations and deliveries operate Monday through Saturday. Sundays, national mercantile holidays, and declared statutory holidays are excluded from transit SLA day calculations.

---

## 5. Parcel Lifecycle & Canonical State Transitions

All parcel state mutations are strictly governed by the authoritative **9-state finite state machine** managed exclusively by the Parcel Aggregate Root (`core-api/internal/parcels`). Direct SQL updates to parcel status are strictly forbidden:

1. **State Transition Path:**
   - `CREATED`: Booking confirmed; valid quote consumed; tracking number allocated.
   - `ASSIGNED`: Courier assigned by dispatcher; assignment recorded.
   - `PICKED_UP`: Parcel collected from sender or merchant warehouse depot.
   - `IN_TRANSIT`: Parcel moving between regional sortation hubs.
   - `OUT_FOR_DELIVERY`: Dispatched with courier for active final-mile delivery attempt.
   - `DELIVERED`: Terminal success state; confirmed with validated proof of delivery.
   - `ATTEMPT_FAILED`: Recoverable operational pause; courier recorded an unsuccessful attempt.
   - `RETURNED`: Terminal return-to-sender state; reached after attempt exhaustion or refusal.
   - `CANCELLED`: Terminal cancellation; **strictly permitted only while in `CREATED` status**. Once assigned or in transit, cancellation is barred (`ErrCancellationNotAllowed`).

2. **Immutable Status History Audit Trail:**
   Every lifecycle transition atomically appends an immutable record to `parcel_status_history` containing `from_status`, `to_status`, `actor_id`, `actor_role`, `reason`, and a server-side timestamp. Deletion of status history records is prohibited by database triggers (`ON DELETE RESTRICT`).

3. **Optimistic Concurrency Control:**
   All parcel mutations enforce version checks (`WHERE id = $1 AND version = $2`). Conflicting concurrent updates return `409 Conflict` (`CONCURRENT_MODIFICATION`).

---

## 6. Delivery Attempts, Failures & Return-to-Sender (RTS) Policy

In accordance with **ADR-004**, delivery attempt records are separated from parcel terminal states:

1. **Maximum Attempt Ceiling:**
   - SwiftDrop couriers will make up to **3 consecutive physical delivery attempts** on separate business days (`attempt_count <= 3`).
   - Each attempt is recorded as an immutable entity in `delivery_attempts` with courier ID, attempt number, GPS coordinates, and timestamp.

2. **Standardized ADR-004 Failure Reason Codes:**
   When an attempt fails, the courier must classify the failure using one of 7 approved reason codes:
   - `RECIPIENT_UNAVAILABLE`: Recipient not present, phone unanswered (Eligible for immediate reattempt).
   - `CUSTOMER_REQUESTED_RESCHEDULE`: Recipient contacted and requested delivery at a later date/time (Eligible for reattempt).
   - `ACCESS_RESTRICTED`: Gated premises, security refusal, dangerous animals, impassable road (Eligible for reattempt).
   - `INVALID_ADDRESS`: Physical address does not exist or missing directions (Requires address verification before reattempt).
   - `REFUSED_BY_RECIPIENT`: Recipient actively rejected package (Transitions immediately to terminal `RETURNED`).
   - `DAMAGED_IN_TRANSIT`: Outer box breached or contents compromised (Quarantined at depot for inspection).
   - `OTHER`: Exceptional operational circumstances (Requires a mandatory descriptive note of $\ge 15$ characters).

3. **Dispatcher Authorization Gate:**
   Couriers **cannot** unilaterally reschedule an `ATTEMPT_FAILED` parcel. Only an authorized user with role `DISPATCHER` or `ADMIN` can review the failure reason and reschedule the parcel back to `OUT_FOR_DELIVERY` or reassign it.

4. **Depot Holding & Automated Return-to-Sender:**
   - After a delivery attempt failure, the parcel is held at the local destination sorting hub for up to **5 business days** awaiting recipient response or dispatch instructions.
   - If the 3rd attempt fails (`attempt_count == 3`) or the recipient refuses delivery, the parcel automatically transitions to `RETURNED`. Return freight is charged to the merchant/sender account.

---

## 7. Proof of Delivery (POD) & Courier Fleet Operations

1. **Mandatory Proof for Completion:**
   A parcel **cannot** transition to `DELIVERED` without an authoritative proof record:
   - **Photographic Proof:** High-resolution digital photograph of the package placed at the recipient's premises or in the recipient's possession, tagged with GPS coordinates and timestamp.
   - **Digital Signature:** Vector stroke data (minimum stroke length $\ge 50$ characters) alongside the printed recipient name.
   - **Recipient Attended OTP:** Validated one-time passcode confirmation.

2. **Assigned Courier Authorization (AT-006 IDOR Defense):**
   Only the courier currently assigned to the parcel may initiate the delivery run, capture proof of delivery, or log attempt failures. Requests from unauthorized couriers are rejected with `403 Forbidden` (`COURIER_NOT_ASSIGNED`).

3. **Fleet Vehicle Capacity Limits:**
   To guarantee delivery safety and transit times, dispatchers cannot assign couriers beyond their vehicle's maximum active parcel load:
   - **Motorcycle / Scooter:** Maximum **25 active parcels**.
   - **Delivery Van:** Maximum **60 active parcels**.
   - **Cargo Truck:** Maximum **120 active parcels**.

4. **Courier Shift Status Policy:**
   Parcel assignments may only be granted to couriers currently marked as active on shift (`ON_SHIFT`). Couriers ending their shift must complete or return all assigned manifests.

---

## 8. Merchant Onboarding, Programmatic API Keys & Webhooks

1. **Merchant Tenant Lifecycle:**
   - Business accounts register as `PENDING` merchant applicants.
   - SwiftDrop Administrators review credentials and transition merchants to `APPROVED` or `REJECTED`.
   - Merchants violating platform terms may be `SUSPENDED` or permanently `DISABLED`.
   - Only `APPROVED` merchants have permission to issue API keys, configure webhooks, and book B2B order manifests.

2. **Programmatic API Key Standards (ADR-003):**
   - **Format:** `sd_live_[0-9A-Za-z]{8}_[0-9A-Za-z]{32}` (8-character prefix + 32-character high-entropy secret).
   - **Active Key Limit:** Maximum of **5 active API keys** per merchant tenant.
   - **Scopes:** Restrictable to `parcels:create` and `parcels:read`.
   - **Zero Plaintext Storage:** Keys are displayed exactly once during generation. SwiftDrop stores only versioned `HMAC-SHA256` hashes with server-side pepper rotation.

3. **Webhook Subscriptions & Delivery Security:**
   - **Transport Security:** Production webhook endpoints strictly require HTTPS (`https://`).
   - **SSRF Defense:** Webhook registrations reject private, loopback, link-local, and cloud metadata IP addresses (`127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.169.254`, `fc00::/7`).
   - **Endpoint Quota:** Maximum of **10 active webhook endpoints** per merchant tenant.
   - **Payload Cryptography:** Webhook payloads are signed using HMAC-SHA256 with server-generated secrets (`whsec_[0-9A-Za-z]{43}`), encrypted at rest with AES-256-GCM.
   - **Retry Schedule:** Webhook deliveries failing with network timeouts or non-2xx HTTP responses retry across 5 exponential backoff intervals: **`0s, 30s, 2m, 10m, 30m`**.
   - **Dead-Letter Queue (DLQ):** Messages failing after 5 retries are routed to `swiftdrop.dlq` (30-day retention) for administrative inspection and replay.

4. **Idempotent Order Creation:**
   Merchant B2B booking requests require an `Idempotency-Key` UUID header. Duplicate submissions with matching payload return the cached response with `Idempotent-Replay: true`. Reusing an idempotency key with modified parameters returns `409 Conflict` (`IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`).

---

## 9. Tracking ID Standards, Real-Time SSE & Data Privacy

1. **Authoritative Tracking Number Specification:**
   - Canonical tracking numbers follow the **18-character format**:
     $$\text{SD-YYYYMMDD-XXXXXX}$$
   - **`SD-`**: Official SwiftDrop network prefix (3 characters).
   - **`YYYYMMDD`**: 8-digit booking date component.
   - **`-`**: Hyphen separator (1 character).
   - **`XXXXXX`**: **6 Crockford Base32 characters** (`0-9`, `A-Z` excluding ambiguous letters `I`, `L`, `O`, `U`).
   - *Examples:* `SD-20261117-EPV1M4`, `SD-20261120-JR47QE`, `SD-20261106-A3X9K7`.

2. **SEC-009 Privacy Masking & Anti-Doxxing Standard:**
   Public parcel tracking endpoints (`GET /api/v1/tracking/public/{trackingNumber}`) are accessible without authentication, enforcing strict privacy sanitization:
   - **Permanently Redacted from Public Access:** Recipient full street addresses, recipient phone numbers, sender personal contact details, courier identity and phone numbers, proof photos, and internal driver notes.
   - **Allowlisted Public Fields:** Canonical tracking number, current operational milestone status, status description, destination city/postal code, and milestone timestamps.

3. **Real-Time Server-Sent Events (SSE) Protocol:**
   - Real-time milestone events stream to client browsers via `GET /api/v1/tracking/public/{trackingNumber}/stream`.
   - **Heartbeat & Proxy Buffering:** Emits `:keepalive\n\n` comments every 15 seconds; sets `X-Accel-Buffering: no` to prevent intermediate gateway proxy stalls.
   - **Horizontal Fan-Out:** Distributed across Tracking API pods using Redis 7 Pub/Sub on channel `tracking:updates`.

---

## 10. User Identity, Roles & Multi-Tenant Access Control

1. **Role-Based Access Control (RBAC) Hierarchy:**
   - **`CUSTOMER`:** Books parcels, estimates fees, accesses personal shipment receipts, and monitors personal order histories.
   - **`MERCHANT`:** Books bulk consignments, configures API keys/webhooks, views business analytics, and monitors tenant deliveries.
   - **`COURIER`:** Views daily delivery manifests, starts delivery runs, records proof of delivery, and logs delivery attempt failures.
   - **`DISPATCHER`:** Monitors the live dispatch board, assigns/reassigns parcels, verifies fleet capacities, and approves delivery reattempts.
   - **`ADMIN`:** Manages user roles, audits security logs, configures delivery zones and postal prefixes, sets system parameters, and reviews merchant applications.

2. **Multi-Tenant Isolation & Anti-IDOR Protections:**
   - Customers may access only parcels associated with their authenticated `user_id`.
   - Merchants may access only orders, webhooks, and credentials associated with their authenticated `tenant_id`.
   - Attempting to access resources belonging to another user or merchant tenant returns `404 Not Found` or `403 Forbidden`.

3. **Authentication Lifecycles:**
   - User credentials must be securely hashed using Argon2id or strong bcrypt with minimum work factors.
   - Stateless JWT access tokens carry user ID, role, and tenant claims. Browser sessions are maintained via secure, `HttpOnly`, `SameSite=Lax` refresh token cookies (`swiftdrop_refresh_token`).

4. **Append-Only Security Audit Trail (SEC-015):**
   All administrative actions, role assignments, zone modifications, and merchant status changes write to `audit_logs`. Database triggers prevent `UPDATE` or `DELETE` on audit tables. Sensitive credentials, secrets, and auth tokens are automatically redacted (`[REDACTED]`).

---

## 11. AI Assistant Operational Policies & Tool Guardrails

1. **Assistant Persona & Grounding:**
   - The AI Assistant is officially named **'SwiftDrop AI'**, serving as the official delivery and logistics assistant for the SwiftDrop courier network in Sri Lanka.
   - The assistant possesses strictly **read-only authority** and cannot directly modify database state, reassign couriers, or alter pricing tariffs.

2. **Zero Hallucination Policy:**
   - The assistant must never invent or hallucinate milestone timestamps, courier names, delivery dates, or tariff rates.
   - If a parcel tracking number is not found or is invalid, the assistant must state that no shipment matches the number and instruct the user to verify the code.

3. **Fee Estimation Invariants:**
   - Delivery fee calculations require package gross weight, origin/destination postal codes, and **all three parcel dimensions (length, width, height in cm)**.
   - The assistant is strictly prohibited from guessing or defaulting package dimensions (e.g. defaulting to 20x15x10 cm). If dimensions are missing, the assistant must prompt the customer for them before executing fee calculations.
   - Fees must always be quoted in whole Sri Lankan Rupees (e.g. `LKR 1,595` or `Rs. 1,595`).

4. **Allowlisted FastMCP Tools:**
   The assistant interfaces with backend services exclusively through sandboxed, allowlisted tools:
   - `track_parcel`: Queries public tracking status with SEC-009 privacy masking.
   - `estimate_shipping_fee`: Queries Core API pricing engine for authoritative tariff calculations.
   - `get_my_active_parcels`: Retrieves active shipments for authenticated customers/merchants with Bearer token forwarding.
   - `explain_company_policies`: Retrieves authoritative operating rules and shipping standards from this manual.
   - `lookup_postal_code`: Resolves 5-digit postal codes and district assignments across all Sri Lankan towns.

5. **Tiered Rate Limiting:**
   AI endpoints enforce defensive rate limits: 20 RPM for anonymous visitors, 60 RPM for authenticated customers, and 100 RPM for merchants and operations staff.

---

## 12. Recipient Claims, Transit Damages & Dispute Resolution

1. **Dispute Filing Window:**
   - All claims regarding damaged package contents, missing items, or delivery discrepancies must be lodged within **7 calendar days** of the parcel reaching `DELIVERED` status.
   - Claims lodged after 7 calendar days are permanently barred from indemnification.

2. **Mandatory Documentation Requirements:**
   To process a damage or loss claim, the claimant must provide:
   - Authoritative SwiftDrop tracking number in canonical format (`SD-YYYYMMDD-XXXXXX`).
   - High-resolution photographic evidence displaying the exterior courier shipping label, outer cardboard packaging damage, and compromised inner contents.
   - Original vendor commercial tax invoice or sales receipt establishing the declared retail value of the items.

3. **Liability Limits & Transit Insurance:**
   - **Standard Uninsured Shipments:** Covered up to a maximum liability ceiling of **LKR 5,000.00** or the proven item value (whichever is lower).
   - **Declared-Value Transit Insurance:** Senders may purchase optional transit insurance during parcel booking at a rate of **1.5% of declared parcel value**, providing indemnification up to the full declared value upon claim approval.

4. **Claims Resolution Timeline:**
   All formal claims are investigated by the SwiftDrop Claims Committee. A formal determination and settlement offer or decline notice is issued within **5 to 7 business days** of complete evidence submission.
