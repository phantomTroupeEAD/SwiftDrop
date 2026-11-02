# Module 01 — AI Delivery Assistant & FastMCP Activity Flows Specification

> **Module Name:** AI Delivery Assistant & Model Context Protocol (FastMCP) Service  
> **Repository Location:** `services/ai-assistant/`  
> **Target Folder:** `docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/`  
> **Document Purpose:** Complete, granular textual specification of all system activities, control flows, object transformations, decision branches, concurrent forks/joins, and terminal states for Module 01. Designed specifically to provide a diagram designer with every detail required to construct formal **UML Activity Diagrams** (with swimlanes/partitions) without containing visual diagrams.  
> **Specification Version:** `2.0.0` (Authoritative Baseline)  
> **Evaluation Scope:** **+5 Bonus Marks** (AI Assistant, Generative UI & FastMCP)

---

## Table of Contents
1. [Activity Modeling Principles & Swimlane Notation Guide](#1-activity-modeling-principles--swimlane-notation-guide)
2. [Primary Activity Flow 1: Conversational Chat Request & Dynamic Tool Orchestration Lifecycle](#2-primary-activity-flow-1-conversational-chat-request--dynamic-tool-orchestration-lifecycle)
3. [Primary Activity Flow 2: Authenticated Parcel Disambiguation & Tracking Lookup Activity](#3-primary-activity-flow-2-authenticated-parcel-disambiguation--tracking-lookup-activity)
4. [Primary Activity Flow 3: FastMCP SSE Dual-Channel Tool Execution Activity](#4-primary-activity-flow-3-fastmcp-sse-dual-channel-tool-execution-activity)
5. [Primary Activity Flow 4: Dynamic Policy Knowledge Hydration & Precedence Resolution Activity](#5-primary-activity-flow-4-dynamic-policy-knowledge-hydration--precedence-resolution-activity)
6. [Primary Activity Flow 5: Operational Health Probes & Graceful Failure Recovery Activity](#6-primary-activity-flow-5-operational-health-probes--graceful-failure-recovery-activity)
7. [Comprehensive UML Element Dictionary for Diagram 1](#7-comprehensive-uml-element-dictionary-for-diagram-1)

---

## 1. Activity Modeling Principles & Swimlane Notation Guide

When converting the specifications in this document into a visual UML Activity Diagram, the diagram designer must organize activities into the following **7 vertical or horizontal Swimlanes (Partitions)**:

1. **`User / Client Application`:** The browser React chat drawer (Member 6), customer web portal, or external desktop agent (Claude Desktop, Cursor IDE).
2. **`Traefik Edge Gateway`:** Edge ingress proxy handling rate limiting (20 RPM) and route dispatching (`/api/v1/ai/*`).
3. **`AI Assistant: Transport & Middleware`:** FastAPI routers (`chat.py`, `health.py`), Bearer token parser, request length validator, and SlowAPI rate limiter.
4. **`AI Assistant: Domain & LLM Engine`:** Prompt synthesizer, delimiter isolation wrapper (`<retrieved_data>`), `OpenAIAdapter` (Production) or `MockLLMAdapter` (CI).
5. **`AI Assistant: Sandboxed Tool Registry`:** The 4 read-only tools (`estimate_shipping_fee`, `track_parcel_status`, `get_my_active_parcels`, `explain_company_policies`).
6. **`AI Assistant: In-Memory RAM Store`:** Thread-safe `TTLCache` LRU session store (max 50 sessions, 15m TTL) and policy hydrator.
7. **`Downstream Core & Tracking Services`:** Core API Pricing (`:8080`), Core API Parcels (`:8080`), Core API Admin (`:8080`), and Tracking API (`:8081`).

---

## 2. Primary Activity Flow 1: Conversational Chat Request & Dynamic Tool Orchestration Lifecycle

This is the central activity flow of the AI Assistant microservice. It traces the lifecycle of a user prompt from initial ingress to final generative UI response.

### 2.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** User types natural language prompt into the floating chat widget and clicks send.
- **Preconditions:** Microservice is running on port `8000`; Traefik proxy is operational.

### 2.2 Step-by-Step Activity Flow

1. **Activity (`User / Client Application`):** Send HTTP POST request to `http://traefik/api/v1/ai/chat` with JSON body `{"message": "...", "conversation_id": "..."}`.
2. **Activity (`Traefik Edge Gateway`):** Intercept request. Evaluate client IP against token bucket rate limiter (20 requests/minute).
   - **Decision Diamond:** *Is request within rate limit?*
     - *[No (Rate Exceeded)]:* Emit HTTP `429 Too Many Requests` with header `Retry-After: 60`. Transition to **Activity Final Node ($\bigodot$)**.
     - *[Yes (Allowed)]:* Forward request to `http://ai-assistant:8000/api/v1/ai/chat`.
3. **Activity (`AI Assistant: Transport & Middleware`):** Ingest HTTP request. Inspect payload character length.
   - **Decision Diamond:** *Is payload length $\le 1,000$ characters and non-empty?*
     - *[No (Length > 1000 or Empty)]:* Return HTTP `400 Bad Request` with code `VALIDATION_FAILED`. Transition to **Activity Final Node ($\bigodot$)**.
     - *[Yes (Valid)]:* Pass payload to Auth Context Extractor.
4. **Activity (`AI Assistant: Transport & Middleware`):** Inspect `Authorization` HTTP header.
   - **Decision Diamond:** *Is valid Bearer JWT present?*
     - *[Yes]:* Parse `sub` (user_id), `tenant_id`, and `role`. Set session context as `Authenticated`.
     - *[No]:* Set session context as `Anonymous`.
5. **Activity (`AI Assistant: In-Memory RAM Store`):** Query LRU Session Cache using `conversation_id`.
   - **Decision Diamond:** *Does active session exist in memory and is TTL $<15$ minutes?*
     - *[Yes]:* Retrieve previous conversation turns (up to sliding window cap of 5 turns / 10 messages).
     - *[No]:* Initialize new `ConversationSession` in `TTLCache` with freshly generated UUIDv4.
6. **Activity (`AI Assistant: Domain & LLM Engine`):** Assemble prompt context.
   - *Fork Node (Concurrent Execution):*
     - Branch A: Ingest conversation history turns from RAM.
     - Branch B: Ingest grounded qualitative guidelines from `company_policies.md` and active live thresholds from Core API `GET /api/v1/admin/settings`.
     - Branch C: Ingest JSON Schema definitions of the 4 approved tools from the Tool Registry.
   - *Join Node:* Combine prompt context, delimiter instructions, and sanitized user prompt into LLM input buffer.
7. **Activity (`AI Assistant: Domain & LLM Engine`):** Inspect environment flag `TEST_MODE`.
   - **Decision Diamond:** *Is `TEST_MODE=true` or is `LLM_API_KEY` unset?*
     - *[Yes (Offline Mock Mode)]:* Dispatch to `MockLLMAdapter`. Perform deterministic regex/keyword evaluation in $<150\text{ms}$.
     - *[No (Production Mode)]:* Dispatch prompt buffer to `OpenAIAdapter` via universal `AsyncOpenAI` client (OpenRouter, NVIDIA NIM, OpenAI) at temperature `0.2` with timeout set to 10 seconds.
8. **Activity (`AI Assistant: Domain & LLM Engine`):** Parse model output.
   - **Decision Diamond:** *Did the model output a structured tool call or final text?*
     - *[Final Text]:* Skip tool execution. Transition directly to Step 12 (Action Card evaluation).
     - *[Tool Call]:* Extract tool name and JSON argument dictionary.
9. **Activity (`AI Assistant: Sandboxed Tool Registry`):** Validate tool name against allow-list.
   - **Decision Diamond:** *Is tool in allow-list (`estimate_shipping_fee`, `track_parcel_status`, `get_my_active_parcels`, `explain_company_policies`)?*
     - *[No (Unauthorized Tool)]:* Reject tool invocation. Return refusal text to model: *"Requested operation is not permitted."*
     - *[Yes (Approved Tool)]:* Validate input arguments against Pydantic schema.
10. **Activity (`AI Assistant: Sandboxed Tool Registry`):** Execute asynchronous HTTP call to backend services:
    - If `estimate_shipping_fee`: Call Core API `POST /api/v1/pricing/quotes`.
    - If `track_parcel_status`: Call Tracking API `GET /api/v1/tracking/public/{num}`.
    - If `get_my_active_parcels`: Call Core API `GET /api/v1/parcels` forwarding caller's Bearer JWT.
    - If `explain_company_policies`: Query local `PolicyContextHydrator`.
11. **Activity (`AI Assistant: Domain & LLM Engine`):** Ingest tool response payload. Wrap untrusted backend data in `<retrieved_data>` delimiter tags. Feed result back into the model to synthesize a conversational, grounded explanation.
12. **Activity (`AI Assistant: Domain & LLM Engine`):** Inspect conversation context for user booking intent or fee estimation results.
    - **Decision Diamond:** *Did the interaction compute a shipping tariff quote?*
      - *[Yes]:* Synthesize generative UI action directive `NAVIGATE_BOOKING` with deep-link URL `/portal/book?origin=...&fee_cents=...` and prefill dictionary. Attach to `data.action`.
      - *[No]:* Leave `data.action` as null.
13. **Activity (`AI Assistant: In-Memory RAM Store`):** Append current user prompt and assistant response to session turns in RAM. If turns exceed 5 alternating turns, evict the oldest turn.
14. **Activity (`User / Client Application`):** Receive HTTP `200 OK` response envelope. Render markdown chat bubble. If `action` exists, render interactive `[Book This Delivery]` button with `data-testid="ai-chat-action-btn"`.
15. **Terminal Node ($\bigodot$):** Interaction complete. Session remains in RAM awaiting next user turn.

---

## 3. Primary Activity Flow 2: Authenticated Parcel Disambiguation & Tracking Lookup Activity

This activity flow models the multi-turn decision logic when a customer asks an ambiguous tracking question without providing a tracking number.

### 3.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** User submits prompt: *"Where is my package?"*
- **Preconditions:** AI Assistant microservice operational.

### 3.2 Step-by-Step Activity Flow

1. **Activity (`AI Assistant: Transport`):** Inspect `Authorization` header.
   - **Decision Diamond:** *Is user authenticated with valid Bearer JWT?*
     - *[No (Anonymous User)]:* Assistant generates guidance message: *"To track your parcel, please provide your tracking number (e.g. SD-20261021-04921) or log in to your account."* Transition to **Activity Final Node ($\bigodot$)**.
     - *[Yes (Authenticated User)]:* Pass JWT context to `ParcelsTool`.
2. **Activity (`AI Assistant: Sandboxed Tool Registry`):** Invoke `get_my_active_parcels()`.
3. **Activity (`Downstream Core API`):** Core API receives `GET /api/v1/parcels` with Bearer token. Evaluates authenticated `user_id`. Queries database for non-delivered shipments (`status NOT IN ('DELIVERED', 'CANCELLED', 'RETURNED')`). Returns JSON array.
4. **Activity (`AI Assistant: Domain & LLM Engine`):** Inspect array length of active parcels returned.
   - **Decision Diamond:** *How many active parcels were found?*
     - **Branch A `[Count == 0]`:**
       - Assistant outputs: *"You do not have any active in-flight deliveries at the moment. If you have an external tracking number, please share it!"*
       - Transition to **Activity Final Node ($\bigodot$)**.
     - **Branch B `[Count == 1]` (Single Parcel Auto-Resolution):**
       - Extract tracking number of the single parcel (e.g., `SD-20261021-04921`).
       - Automatically invoke tool `track_parcel_status("SD-20261021-04921")`.
       - Query Tracking API `GET /api/v1/tracking/public/SD-20261021-04921`.
       - Format milestone history into friendly answer: *"Your active parcel SD-20261021-04921 to Kandy is Out for Delivery with our courier and expected by 2:00 PM."*
       - Attach `VIEW_TRACKING` action card.
       - Transition to **Activity Final Node ($\bigodot$)**.
     - **Branch C `[Count > 1]` (Multiple Parcels Disambiguation):**
       - **Zero Guessing Invariant:** Assistant does not guess which parcel the user meant.
       - Formats numbered list showing each tracking code, destination city, and current status.
       - Outputs: *"You currently have 2 active shipments: 1) SD-101 to Kandy (In Transit), 2) SD-102 to Galle (Out for Delivery). Which one would you like details for?"*
       - Stores candidate IDs in session memory.
       - Transition to **Activity Final Node ($\bigodot$)**.

---

## 4. Primary Activity Flow 3: FastMCP SSE Dual-Channel Tool Execution Activity

This activity flow models the bidirectional execution of tools by external AI agents (Claude Desktop, Cursor IDE) over standard HTTP using Server-Sent Events.

### 4.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** External AI client initiates connection to Model Context Protocol server.
- **Preconditions:** Service running; FastMCP protocol server active.

### 4.2 Step-by-Step Activity Flow

1. **Activity (`External AI Client`):** Dispatch HTTP GET to `http://traefik/api/v1/ai/mcp` with header `Accept: text/event-stream`.
2. **Activity (`FastMCP Server: Channel 1 Downstream`):** Accept HTTP request. Establish persistent downstream SSE connection. Generate unique session UUID (`session_id`).
3. **Activity (`FastMCP Server: Channel 1 Downstream`):** Emit initial SSE `endpoint` event containing session URL:
   `event: endpoint\ndata: /api/v1/ai/mcp/messages?session_id=7d4e9f1a-b3c2-...\n\n`
4. **Activity (`External AI Client`):** Parse `endpoint` event. Record session URL for upstream requests.
5. **Activity (`External AI Client`):** Construct JSON-RPC 2.0 tool invocation payload:
   `{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "estimate_shipping_fee", "arguments": {...}}}`.
6. **Activity (`External AI Client`):** Dispatch HTTP POST to `http://traefik/api/v1/ai/mcp/messages?session_id=7d4e9f1a-b3c2-...` (Channel 2 Upstream).
7. **Activity (`FastMCP Server: Channel 2 Upstream`):** Ingest JSON-RPC request. Validate `session_id` exists in active session registry.
8. **Activity (`FastMCP Server: Channel 2 Upstream`):** Check tool name against allow-list and validate parameters using Pydantic.
   - **Decision Diamond:** *Are arguments valid?*
     - *[No]:* Construct JSON-RPC error response (`code: -32602, message: "Invalid params"`).
     - *[Yes]:* Execute asynchronous HTTP call to backend Core API `POST /api/v1/pricing/quotes`.
9. **Activity (`Downstream Core API`):** Compute fee in integer cents; return JSON quote payload.
10. **Activity (`FastMCP Server: Channel 1 Downstream`):** Wrap Core API quote into standard JSON-RPC response envelope (`{"jsonrpc": "2.0", "id": 1, "result": {...}}`).
11. **Activity (`FastMCP Server: Channel 1 Downstream`):** Push message frame downstream through the persistent Channel 1 SSE stream:
    `event: message\ndata: {"jsonrpc": "2.0", "id": 1, "result": {"fee_cents": 65000}}\n\n`
12. **Activity (`External AI Client`):** Ingest SSE message frame; display authoritative result to evaluator in Cursor / Claude Desktop.
13. **Terminal Node ($\bigodot$):** Tool call executed successfully over dual-channel HTTP.

---

## 5. Primary Activity Flow 4: Dynamic Policy Knowledge Hydration & Precedence Resolution Activity

This activity flow models how the service ingests qualitative company guidelines, fetches active operational overrides, resolves conflicts, and caches data.

### 5.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** AI Assistant service initializes or user asks a policy inquiry.
- **Preconditions:** Core API running; `company_policies.md` present on disk.

### 5.2 Step-by-Step Activity Flow

1. **Activity (`PolicyContextHydrator`):** Check in-memory cache for existing policy context.
   - **Decision Diamond:** *Is policy context cached in RAM and is timestamp $<30$ minutes old?*
     - *[Yes (Cache Hit)]:* Provide cached context directly to prompt engine. Skip to Step 6.
     - *[No (Cache Miss)]:* Initiate parallel hydration.
2. **Fork Node (Concurrent Ingestion):**
   - **Branch 1:** Read qualitative guidelines from disk: `services/ai-assistant/knowledge/company_policies.md` (prohibited dangerous goods, packaging requirements).
   - **Branch 2:** Query Core API live settings endpoint: `GET http://core-api:8080/api/v1/admin/settings` (5-second timeout).
3. **Join Node:** Synchronize completion of Branch 1 and Branch 2.
4. **Activity (`PolicyContextHydrator`):** Evaluate responses for quantitative conflicts.
   - **Decision Diamond:** *Did Core API respond successfully with live numerical settings?*
     - *[No (Core API Offline/Timeout)]:* Fallback gracefully to static numbers from `company_policies.md`. Log warning.
     - *[Yes (Settings Received)]:* Apply **Dynamic Live Precedence Invariant**.
5. **Activity (`PolicyContextHydrator`):** Synthesize injection prompt:
   - Inject live operational numbers (`max_parcel_weight_kg`, `max_cod_amount_cents`) under `[AUTHORITATIVE ACTIVE OVERRIDES - HIGHEST PRECEDENCE]` header.
   - Append qualitative text under `[BASE COMPANY POLICIES - GENERAL GUIDELINES]` header.
   - Store synthesized text in memory singleton cache with 30-minute expiration.
6. **Activity (`Domain & LLM Engine`):** Feed synthesized policy context to LLM system prompt. LLM answers user inquiry accurately citing live limits.
7. **Terminal Node ($\bigodot$):** Policy response grounded with 100% currency.

---

## 6. Primary Activity Flow 5: Operational Health Probes & Graceful Failure Recovery Activity

This activity flow models the execution of Kubernetes readiness and liveness probes, ensuring decoupled operation without cascading cluster outages.

### 6.1 Initial Node & Preconditions
- **Initial Node ($\bullet$):** Kubernetes Kubelet dispatches periodic HTTP GET probe.
- **Preconditions:** Container running on port `8000`.

### 6.2 Step-by-Step Activity Flow

1. **Activity (`Kubelet`):** Dispatch HTTP probe.
   - **Decision Diamond:** *Which probe endpoint is requested?*
     - **Branch A (`/health/live`):**
       - Activity: FastAPI evaluates AsyncIO event loop responsive.
       - Emit HTTP `200 OK {"status": "alive"}`.
       - Transition to **Activity Final Node ($\bigodot$)**.
     - **Branch B (`/health/ready`):**
       - Activity: Inspect in-memory cache initialization and local model configuration status.
       - **Decoupled Architecture Rule:** `/health/ready` **never pings Core API or Tracking API**.
       - **Decision Diamond:** *Is in-memory cache initialized and process memory healthy?*
         - *[Yes]:* Emit HTTP `200 OK {"status": "ready", "llm_backend": "mock|openai", "cache_status": "ok"}`.
         - *[No]:* Emit HTTP `503 Service Unavailable {"status": "unavailable"}`.
       - Transition to **Activity Final Node ($\bigodot$)**.

---

## 7. Comprehensive UML Element Dictionary for Diagram 1

To assist the diagram designer in drawing **Activity Diagram 1**, use this exact dictionary:

| UML Element Type | Element Name / Label | Swimlane / Partition |
| :--- | :--- | :--- |
| **Initial Node** | `User Submits Prompt` | User / Client Application |
| **Action Node** | `Evaluate Ingress Rate Limit (20 RPM)` | Traefik Edge Gateway |
| **Decision Diamond**| `Rate Limit Exceeded?` | Traefik Edge Gateway |
| **Action Node** | `Validate Message Length (<1000 chars)` | AI Assistant: Transport |
| **Decision Diamond**| `Length Valid?` | AI Assistant: Transport |
| **Action Node** | `Parse Authorization Header (Bearer JWT)`| AI Assistant: Transport |
| **Decision Diamond**| `User Authenticated?` | AI Assistant: Transport |
| **Action Node** | `Lookup Conversation in TTLCache` | AI Assistant: In-Memory Store |
| **Fork Node** | `Parallel Prompt Assembly` | AI Assistant: Domain & LLM |
| **Action Node** | `Fetch Grounded Policy Guidelines` | AI Assistant: In-Memory Store |
| **Action Node** | `Fetch Tool JSON Schemas` | AI Assistant: Tool Registry |
| **Join Node** | `Assemble System Prompt & Input Buffer`| AI Assistant: Domain & LLM |
| **Decision Diamond**| `TEST_MODE == true?` | AI Assistant: Domain & LLM |
| **Action Node** | `Execute Deterministic Mock Adapter` | AI Assistant: Domain & LLM |
| **Action Node** | `Invoke Universal AsyncOpenAI Client` | AI Assistant: Domain & LLM |
| **Decision Diamond**| `Output Type == Tool Call?` | AI Assistant: Domain & LLM |
| **Action Node** | `Validate Tool Name against Allow-List` | AI Assistant: Tool Registry |
| **Action Node** | `Execute Core API Pricing Quote Call` | Downstream Core Services |
| **Action Node** | `Execute Tracking API Lookup Call` | Downstream Core Services |
| **Action Node** | `Wrap Result in <retrieved_data> Tags`| AI Assistant: Domain & LLM |
| **Action Node** | `Synthesize Natural Language Response` | AI Assistant: Domain & LLM |
| **Decision Diamond**| `Tariff Quote Calculated?` | AI Assistant: Domain & LLM |
| **Action Node** | `Attach NAVIGATE_BOOKING Action Card` | AI Assistant: Domain & LLM |
| **Action Node** | `Store Turns in In-Memory Session Cache`| AI Assistant: In-Memory Store |
| **Action Node** | `Render Markdown & Action Card Button` | User / Client Application |
| **Activity Final** | `Session Active (Awaiting Next Turn)` | User / Client Application |

---
*End of Module 01 Activity Flows Specification.*
