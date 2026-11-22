# In-App Notifications Module (`internal/notifications`)

> **Domain:** In-App User Notifications & Inbox Feed  
> **Primary Owner:** Member 8 / Member 5 — Notifications & Real-Time Projections  
> **Clean Architecture Boundary:** Modular Monolith Internal Module

---

## 1. Module Overview & Responsibilities

The `notifications` module manages in-app notifications delivered to authenticated SwiftDrop users across all roles (Customers, Merchants, Couriers, Dispatchers, and Admins).

### Key Responsibilities:
- User notification inbox retrieval with keyset (cursor-based) pagination.
- Real-time unread notification count aggregation.
- Notification state transitions (`is_read: true`, `read_at` recording).
- Batch mark-all-as-read operations scoped strictly to the authenticated caller.
- Support for operational broadcast notifications targeting operations staff (e.g. `role:DISPATCHER`).

---

## 2. Domain Invariants & Business Rules

1. **Mandatory Identity Scoping & Object Isolation:**
   - Every read and write operation is strictly scoped to the authenticated caller (`user_id`).
   - Direct object reference access to foreign notifications is rejected with `403 Forbidden` (`NOTIFICATION_FORBIDDEN`).
2. **Operational Role Broadcast Scoping:**
   - Users with the `DISPATCHER` role automatically receive personal notifications addressed to their `user_id` plus operational broadcast notifications addressed to `role:DISPATCHER`.
   - Non-dispatcher users cannot query or mark read any `role:DISPATCHER` notifications.
3. **Notification Aggregate Invariants:**
   - An in-app notification must have a valid non-empty `id`, `user_id`, `event_type`, `aggregate_id`, `title`, and `body`.
   - In-memory domain validation (`Validate()`) enforces these invariants prior to persistence.
4. **Deterministic Keyset Pagination:**
   - Pagination uses base64url-encoded tuples `(created_at, id)` ordered by `created_at DESC, id DESC`.
   - Prevents page-drift and duplicate entries during concurrent notification arrivals.
5. **State Transition Idempotency:**
   - Marking a notification as read transitions `is_read` to `true` and records `read_at`.
   - Subsequent calls preserve the initial `read_at` timestamp.

---

## 3. Layer Breakdown

```
internal/notifications/
├── domain/                      # Pure Domain Logic (Zero HTTP / SQL imports)
│   ├── notification.go          # Notification aggregate root, invariants, errors
│   └── notification_test.go     # Unit tests for aggregate invariants & state changes
├── application/                 # Use Cases & Collaborator Ports
│   ├── ports.go                 # NotificationRepository interface, cursor models
│   ├── service.go               # Notifications application service & query use cases
│   └── service_test.go          # Mock-backed unit tests for all application use cases
├── infrastructure/              # Concrete PostgreSQL Persistence
│   ├── postgres_repository.go   # PostgresRepository implementing NotificationRepository
│   └── postgres_repository_test.go # Smoke tests & interface conformance assertions
├── transport/                   # HTTP REST Transport Layer
│   ├── dto.go                   # Response DTOs & domain-to-transport mappers
│   ├── handlers.go              # HTTP handlers with standard swag docstring annotations
│   ├── routes.go                # ServeMux route registration with SecurityMiddleware
│   └── handlers_test.go         # Comprehensive HTTP handler unit tests
├── README.md                    # Module Living Documentation
└── API.md                       # Living REST API Specifications
```

---

## 4. Event & Asynchronous Interactions

- **Consumed Events:** Background consumers in `services/messaging/` consume committed domain events from `q.inapp.notifications` (e.g. `parcel.booked`, `parcel.assigned`, `parcel.delayed`, `delivery.attempt_failed`, `delivery.succeeded`).
- **Persistence Target:** Consumer workers populate the `user_notifications` table with appropriate `user_id`, `event_type`, `aggregate_id`, `title`, and `body`.
- **Outbox Integrity:** All state updates on `user_notifications` adhere to transactional durability and outbox consistency.

---

## 5. Data Ownership & Schema

This module is the authoritative reader and state updater for the `user_notifications` table:
- **Migration:** `core-api/migrations/20261120000001_create_user_notifications.up.sql`
- **Columns:**
  - `id` (UUID PK default `gen_random_uuid()`)
  - `user_id` (`TEXT NOT NULL`)
  - `tenant_id` (`TEXT NULL`)
  - `event_type` (`TEXT NOT NULL`)
  - `aggregate_id` (`TEXT NOT NULL`)
  - `title` (`TEXT NOT NULL`)
  - `body` (`TEXT NOT NULL`)
  - `metadata` (`JSONB NOT NULL DEFAULT '{}'`)
  - `is_read` (`BOOLEAN NOT NULL DEFAULT FALSE`)
  - `created_at` (`TIMESTAMPTZ NOT NULL DEFAULT now()`)
  - `read_at` (`TIMESTAMPTZ NULL`)
- **Indexes:**
  - `idx_user_notifications_user_unread` (`user_id, is_read, created_at DESC`)
  - `idx_user_notifications_tenant` (`tenant_id, created_at DESC WHERE tenant_id IS NOT NULL`)
  - `idx_user_notifications_created_at` (`created_at`)

---

## 6. Implementation Status

- **Status:** Complete & Verified
- **Tests Passing:** `go test -v ./internal/notifications/...` (100% pass rate)
- **Active Endpoints:**
  - `GET /api/v1/notifications`
  - `GET /api/v1/notifications/unread-count`
  - `PATCH /api/v1/notifications/{id}/read`
  - `PATCH /api/v1/notifications/read-all`
