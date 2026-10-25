# Member 10 — AI Assistant & Quality Assurance Alignment Guide

> **Author / Domain Owner:** Member 10 — AI Assistant & Cross-Workflow Quality (Kakkaya)  
> **Target Modules:** `services/ai-assistant/` (AI Service) and `tests/e2e/` (Cross-Workflow Quality)  
> **Status:** Authoritative Architectural Alignment & Action Sheet for Team Meeting 2  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Domain Overview & Scope of Work

Member 10 owns two high-impact cross-cutting capabilities:
1. **The AI Delivery Assistant Service & MCP Server (`services/ai-assistant/`):** A lightweight, secure Python microservice allowing customers to query parcel tracking and calculate delivery fee quotes using natural language.
2. **Cross-Workflow Quality & End-to-End Test Suite (`tests/e2e/`):** Orchestrating the system-wide Playwright test matrix to verify that all 10 members' components integrate seamlessly.

### 1.1 In-Scope Responsibilities
- Developing the FastAPI microservice with Pydantic v2 schemas and FastMCP server.
- Implementing read-only AI tools:
  - `track_parcel`: Queries Member 5's Tracking API (`GET /api/v1/tracking/public/{trackingNumber}`).
  - `get_my_active_parcels`: Queries Member 2's Core API (`GET /api/v1/parcels` with Bearer JWT).
  - `estimate_delivery_fee`: Queries Member 2's Pricing Engine (`POST /api/v1/pricing/quotes`).
  - `get_service_coverage`: Queries Member 7's Delivery Zones (`GET /api/v1/admin/zones`).
- Prompt injection protection, strict system guardrails, and hallucination prevention.
- Ephemeral session caching (in-memory LRU / Redis). **Zero database persistence.**
- Building and maintaining the automated Playwright E2E test suite in `tests/e2e/`.
- Leading the live presentation rehearsal to ensure demo readiness.

### 1.2 Out-of-Scope Responsibilities
- Authoring unit tests for other members' Go modules (Each member tests their own domain).
- Direct SQL connections to PostgreSQL or RabbitMQ broker queues (Strictly forbidden).
- Executing parcel status mutations or financial transactions via AI tools.

---

## 2. Identified Flaws & Necessary Changes

1. **Container Port Conflict (Member 9 K1):** In `decisions-and-questionnaire.md` (Q5.1) and `doc-08`, Member 10 specified port `8003`.
   - *Fix:* Standardize internal container port to **`8000`** (Agreement A1 and ADR-001).
2. **Health Probe Paths & Semantics (Member 9 K2):** Member 10 proposed `/healthz` and `/readyz`, with `/readyz` pinging Core API and Tracking API.
   - *Fix:* Use standard endpoints **`/health/live`** and **`/health/ready`** (Agreement A2). Readiness checks **internal memory/model configuration only**. Do NOT check Core API or Tracking API in the readiness probe, as an upstream network blip would trigger a cascading failure.
3. **Database Elimination (Member 9 K3):** In `doc-05`, Member 10 defined an `ai_conversations` table in PostgreSQL.
   - *Fix:* Remove `ai_conversations` from the PostgreSQL migration suite. The AI service is strictly stateless, keeping session state client-side or in an ephemeral in-memory cache.

---

## 3. FastMCP Tool Definitions & Secure Execution Contracts

Tools are registered using FastMCP and validate arguments strictly via Pydantic v2:

```python
from mcp.server.fastmcp import FastMCP
import httpx
from pydantic import BaseModel, Field

mcp = FastMCP("SwiftDrop Assistant")

CORE_API_URL = "http://core-api:8080"
TRACKING_API_URL = "http://tracking-api:8081"

@mcp.tool()
async def track_parcel(tracking_number: str = Field(..., description="Parcel tracking number, e.g. SD-20261025-00123")) -> dict:
    """Fetch sanitized milestone progress for a parcel."""
    async with httpx.AsyncClient(timeout=5.0) as client:
        resp = await client.get(f"{TRACKING_API_URL}/api/v1/tracking/public/{tracking_number}")
        if resp.status_code == 404:
            return {"error": "Tracking number not found"}
        resp.raise_for_status()
        return resp.json()["data"]

@mcp.tool()
async def estimate_delivery_fee(
    origin_zone_id: str, 
    destination_zone_id: str, 
    weight_kg: float, 
    length_cm: float, 
    width_cm: float, 
    height_cm: float
) -> dict:
    """Calculate the delivery fee for a package between two zones."""
    payload = {
        "origin_zone_id": origin_zone_id,
        "destination_zone_id": destination_zone_id,
        "weight_kg": weight_kg,
        "length_cm": length_cm,
        "width_cm": width_cm,
        "height_cm": height_cm
    }
    async with httpx.AsyncClient(timeout=5.0) as client:
        resp = await client.post(f"{CORE_API_URL}/api/v1/pricing/quotes", json=payload)
        resp.raise_for_status()
        return resp.json()["data"]
```

---

## 4. Prompt Safety & Guardrails System Prompt

The AI Assistant must always execute under the following system prompt:

```text
You are the SwiftDrop Parcel Delivery Assistant. You help customers track shipments, estimate delivery fees, and understand our delivery zones.

STRICT OPERATIONAL RULES:
1. ONLY use information returned by the provided tools. NEVER invent tracking numbers, delivery statuses, or fee prices.
2. If a customer asks where their parcel is without providing a tracking number, ask them politely for their tracking number (format: SD-YYYYMMDD-XXXXX).
3. Public tracking data omits recipient street addresses and phone numbers for privacy; do not guess or apologize for masked fields.
4. You have NO permission to book parcels, assign couriers, or modify database records.
```

---

## 5. System-Wide Playwright E2E Test Suite (`tests/e2e/`)

Member 10 provides the automated integration test verifying the entire 10-member workflow:

```typescript
import { test, expect } from '@playwright/test';

test('Full Parcel Delivery Lifecycle Integration', async ({ page }) => {
  // 1. Customer Login (Member 1 & 6)
  await page.goto('/auth/login');
  await page.fill('[data-testid="login-email-input"]', 'customer.alice@swiftdrop.com');
  await page.fill('[data-testid="login-password-input"]', 'CustomerPass123!');
  await page.click('[data-testid="login-submit-btn"]');
  await expect(page).toHaveURL('/portal/dashboard');

  // 2. Book Parcel (Member 2 & 6)
  await page.goto('/portal/book');
  await page.fill('[data-testid="booking-weight-input"]', '2.5');
  await page.click('[data-testid="booking-submit-btn"]');
  const trackingNumber = await page.locator('[data-testid="tracking-number-display"]').innerText();

  // 3. Dispatcher Assignment (Member 3 & 7)
  await page.goto('/ops/dispatch');
  await page.click(`[data-testid="assign-parcel-${trackingNumber}"]`);

  // 4. Public Tracking Verification (Member 5 & 6)
  await page.goto(`/track/${trackingNumber}`);
  await expect(page.locator('[data-testid="tracking-status-badge"]')).toHaveText(/ASSIGNED|PICKED_UP/);

  // 5. AI Assistant Query (Member 10)
  const aiResp = await page.request.post('/api/v1/ai/chat', {
    data: { message: `Where is my parcel ${trackingNumber}?` }
  });
  expect(aiResp.ok()).toBeTruthy();
});
```

---

## 6. Member 9 DevOps & Infrastructure Configuration

| Config Parameter | Required Specification | Alignment Rule |
| :--- | :--- | :--- |
| **Container & Port** | `ai-assistant` binding to port `:8000` | Agreement A1 & K1 |
| **Health Probes** | `/health/live` & `/health/ready` (checks in-memory model config only) | Agreement A2 & K2 |
| **Database Requirement**| **Zero database tables.** Do not connect to PostgreSQL. | Agreement D1 & K3 |
| **Kubernetes Secrets**| Mounted from `integrations-secrets` (`OPENAI_API_KEY`) | Agreement A5 |
| **Traefik Route** | Traefik routes `/api/v1/ai/*` to `ai-assistant:8000` | Agreement A9 |

---
*End of Member 10 Specification & Alignment Guide.*
