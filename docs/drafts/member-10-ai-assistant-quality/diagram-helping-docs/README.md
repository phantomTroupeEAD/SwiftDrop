# Member 10 — Diagram-Helping Documentation Index

> **Domain:** Member 10 — AI Assistant/MCP & Cross-Workflow Quality  
> **Folder:** `docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/`  
> **Purpose:** Authoritative reference documentation containing complete, granular use case specifications for both of Member 10's modules, structured specifically to facilitate drawing UML Use Case Diagrams, Interaction Overview Diagrams, and test architecture diagrams.

---

## The Two Member 10 Modules

Member 10's workload is cleanly separated into two independent systems:

```
docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/
├── README.md                                                 # This index
├── MODULE_01_AI_ASSISTANT_AND_MCP_USE_CASES.md               # Module 1: AI Assistant & FastMCP Service (24 Use Cases)
└── MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_USE_CASES.md     # Module 2: Cross-Workflow Quality & E2E Testing (39 Use Cases)
```

---

### [Module 01: AI Delivery Assistant & FastMCP Service](./MODULE_01_AI_ASSISTANT_AND_MCP_USE_CASES.md)
- **Target Repository Location:** `services/ai-assistant/`
- **Assigned Container Port:** `8000` (Traefik Gateway route: `/api/v1/ai`)
- **Evaluation Weightage:** **+5 Bonus Marks**
- **Total Use Cases:** **24 Use Cases**
- **Core Capabilities Covered:**
  - **Category 1 (8 Use Cases):** Conversational Tools & Fact Grounding (`UC-AI-01` to `UC-AI-08`)
  - **Category 2 (2 Use Cases):** Generative UI Action Directives (`UC-AI-09` to `UC-AI-10`)
  - **Category 3 (4 Use Cases):** Model Context Protocol (FastMCP) STDIO & SSE (`UC-AI-11` to `UC-AI-14`)
  - **Category 4 (1 Use Case):** Deterministic Offline Mock LLM CI Simulation (`UC-AI-15`)
  - **Category 5 (6 Use Cases):** Guardrails, Adversarial & Anti-Abuse Security (`UC-AI-16` to `UC-AI-21`)
  - **Category 6 (3 Use Cases):** Ephemeral State, Sessions & Decoupled Health Probes (`UC-AI-22` to `UC-AI-24`)

---

### [Module 02: Cross-Workflow Quality & E2E Testing](./MODULE_02_CROSS_WORKFLOW_QUALITY_AND_E2E_USE_CASES.md)
- **Target Repository Location:** `tests/e2e/` & `tests/e2e/mock-webhook-receiver/`
- **Assigned Container Port:** `9999` (`mock-webhook`)
- **Evaluation Weightage:** **15 Marks** (Testing & Verification) + **15 Marks** (Live Demo Readiness)
- **Total Use Cases:** **39 Use Cases**
- **Core Capabilities Covered:**
  - **Category 7 (5 Use Cases):** The 5 Core Multi-Actor E2E Journeys (`UC-QA-01` to `UC-QA-05`)
    - Scenario 1A: Pure Web Customer Journey (Core Rubric — Zero AI Bot)
    - Scenario 2: Merchant B2B Ordering, Webhook Dispatch & DLQ Flow
    - Scenario 3: Dispatcher Assignment & Courier Proof-of-Delivery
    - Scenario 4: AI Delivery Assistant & FastMCP Conversational Flow
    - Scenario 5: Cross-Cutting Security, Tenant Isolation & IDOR Invariants
  - **Category 8 (16 Use Cases):** The 16 Mandatory SRS Acceptance Tests (`AT-001` to `AT-016`)
  - **Category 9 (16 Use Cases):** System Chaos, Edge-Case & Vulnerability Attacks (`CHAOS-01` to `CHAOS-16`)
  - **Category 10 (2 Use Cases):** Mock Webhook Visual Dashboard & Live Demo Artifacts (`UC-DEMO-01` to `UC-DEMO-02`)

---
*Maintained by Member 10 (AI Assistant/MCP & Cross-Workflow Quality).*
