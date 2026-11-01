# Document 1 — Module Scope & Responsibility Specification

**Project:** SwiftDrop — Parcel Delivery & Courier Management System  
**Module:** Tracking API & Real-Time Updates  
**Module Owner:** Member 5  
**Version:** 0.1 — Initial Draft  
**Status:** Draft for team review  
**Primary Technology:** Go, REST/JSON, Server-Sent Events (SSE), PostgreSQL  
**Related Documents:** SwiftDrop SRS, Engineering Standards & Best Practices, Team Work Division

---

## 1. Purpose and Business Problem

The Tracking API provides customers with a way to check the current status of their parcels and follow delivery progress.

Without a dedicated tracking service, tracking requests could compete with parcel booking and other core business operations for resources. A separately deployable Tracking API allows read-heavy tracking traffic to scale independently.

The module provides:

- Public parcel tracking using a tracking number.
- Current parcel status and permitted status history.
- Live status updates through Server-Sent Events (SSE).
- Authenticated access to the permitted tracking history of a customer's own parcels.
- Reliable retrieval of the latest authoritative status after a connection interruption.

The Tracking API is a read-oriented service. It does not own the parcel delivery lifecycle or decide when a parcel changes status.

## 2. Scope and Responsibilities

### 2.1 In scope

The Tracking API is responsible for:

1. Providing documented REST endpoints for public parcel tracking.
2. Returning only approved, privacy-safe tracking information.
3. Providing authenticated tracking history where required.
4. Reading current parcel and status-history data from the authoritative PostgreSQL database or an explicitly approved consistent read interface.
5. Delivering live status updates through SSE, subject to the agreed event-delivery design.
6. Defining event handling, heartbeat, reconnection, event identifiers, and frontend fallback behavior.
7. Validating tracking identifiers and returning safe responses for unknown or invalid tracking requests.
8. Enforcing or coordinating rate limits and applying authentication and authorization to protected endpoints.
9. Supporting independent deployment and scaling.
10. Providing unit, integration, security, contract, and real-time behavior tests.
11. Maintaining the module's API contract, privacy rules, configuration documentation, and README.

### 2.2 Explicit exclusions

The Tracking API must not:

- Create, book, cancel, or modify parcels.
- Change parcel delivery statuses.
- Decide or enforce permitted parcel lifecycle transitions.
- Assign couriers or manage delivery operations.
- Calculate delivery fees or manage pricing.
- Own customer registration, login, or token issuance.
- Own merchant integration or outbound merchant webhook delivery.
- Become the authoritative source of parcel or status-history data.
- Create duplicate parcel or status-history tables that conflict with the shared schema.
- Expose full customer addresses, phone numbers, merchant secrets, credentials, or other unapproved private information.
- Treat Redis, RabbitMQ, an in-memory cache, or SSE events as the authoritative record of parcel status.

These responsibilities remain with the relevant core domain modules and infrastructure components.

## 3. Actors and Consumers

| Actor or module | Interaction with Tracking API |
|---|---|
| Public customer or tracking-page visitor | Requests privacy-safe tracking information using a tracking number. |
| Authenticated customer | Retrieves permitted history for parcels they are authorized to access. |
| Customer frontend (Member 6) | Calls tracking endpoints and consumes SSE updates; uses the agreed fallback behavior. |
| API Gateway (Member 7) | Routes requests and applies agreed gateway-level controls. |
| Core parcel/status modules (Members 2 and 3) | Own parcel data and status changes that the Tracking API reads. |
| Authentication owner (Member 1) | Supplies or coordinates the authentication mechanism and relevant identity claims. |
| Event integration owner (Member 8) | Coordinates the event contract and delivery mechanism for status updates. |
| DevOps (Member 9) | Configures deployment, service networking, health checks, resources, and scaling. |

## 4. Dependencies

### 4.1 Upstream dependencies

The Tracking API depends on:

- **PostgreSQL:** Authoritative committed parcel and status-history data.
- **Core parcel/status modules:** Correctly persisted status changes and history.
- **Authentication and authorization:** Verification of protected requests and parcel ownership.
- **API Gateway:** Agreed external routing, rate limiting, and request controls.
- **Event-delivery mechanism:** Agreed integration for informing connected clients of status changes.
- **Deployment configuration:** Environment variables, network access, health checks, and secrets managed through the team's approved configuration approach.

### 4.2 Downstream dependencies and consumers

The module provides tracking information to:

- The customer tracking interface.
- Authenticated customer history views.
- Other explicitly authorized consumers of the published tracking API.

The frontend must not depend on undocumented database details or internal event implementation details. Consumers should use the published API and event contracts.

## 5. Bounded Context and Data Ownership

### 5.1 Bounded context

The module's bounded context is **parcel tracking and read delivery**.

It is responsible for presenting parcel progress through a stable, privacy-safe interface. It is not responsible for deciding the business meaning or validity of a parcel status transition.

### 5.2 Authoritative data

PostgreSQL remains the authoritative source for parcel and status-history data.

The Tracking API must return the latest committed status and chronological history. It must not present an unapproved stale projection as current.

SSE communicates updates to connected clients; it is not a durable history store. After reconnecting or missing an event, the client must retrieve the latest authoritative state.

### 5.3 Ownership matrix

| Resource | Owner or responsibility | Member 5 boundary |
|---|---|---|
| Parcel records | Core parcel/domain owner | Read only; no parcel mutations. |
| Parcel status history | Approved domain/schema owner | Read only; no direct history modification. |
| Tracking API source code | Member 5 | Owns implementation and module documentation. |
| Tracking REST API contract | Member 5, coordinated with consumers and gateway owner | Defines and maintains the agreed interface. |
| SSE contract | Member 5, coordinated with Member 8 and Member 6 | Defines client-facing event behavior. |
| RabbitMQ exchange/queue and event envelope | Must be agreed with Member 8 | Does not invent or independently redefine the shared contract. |
| Redis Pub/Sub configuration | Shared decision with Member 8 and Member 9 | Integrates only if the team selects this approach. |
| Read-only DB credentials | Provisioned through the agreed database/deployment process | Uses approved credentials; never commits secrets. |
| Dockerfile and service deployment configuration | Member 5 implements service-specific files; Member 9 coordinates deployment conventions | Must align with shared deployment standards. |
| Gateway and ingress configuration | Member 7 / Member 9, according to the team's ownership agreement | Supplies route requirements and verifies integration. |

**Database rule:** Member 5 does not create a separate competing source of truth. Any new tracking-owned table or projection requires explicit design approval before implementation.

## 6. Invariants the Module Must Protect

The following rules must remain true:

1. **Authoritative status:** The latest committed parcel status is the source of truth.
2. **Privacy:** Public tracking responses contain only approved fields.
3. **Authorization:** Private history is available only to an authenticated, authorized customer for their own parcel, subject to the agreed access policy.
4. **No mutations:** Tracking requests never modify parcel status or history.
5. **History integrity:** The API presents status history in the agreed chronological order and does not manufacture missing events.
6. **Safe failure:** Unknown tracking identifiers and internal dependency failures do not expose database details, secrets, or sensitive information.
7. **Real-time consistency:** SSE updates do not replace authoritative reads; reconnecting clients retrieve the current state.
8. **Replica consistency:** If multiple replicas serve connected clients, the agreed fan-out mechanism must deliver relevant updates across replicas.
9. **No implicit stale cache:** Caching is introduced only with explicit freshness and invalidation rules.
10. **Contract stability:** API and event contract changes are coordinated with affected modules and documented.

## 7. What Other Modules May Request

Other modules and approved consumers may:

- Request public tracking information for a tracking number.
- Request permitted tracking history with the required authentication and authorization.
- Subscribe to live status updates using the agreed SSE interface.
- Retrieve the latest tracking state after reconnecting.
- Consume documented responses, error codes, and event schemas.
- Request compatible contract changes through team review.

They may not use the Tracking API to change parcel state, bypass ownership checks, access private fields through public endpoints, or depend on undocumented internal implementation details.

## 8. Proposed Interface Boundaries

The following paths are **proposed and subject to final team agreement**.

| Interface | Proposed path | Purpose |
|---|---|---|
| Public lookup | `GET /api/v1/tracking/public/{trackingNumber}` | Return current privacy-safe tracking information. |
| Public SSE stream | `GET /api/v1/tracking/public/{trackingNumber}/stream` | Deliver live tracking updates. |
| Authenticated history | To be finalized against the approved API contract | Return permitted history for an authorized customer's parcel. |
| Liveness probe | `GET /health/live` | Indicate whether the service process is alive. |
| Readiness probe | `GET /health/ready` | Indicate whether the service is ready to receive traffic. |
| Metrics | `GET /metrics` | Expose operational metrics to the agreed monitoring system. |

The exact authenticated-history route, response DTOs, error envelope, authentication mechanism, and event schema must be finalized in Document 4.

## 9. Ownership of Code and Configuration

The following is a proposed service-level organization. It must be aligned with the actual repository structure before implementation.

| Resource | Proposed ownership |
|---|---|
| Tracking API handlers and application logic | Member 5 |
| Tracking repository/read queries | Member 5, using the approved shared schema |
| Public/private response DTOs | Member 5 |
| SSE handler and client-facing event mapping | Member 5 |
| Tracking API unit and integration tests | Member 5 |
| Tracking API Dockerfile and module README | Member 5 |
| Shared parcel schema and parcel lifecycle rules | Relevant core domain owners |
| Shared event envelope and event publishing contract | Member 8 and relevant producers |
| Gateway routing and external access configuration | Member 7, coordinated with Member 9 |
| Cluster deployment, resource policy, and scaling configuration | Member 9, coordinated with Member 5 |

Actual filenames, package paths, environment-variable names, and database role names will be recorded after the repository and team conventions are confirmed.

## 10. Assumptions and Unresolved Decisions

| Item | Current position | Required follow-up |
|---|---|---|
| Service language | Go, per Team Work Division | Confirm actual module conventions. |
| REST base path | `/api/v1` recommended by Engineering Standards | Confirm gateway routing with Member 7. |
| Public and SSE routes | Proposed in Section 8 | Agree with Members 6, 7, and 9. |
| Database schema | PostgreSQL is authoritative | Obtain approved table and column details from Member 2. |
| Database role and pool limits | Read-only access required; exact values not confirmed | Coordinate with Members 2 and 9. |
| Authentication and ownership verification | Protected history must be authorized | Confirm the authentication contract with Member 1. |
| RabbitMQ exchange, queue, and routing key | Not confirmed | Obtain the agreed event contract from Member 8. |
| Event payload and duplicate/out-of-order handling | Not confirmed | Coordinate with Member 8. |
| Redis fan-out | Optional, depending on the selected multi-replica design | Agree with Members 8 and 9. |
| Health paths | SRS paths proposed: `/health/live`, `/health/ready`, `/metrics` | Confirm shared team decision. |
| Rate limits, heartbeat, timeout, and scaling values | Initial proposals only | Confirm configuration and validate with testing. |
| Caching | No cache assumed initially | Introduce only with approved freshness/invalidation behavior. |

## 11. Out-of-Scope Work

The following are outside this module's ownership:

- Designing or changing the entire shared database schema.
- Implementing parcel booking, dispatch, courier assignment, or delivery-status mutation.
- Implementing customer login or issuing tokens.
- Owning the shared RabbitMQ infrastructure or the outbound webhook worker.
- Defining the gateway's global security policy.
- Choosing cluster-wide PostgreSQL connection limits or Kubernetes capacity policy.
- Building frontend screens owned by Member 6.
- Adding a separate tracking database or stale projection without approval.

## 12. Definition of Done

The scope specification is ready for approval when:

- The Tracking API's responsibilities and exclusions are agreed.
- The data authority and read-only boundary are clear.
- Public and private tracking access boundaries are documented.
- Ownership of code, database access, APIs, events, and deployment configuration is recorded.
- Unresolved integration decisions have named owners or clear follow-up actions.
- The specification is consistent with the approved SRS and team engineering standards.

---

**End of Document 1 — Initial Draft**
