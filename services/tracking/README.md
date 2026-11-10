# Tracking API Service (`services/tracking`)

> **Service:** Read-Optimized Tracking API & Real-Time SSE Streams  
> **Primary Owner:** Member 5 — Tracking API & Real-Time Updates  
> **Deployment Mode:** Independent Deployable Go Service (Port `:8081`)

---

## 1. Service Overview & Responsibilities
The `tracking` service is a standalone read-optimized projection service designed to serve heavy public tracking traffic and real-time live update streams without imposing read locks on the transactional Core API.

### Key Responsibilities:
- Public tracking lookup by canonical tracking number (`SD-YYYYMMDD-XXXXXX`).
- Canonical Server-Sent Events (SSE) streaming (`GET /api/v1/tracking/public/{tracking_number}/stream`).
- Strict SEC-009 public data sanitization and PII redaction.
- Decoupled real-time event source abstraction (`TrackingEventSource`) for Redis Pub/Sub / RabbitMQ integration in Phase 5.
- Truthful health checks (`/health/live`, `/health/ready`).

---

## 2. Invariants & Rules
1. **Read-Only Model:** The tracking service NEVER mutates parcel lifecycle state. It is strictly a projection reader.
2. **Strict Privacy Masking (SEC-009):** Public tracking queries MUST NEVER return recipient full street addresses, phone numbers, sender details, proof photos, or internal operational notes.
3. **SSE Connection Health:** The SSE stream emits periodic heartbeat pings (`:keepalive\n\n`) every 15 seconds to prevent edge proxy disconnections.
4. **Truthful Event Availability:** If the real-time event broker is unconfigured, the SSE endpoint explicitly returns HTTP 503 `DEPENDENCY_UNAVAILABLE` rather than accepting dead or synthetic streams.
5. **Bounded Concurrency & Backpressure:** Per-client subscription buffers are bounded to prevent memory leaks, with non-blocking broadcast drops for stalled clients.

---

## 3. Changelog & Implementation Status
| Date | Phase | Change Summary | Owner |
| :--- | :--- | :--- | :--- |
| 2026-10-21 | - | Initial service architecture baseline created | Architecture Setup |
| 2026-11-10 | Phase 1 | Standalone Go Tracking API foundation, HTTP server, probes | Member 5 |
| 2026-11-10 | Phase 2 | Public tracking REST API, PostgreSQL repository adapter, SEC-009 privacy | Member 5 |
| 2026-11-10 | Phase 4 | Standalone SSE streaming infrastructure, heartbeat, reconnect, backpressure, event source abstraction | Member 5 |
