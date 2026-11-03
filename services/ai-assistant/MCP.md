# SwiftDrop — Model Context Protocol (FastMCP) Integration Guide

> **Service:** `services/ai-assistant`  
> **Role:** Member 10 — AI Assistant / MCP & Cross-Workflow Quality  
> **Primary Connection Standard:** Remote Server-Sent Events (SSE) via FastMCP  
> **Default Endpoint:** `http://127.0.0.1:8002/mcp` (or remote hosted gateway)  
> **Server Name:** `swiftdrop`  
> **Specification Version:** `2.13.0`

---

## 1. ⚡ Quick Start: Connect in 60 Seconds (Recommended)

You do **not** need to clone the repository, install Python, or configure local database containers to use SwiftDrop MCP. 

The primary and recommended integration method is **Remote Server-Sent Events (SSE)**. The SwiftDrop MCP service runs as an independent daemon exposing a standard JSON-RPC 2.0 SSE endpoint at `http://127.0.0.1:8002/mcp`. You simply paste the configuration block below into your AI client, and your assistant immediately gains live access to parcel tracking, fee estimation, postal directory lookups, and courier policy guidance.

```
                  ZERO-FRICTION MCP ARCHITECTURE
                  
   [Antigravity IDE]     [Claude Desktop]      [Cursor IDE]     [Autonomous Agents]
          │                     │                   │                    │
          └─────────────────────┼───────────────────┼────────────────────┘
                                │
                                ▼ (Standard JSON-RPC 2.0 over SSE)
                  ┌───────────────────────────---┐
                  │ http://ai.swiftdrop.work/mcp │
                  │    Server: 'swiftdrop'       │
                  └─────────────┬─────────────---┘
                                │
                                ▼
         ┌──────────────────────────────────────────────┐
         │       SwiftDrop FastMCP Gateway Daemon       │
         │  - 100% Read-Only Safety                     │
         │  - Zero Direct Database Access               │
         │  - SEC-009 Customer Privacy Masking          │
         │  - In-Memory Sri Lanka Postal Directory      │
         └──────────────────────────────────────────────┘
```

---

## 2. Drop-in Client Configuration Recipes

Choose your AI environment below and add the configuration block to your settings file:

### 2.1 Google Antigravity Configuration
File location: `~/.gemini/config/mcp_config.json`

```json
{
  "mcpServers": {
    "swiftdrop": {
      "command": "npx.cmd",
      "args": [
        "-y",
        "mcp-remote",
        "http://127.0.0.1:8002/mcp"
      ],
      "env": {
        "SWIFTDROP_API_KEY": "sd_live_demo_merchant_123"
      }
    }
  }
}
```
*(On macOS / Linux, replace `npx.cmd` with `npx`).*

---

### 2.2 Claude Desktop Configuration
File location:
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`
- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "swiftdrop": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote",
        "http://127.0.0.1:8002/mcp"
      ],
      "env": {
        "SWIFTDROP_API_KEY": "sd_live_demo_merchant_123"
      }
    }
  }
}
```

---

### 2.3 Cursor IDE Configuration
File location: `<workspace>/.cursor/mcp.json` or Global Cursor Settings

Cursor natively supports direct SSE URLs without requiring `npx`:

```json
{
  "mcpServers": {
    "swiftdrop": {
      "url": "http://127.0.0.1:8002/mcp",
      "headers": {
        "X-API-Key": "sd_live_demo_merchant_123"
      }
    }
  }
}
```

---

### 2.4 Windsurf / VS Code Roo Code Configuration
File location: Global Settings $\rightarrow$ `mcp_settings.json`

```json
{
  "mcpServers": {
    "swiftdrop": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote",
        "http://127.0.0.1:8002/mcp"
      ],
      "env": {
        "SWIFTDROP_API_KEY": "sd_live_demo_merchant_123"
      }
    }
  }
}
```

---

## 3. Authentication & API Key Management

The SwiftDrop MCP gateway supports both anonymous public operations and authenticated merchant capabilities:

### Public Tools (Zero Configuration Required)
Tools that query non-sensitive or public data do not require credentials:
- `track_parcel` (Milestone tracking with SEC-009 privacy masking)
- `estimate_shipping_fee` (Tariff calculations & booking preview cards)
- `lookup_postal_code` (Sri Lanka postal directory)
- `explain_company_policies` (Shipping guidelines & prohibited goods)

### Authenticated Tools (`get_my_active_parcels`)
Tools accessing private merchant or customer shipments require authentication. SwiftDrop uses an **Out-of-Band Auth Hierarchy** so secret keys are never typed into LLM prompts:

1. **`mcp-remote` Environment Injection:** Define `SWIFTDROP_API_KEY` in your client config (as shown in Section 2). The proxy automatically injects the credential into the SSE connection.
2. **SSE Gateway Header:** Directly configure `"headers": { "X-API-Key": "sd_live_..." }` in clients supporting HTTP headers (e.g. Cursor).
3. **Canonical Key Format:** Merchant keys must match `^sd_(live|test)_[a-zA-Z0-9_-]{8,}$` (e.g. `sd_live_demo_merchant_123`).

---

## 4. Complete Tool Catalog & Schemas

Once connected, your AI assistant automatically gains access to five production-grade tools:

### 4.1 `estimate_shipping_fee`
Calculates official shipping fees, transit SLAs, and generates a 1-click booking card based on package weight, dimensions, and postal codes.

- **Parameters:**
  - `weight_kg` *(float, required)*: Package gross weight in kg (`0.01 <= weight_kg <= 50.0`).
  - `pickup_postal_code` *(string, required)*: 5-digit origin postal code (e.g. `"00100"`).
  - `delivery_postal_code` *(string, required)*: 5-digit destination postal code (e.g. `"20000"`).
  - `service_type` *(string, optional)*: `"STANDARD"` (default), `"EXPRESS"`, or `"SAME_DAY"`.
  - `length_cm` *(float, optional)*: Package length in cm (`0.1 <= length_cm <= 300.0`).
  - `width_cm` *(float, optional)*: Package width in cm (`0.1 <= width_cm <= 300.0`).
  - `height_cm` *(float, optional)*: Package height in cm (`0.1 <= height_cm <= 300.0`).
- **Courier Invariants & Guardrails:**
  - **All-or-None 3D Dimensions:** If any dimension is specified, all three must be provided. Partial dimensions return a clean validation error.
  - **Maximum Girth Limit:** Length $+ 2 \times (\text{Width} + \text{Height}) \le 300.0\text{ cm}$.
  - **Maximum Volumetric Weight:** $(L \times W \times H) / 5000 \le 50.0\text{ kg}$.
  - **Billable Weight Calculation:** Evaluated automatically as $\max(\text{gross\_weight}, \text{volumetric\_weight})$.
- **Example Natural Language Prompt:**
  > *"How much to send a 2.5kg package from Colombo 01 (00100) to Kandy (20000) using Express delivery?"*

---

### 4.2 `track_parcel`
Retrieves public milestone tracking events, current transit status, and destination city for a shipment.

- **Parameters:**
  - `tracking_number` *(string, required)*: Tracking code matching `^SD-\d{8}-\d{5}$` (e.g. `SD-20261102-00001`).
- **SEC-009 Privacy Masking:** Strictly scrubs recipient phone numbers, full street addresses, and driver delivery notes. Only sanitized milestone timestamps and destination cities are returned.
- **Example Natural Language Prompt:**
  > *"Where is my package SD-20261102-00001 right now?"*

---

### 4.3 `get_my_active_parcels`
Retrieves active shipments (`BOOKED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY`) belonging to the authenticated merchant or customer.

- **Parameters:**
  - `status` *(string, optional)*: Filter by milestone (e.g. `"IN_TRANSIT"`). Dispatches both `status` and `status_filter` parameters to upstream Core API.
  - `limit` *(integer, optional)*: Maximum items to return (`1 <= limit <= 50`, default: `10`).
  - `authorization_token` *(string, optional)*: Optional customer Bearer JWT (defaults to environment or gateway credential).
- **Tenant Isolation:** Safe against IDOR; Core API derives merchant tenant ID directly from validated token claims.
- **Example Natural Language Prompt:**
  > *"Show me all my active deliveries that are currently in transit."*

---

### 4.4 `explain_company_policies`
Explains courier delivery rules, prohibited goods, packing standards, transit SLAs, and dispute claims.

- **Parameters:**
  - `query` *(string, required)*: Topic of inquiry (e.g. `"prohibited items"`, `"fragile packing"`, `"dispute deadline"`).
- **Token Inflation Defense:** Unmatched general queries return a structured 4-chapter policy summary index (< 800 chars) instead of dumping the full 5KB markdown into the LLM context window.
- **Example Natural Language Prompt:**
  > *"Can I send lithium-ion batteries or perfumes through SwiftDrop?"*

---

### 4.5 `lookup_postal_code`
Looks up official Sri Lankan 5-digit postal codes, administrative districts, and provinces for any city, town, or rural postal area.

- **Parameters:**
  - `city_or_area` *(string, required)*: Name of city, town, or postal division (e.g. `"Walasmulla"`, `"Kadugannawa"`, `"Mawanella"`, `"Mirissa"`).
- **Coverage:** Complete in-memory directory covering all 25 districts and commercial towns.
- **Example Natural Language Prompt:**
  > *"What is the postal code for Walasmulla or Kadugannawa?"*

---

## 5. Authoritative Context Resources

FastMCP context resources allow AI models to inspect static knowledge directly without executing a tool call:

| Resource URI | Content | Purpose |
| :--- | :--- | :--- |
| **`swiftdrop://policies`** | Markdown shipping guidelines | Factual grounding on packaging and claim procedures. |
| **`swiftdrop://sla-matrix`** | JSON delivery transit time SLAs | Authoritative reference for Express (24h) and Standard (48–72h) delivery times. |

---

## 6. Testing & Connectivity Verification

You can test connectivity to the SwiftDrop SSE gateway using standard command-line tools:

```bash
# 1. Test SSE handshake
curl -i -N http://127.0.0.1:8002/mcp

# 2. Test handshake with Merchant API Key
curl -i -N -H "X-API-Key: sd_live_demo_merchant_123" http://127.0.0.1:8002/mcp
```

Expected HTTP Response:
```http
HTTP/1.1 200 OK
content-type: text/event-stream
cache-control: no-cache
x-accel-buffering: no
```

---

## 7. Troubleshooting & FAQ

#### Q: Connecting to `http://127.0.0.1:8002/mcp` returns `401 Unauthorized`.
**Fix:** The server has handshake authentication enabled (`--require-auth`). Ensure your client configuration passes a valid `X-API-Key` matching `^sd_(live|test)_[a-zA-Z0-9_-]{8,}$` (e.g. `sd_live_demo_merchant_123`).

#### Q: Why did `estimate_shipping_fee` return a `422 Unprocessable Entity`?
**Fix:** Verify that:
1. All three dimensions (`length_cm`, `width_cm`, `height_cm`) were supplied together, or none at all.
2. Girth constraint is respected: $\text{Length} + 2 \times (\text{Width} + \text{Height}) \le 300\text{ cm}$.
3. Volumetric weight is within limits: $(L \times W \times H) / 5000 \le 50\text{ kg}$.
4. Gross weight is at least $0.01\text{ kg}$.

#### Q: Can the MCP server create shipments or make financial charges?
**Answer:** **No.** By design, the MCP server is 100% read-only. For parcel bookings, `estimate_shipping_fee` returns a pre-filled `NAVIGATE_BOOKING` action card (`/portal/book`) for human confirmation in the web application.

---

## 8. 🧑‍💻 [Advanced / Contributor] Running Locally via STDIO (`swiftdrop-local`)

> [!NOTE]
> This section is strictly for **SwiftDrop core developers** who are actively modifying the Python source code or running offline unit tests. End-users and AI tool integrators should use the **SSE (`swiftdrop`)** configuration in Section 2.

If you are developing inside the SwiftDrop repository and wish to run the FastMCP server directly as a local subprocess without a network daemon:

### Local Subprocess Setup
1. **Prepare Virtual Environment:**
   ```powershell
   cd services/ai-assistant
   python -m venv .venv
   .\.venv\Scripts\Activate.ps1
   pip install -r requirements.txt
   ```
2. **Verify Tool Handshake Directly:**
   ```powershell
   python scripts/verify_mcp_cli.py
   ```
3. **Configure Client for STDIO (`swiftdrop-local`):**
   Add to your `mcp_config.json`:
   ```json
   {
     "mcpServers": {
       "swiftdrop-local": {
         "command": "E:\\Projects\\SwiftDrop\\services\\ai-assistant\\.venv\\Scripts\\python.exe",
         "args": [
           "E:\\Projects\\SwiftDrop\\services\\ai-assistant\\src\\mcp\\server.py"
         ],
         "env": {
           "PYTHONPATH": "E:\\Projects\\SwiftDrop\\services\\ai-assistant",
           "CORE_API_URL": "http://localhost:8080",
           "TRACKING_API_URL": "http://localhost:8081",
           "MOCK_LLM_MODE": "true",
           "USE_MOCK_UPSTREAM": "true",
           "SWIFTDROP_API_KEY": "sd_live_demo_merchant_123"
         }
       }
     }
   }
   ```
