# Member 10 — Audit Findings Remediation & Webhook Test Artifact Specification

> **Module:** AI Assistant Service & Cross-Workflow Quality Assurance / E2E Testing  
> **Assigned Member:** Member 10  
> **Source Audit:** `docs/drafts/DOCS_AUDIT_MEMBER_PROBLEMS_AND_REQUIRED_CHANGES.md` (§2, Member 10)  
> **Status:** Approved Remediation Blueprint (Pre-Implementation Specification)  
> **Date:** October 26, 2026  
> **Target Branch:** `docs/module-spec-reviews`

---

## 1. Executive Summary & Purpose

A cross-member repository audit revealed 8 contract, networking, and naming discrepancies affecting Member 10's draft specifications (`docs/drafts/member-10-ai-assistant-quality/`). Most notably, the original end-to-end webhook test design relied on `http://localhost:9999/webhook`, which fails in containerized environments, violates Member 8's Server-Side Request Forgery (SSRF) guardrails, and lacks multi-tenant isolation verification.

This document serves as the authoritative blueprint for resolving all 8 audit findings. It establishes:
1. The formal specification for a standalone **Multi-Tenant Mock Webhook Receiver & Visual HTML Dashboard** (`mock-webhook:9999`), enabling live visual demonstration of event delivery, tenant isolation, and dead-letter queue (DLQ) behavior.
2. Concrete schema, route, and event-contract harmonizations aligning Member 10 with Member 4 (Merchants), Member 8 (Messaging/Events), Member 9 (DevOps/CI), and Member 1 (Identity).
3. A file-by-file update plan before editing Member 10's core draft documents (`doc-01` through `doc-08`).

---

## 2. Master Remediation Matrix

| # | Sev | Audit Issue | Affected Member 10 Documents | Root Cause & Resolution Summary |
| :- | :-: | :--- | :--- | :--- |
| **1** | **C** | **Webhook E2E Mock Networking & SSRF** | `doc-05` (§B.3), `doc-06` (Suite 2), `team-handoff-guide` | `localhost:9999` is unroutable from containers and blocked by SSRF filters. Deploy containerized `mock-webhook:9999` with multi-tenant endpoints and visual HTML dashboard; establish dev SSRF bypass flags with M8. |
| **2** | **C** | **Merchant Route Pluralization Mismatch** | `doc-02`, `doc-03`, `doc-04`, `doc-06` | Calls `POST /api/v1/merchants/orders` (plural). Member 4's authoritative contract and SRS mandate singular `POST /api/v1/merchant/orders`. |
| **3** | **H** | **Event Contract & DLQ Alignment** | `doc-02`, `doc-03`, `doc-04`, `doc-06` | Asserts `parcel.created` instead of canonical `parcel.booked.v1`. Uses incorrect queue names (`swiftdrop.webhooks.orders` vs `q.webhooks.dispatcher`) and DLQ (`swiftdrop.webhooks.dlq` vs `swiftdrop.dlq`). Production retry is ~42m; specify compressed retry schedule for CI. |
| **4** | **H** | **CI Service Topology & Commands** | `doc-06`, `doc-08` | Omits core infrastructure components (migrations, relay, worker, Mailpit, gateway). Uses manual `docker compose up` and raw SQL seed instead of standardized `make up` (A7) and `make seed` (K13). |
| **5** | **H** | **Orphan Policy Endpoints & Database Tables** | `doc-01`, `doc-02`, `doc-03`, `doc-04`, `doc-05` | Defines `/system/policies` and `system_policies` table owned by no member. Eliminate orphan endpoints; ground company policy strictly via local embedded `company_policies.md` markdown hydrator (zero DB access). |
| **6** | **M** | **Tool Names & Booking Prefill Route** | `doc-02`, `doc-04`, `doc-06` | Prefill link references `/customer/book` instead of frontend standard `/portal/book`. Tool names in helper summaries drift slightly from exact SRS FR-AI-007 specifications. |
| **7** | **M** | **Environment Variables & Rate Limits** | `doc-04`, `doc-07`, `doc-08` | Inconsistent service token name (`INTERNAL_SERVICE_KEY` vs `AI_SERVICE_ROLE_TOKEN`); missing LLM config in env docs (`LLM_BASE_URL`, `LLM_API_KEY`); rate limits say 10/30 RPM vs standard 20 RPM matching Traefik. |
| **8** | **L** | **Checklist Completion & Gateway Routing** | `README.md`, `team-handoff-guide` | Open checkboxes (`[ ]`) left in handoff guides. Confirm Traefik `PathPrefix('/api/v1/ai')` cleanly routes standard REST and MCP SSE traffic (`/api/v1/ai/mcp/sse`). |
| **9** | **H** | **MCP Client Authentication & Token Lifecycle** | `doc-04`, `doc-07`, `team-handoff-guide` | Short-lived 15-minute JWTs break developer experience in external MCP clients (Claude Desktop/Cursor). Avoid adding PAT workload to Member 1 by leveraging Member 4's `parcels:read` scoped API keys and React silent refresh. |
| **10** | **M** | **MCP SSE Transport Dual-Channel Mechanics** | `doc-03`, `doc-04`, `doc-08` | Clarify how bidirectional request/response works without WebSockets by formalizing the MCP Dual-Channel pattern (downstream SSE stream paired with upstream HTTP POST). |

---

## 3. Deep-Dive Specification: Multi-Tenant Mock Webhook Receiver & Visual HTML Dashboard

### 3.1 Architecture & Role in E2E Verification
During the 15-mark Live Demonstration and automated CI workflows (SRS AT-008, AT-011, Scenario 2), asynchronous event dispatching through RabbitMQ and background workers is invisible to viewers. 

The `mock-webhook` service is a lightweight, dedicated test artifact running as a Docker Compose service. It receives HTTP webhooks dispatched by Member 8's Event Worker, verifies HMAC-SHA256 signatures, logs incoming payloads in memory, and serves an interactive, real-time visual HTML dashboard at `http://localhost:9999/dashboard`.

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                        SWIFTDROP E2E TEST WORKFLOW TOPOLOGY                            │
│                                                                                        │
│   [Core API] ──(Tx Commit)──> [Outbox] ──(Relay)──> [RabbitMQ] ──> [Event Worker]      │
│                                                                           │            │
│   ┌───────────────────────────────────────────────────────────────────────┴────────┐   │
│   │ mock-webhook container (:9999) — Multi-Tenant Test Artifact                   │   │
│   │                                                                                │   │
│   │  POST /webhooks/merchant-alpha  (Apex Retail: 200 OK + HMAC Check)             │   │
│   │  POST /webhooks/merchant-beta   (Zenith Logistics: 200 OK + HMAC Check)        │   │
│   │  POST /webhooks/merchant-chaos  (Chaos Merchant: 500 Error -> DLQ Test)        │   │
│   │  POST /webhooks/merchant-slow   (Latency: 10s Sleep -> Worker Timeout Test)    │   │
│   │                                                                                │   │
│   │  GET /dashboard ──> Real-Time Visual HTML Dashboard                            │   │
│   │                     - Multi-Tenant Tabbed Inboxes                              │   │
│   │                     - Cross-Tenant Isolation Indicator (0 Cross-Talk)          │   │
│   │                     - HMAC Verification Badges (VALID / INVALID)               │   │
│   │                     - Raw CloudEvent Payload & Header Accordion                │   │
│   │                     - DLQ Retry Ladder & Backoff Visualizer                    │   │
│   └──────────────────────────────────────┬─────────────────────────────────────────┘   │
│                                          │                                             │
│       ┌──────────────────────────────────┴────────────────────────────────┐            │
│       ▼                                                                   ▼            │
│  [CI / Playwright E2E Runner]                                   [Live Presentation]     │
│  - Asserts JSON via GET /api/merchants/:id/events               - Open in browser tab  │
│  - Takes full-page screenshot as test evidence                  - 15-mark live demo    │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Proving Multi-Tenant Isolation (SEC-004 / SEC-005)
A primary evaluation criterion is multi-tenant security. The mock webhook receiver explicitly provisions distinct endpoints and signing secrets for separate test merchants:

1. **Merchant Alpha (Apex Retail):**
   - **Tenant ID:** `mer_alpha_11111111-1111-1111-1111-111111111111`
   - **Webhook URL:** `http://mock-webhook:9999/webhooks/merchant-alpha`
   - **Shared Signing Secret:** `whsec_alpha_test_secret_32_bytes_len!`
2. **Merchant Beta (Zenith Logistics):**
   - **Tenant ID:** `mer_beta_22222222-2222-2222-2222-222222222222`
   - **Webhook URL:** `http://mock-webhook:9999/webhooks/merchant-beta`
   - **Shared Signing Secret:** `whsec_beta_test_secret_32_bytes_leng!`
3. **Chaos Merchant (Failure & DLQ Simulation):**
   - **Tenant ID:** `mer_chaos_33333333-3333-3333-3333-333333333333`
   - **Webhook URL:** `http://mock-webhook:9999/webhooks/merchant-chaos`
   - **Behavior:** Always returns HTTP 500 Internal Server Error to trigger Member 8's retry policy and push into `swiftdrop.dlq`.

#### Visual Isolation Proof in Dashboard
When Merchant Alpha books an order:
- The event appears **only** under Merchant Alpha's dashboard feed.
- Merchant Beta's event counter remains strictly at `0`.
- The dashboard highlights a green badge: `[TENANT ISOLATION VERIFIED: ZERO DATA LEAKAGE]`.

### 3.3 Endpoint Contract for `mock-webhook`

| Method | Path | Target Caller | Purpose & Behavior |
| :--- | :--- | :--- | :--- |
| `POST` | `/webhooks/merchant-alpha` | Event Worker | Receives CloudEvents for Merchant Alpha. Validates HMAC with Alpha secret. Stores in memory. Responds `200 OK`. |
| `POST` | `/webhooks/merchant-beta` | Event Worker | Receives CloudEvents for Merchant Beta. Validates HMAC with Beta secret. Stores in memory. Responds `200 OK`. |
| `POST` | `/webhooks/merchant-chaos` | Event Worker | Simulated failing endpoint. Records attempt number from `X-SwiftDrop-Delivery-Attempt`. Responds `500 Internal Server Error`. |
| `POST` | `/webhooks/merchant-slow` | Event Worker | Simulated hanging endpoint. Sleeps for 10 seconds before responding. Tests worker HTTP timeout (5s). |
| `GET` | `/dashboard` | Browser / Evaluator | Serves the rich, responsive HTML/Tailwind visual dashboard. Auto-refreshes every 2 seconds via Server-Sent Events (`/api/stream`) or polling. |
| `GET` | `/api/events` | Playwright Runner | Returns JSON array of all captured events across all tenants. |
| `GET` | `/api/merchants/{id}/events`| Playwright Runner | Returns JSON array of events scoped strictly to a single tenant for automated isolation assertions. |
| `POST` | `/api/reset` | Test Setup Script | Flushes all captured events and counters between test runs. |

### 3.4 Container Networking & SSRF Guardrail Resolution
To resolve the container networking failure and SSRF blocking:
1. **Docker Compose Service Definition:**
   ```yaml
   mock-webhook:
     image: python:3.11-slim
     container_name: swiftdrop-mock-webhook
     command: python -m http.server 9999 # (or lightweight FastAPI / aiohttp test script)
     volumes:
       - ./tests/e2e/mock-webhook-receiver:/app
     working_dir: /app
     ports:
       - "9999:9999"
     networks:
       - swiftdrop-network
     environment:
       - PORT=9999
       - ALPHA_SECRET=whsec_alpha_test_secret_32_bytes_len!
       - BETA_SECRET=whsec_beta_test_secret_32_bytes_leng!
   ```
2. **SSRF Test-Mode Flags (Agreed with Member 8):**
   In dev/test environments (`docker-compose.yml` / CI), Member 8's Event Worker accepts:
   - `WEBHOOK_ALLOW_INSECURE_HTTP=true` (allows `http://` for local test hosts)
   - `WEBHOOK_ALLOWED_HOSTS=mock-webhook,localhost,127.0.0.1` (whitelists test container hostname)
   - In production, these variables default to `false` and empty, enforcing strict HTTPS on port 443 only.

### 3.5 Automated Playwright Assertion Snippet
In `tests/e2e/specs/scenario-02-merchant-dispatch.spec.ts`:
```typescript
test('Verify asynchronous webhook dispatch and strict multi-tenant isolation', async ({ request, page }) => {
  // 1. Reset mock webhook state
  await request.post('http://localhost:9999/api/reset');

  // 2. Merchant Alpha books an order
  const bookingRes = await request.post('http://localhost:8000/api/v1/merchant/orders', {
    headers: { 'X-API-Key': 'sk_live_alpha_test_key_123' },
    data: { recipient_name: 'Alice', destination_zone: 'ZONE_COLOMBO', fee_cents: 8000 }
  });
  expect(bookingRes.status()).toBe(201);
  const { tracking_number } = (await bookingRes.json()).data;

  // 3. Poll mock receiver for Merchant Alpha delivery
  await expect.poll(async () => {
    const res = await request.get('http://localhost:9999/api/merchants/mer_alpha/events');
    const data = await res.json();
    return data.find((e: any) => e.data.tracking_number === tracking_number)?.event_type;
  }, { timeout: 10000, intervals: [500] }).toBe('parcel.booked.v1');

  // 4. Assert Strict Isolation: Merchant Beta received ZERO events
  const betaEvents = await (await request.get('http://localhost:9999/api/merchants/mer_beta/events')).json();
  expect(betaEvents.length).toBe(0);

  // 5. Visual Proof: Navigate to dashboard and capture artifact screenshot
  await page.goto('http://localhost:9999/dashboard');
  await page.screenshot({ path: 'test-results/artifacts/webhook-isolation-proof.png', fullPage: true });
});
```

---

## 4. Technical Specifications for Audit Items 2–8

### 4.1 Issue 2: Merchant Order Ingress Route Pluralization
- **Problem:** Member 10 draft docs called `POST /api/v1/merchants/orders`.
- **Authoritative Contract:** Member 4 (Merchant Integration) and SRS FR-MER-001 define singular `POST /api/v1/merchant/orders`.
- **Remediation:** Update all occurrences in `doc-02`, `doc-03`, `doc-04`, `doc-06`, and `team-handoff-guide` to singular `/api/v1/merchant/orders`.

### 4.2 Issue 3: Event Contract, Dead-Letter Queue & Retry Harmonization
- **Problem:**
  - Used non-standard event name `parcel.created`.
  - Used inaccurate queue names (`swiftdrop.webhooks.orders`, `swiftdrop.webhooks.dlq`).
  - Expected production retry backoff (~42 minutes across 5 attempts: 30s, 2m, 8m, 32m), which stalls CI test suites.
- **Remediation:**
  1. **Canonical Event Name:** Assert `parcel.booked.v1` (CloudEvent standard agreed with Member 8).
  2. **Authoritative Queue & Exchange:**
     - Webhook Dispatch Queue: `q.webhooks.dispatcher`
     - Dead-Letter Exchange: `swiftdrop.dlq.exchange`
     - Dead-Letter Queue: `swiftdrop.dlq`
  3. **CI Compressed Retry Schedule:**
     - Document that in the CI/test environment, Member 8's Event Worker is configured with:
       `WEBHOOK_RETRY_INTERVALS_MS=500,1000,2000` (3 attempts over 3.5 seconds before routing to DLQ).
     - Allows E2E test Suite 2 to verify full dead-lettering in under 5 seconds.

### 4.3 Issue 4: CI Service Topology & Standardized Make Commands
- **Problem:** `doc-06` and `doc-08` listed manual `docker compose up` commands and omitted essential infrastructure services (migrations, relay, worker, Mailpit, Traefik).
- **Authoritative Standard:** Project Agreement A7 mandates `make up` and Agreement K13 mandates `make seed`.
- **Remediation:**
  Update CI/local orchestration instructions to reflect the full 10-service topology:
  ```bash
  # 1. Start full topology via standard Makefile targets
  make up

  # Topology active:
  # - postgres (authoritative relational store)
  # - rabbitmq (event broker)
  # - core-api (Go modular monolith)
  # - outbox-relay (publisher daemon)
  # - event-worker (async webhook & notification dispatcher)
  # - tracking-api (read-optimized SSE service)
  # - ai-assistant (FastAPI assistant & FastMCP server)
  # - traefik (API Gateway / Ingress)
  # - mailpit (local SMTP mock receiver)
  # - mock-webhook (multi-tenant webhook test artifact)

  # 2. Apply migrations and seed data
  make seed

  # 3. Execute automated test suites
  make test-e2e
  ```

### 4.4 Issue 5: Removal of Orphan Policy Endpoints & Dynamic Override Priority

#### The Problem:
`doc-01`, `doc-02`, `doc-03`, `doc-04`, and `doc-05` referenced hypothetical `system_policies` tables and endpoints `GET /api/v1/system/policies` and `PUT /api/v1/admin/policies`. No other team member owns or implemented these endpoints or tables in the modular monolith.

#### The Resolution (Hybrid Grounding with Authoritative Live Overrides):
1. **Eliminate Orphan Artifacts:**
   - Delete all references to the non-existent `system_policies` database table.
   - Delete endpoints `GET /api/v1/system/policies`, `GET /api/v1/admin/policies`, `PUT /api/v1/admin/policies`, and `POST /api/v1/admin/policies/upload`.

2. **Base Policy Knowledge (`company_policies.md`):**
   - Authoritative narrative policies (prohibited items descriptions, packaging guides, insurance rules, claims procedures) are version-controlled in `services/ai-assistant/knowledge/company_policies.md`.
   - The AI Assistant's `PolicyContextHydrator` loads this static markdown at startup/runtime as the baseline grounding context.

3. **Dynamic API Configuration & Precedence Rule (Live Overrides):**
   - Static markdown files can become outdated if operational rules change during runtime. Therefore, the `PolicyContextHydrator` also queries Member 7's approved Core API endpoint:
     `GET /api/v1/admin/system-configurations` (cached in-memory with a 5-minute TTL).
   - **Strict Conflict Resolution Invariant:**
     > **Rule:** *When any numerical threshold, rate, status, or limit in `GET /api/v1/admin/system-configurations` contradicts the text in `company_policies.md`, the live API data takes absolute precedence.*

#### Concrete Example of Conflict Resolution:
- **Static File (`company_policies.md`):**
  ```markdown
  ### 2. Weight and Sizing Restrictions
  - Maximum allowable weight for domestic courier parcels is 30.0 kg.
  - Cash on Delivery (COD) limit: LKR 50,000.
  ```
- **Live Core API Response (`GET /api/v1/admin/system-configurations`):**
  ```json
  {
    "data": {
      "max_parcel_weight_kg": 25.0,
      "max_cod_amount_cents": 3500000,
      "fuel_surcharge_percentage": 4.5,
      "updated_at": "2026-10-26T10:00:00Z"
    }
  }
  ```
- **Injected Prompt Structure:**
  ```text
  [AUTHORITATIVE ACTIVE OVERRIDES - HIGHEST PRECEDENCE]
  - Live Max Parcel Weight: 25.0 kg (Overrides static policy)
  - Live COD Limit: LKR 35,000 (Overrides static policy)

  [BASE COMPANY POLICIES - GENERAL GUIDELINES]
  (Content of company_policies.md)
  ```
- **Customer Query:** *"Can I send a 28 kg package via cash on delivery?"*
- **Assistant Response:** *"Our standard policy documentation allows up to 30 kg, but under current operational system configurations, the maximum allowable parcel weight is 25 kg. Therefore, a 28 kg package cannot be accepted at this time."*

### 4.5 Issue 6: Portal Route Standard & Assistant Tool Names Alignment

#### The Problem:
1. **Broken Deep Link Navigation:** When the AI assistant calculates a shipping quote, it generates a markdown deep-link allowing the customer to click and jump straight into the booking form with their parameters pre-filled. The draft documentation previously specified `/customer/book?weight=2.5...`. However, Member 6's React frontend router defines the portal booking route as `/portal/book`. If a user clicked the AI-generated link, it returned a **404 Page Not Found**.
2. **Tool Identifier Drift:** Several draft tables and diagrams used casual tool names like `get_tariff_quote` or `check_shipping_cost` instead of the formal function names mandated in SRS **FR-AI-007**. When evaluators or automated grading scripts run tests against the FastMCP tool registry, mismatched names cause automated assertion failures.

#### The Remediation:
1. **Standardized Deep-Link Route:**
   - Standardize all AI booking action links to `/portal/book` with query parameters.
   - Example generated markdown action link:
     `[Complete Booking in Portal](http://localhost:5173/portal/book?weight=2.5&zone=ZONE_CP_COLOMBO&length=20&width=15&height=10)`
   - Note `/customer/book` as a supported client-side redirect alias in Member 6's router for backward compatibility.

2. **Strict Tool Naming Alignment with SRS FR-AI-007:**
   The FastMCP tool registry strictly registers the exact 4 tools specified in the SRS:
   - **`estimate_shipping_fee`**: Calculates estimated shipping cost by calling Core API `POST /api/v1/pricing/quotes`.
   - **`track_parcel_status`**: Queries Core API `GET /api/v1/tracking/public/{tracking_number}` for sanitized milestone delivery statuses.
   - **`get_my_active_parcels`**: Calls Core API `GET /api/v1/parcels` using the customer's authenticated JWT to list their active, non-delivered shipments.
   - **`explain_company_policies`**: Queries the internal grounded policy knowledge base (`company_policies.md` + live system config overrides).

#### Concrete Example of User Interaction & Tool Execution:
- **Customer:** *"How much will it cost to send a 3 kg package (30x20x10 cm) to Kandy?"*
- **Assistant Invokes Tool:**
  ```json
  {
    "tool": "estimate_shipping_fee",
    "parameters": {
      "weight_kg": 3.0,
      "destination_zone_id": "ZONE_CP_KANDY",
      "dimensions": { "length_cm": 30, "width_cm": 20, "height_cm": 10 },
      "service_type": "STANDARD"
    }
  }
  ```
- **Tool Output from Core API:**
  ```json
  { "fee_cents": 65000, "currency": "LKR", "estimated_delivery_days": 2 }
  ```
- **Assistant Final Response with Prefill Deep Link:**
  > The estimated shipping fee to Kandy for your 3 kg package is **LKR 650.00** (Standard Delivery: approx. 2 business days).  
  >  
  > Ready to send it? Click below to prefill your booking details:  
  > 🔗 **[Proceed to Book Parcel](http://localhost:5173/portal/book?weight=3.0&zone=ZONE_CP_KANDY&length=30&width=20&height=10)**

### 4.6 Issue 7: Environment Variables & Ingress Rate Limiting Harmonization
- **Problem:**
  - Secret token used conflicting names (`INTERNAL_SERVICE_KEY` vs `AI_SERVICE_ROLE_TOKEN`).
  - LLM configuration parameters were missing from environment specifications.
  - Rate limits listed 10 RPM anonymous / 30 RPM authenticated, conflicting with Traefik's configured 20 RPM limit.
- **Remediation:**
  1. Standardize internal service communication token to:
     `AI_SERVICE_ROLE_TOKEN` (with fallback to `INTERNAL_SERVICE_KEY` for legacy compatibility).
  2. Document standard LLM configuration:
     - `LLM_API_KEY`: Secret API key for OpenAI / Gemini provider.
     - `LLM_BASE_URL`: Base API endpoint (default: `https://api.openai.com/v1`).
     - `LLM_MODEL`: Provider model identifier (default: `gpt-4o-mini`).
     - `LLM_TEMPERATURE`: Fixed at `0.2` for deterministic tool selection.
  3. Standardize API rate limiting to **20 requests per minute** (matching Traefik edge rate limiter configuration).

### 4.7 Issue 8: Gateway Prefix Verification & Review Checklist Resolution
- **Problem:** Unchecked checklist boxes in handoff documents; need verification that Traefik gateway routes MCP SSE traffic.
- **Remediation:**
  1. Confirm Traefik configuration: `PathPrefix('/api/v1/ai')` correctly forwards all subpaths, including REST endpoints (`/api/v1/ai/assistant/chat`) and MCP SSE endpoints (`/api/v1/ai/mcp/sse`).
  2. Mark all handoff and interface review checkboxes as completed (`[x]`) across `README.md` and `team-integration-and-handoff-guide.md`.

### 4.8 Issue 9 (Architectural Clarification): MCP Client Authentication, Token Lifecycle & Least Privilege

#### The Problem:
1. **Short-Lived JWT Friction in Desktop Clients:** SwiftDrop's user authentication issues 15-minute asymmetric RS256 JWT access tokens. If an external developer or power user connects an MCP client (such as Claude Desktop or Cursor) using an access token, it expires within 15 minutes, forcing them to manually re-login, copy a new token, and restart their desktop client.
2. **Workload Assessment for Member 1:** Forcing Member 1 (Identity) to implement a native Personal Access Token (PAT) subsystem would require new database migrations (`user_api_keys`), cryptographic hashing, and CRUD endpoints, substantially overloading Member 1 and violating `AGENTS.md` scope boundaries.
3. **Security & Privilege Escalation Risk:** If an external client uses a general merchant API key that possesses write access (`parcels:create`), a leak of the desktop configuration file could allow an attacker to bypass MCP and execute unauthorized courier bookings or incur fraudulent charges directly on Core API.

#### The Remediation (Multi-Tiered Authentication & Principle of Least Privilege):
To solve the token lifecycle without adding work to Member 1 or compromising security:

1. **Web Portal AI Chat (95% of Users):**
   - The embedded chat widget inside the React web portal relies on standard short-lived JWTs.
   - The frontend automatically executes silent background token refresh against Member 1's `POST /api/v1/auth/refresh` using the HTTP-only refresh cookie. The user experiences seamless, uninterrupted conversation.

2. **Public Tools Require Zero Authentication:**
   - Three of the four FastMCP tools (`estimate_shipping_fee`, `track_parcel_status`, `explain_company_policies`) are explicitly **public and anonymous**.
   - Any external MCP client can query shipping quotes, check tracking numbers, or review policies without configuring any authorization credentials.

3. **External MCP Clients (Claude Desktop / Cursor): Reusing Member 4 Scoped API Keys:**
   - When an external merchant or developer connects Claude Desktop to inspect their shipments via `get_my_active_parcels`, FastMCP accepts an API key header (`X-API-Key: sd_live_...`).
   - The AI Assistant acts purely as a **stateless pass-through client** (preserving the Zero-Database Invariant) and forwards the key to Core API (`GET /api/v1/merchant/orders`).
   - **Enforcement of Least Privilege (PoLP):**
     - Documentation and portal setup strictly mandate that API keys generated for MCP integrations must be provisioned with **`parcels:read` scope ONLY** (leveraging Member 4's existing `BR-K-07` and `BR-O-01` rules).
     - If that key is ever leaked or used to attempt order creation (`POST /api/v1/merchant/orders`), Core API rejects the request immediately with `403 Forbidden (MERCHANT_SCOPE_DENIED)`.
     - An MCP integration can never trigger unauthorized courier bookings.

4. **Live Demo & Evaluation Convenience:**
   - For evaluator grading sessions, `seeds/dev_seed.sql` includes a pre-signed, long-lived 30-day development token (`TEST_USER_TOKEN`). Evaluators can paste this token into their Claude Desktop config once without experiencing mid-demo token expiration.

---

### 4.9 Issue 10 (Architectural Clarification): MCP SSE Transport Dual-Channel Mechanics

#### The Architectural Dilemma:
Server-Sent Events (SSE) is by definition a **unidirectional protocol** (Server $\to$ Client). How does an external AI agent or client send interactive tool requests and receive responses without using WebSockets?

#### The MCP Dual-Channel Solution:
The official Model Context Protocol specification solves this over standard HTTP by combining **a persistent downstream SSE stream with standard upstream HTTP POST requests**:

```
┌─────────────────────────┐                                ┌─────────────────────────┐
│       MCP Client        │                                │     FastMCP Server      │
│(Claude Desktop / Cursor)│                                │    (Python Service)     │
└────────────┬────────────┘                                └────────────┬────────────┘
             │                                                          │
             │ [Channel 1: Downstream SSE Stream]                       │
             │ GET /api/v1/ai/mcp/sse                                   │
             ├─────────────────────────────────────────────────────────>│
             │                                                          │
             │ 1. Server establishes SSE stream & emits session URL:    │
             │    event: endpoint                                       │
             │    data: /api/v1/ai/mcp/messages?session_id=7d4e9f...    │
             │<═════════════════════════════════════════════════════════╡ (Persistent
             │                                                          │  SSE Stream)
             │ [Channel 2: Upstream HTTP POST Requests]                 │
             │ POST /api/v1/ai/mcp/messages?session_id=7d4e9f...        │
             │ Body: {"jsonrpc": "2.0", "method": "tools/call", ...}    │
             ├─────────────────────────────────────────────────────────>│
             │                                                          │
             │ 2. FastMCP executes tool (queries Core API) and pushes   │
             │    result back down Channel 1:                           │
             │    event: message                                        │
             │    data: {"result": {"fee_cents": 65000}}                │
             │<═════════════════════════════════════════════════════════╡
```

#### Step-by-Step Execution:
1. **Downstream Channel Initialization:**
   The client makes an HTTP GET request to `GET /api/v1/ai/mcp/sse` with `Accept: text/event-stream`. The server keeps this connection open and pushes down an `endpoint` event containing a unique session identifier:
   `data: /api/v1/ai/mcp/messages?session_id=7d4e9f1a-b3c2...`
2. **Upstream Request Channel:**
   When the client needs to query capabilities (`tools/list`) or execute a tool (`tools/call`), it performs a standard HTTP `POST` to that session endpoint with a JSON-RPC 2.0 payload.
3. **Execution & Stream Delivery:**
   FastMCP routes the tool request to the appropriate internal handler (e.g. calling Core API REST endpoints), packages the result into a JSON-RPC response, and transmits it down the existing persistent SSE connection.

#### Why This Is Superior to WebSockets for SwiftDrop:
- **Zero Protocol Upgrades:** Traefik and upstream corporate firewalls handle standard HTTP GET and POST natively without socket upgrade timeouts (`Upgrade: websocket`).
- **Native Header Support:** Standard HTTP headers (`Authorization: Bearer ...`, `X-API-Key: ...`) work natively on initial handshake and subsequent POST requests (unlike browser WebSockets).
- **Stateless Edge Routing:** Traefik can route `/api/v1/ai/*` directly without sticky-session or persistent TCP pinhole requirements.

---

## 5. File-by-File Update Execution Plan

The following table maps the planned changes to each target file in `docs/drafts/member-10-ai-assistant-quality/`:

| Target File | Planned Updates |
| :--- | :--- |
| `doc-01-scope-and-responsibilities.md` | - Remove orphan `system_policies` table references.<br>- Clarify pure markdown policy hydration from `company_policies.md`.<br>- Confirm zero-database access invariant. |
| `doc-02-functional-flows-and-business-rules.md` | - Update sequence diagrams: replace `POST /api/v1/merchants/orders` with `POST /api/v1/merchant/orders`.<br>- Update event names: replace `parcel.created` with `parcel.booked.v1`.<br>- Update queue names: `q.webhooks.dispatcher` and `swiftdrop.dlq`.<br>- Add multi-tenant webhook sequence with Merchant Alpha, Beta, and Chaos. |
| `doc-03-internal-architecture-and-design.md` | - Remove `/api/v1/system/policies` hydrator arrow; ground hydrator directly from `company_policies.md` with dynamic live overrides from `GET /api/v1/admin/system-configurations`.<br>- Add `mock-webhook:9999` multi-tenant architecture and HTML dashboard component diagram.<br>- Document MCP SSE Dual-Channel architecture (Channel 1 downstream SSE, Channel 2 upstream HTTP POST). |
| `doc-04-interface-and-integration-contract.md` | - Delete Endpoint 5 (`/api/v1/system/policies` and `/api/v1/admin/policies*`).<br>- Document `mock-webhook` receiver endpoints (`/webhooks/merchant-alpha`, `/webhooks/merchant-beta`, `/webhooks/merchant-chaos`, `GET /dashboard`, `GET /api/events`).<br>- Standardize environment variables (`AI_SERVICE_ROLE_TOKEN`, `LLM_*`).<br>- Document Dual-Mode Authentication (JWT for web portal, `parcels:read` scoped API keys for external MCP clients). |
| `doc-05-data-and-persistence.md` | - Expand §B.3 to formally document the `mock-webhook` service specification, in-memory event schema, multi-tenant signing secrets, and HTML dashboard layout.<br>- Remove `system_policies` from Core Monolith table lists. |
| `doc-06-testing-strategy-and-acceptance.md` | - Update Suite 2 (Scenario 2) to incorporate the multi-tenant webhook isolation test case.<br>- Document compressed retry timing (`WEBHOOK_RETRY_INTERVALS_MS=500,1000,2000`).<br>- Update CI commands to `make up` and `make seed`, listing the full 10-service topology. |
| `doc-07-security-failure-and-recovery.md` | - Document SSRF resolution flags (`WEBHOOK_ALLOW_INSECURE_HTTP`, `WEBHOOK_ALLOWED_HOSTS`).<br>- Align rate limiting to 20 RPM.<br>- Detail multi-tenant HMAC-SHA256 signature verification mechanics.<br>- Formally specify Principle of Least Privilege (PoLP) for MCP API keys (enforcing `parcels:read` scope only to prevent unauthorized courier dispatches). |
| `doc-08-implementation-deployment-and-ops.md` | - Add `mock-webhook` service definition to Docker Compose configuration.<br>- Confirm Traefik `PathPrefix('/api/v1/ai')` routing for REST and MCP SSE.<br>- Update operational runbooks to use `make up` and `make seed`.<br>- Detail Traefik configuration for streaming SSE connections without timeout drops. |
| `team-integration-and-handoff-guide.md` | - Update Member 4 interface contract (`POST /api/v1/merchant/orders` and `parcels:read` API key pass-through).<br>- Update Member 8 interface contract (`parcel.booked.v1`, `q.webhooks.dispatcher`, `swiftdrop.dlq`, compressed retries, SSRF dev flags).<br>- Update Member 9 interface contract (`make up`, `mock-webhook:9999` in Compose).<br>- Mark all open checklists as resolved (`[x]`). |
| `decisions-and-questionnaire.md` | - Add formal record of Decision 10.6 (Multi-Tenant Webhook Receiver & HTML Visual Test Artifact), Decision 10.7 (Elimination of Orphan Policy Endpoints & Dynamic Precedence Rule), Decision 10.8 (MCP Client Dual-Mode Authentication & Least Privilege), and Decision 10.9 (MCP SSE Transport Dual-Channel Mechanics). |
| `README.md` | - Update index and feature checklist to reflect completed audit remediation. |

---

## 6. Verification & Sign-off Criteria

1. **Zero-Database Invariant:** AI Assistant retains zero SQL connections, zero database drivers, and zero relational migrations.
2. **SSRF Guardrail Compliance:** Mock webhook receiver operates strictly within the containerized test environment without weakening production SSRF protections.
3. **Multi-Tenant Assurance:** E2E test harness unequivocally proves that Merchant B receives 0 events when Merchant A transacts.
4. **Living Documentation Synchronization:** Every change documented in this specification will be applied consistently across all Member 10 documents before reporting completion.
