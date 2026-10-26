# Document 5 — Data & Persistence Specification

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 10 — AI Delivery Assistant (Service & MCP) & Cross-Workflow Quality Coordination  
> **Target Production Locations:** `services/ai-assistant/` & `tests/e2e/`  
> **Authoritative Persistence:** PostgreSQL (Core Monolith); In-Memory Ephemeral Cache (AI Service)

---

## 1. Executive Summary & Storage Philosophy

This document defines the data models, persistence boundaries, caching strategies, and data lifecycle for Member 10's two mandates:
- **Part A (AI Delivery Assistant & FastMCP Service):** Governed by the **Zero Direct Database Access Invariant** (`AGENTS.md`). The service owns zero persistent relational tables, maintaining only ephemeral, short-lived session context in memory, with authoritative operational policies fetched over REST from Core API.
- **Part B (Cross-Workflow Quality & E2E Testing Framework):** Defines the database seed data contracts, dynamic test fixture factories, and ephemeral test state management required to run automated Playwright and Pytest verification without polluting production databases.

---

## Part A: AI Assistant Data Models, Caching & Privacy

### A.1 Relational Database Ownership & Boundaries

| Service Tier | PostgreSQL Access Level | Tables Owned | Justification |
| :--- | :--- | :--- | :--- |
| **AI Assistant (`services/ai-assistant/`)** | **NONE (Zero DB Access)** | **None** | Strictly prohibited from opening PostgreSQL connections. Decoupled from database migrations and connection pool limits (`AGENTS.md`). |
| **Core Monolith (`core-api/internal/admin`)** | Authoritative Persistence | `system_configurations` (Member 7) | Core API stores operational parameters (max weight, COD limits, fuel surcharges). |

---

### A.2 Policy Grounding Architecture (Pure Stateless Knowledge Base)

The AI Assistant owns **zero database tables** and does not require relational schema migrations for policy documents:

1. **Version-Controlled Policy Knowledge (`company_policies.md`):**
   - The authoritative qualitative policy guidelines (prohibited items, packaging regulations, claims workflows, liability caps) reside in `services/ai-assistant/knowledge/company_policies.md`.
   - The `PolicyContextHydrator` loads this static file into memory at service startup.

2. **Live Dynamic Configuration Overrides:**
   - Operational numbers and limits change dynamically during business operations. The assistant queries Core API's approved endpoint:
     `GET /api/v1/admin/system-configurations` (cached in-memory with a 5-minute TTL).
   - **Dynamic Precedence Invariant:**
     > *Whenever quantitative thresholds, rates, or limits in `GET /api/v1/admin/system-configurations` conflict with text in `company_policies.md`, the live API data takes absolute precedence.*

3. **In-Memory Injection Schema:**
   The hydrator synthesizes the base policy and live overrides into the LLM system prompt:
   ```text
   [AUTHORITATIVE ACTIVE OVERRIDES - HIGHEST PRECEDENCE]
   - Live Max Weight Limit: {max_parcel_weight_kg} kg
   - Live COD Limit: LKR {max_cod_amount_cents / 100}

   [BASE COMPANY POLICIES - GENERAL GUIDELINES]
   {company_policies_markdown}
   ```

---

### A.3 In-Memory Ephemeral Session Store (RAM Only)

The AI Assistant maintains multi-turn conversation context in local process memory using a thread-safe Least Recently Used (LRU) cache.

```mermaid
classDiagram
    class ConversationSession {
        +conversation_id: str (UUIDv4)
        +user_id: Optional[str]
        +role: Optional[str]
        +turns: List[ChatMessage]
        +created_at: datetime
        +last_accessed_at: datetime
        +add_turn(role: str, content: str, tool_calls: list)
        +prune_old_turns(max_turns: int)
        +is_expired(ttl_minutes: int) bool
    }

    class ChatMessage {
        +role: str ("user" | "assistant" | "tool")
        +content: str
        +timestamp: datetime
        +tool_calls: Optional[List[dict]]
        +action: Optional[ActionPayload]
    }

    class ActionPayload {
        +type: str ("NAVIGATE_BOOKING")
        +label: str
        +url: str
        +prefill_data: dict
    }

    ConversationSession "1" *-- "*" ChatMessage
    ChatMessage "0..1" *-- "1" ActionPayload
```

#### Session Eviction & Memory Bounds:
- **Maximum Active Sessions (`AI_SESSION_MAX_ACTIVE`):** Default `50` concurrent sessions per container replica. When the 51st session arrives, the least-recently accessed session is evicted from RAM.
- **Session Time-To-Live (`AI_SESSION_TTL_MINUTES`):** Default `15` minutes. Inactive sessions older than 15 minutes are garbage collected.
- **Turn Cap:** Each session stores at most **5 alternating turns** (10 messages). Older turns are trimmed to prevent prompt token bloat.

---

### A.3.1 Concrete In-Memory Data Scenarios

Below is an exact JSON snapshot of what an active `ConversationSession` holding `ChatMessage` and `ActionPayload` objects looks like in memory during a live 2-turn customer inquiry:

```json
{
  "conversation_id": "0192a6b2-9d3e-7a1b-8541-2c9e8a7b6543",
  "user_id": "usr_cust_001",
  "role": "CUSTOMER",
  "created_at": "2026-10-24T01:00:00Z",
  "last_accessed_at": "2026-10-24T01:02:15Z",
  "turns": [
    {
      "turn_id": 1,
      "role": "user",
      "content": "How much does it cost to send a 2.5 kg box from Colombo to Kandy?",
      "timestamp": "2026-10-24T01:00:00Z",
      "tool_calls": null,
      "action": null
    },
    {
      "turn_id": 2,
      "role": "assistant",
      "content": "Sending a 2.5 kg parcel from Colombo Central (ZONE_COLOMBO_01) to Kandy (ZONE_KANDY_01) is estimated at LKR 650.00 (Base fare LKR 350.00 + weight surcharge LKR 300.00). Delivery takes 1 business day.\n\nWould you like to book this delivery now?",
      "timestamp": "2026-10-24T01:00:02Z",
      "tool_calls": [
        {
          "name": "estimate_shipping_fee",
          "arguments": {
            "origin_zone_id": "ZONE_COLOMBO_01",
            "destination_zone_id": "ZONE_KANDY_01",
            "weight_kg": 2.5
          },
          "result": {
            "currency": "LKR",
            "fee_cents": 65000,
            "base_fee_cents": 35000,
            "weight_surcharge_cents": 30000,
            "estimated_transit_days": 1
          }
        }
      ],
      "action": {
        "type": "NAVIGATE_BOOKING",
        "label": "Proceed to Booking (Pre-filled)",
        "url": "/customer/book?origin=ZONE_COLOMBO_01&dest=ZONE_KANDY_01&weight=2.5&fee_cents=65000",
        "prefill_data": {
          "origin_zone_id": "ZONE_COLOMBO_01",
          "destination_zone_id": "ZONE_KANDY_01",
          "weight_kg": 2.5,
          "estimated_fee_cents": 65000,
          "currency": "LKR"
        }
      }
    },
    {
      "turn_id": 3,
      "role": "user",
      "content": "Can I put perfume or aerosol sprays in this package?",
      "timestamp": "2026-10-24T01:02:10Z",
      "tool_calls": null,
      "action": null
    },
    {
      "turn_id": 4,
      "role": "assistant",
      "content": "No, perfumes and aerosol sprays are strictly prohibited under SwiftDrop's Dangerous Goods & Prohibited Items policy because they contain pressurized, flammable liquids. Please remove these items before scheduling your dispatch.",
      "timestamp": "2026-10-24T01:02:12Z",
      "tool_calls": null,
      "action": null
    }
  ]
}
```

#### Breakdown of In-Memory Properties:
1. **`conversation_id`:** Time-ordered UUIDv7 generated on the first message.
2. **Turn 1 & 2 (Tool Execution + Action Card):**
   - The user query triggers tool execution (`estimate_shipping_fee`).
   - The tool call and its validated output are recorded in `tool_calls`.
   - The assistant emits both human text and the structured `action` object (`NAVIGATE_BOOKING`) with `prefill_data`. The React frontend renders this as a 1-click booking card.
3. **Turn 3 & 4 (Knowledge Base Grounding):**
   - The user asks about policy restrictions.
   - The assistant answers directly using its cached system prompt policy context (`company_policies.md`), without calling any tool, and correctly flags perfume as prohibited.
4. **Memory Pruning:** If the conversation reaches 6 turns, Turn 1 and Turn 2 are automatically evicted to maintain the strict 5-turn cap.

---

### A.4 In-Memory Policy Cache & Invalidation Protocol

To ground responses in real-time company rules without making an HTTP request to Core API on every single chat turn:
1. **Local Memory Storage:**
   - Active policy markdown text, operational constants, and `fetched_at` timestamp are cached in a Python singleton dictionary.
2. **Freshness & Refresh Strategy:**
   - **Background Polling:** Cached content expires after **30 minutes** (`POLICY_CACHE_TTL_MINUTES=30`). The next query asynchronously re-fetches `GET /api/v1/system/policies`.
   - **Instant Push Invalidation:** When an Admin edits or uploads a policy via the Admin Dashboard, Core API calls `POST /api/v1/ai/admin/refresh-knowledge` (secured via `X-Internal-Service-Key`), immediately purging the cache.
3. **Static Disk Fallback:**
   - If Core API is unreachable on container startup, the service loads `services/ai-assistant/knowledge/company_policies.md` from disk as a reliable baseline.

---

### A.5 Data Privacy, PII Redaction & Zero Retention

1. **Zero Database Storage of User Chats:**
   - Conversations are **never persisted to disk or database**. They live strictly in volatile memory.
   - Restarting or scaling down a pod permanently purges all active session memory.
2. **PII Masking at the Ingestion & Output Boundary:**
   - **Public Tracking Inquiries:** Public tracking responses mask recipient phone numbers and street addresses. Only city and milestone timestamps are retained in memory.
   - **Logs Sanitization:** Structured loggers strip JWT authorization headers, API keys, and sensitive user information before emitting stdout JSON logs.

---

## Part B: Cross-Workflow Quality & E2E Test Data Specification

### B.1 Seed Data Requirements (`seeds/dev_seed.sql`)

The automated E2E test suite requires Member 1, Member 2, and Member 3 to seed the following deterministic records in local and CI development databases:

#### 1. Deterministic User Accounts
| Role | Email | User ID | Password | Purpose |
| :--- | :--- | :--- | :--- | :--- |
| `CUSTOMER` | `customer.a@swiftdrop.lk` | `usr_cust_001` | `Password123!` | Primary customer booking & tracking journey |
| `CUSTOMER` | `customer.b@swiftdrop.lk` | `usr_cust_002` | `Password123!` | IDOR cross-tenant security verification |
| `COURIER` | `courier.kandy@swiftdrop.lk`| `usr_cour_001` | `Password123!` | Courier status advancement & proof upload |
| `COURIER` | `courier.colombo@swiftdrop.lk`| `usr_cour_002` | `Password123!` | Courier isolation verification |
| `DISPATCHER` | `dispatcher@swiftdrop.lk` | `usr_disp_001` | `Password123!` | Dispatcher unassigned board & assignment |
| `MERCHANT` | `merchant.store@swiftdrop.lk`| `usr_merch_001`| `Password123!` | B2B API order generation & webhook testing |
| `ADMIN` | `admin@swiftdrop.lk` | `usr_admin_001`| `Password123!` | Policy upload and tariff configuration |

#### 2. Deterministic Delivery Zones & Tariffs
```sql
INSERT INTO delivery_zones (id, code, name, base_surcharge_cents, enabled) VALUES
('ZONE_COLOMBO_01', 'COL_01', 'Colombo Central', 0, true),
('ZONE_KANDY_01', 'KDY_01', 'Kandy Central', 10000, true),
('ZONE_GALLE_01', 'GAL_01', 'Galle Fort', 8000, true)
ON CONFLICT (id) DO NOTHING;
```

---

### B.2 Dynamic Test Parcel Lifecycle & Collision Prevention

To prevent tests from conflicting with each other during concurrent or repeated test runs:
1. **Dynamic Parcel ID Generation:**
   - Every parcel created by a test receives a dynamic tracking code:
     $$\text{tracking\_number} = \text{"SD-TEST-" } + \text{current\_timestamp} + \text{"-" } + \text{random\_hex(4)}$$
     *(Example: `SD-TEST-1730000000-A4B1`)*
2. **Test Isolation:**
   - Tests do not assume hardcoded parcel IDs.
   - Playwright extracts the freshly generated tracking number from the confirmation card (`data-testid="booking-success-tracking-number"`) and passes it into subsequent tracking assertions.

---

### B.3 Multi-Tenant Mock Webhook Receiver & Visual HTML Dashboard (`mock-webhook:9999`)

To verify Scenario 2 (Merchant API $\to$ RabbitMQ $\to$ Webhook dispatch) with verifiable **Multi-Tenant Isolation (SEC-004 / SEC-005)** and produce visual grading evidence for the live demo rubric:

1. **Standalone Container Architecture:**
   - Deployed as a dedicated service `mock-webhook:9999` in `docker-compose.yml`.
   - Solves container networking: Event Worker connects directly to `http://mock-webhook:9999/webhooks/...`.
   - Satisfies SSRF guardrails in dev/test via environment flags: `WEBHOOK_ALLOW_INSECURE_HTTP=true` and `WEBHOOK_ALLOWED_HOSTS=mock-webhook,localhost`.

2. **Multi-Tenant Provisioning:**
   - **Merchant Alpha (Apex Retail):** Endpoint `POST /webhooks/merchant-alpha`, signing secret `whsec_alpha_test_secret_32_bytes_len!`.
   - **Merchant Beta (Zenith Logistics):** Endpoint `POST /webhooks/merchant-beta`, signing secret `whsec_beta_test_secret_32_bytes_leng!`.
   - **Chaos Merchant (Failure / DLQ Simulation):** Endpoint `POST /webhooks/merchant-chaos`. Always returns HTTP 500 to verify retries and routing to `swiftdrop.dlq`.

3. **In-Memory Event Store Schema (RAM Only):**
   ```python
   class CapturedWebhookEvent(BaseModel):
       event_id: str
       tenant_id: str
       target_endpoint: str
       event_type: str  # e.g., "parcel.booked.v1"
       delivery_attempt: int
       headers: Dict[str, str]
       payload: Dict[str, Any]
       hmac_signature_received: str
       hmac_signature_computed: str
       hmac_valid: bool
       received_at: datetime
   ```

4. **Live Visual HTML Dashboard (`http://localhost:9999/dashboard`):**
   - Serves an interactive, real-time UI rendered in browser.
   - Shows tenant tabs: Merchant Alpha feed, Merchant Beta feed, and DLQ Retry log.
   - Displays a green badge: `[TENANT ISOLATION VERIFIED: 0 CROSS-TALK]` confirming Merchant Beta received zero events when Merchant Alpha booked an order.
   - Provides expandable raw JSON CloudEvent and header inspectors.
   - Allows Playwright E2E tests to take a full-page screenshot (`test-results/artifacts/webhook-dashboard.png`) as live proof for evaluators.

---

## 2. Review Checklist for Team Meeting 2

- [x] **Member 1 (Auth):** Confirm that `seeds/dev_seed.sql` includes the test accounts in Section B.1 with matching passwords.
- [x] **Member 2 & 7 (Pricing/Admin):** Confirm the zone IDs (`ZONE_COLOMBO_01`, `ZONE_KANDY_01`, `ZONE_GALLE_01`) in the `delivery_zones` table.
- [x] **Member 7 & 1 (Admin/Core API):** Operational system configurations are queried dynamically via `GET /api/v1/admin/system-configurations` to override static policy text.
- [x] **Member 8 & 9 (Events/DevOps):** Standalone `mock-webhook:9999` container integrated in `docker-compose.yml` with dev SSRF bypass flags.
