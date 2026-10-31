# Member 02 - Parcel Aggregate & Pricing Specifications

> **Owner:** Member 02
> **Status:** Initial Draft Space for Review in Meeting 2
> **Target Production Location:** See docs/architecture/domain-boundaries.md

---

## Required Documents Checklist

- [x] doc-01-scope-and-responsibilities.md
- [x] doc-02-functional-flows-and-business-rules.md
- [x] doc-03-internal-architecture-and-design.md
- [x] doc-04-interface-and-integration-contract.md
- [x] doc-05-data-and-persistence.md
- [x] doc-06-testing-strategy-and-acceptance.md
- [x] doc-07-security-failure-and-recovery.md
- [x] doc-08-implementation-deployment-and-ops.md

---

## Clean, Simple & Authoritative Member 2 Diagram Set (Canonical 8 Diagrams)

The canonical set of 8 clean, simple, uncrowded, and technically accurate diagrams for Member 2 (Parcel Management & Pricing) is maintained directly under `diagrams/`:

| Diagram # | Diagram Name | Type | Lucidchart Edit Link | High-Res Preview & Source |
| :--- | :--- | :--- | :--- | :--- |
| **01** | **Parcel Management & Pricing Use-Case Diagram** | UML Use Case | [Open in Lucidchart](https://lucid.app/lucidchart/da53bbf3-1b48-450c-9945-b1816a32ae17/edit) | [`01-parcel-pricing-use-case.png`](diagrams/01-parcel-pricing-use-case.png) / [`.puml`](diagrams/01-parcel-pricing-use-case.puml) |
| **02** | **Delivery Fee Estimation Activity Diagram** | UML Activity (5 Swimlanes) | [Open in Lucidchart](https://lucid.app/lucidchart/e9ac9f8d-5d88-4e74-9a52-5e09e5c696b4/edit) | [`02-delivery-fee-estimation-activity.png`](diagrams/02-delivery-fee-estimation-activity.png) / [`.puml`](diagrams/02-delivery-fee-estimation-activity.puml) / [`.svg`](diagrams/02-delivery-fee-estimation-activity.svg) |
| **03** | **Delivery Fee Estimation Sequence Diagram** | UML Sequence | [Open in Lucidchart](https://lucid.app/lucidchart/456c449a-5f68-4d12-a051-027a04e87663/edit) | [`03-delivery-fee-estimation-sequence.png`](diagrams/03-delivery-fee-estimation-sequence.png) / [`.puml`](diagrams/03-delivery-fee-estimation-sequence.puml) |
| **04** | **Customer Parcel Booking Activity Diagram** | UML Activity (5 Swimlanes) | [Open in Lucidchart](https://lucid.app/lucidchart/48f368fa-a237-4fea-bb84-c454dd788216/edit) | [`04-parcel-booking-activity.png`](diagrams/04-parcel-booking-activity.png) / [`.puml`](diagrams/04-parcel-booking-activity.puml) / [`.svg`](diagrams/04-parcel-booking-activity.svg) |
| **05** | **Customer Parcel Booking Sequence Diagram** | UML Sequence | [Open in Lucidchart](https://lucid.app/lucidchart/c2b3114e-a7b8-4313-8172-1a82336a8cc1/edit) | [`05-parcel-booking-sequence.png`](diagrams/05-parcel-booking-sequence.png) / [`.puml`](diagrams/05-parcel-booking-sequence.puml) |
| **06** | **Parcel Lifecycle State Machine Diagram** | UML State Machine | [Open in Lucidchart](https://lucid.app/lucidchart/43a950d5-3596-4bb5-84c7-64f813f48a7c/edit) | [`06-parcel-lifecycle-state-machine.png`](diagrams/06-parcel-lifecycle-state-machine.png) / [`.puml`](diagrams/06-parcel-lifecycle-state-machine.puml) |
| **07** | **Parcel & Pricing Component Diagram** | UML Component (Clean Arch) | [Open in Lucidchart](https://lucid.app/lucidchart/983208a4-c9f1-44f3-a6d8-118b89757207/edit) | [`07-parcel-pricing-component.png`](diagrams/07-parcel-pricing-component.png) / [`.puml`](diagrams/07-parcel-pricing-component.puml) |
| **08** | **Parcel & Pricing Entity Relationship Diagram** | Crow's Foot ERD | [Open in Lucidchart](https://lucid.app/lucidchart/d51127dc-51ad-4542-843a-423d44d6fd16/edit) | [`08-parcel-pricing-erd.png`](diagrams/08-parcel-pricing-erd.png) / [`.puml`](diagrams/08-parcel-pricing-erd.puml) |

---

## Authoritative Design Rules & Unresolved Decisions

The canonical diagram set enforces the following domain rules and explicit status determinations:

1. **Monetary Representation:** Authoritative delivery fees are modeled exclusively as integer minor units (`BIGINT` / `int64` LKR cents). No floating-point or `NUMERIC(6,2)` types for money.
2. **Address Modeling:** Parcel addresses are stored as immutable JSONB snapshots (`pickup_address`, `dropoff_address`) directly on the `parcels` table.
3. **Parcel Lifecycle State Machine:** Strictly adheres to the canonical 9 states from ADR-004 (`CREATED`, `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY`, `DELIVERED`, `ATTEMPT_FAILED`, `RETURNED`, `CANCELLED`). Transient delivery failures increment `attempt_count`; reaching the limit of 3 transitions to `RETURNED`.
4. **Clean Architecture Module Isolation:** Domain packages (`domain/`) have zero imports of transport or database packages; all outbound operations use interfaces implemented by `infrastructure/`.
5. **[PROVISIONAL — PENDING TEAM CONFIRMATION]:**
   - **Tariff Model & Volumetric Divisor:** The complete zone-to-zone tariff matrix across 7 weight bands, the volumetric divisor of 6,000 ($VW = L \times W \times H / 6000$), and the 1.00 kg Option A volumetric exemption threshold are provisional proposals from Member 2 pending final administrative sign-off by Member 7.
   - **Quote Freeze TTL:** The 15-minute price freeze guarantee (`expires_at = NOW + 15m`) is a provisional proposal pending final team confirmation.
