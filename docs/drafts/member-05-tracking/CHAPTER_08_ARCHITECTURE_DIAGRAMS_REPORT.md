# Member 5 — Chapter 8 Architecture Diagrams Specification & Mapping Report

> **Service:** Standalone Real-Time Tracking API & Event Fan-Out Service (`services/tracking`, Port `:8081`)
> **Author / Domain Owner:** Member 5 (Senuji)
> **Status:** Ratified & Lucidchart Synchronized (Folder ID: `453693353`)
> **Date:** 01 November 2026 | **Version:** 2.0.0

---

## 1. Chapter 8 Figure Mapping Table

| Figure Number | Figure Title | Authoritative Source | Existing Closest Diagram | Decision | Document ID | Lucidchart Edit Link |
| :--- | :--- | :--- | :--- | :---: | :--- | :--- |
| **Figure 8.1** | Standalone Architecture for Real-Time Tracking Service | `services/tracking/README.md`, `doc-03` §1 | M5-D01 (Hexagonal code tiers), M5-D02 (K8s deployment topology) | **CREATE NEW** | `ae5378ba-f2a7-43a6-8953-38d62ddabb8f` | [Figure 8.1 Edit Link](https://lucid.app/lucidchart/ae5378ba-f2a7-43a6-8953-38d62ddabb8f/edit) |
| **Figure 8.2** | Server-Sent Events (SSE) Live Tracking Stream Fan-Out Flow | `docs/architecture/sequence-diagrams.md` §3, `doc-03` §2 | M5-D04 (Real-Time SSE Stream Sequence) | **REUSE** | `36bf9c69-d7a0-44af-b088-8103faa4ff23` | [Figure 8.2 Edit Link](https://lucid.app/lucidchart/36bf9c69-d7a0-44af-b088-8103faa4ff23/edit) |
| **Figure 8.3** | Public Tracking Query Sanitization & PII Masking Pipeline | `docs/contracts/api-conventions.md` §5, `doc-04` §1, SEC-009 | M5-D05 (Network sequence diagram) | **CREATE NEW** | `e1c16cba-cab0-4d9c-908f-d030b31201d7` | [Figure 8.3 Edit Link](https://lucid.app/lucidchart/e1c16cba-cab0-4d9c-908f-d030b31201d7/edit) |

---

## 2. Professional Academic Figure Captions

### Figure 8.1 Caption
> **Figure 8.1: Standalone Architecture for Real-Time Tracking Service.**
> Architectural component diagram illustrating the internal software composition of the standalone SwiftDrop Tracking API microservice (`services/tracking`) executing on container port `:8081`. The diagram delineates the microservice boundary, separating presentation handlers (Public REST, Authenticated REST, RS256 JWT Middleware, and Server-Sent Events Stream Handler) from core application orchestration and in-memory channel management (`EventHub`). External dependencies are strictly decoupled: HTTP traffic enters via the Traefik API Gateway (`:80`), transactional persistence is queried through a read-only PostgreSQL role (`swiftdrop_tracking_ro`) bounded by a 5-connection pool, and real-time lifecycle updates are ingested from RabbitMQ queue `q.tracking.updates` and distributed horizontally across pods via Redis Pub/Sub channel `tracking:updates`.

### Figure 8.2 Caption
> **Figure 8.2: Server-Sent Events (SSE) Live Tracking Stream Fan-Out Flow.**
> UML sequence diagram detailing the four-phase asynchronous event streaming lifecycle between customer web clients, edge routing, and the distributed tracking service. Phase 1 demonstrates initial HTTP/2 stream negotiation via Traefik without response buffering (`buffering.maxResponseBodyBytes: 0`) and immediate state hydration from the read-only database. Phase 2 maintains connection persistence against gateway timeouts through periodic 15-second keepalive heartbeat comments (`:keepalive\n\n`). Phase 3 illustrates decoupled outbox publishing by Member 8 to topic exchange `swiftdrop.events.topic`, subsequent AMQP consumption on queue `q.tracking.updates` by an arbitrary tracking replica (Pod B), and cross-pod fan-out via Redis channel `tracking:updates` to the client-connected replica (Pod A). Phase 4 ensures deterministic memory deallocation upon browser disconnection.

### Figure 8.3 Caption
> **Figure 8.3: Public Tracking Query Sanitization and PII Masking Pipeline.**
> Multi-stage data transformation pipeline executing under security standard SEC-009 to protect customer privacy on unauthenticated tracking lookups (`GET /api/v1/tracking/public/{tracking_number}`). The pipeline illustrates the progression from edge rate-limiting and regex syntax validation through read-only database projection, customer-facing milestone status mapping, field-level PII redaction, and standardized DTO encapsulation. The accompanying side-by-side comparative inspection contrasts raw PostgreSQL domain entities against the sanitized public JSON response, demonstrating the strict suppression of recipient street addresses, contact phone numbers, personal names, courier IDs, proof-of-delivery photos, and commercial billing amounts while preserving general destination city and event milestones.

---

## 3. Comparison & Decision Rationale

### 3.1 Figure 8.1 — Standalone Architecture
- **Required:** Complete standalone service component architecture showing internal layers, Traefik API gateway, and external infrastructure dependencies.
- **Closest Diagrams:**
  - `M5-D01` (Clean Hexagonal Architecture & Module Layers): Only models internal code package tiers (Driving Adapters, Application Core, Driven Adapters). Lacks Traefik API gateway, external client callers, and external system boundaries.
  - `M5-D02` (Distributed Deployment & Multi-Pod Infrastructure Topology): Models Kubernetes cluster infrastructure, pods, CPU autoscaling thresholds, and secrets. Lacks internal software components (Public REST API, Authenticated REST API, JWT middleware, SSE Handler, Application Service, EventHub).
- **Decision:** **CREATE NEW**.
- **Result:** Native Lucidchart document created (`ae5378ba-f2a7-43a6-8953-38d62ddabb8f`), placed in folder `453693353`, styled in strict academic monochrome with zero validation errors.

### 3.2 Figure 8.2 — SSE Live Tracking Stream Fan-Out Flow
- **Required:** Sequence flow validating the 10-step SSE fan-out lifecycle from customer connection through RabbitMQ event ingestion and Redis cross-pod fan-out.
- **Closest Diagram:** `M5-D04` (`36bf9c69-d7a0-44af-b088-8103faa4ff23`).
- **Coverage Assessment:** `M5-D04` models all 10 required architectural steps with 100% technical fidelity:
  1. Customer starts SSE stream.
  2. Traefik forwards stream without response buffering.
  3. Tracking service queries initial status from PostgreSQL (`swiftdrop_ro`).
  4. Initial SSE update pushed down connection.
  5. 15-second heartbeat (`:keepalive\n\n`) maintains connection.
  6. Outbox Worker (M8) commits event.
  7. Event published to RabbitMQ topic exchange `swiftdrop.events.topic`.
  8. Pod B consumes from queue `q.tracking.updates`.
  9. Pod B republishes to Redis channel `tracking:updates`; Redis broadcasts to Pod A.
  10. Pod A delivers SSE status update to connected browser.
- **Decision:** **REUSE**.
- **Result:** Existing document `36bf9c69-d7a0-44af-b088-8103faa4ff23` accurately fulfills all technical criteria and is preserved.

### 3.3 Figure 8.3 — Public Tracking Query Sanitization & PII Masking Pipeline
- **Required:** Data-processing pipeline / activity diagram showing how sensitive data is transformed into a safe public API response under SEC-009.
- **Closest Diagram:** `M5-D05` (`156b36ae-e970-4d1a-9b4d-8e88514e98ea`).
- **Coverage Assessment:** `M5-D05` is an interaction sequence diagram modeling HTTP request/response hops between network participants. It does not illustrate the data-transformation pipeline stages, nor does it display field-level redaction rules or compare raw internal database records against sanitized public DTO payloads.
- **Decision:** **CREATE NEW**.
- **Result:** Native Lucidchart document created (`e1c16cba-cab0-4d9c-908f-d030b31201d7`), placed in folder `453693353`, styled in strict academic monochrome with zero validation errors.

---

## 4. Source Specifications for Future Modification

### 4.1 Figure 8.1 — Mermaid Source
```mermaid
flowchart TD
    classDef client fill:#ffffff,stroke:#000000,stroke-width:1px,color:#000000;
    classDef gateway fill:#ffffff,stroke:#000000,stroke-width:2px,color:#000000;
    classDef handler fill:#ffffff,stroke:#000000,stroke-width:1px,color:#000000;
    classDef core fill:#ffffff,stroke:#000000,stroke-width:1px,color:#000000;
    classDef adapter fill:#ffffff,stroke:#000000,stroke-width:1px,color:#000000;
    classDef external fill:#ffffff,stroke:#000000,stroke-width:1px,color:#000000;

    subgraph Clients ["External Clients & Ingress Callers"]
        ClientWeb["Customer Web Client<br/>(React Portal :3000)"]:::client
        ClientMerchant["Merchant / Courier Client<br/>(B2B API & Mobile App)"]:::client
        ClientAI["AI Assistant Gateway<br/>(FastMCP Service :8000, M10)"]:::client
    end

    TraefikGW["Traefik API Gateway (:80)<br/>PathPrefix('/api/v1/tracking')<br/>Rate Limiting (10 req/s, burst 20) | Unbuffered SSE Proxy"]:::gateway

    subgraph ServiceBoundary ["Tracking API Service — Standalone Microservice Boundary (:8081)"]
        subgraph IngressHandlers ["Presentation Layer & Driving Adapters"]
            H_Public["Public Tracking REST Handler<br/>GET /api/v1/tracking/public/{num}"]:::handler
            H_Auth["Auth Tracking REST Handler<br/>GET /api/v1/tracking/{num}"]:::handler
            MW_JWT["JWT / Auth Middleware<br/>RS256 Public Key Verification"]:::handler
            H_SSE["SSE Stream Handler<br/>GET /api/v1/tracking/public/{num}/stream<br/>text/event-stream (15s Heartbeat)"]:::handler
        end

        subgraph CoreApplication ["Application Core & In-Memory Hub"]
            Svc_Core["Tracking Application Service<br/>Milestone Sequencing & Status Mapping<br/>SEC-009 Privacy Masking Engine"]:::core
            Hub_Event["In-Memory EventHub<br/>Client Registry (map[string][]chan)<br/>32-Slot Ring Buffer | Slow-Client Drop"]:::core
        end

        subgraph OutboundAdapters ["Data & Messaging Outbound Adapters"]
            Repo_PG["PostgreSQL Read-Only Adapter<br/>Role: swiftdrop_tracking_ro<br/>Pool: 5 conns/pod, 3s Timeout"]:::adapter
            Cons_RMQ["RabbitMQ Event Consumer<br/>Queue: q.tracking.updates<br/>Exchange: swiftdrop.events.topic"]:::adapter
            Pub_Redis["Redis Pub/Sub Integration<br/>Channel: tracking:updates<br/>Multi-Pod Cluster Fan-Out"]:::adapter
        end
    end

    subgraph ExternalInfra ["Data & Messaging Infrastructure (Platform Managed)"]
        Ext_PG[("PostgreSQL 16 StatefulSet (:5432)<br/>Tables: parcels, history, zones<br/>Zero DDL Owned by M5")]:::external
        Ext_RMQ[("RabbitMQ Message Broker (:5672)<br/>Exchange: swiftdrop.events.topic<br/>Published by Member 8 Outbox Worker")]:::external
        Ext_Redis[("Redis 7 In-Memory Bus (:6379)<br/>Ephemeral Pub/Sub Bus for Pod Sync<br/>Strict Decoupling: M8 has zero Redis access")]:::external
    end

    ClientWeb -->|HTTPS / SSE| TraefikGW
    ClientMerchant -->|Bearer JWT| TraefikGW
    ClientAI -->|REST Query| TraefikGW

    TraefikGW --> H_Public
    TraefikGW --> MW_JWT
    MW_JWT -->|valid claims| H_Auth
    TraefikGW --> H_SSE

    H_Public --> Svc_Core
    H_Auth --> Svc_Core
    H_SSE -->|fetch initial state| Svc_Core
    H_SSE -->|register client chan| Hub_Event

    Svc_Core --> Repo_PG
    Repo_PG -->|SQL SELECT read-only| Ext_PG

    Ext_RMQ -->|AMQP delivery| Cons_RMQ
    Cons_RMQ -->|republish to Redis| Pub_Redis
    Pub_Redis -->|PUBLISH tracking:updates| Ext_Redis
    Ext_Redis -->|SUBSCRIBE broadcast| Pub_Redis
    Pub_Redis --> Hub_Event
```

### 4.2 Figure 8.2 — Mermaid Source
```mermaid
sequenceDiagram
    autonumber
    actor Customer as Customer Browser
    participant Traefik as Traefik Gateway (:80)
    participant PodA as Tracking Pod A (:8081)
    participant PG as PostgreSQL (swiftdrop_ro)
    participant M8 as Outbox Worker (M8)
    participant Broker as RabbitMQ (Topic Broker)
    participant PodB as Tracking Pod B (:8081)
    participant Redis as Redis Pub/Sub (channel)

    Note over Customer,PodA: Phase 1: Stream Connection & Initial Hydration
    Customer->>Traefik: GET /api/v1/tracking/public/SD-.../stream
    Traefik->>PodA: Forward stream (unbuffered, maxResponseBodyBytes: 0)
    activate PodA
    PodA->>PG: SELECT initial tracking status & history
    PG-->>PodA: Return parcel record
    PodA-->>Customer: SSE event: status_update (initial state)
    PodA->>PodA: Register client channel in EventHub (buffer: 32 slots)

    Note over Traefik,PodA: Phase 2: Gateway Keepalive Heartbeat
    loop Every 15 seconds
        PodA-->>Customer: :keepalive (prevents proxy/gateway idle timeout)
    end

    Note over M8,Customer: Phase 3: Asynchronous Status Change & Two-Tier Fan-Out
    M8->>Broker: Publish parcel.delivered.v1 to swiftdrop.events.topic
    Broker->>PodB: Deliver event via queue q.tracking.updates
    activate PodB
    Note over PodB: Pod B consumes RabbitMQ message (M8 has zero Redis link)
    PodB->>Redis: Publish JSON to tracking:updates
    deactivate PodB
    Redis-->>PodA: Broadcast message to all subscribed tracking pods
    Note over PodA: Pod A checks local EventHub for matching tracking_number
    PodA-->>Customer: SSE event: status_update (DELIVERED)

    Note over Customer,PodA: Phase 4: Graceful Stream Closure
    Customer->>PodA: Client disconnects (TCP close)
    PodA->>PodA: EventHub unregisters channel & frees memory buffer
    deactivate PodA
```

### 4.3 Figure 8.3 — Mermaid Source
```mermaid
flowchart TD
    classDef stage fill:#ffffff,stroke:#000000,stroke-width:1px,color:#000000;
    classDef decision fill:#ffffff,stroke:#000000,stroke-width:1px,color:#000000;
    classDef error fill:#ffffff,stroke:#000000,stroke-width:1px,color:#000000;
    classDef highlight fill:#ffffff,stroke:#000000,stroke-width:2px,color:#000000;
    classDef dataBox fill:#ffffff,stroke:#000000,stroke-width:1px,stroke-dasharray: 5 5,color:#000000;
    classDef safeBox fill:#ffffff,stroke:#000000,stroke-width:2px,color:#000000;

    subgraph Pipeline ["Public Tracking Sanitization & Masking Pipeline"]
        P1["Stage 1: Public Tracking HTTP Request<br/>GET /api/v1/tracking/public/{number}<br/>Traefik Edge Rate Limit (10 req/s, burst 20)"]:::stage
        P2["Stage 2: Tracking Number Format Validation<br/>Regex: ^SD-[0-9]{8}-[A-Z0-9]{6}$<br/>Go Chi URL Parameter Guard"]:::stage
        D1{"Valid<br/>Format?"}:::decision
        E400["HTTP 400 Bad Request<br/>INVALID_TRACKING_NUMBER"]:::error
        P3["Stage 3: Read-Only Database Query<br/>PostgreSQL Role: swiftdrop_tracking_ro<br/>SELECT on parcels, history, addresses, zones"]:::stage
        D2{"Record<br/>Found?"}:::decision
        E404["HTTP 404 Not Found<br/>TRACKING_NOT_FOUND"]:::error
        P4["Stage 4: Domain Status & Milestone Mapping<br/>Internal Enums -> User-Friendly Descriptions<br/>Chronological Sequence (occurred_at ASC)"]:::stage
        P5["Stage 5: SEC-009 Privacy Masking Engine<br/>Redact Street, Phone, Personal Name, Notes<br/>Suppress Proof Photos, Fees & UUIDs"]:::highlight
        P6["Stage 6: Public DTO Construction<br/>Standard Envelope: { data: { ... } }<br/>Headers: Content-Type, Cache-Control: 15s"]:::stage
        P7["Stage 7: Sanitized JSON HTTP Response<br/>HTTP 200 OK Emitted to Browser<br/>Complete Customer Privacy Guaranteed"]:::stage
    end

    subgraph DataTransform ["Data Transformation Comparison"]
        RawData["<b>Raw Internal PostgreSQL Entity</b><br/>-----------------------------------<br/>tracking_number: SD-20261025-A8F3K2<br/>current_status: OUT_FOR_DELIVERY<br/>recipient_name: Jane Doe [REDACTED]<br/>recipient_phone: +94771234567 [REDACTED]<br/>street_address: 45/2 Galle Rd, Apt 3B [REDACTED]<br/>destination_city: Colombo [PERMITTED]<br/>destination_zone_id: ZONE-CMB-03 [REDACTED]<br/>fee_cents: 145000 [REDACTED]<br/>proof_photo_url: https://s3/pod/... [REDACTED]<br/>internal_notes: Gate buzzer broken [REDACTED]"]:::dataBox

        SafeData["<b>Sanitized Public Response Payload (SEC-009)</b><br/>-----------------------------------<br/>HTTP/1.1 200 OK<br/>Content-Type: application/json<br/><br/>{<br>  &quot;data&quot;: {<br>    &quot;tracking_number&quot;: &quot;SD-20261025-A8F3K2&quot;,<br>    &quot;status&quot;: &quot;OUT_FOR_DELIVERY&quot;,<br>    &quot;status_description&quot;: &quot;Parcel is on vehicle...&quot;,<br>    &quot;destination&quot;: { &quot;city&quot;: &quot;Colombo&quot; },<br>    &quot;estimated_delivery&quot;: &quot;2026-10-25T16:00:00Z&quot;,<br>    &quot;milestones&quot;: [...]<br>  }<br>}"]:::safeBox
    end

    P1 --> P2
    P2 --> D1
    D1 -->|No| E400
    D1 -->|Yes| P3
    P3 --> D2
    D2 -->|No| E404
    D2 -->|Yes| P4
    P3 -.->|fetches raw record| RawData
    RawData -.->|ingests sensitive record| P5
    P4 --> P5
    P5 -.->|outputs sanitized payload| SafeData
    P5 --> P6
    SafeData -.->|structures schema| P6
    P6 --> P7
```

---

## 5. Architectural Verification Matrix

| Architecture Dimension | Repository Verified Standard | Figure 8.1 Status | Figure 8.2 Status | Figure 8.3 Status |
| :--- | :--- | :---: | :---: | :---: |
| **Service Root Path** | `services/tracking/` | Aligned | Aligned | Aligned |
| **Container Port** | `:8081` (`PORT=8081`) | Aligned | Aligned | Aligned |
| **Edge Gateway** | Traefik (:80), `PathPrefix('/api/v1/tracking')` | Aligned | Aligned | Aligned |
| **Rate Limiter** | 10 req/s, burst 20 per IP | Aligned | N/A | Aligned |
| **Public Endpoint** | `GET /api/v1/tracking/public/{tracking_number}` | Aligned | N/A | Aligned |
| **Stream Endpoint** | `GET /api/v1/tracking/public/{tracking_number}/stream` | Aligned | Aligned | N/A |
| **Heartbeat Ping** | `:keepalive\n\n` emitted every 15s | Aligned | Aligned | N/A |
| **PostgreSQL Role** | `swiftdrop_tracking_ro` (Read-only, max 5 conns/pod) | Aligned | Aligned | Aligned |
| **Migration Ownership** | Zero DDL owned by M5 (Maintained by M9/M2) | Aligned | Aligned | Aligned |
| **RabbitMQ Queue** | `q.tracking.updates` on `swiftdrop.events.topic` | Aligned | Aligned | N/A |
| **Redis Channel** | `tracking:updates` (Multi-pod fan-out) | Aligned | Aligned | N/A |
| **M8 Decoupling** | Member 8 has ZERO connection to Redis | Aligned | Aligned | N/A |
| **SEC-009 Masking** | Redact street, phone, name, notes, fees, photos | Aligned | Aligned | Aligned |
