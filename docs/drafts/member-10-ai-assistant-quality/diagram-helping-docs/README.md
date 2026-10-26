# Member 10 — Diagram-Helping Documentation Index

> **Domain:** Member 10 — AI Assistant/MCP & Cross-Workflow Quality  
> **Folder:** `docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/`  
> **Purpose:** Authoritative reference documentation containing complete, granular use case, architecture, and activity flow specifications for both Member 10 systems. Designed specifically to facilitate drawing UML Use Case, Component, Deployment, Sequence, and Activity diagrams.

---

## Directory Inventory

Member 10's workload is cleanly separated into two independent systems. Each system possesses a dedicated **Use Cases Specification Document**, an **Architecture Specification Document**, and an **Activity Flows Specification Document**:

```
docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/
├── README.md                                                 # Master directory index
│
├── [Module 01: AI Delivery Assistant & FastMCP Service]
│   ├── MODULE_01_AI_ASSISTANT_AND_MCP_USE_CASES.md          # Complete Use Cases (24 Use Cases)
│   ├── MODULE_01_AI_ASSISTANT_AND_MCP_ARCHITECTURE.md       # Component & Service Architecture Blueprint
│   ├── MODULE_01_AI_ASSISTANT_AND_MCP_ACTIVITY_FLOWS.md      # UML Activity Flows & Swimlane Guide (Flows 1–5)
│   └── MODULE_01_AI_ASSISTANT_AND_MCP_SYSTEM_SEQUENCES.md   # UML System Sequence Flows (SEQ-AI-01 to 12)
│
└── [Module 02: Cross-Workflow Quality & E2E Testing]
    ├── MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_USE_CASES.md     # Complete Use Cases (39 Use Cases)
    ├── MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_ARCHITECTURE.md  # Quality & E2E Testing Topology Blueprint
    ├── MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_ACTIVITY_FLOWS.md # UML Activity Flows & Swimlane Guide (Flows 1–4)
    └── MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_SYSTEM_SEQUENCES.md # UML System Sequence Flows (SEQ-QA-01 to 14)
```

---

### Module 01: AI Delivery Assistant & FastMCP Service
- **Target Repository Location:** `services/ai-assistant/` (Container Port `8000`)
- **Evaluation Weightage:** **+5 Bonus Marks**
- **Authoritative Documents:**
  1. **[Module 01 Use Cases Specification](./MODULE_01_AI_ASSISTANT_AND_MCP_USE_CASES.md):**  
     Exhaustive specifications for all 24 use cases across Conversational Tools (`UC-AI-01` to `08`), Generative UI Action Directives (`UC-AI-09` to `10`), Model Context Protocol (`UC-AI-11` to `14`), Offline Mock CI Engine (`UC-AI-15`), Guardrails & Anti-Abuse (`UC-AI-16` to `21`), and Ephemeral State & Health Probes (`UC-AI-22` to `24`).
  2. **[Module 01 Architecture Specification](./MODULE_01_AI_ASSISTANT_AND_MCP_ARCHITECTURE.md):**  
     Comprehensive technical blueprint covering the microservice topology, clean 5-layer internal architecture, FastMCP STDIO and SSE Dual-Channel transports, LLM abstraction engine, in-memory RAM LRU session lifecycle, security isolation model, decoupled health probes, and diagram visual layout guide.
  3. **[Module 01 Activity Flows Specification](./MODULE_01_AI_ASSISTANT_AND_MCP_ACTIVITY_FLOWS.md):**  
     Granular step-by-step control flows, decision diamonds, forks/joins, swimlanes, and UML element dictionary for drawing **Activity Diagram 1** (Conversational Chat Lifecycle, Authenticated Disambiguation, FastMCP SSE Dual-Channel, Dynamic Policy Precedence, Health Probes).
  4. **[Module 01 System Sequences Specification](./MODULE_01_AI_ASSISTANT_AND_MCP_SYSTEM_SEQUENCES.md):**  
     Exhaustive lifeline definitions, synchronous/asynchronous interactions, payload schemas, and alternate/exception fragments for all 12 system sequence flows (`SEQ-AI-01` to `SEQ-AI-12`).

---

### Module 02: Cross-Workflow Quality & E2E Testing
- **Target Repository Location:** `tests/e2e/` & `mock-webhook:9999`
- **Evaluation Weightage:** **15 Marks** (Testing & Verification) + **15 Marks** (Live Demo Readiness)
- **Authoritative Documents:**
  1. **[Module 02 Use Cases Specification](./MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_USE_CASES.md):**  
     Exhaustive specifications for all 39 quality use cases across the 5 Core Multi-Actor E2E Journeys (`UC-QA-01` to `05`), 16 Mandatory SRS Acceptance Tests (`AT-001` to `016`), 16 System Chaos & Vulnerability Attacks (`CHAOS-01` to `16`), and Live Demo Visual Artifacts (`UC-DEMO-01` to `02`).
  2. **[Module 02 Architecture Specification](./MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_ARCHITECTURE.md):**  
     Comprehensive technical blueprint covering the Python test orchestrator (Playwright + HTTPX + Pika), full multi-actor system topology under test, the dedicated `mock-webhook:9999` container, real-time visual HTML dashboard, transactional outbox verification pipeline, chaos penetration architecture, and diagram visual layout guide.
  3. **[Module 02 Activity Flows Specification](./MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_ACTIVITY_FLOWS.md):**  
     Granular step-by-step control flows, decision diamonds, forks/joins, swimlanes, and UML element dictionary for drawing **Activity Diagram 2** (Customer-to-Delivery Multi-Actor Lifecycle, Async Outbox & Webhook DLQ Pipeline, Chaos Penetration Attacks, Test Evidence & Trace Capture).
  4. **[Module 02 System Sequences Specification](./MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_SYSTEM_SEQUENCES.md):**  
     Exhaustive lifeline definitions, synchronous/asynchronous broker acknowledgments, database transaction boundaries, HMAC signing, DLQ exponential backoffs, and chaos assertions for all 14 end-to-end system sequence flows (`SEQ-QA-01` to `SEQ-QA-14`).

---
*Maintained by Member 10 (AI Assistant/MCP & Cross-Workflow Quality).*
