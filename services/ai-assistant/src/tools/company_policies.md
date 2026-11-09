# SwiftDrop Courier Services — Authoritative Company Policies & Operating Standards

> **Document Type:** Operational Knowledge Base & Courier Policy Manual  
> **Applicability:** All SwiftDrop hubs, merchant accounts, customer interactions, and automated assistants  
> **Effective Date:** 2026-01-01 | **Status:** Active Baseline  

---

## 1. Parcel Dimension & Packaging Standards

1. **Weight Limitations:**
   - Maximum allowable gross weight per parcel: **50.0 kg**.
   - Minimum billable weight: **0.1 kg** (100 grams).
   - Any consignment exceeding 50.0 kg must be broken into separate shipments or booked under Dedicated Freight.

2. **Dimension Constraints:**
   - Maximum parcel girth: **300.0 cm** ($\text{Length} + 2 \times (\text{Width} + \text{Height}) \le 300\text{ cm}$).
   - Maximum single dimension (Length, Width, or Height): **150.0 cm**.
   - Volumetric weight conversion: $\text{Volumetric Weight (kg)} = \frac{\text{Length (cm)} \times \text{Width (cm)} \times \text{Height (cm)}}{5000}$.
   - Billing is computed based on the higher of actual gross weight and volumetric weight.

3. **Packaging Requirements:**
   - All shipments must be secured in rigid corrugated cardboard boxes or tear-resistant courier poly mailers.
   - Fragile items (glassware, ceramics, delicate electronics) must have minimum 5 cm of bubble wrap cushioning on all sides and bear clear "FRAGILE" handling labels.
   - Liquids must be double-sealed in leakproof polyethylene bags inside secondary protective containers.

---

## 2. Prohibited & Restricted Commodities

The following items are strictly **prohibited** from booking or carriage across the SwiftDrop network:

- **Hazardous Materials:** Corrosives, flammable liquids/gases, toxic chemicals, and radioactive substances.
- **Batteries:** Loose lithium-ion or standalone power banks exceeding 100Wh without certified UN38.3 test documentation and hazardous cargo declaration.
- **Controlled Substances:** Narcotics, illegal pharmaceuticals, and unprescribed restricted drugs.
- **Living Organisms:** Live animals, birds, reptiles, insects, and biological cultures.
- **Perishable Cargo:** Unsealed, unrefrigerated raw food items, dairy, and raw seafood.
- **Weapons & Explosives:** Firearms, replicas, ammunition, fireworks, gunpowder, and martial arts weapons.
- **Currency & Bullion:** Physical cash, gold/silver bullion, loose precious gems, and negotiable bearer bonds.

*Any prohibited shipment detected at hub sorting will be quarantined immediately, and relevant civil enforcement authorities notified.*

---

## 3. Transit Service Level Agreements (SLAs)

SwiftDrop operates standard and expedited dispatch routes across Sri Lanka:

| Service Tier | Coverage Area / Postal Codes | Delivery SLA | Operational Notes |
| :--- | :--- | :--- | :--- |
| **Express Delivery** | Colombo Metropolitan & Suburbs (10000–10500) | **Same-Day / 24 Hours** | Bookings confirmed before 11:00 AM dispatched on afternoon run. |
| **Standard Delivery** | Western & Southern Provinces (11000–13500, 80000–82500) | **48 to 72 Hours** | Daily scheduled linehaul between central sorting hubs. |
| **Outstation Delivery** | Northern, Eastern, Central & Uva Provinces (20000–72500) | **3 to 5 Business Days** | Linehaul via regional sorting centers. Sundays and public holidays excluded. |

---

## 4. Delivery Attempt & Handover Policy

1. **Attempt Threshold:**
   - SwiftDrop couriers will make up to **3 consecutive physical delivery attempts** on separate business days.
   - Before each attempt, automated SMS/email notifications and courier phone pre-alerts are dispatched to the recipient.

2. **Failed Attempts & Storage:**
   - If a delivery attempt fails due to recipient unavailability, incorrect address, or refusal to pay COD, the parcel is returned to the nearest local hub.
   - Parcels are held at the destination hub for **5 business days** awaiting recipient instructions or hub pickup.

3. **Return-to-Sender (RTS):**
   - If after 3 delivery attempts and 5 days of hub holding the parcel remains undeliverable, it enters automated `RETURN_TO_SENDER` workflow.
   - Return freight is charged to the merchant/sender account according to contractual terms.

---

## 5. Recipient Claims, Damages & Dispute Resolution

1. **Dispute Window:**
   - All claims regarding damaged contents, missing items, or delivery discrepancies must be lodged within **7 calendar days** of the parcel reaching `DELIVERED` status.
   - Claims lodged after 7 calendar days will not be eligible for indemnification.

2. **Claim Requirements:**
   - Valid SwiftDrop tracking number (format: `SD-YYYYMMDD-XXXXX`).
   - High-resolution photographic evidence of the outer packaging, shipping label, and damaged contents.
   - Original commercial invoice or purchase receipt proving item value.

3. **Liability Limits:**
   - Standard uninsured shipments are covered up to a maximum liability of LKR 5,000 or the declared item value (whichever is lower).
   - Additional declared-value transit insurance is available during booking at a rate of 1.5% of declared value.
