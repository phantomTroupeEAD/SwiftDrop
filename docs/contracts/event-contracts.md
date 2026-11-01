# SwiftDrop — Event Contracts & Messaging Standards

> **Authority:** All asynchronous publishers (Outbox Relay) and consumers (Event Worker, Notification Service, Webhook Worker) must adhere to these schemas and contracts.

---

## 1. Canonical Event Envelope Schema

Every domain and integration event emitted in SwiftDrop must use the following standard JSON envelope:

```json
{
  "event_id": "evt_01HXYZ9876543210ABC",
  "event_type": "parcel.delivered.v1",
  "aggregate_type": "parcel",
  "aggregate_id": "prcl_01HXYZ1234567890",
  "version": 1,
  "timestamp": "2026-10-21T11:15:00Z",
  "correlation_id": "corr_c8a9f24e-7b12-4f33-912a",
  "producer": "core-api.parcels",
  "payload": {
    "tracking_number": "SD-20261021-99812",
    "previous_status": "OUT_FOR_DELIVERY",
    "new_status": "DELIVERED",
    "actor_id": "usr_courier_123",
    "actor_role": "COURIER",
    "proof_id": "prf_photo_456",
    "occurred_at": "2026-10-21T11:14:55Z"
  }
}
```

### Envelope Field Definitions
- `event_id`: Unique UUIDv7 or ULID for event deduplication.
- `event_type`: Dot-separated namespace and version (e.g., `parcel.booked.v1`).
- `aggregate_type`: The source bounded context aggregate (e.g., `parcel`, `courier`, `merchant`).
- `aggregate_id`: Primary ID of the mutated aggregate.
- `version`: Monotonically increasing schema version for backward compatibility.
- `timestamp`: UTC ISO-8601 timestamp when the event was generated.
- `correlation_id`: Distributed tracing ID passed from the initial HTTP request.
- `producer`: Service and module name emitting the event.
- `payload`: Event-specific business data.

---

## 2. Core Event Catalog

| Event Type | Producer Module | Triggering Action | Key Payload Fields |
| :--- | :--- | :--- | :--- |
| `parcel.booked.v1` | `internal/parcels` | Customer / Merchant books new parcel | `tracking_number`, `sender_id`, `recipient_address`, `fee` |
| `parcel.assigned.v1` | `internal/dispatch`| Dispatcher assigns courier | `tracking_number`, `courier_id`, `assigned_by`, `assigned_at` |
| `parcel.picked_up.v1`| `internal/parcels` | Courier collects parcel from sender | `tracking_number`, `courier_id`, `hub_id`, `picked_up_at` |
| `parcel.out_for_delivery.v1` | `internal/dispatch` | Courier starts final mile run | `tracking_number`, `courier_id`, `estimated_delivery_window` |
| `parcel.delivered.v1`| `internal/parcels` | Courier confirms delivery with proof | `tracking_number`, `courier_id`, `proof_type`, `proof_url` |
| `parcel.delivery_failed.v1` | `internal/delivery`| Courier records unsuccessful attempt | `tracking_number`, `courier_id`, `attempt_number`, `reason_code` |
| `merchant.webhook_dispatch.v1` | `internal/merchants` | Event Worker requests webhook push | `merchant_id`, `webhook_url`, `event_payload`, `signature` |

---

## 3. Transactional Outbox Table Schema

State changes and event creation must commit atomically in the Core API PostgreSQL database:

```sql
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version INTEGER DEFAULT 1 NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    schema_version VARCHAR(16) DEFAULT '1.0.0' NOT NULL,
    payload JSONB NOT NULL,
    correlation_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) DEFAULT 'PENDING' NOT NULL 
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
    attempt_count INTEGER DEFAULT 0 NOT NULL,
    next_attempt_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    published_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending 
ON outbox_events (created_at ASC) 
WHERE status = 'PENDING';
```

---

## 4. RabbitMQ Topology & Exchange Conventions

- **Exchange:** Topic exchange named `swiftdrop.events.topic` (Durable).
- **Routing Keys:** `{aggregate_type}.{action}.v{version}` (e.g., `parcel.delivered.v1`, `parcel.*.v1`).
- **Queues:**
  - `q.tracking.updates`: Consumed by Tracking API to update read projections / SSE subscribers.
  - `q.notifications.worker`: Consumed by Event Worker to trigger customer SMS/email notifications.
  - `q.webhooks.dispatcher`: Consumed by Webhook Worker to sign and post updates to merchant endpoints.
- **Dead-Letter Queue (DLQ):**
  - Messages failing processing after 5 bounded exponential backoff retries must be routed to `swiftdrop.dlq`.
  - The DLQ maintains poison messages for operator inspection without stalling message consumption.

---

## 5. Consumer Idempotency Rules

Because RabbitMQ delivers with **at-least-once** semantics, network blips or crashes may cause duplicate event delivery.

- **Mandatory Deduplication:** Every consumer must store processed `event_id` records in the `processed_events` table:
```sql
CREATE TABLE IF NOT EXISTS processed_events (
    event_id VARCHAR(64) NOT NULL,
    consumer_name VARCHAR(64) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (event_id, consumer_name)
);
```
- If an incoming event's `event_id` already exists for this consumer, the consumer MUST acknowledge the message and skip processing without error.
