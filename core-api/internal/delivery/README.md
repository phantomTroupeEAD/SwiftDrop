# Delivery Records Module (`internal/delivery`)

> **Domain:** Delivery Attempts, Proof of Delivery (Photo/Signature) & Failure Reason Tracking  
> **Primary Owner:** Member 3 — Dispatch, Couriers & Proof of Delivery  
> **Clean Architecture Boundary:** Modular Monolith Internal Module

---

## 1. Module Overview & Responsibilities
The `delivery` module governs physical courier interactions at the recipient's doorstep: capturing proof of delivery (POD), handling customer recipient confirmation, recording delivery failure reasons, and managing re-attempt policies.

### Key Responsibilities:
- Recording successful delivery with verified proof attachments (secure photo upload or recipient signature).
- Logging failed delivery attempts with structured reason codes.
- Incrementing attempt counters (triggering return-to-hub after 3 failed attempts).
- Coordinating with `internal/parcels` to execute the terminal `DELIVERED` or intermediary `ATTEMPT_FAILED` state transition.

---

## 2. Domain Invariants & Business Rules
1. **Mandatory Proof for Completion:** A parcel CANNOT transition to `DELIVERED` without a valid proof record (photo URL, recipient signature data, or OTP confirmation).
2. **Assigned Courier Check:** Only the courier currently assigned to the parcel may record a delivery or attempt.
3. **Attempt Limit:** A maximum of 3 delivery attempts are permitted. On the 3rd failed attempt, the parcel must transition to `RETURNED`.
4. **Required Failure Classification:** Every failed attempt MUST record a standard reason code (`RECIPIENT_UNAVAILABLE`, `INCORRECT_ADDRESS`, `PREMISES_INACCESSIBLE`, `RECIPIENT_REJECTED`).

---

## 3. Layer Breakdown
```
internal/delivery/
├── domain/                  # DeliveryAttempt entity, ProofOfDelivery aggregate, FailureReason enum
├── application/             # CompleteDelivery, RecordFailedAttempt use cases
├── infrastructure/          # PostgresDeliveryRepository, ObjectStorageClient (for photos)
└── transport/               # HTTP handlers (/api/v1/deliveries/*) and DTOs
```

---

## 4. Database Ownership
- `delivery_attempts`: Attempt ID, parcel ID, courier ID, attempt number (1-3), outcome (`SUCCESS`, `FAILED`), failure reason code, proof type (`PHOTO`, `SIGNATURE`, `RECIPIENT_CONFIRMATION`), proof attachment URL / storage path, signature data, received by name, GPS coordinates, notes, attempted_at timestamp. Migrated via `000007_create_dispatch_and_delivery.up.sql`.

---

## 5. Events Emitted
- `parcel.delivered.v1`: Published upon successful delivery with proof.
- `parcel.delivery_failed.v1`: Published upon failed attempt.
- `parcel.returned.v1`: Published upon 3rd failed attempt or refusal.

---

## 6. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Standardized single delivery_attempts table DDL, decoupled storage health checks, and ADR-004 failure reason codes | Member 3 & Review Agent |
