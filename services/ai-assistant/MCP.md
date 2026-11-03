# SwiftDrop — Model Context Protocol (FastMCP) Integration Guide

> **Service:** `services/ai-assistant`  
> **Protocol:** Model Context Protocol (MCP) Standard — JSON-RPC 2.0 over Server-Sent Events (SSE)  
> **Server Name:** `swiftdrop`  
> **Endpoint:** `http://127.0.0.1:8002/mcp`  

---

## 1. Quick Start: Connect via SSE (Recommended)

SwiftDrop MCP is exposed as a standard Server-Sent Events (SSE) service. You do not need to install Python or clone the repository to use SwiftDrop tools. Add the server definition to your AI client configuration to start using the tools immediately.

```
   [Antigravity IDE]     [Claude Desktop]      [Cursor IDE]     [Autonomous Agents]
          │                     │                   │                    │
          └─────────────────────┼───────────────────┼────────────────────┘
                                │
                                ▼ (JSON-RPC 2.0 over SSE)
                  ┌───────────────────────────┐
                  │ http://127.0.0.1:8002/mcp │
                  │    Server: 'swiftdrop'    │
                  └───────────────────────────┘
```

---

## 2. Client Configurations

### 2.1 Google Antigravity
File: `~/.gemini/config/mcp_config.json`

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
*(On macOS / Linux, use `"npx"` instead of `"npx.cmd"`).*

---

### 2.2 Claude Desktop
File:
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

### 2.3 Cursor IDE
File: `<workspace>/.cursor/mcp.json` or Global Cursor Settings

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

### 2.4 Windsurf / VS Code Roo Code
File: `mcp_settings.json`

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

## 3. Authentication

- **Public Tools:** `track_parcel`, `estimate_shipping_fee`, `lookup_postal_code`, and `explain_company_policies` require no authentication.
- **Authenticated Tools:** `get_my_active_parcels` requires a Merchant API Key (`sd_live_...`) or Bearer token passed via:
  - Header: `"X-API-Key": "sd_live_..."`
  - Environment variable: `SWIFTDROP_API_KEY="sd_live_..."`

---

## 4. Available Tools

| Tool | Description | Access |
| :--- | :--- | :--- |
| **`track_parcel`** | Retrieve tracking status and delivery milestones for a parcel. | Public |
| **`estimate_shipping_fee`** | Calculate shipping rate quote and estimated transit time. | Public |
| **`get_my_active_parcels`** | List active shipments for the authenticated merchant or customer. | Authenticated |
| **`explain_company_policies`** | Query courier shipping policies, prohibited items, and claims rules. | Public |
| **`lookup_postal_code`** | Lookup Sri Lankan 5-digit postal code by city or area name. | Public |

### Tool Parameters

#### `track_parcel`
- `tracking_number` *(string, required)*: Parcel tracking code (format: `SD-YYYYMMDD-XXXXX`).

#### `estimate_shipping_fee`
- `weight_kg` *(float, required)*: Package gross weight in kilograms (`0.01` to `50.0`).
- `pickup_postal_code` *(string, required)*: 5-digit origin postal code.
- `delivery_postal_code` *(string, required)*: 5-digit destination postal code.
- `service_type` *(string, optional)*: Delivery tier (`"STANDARD"`, `"EXPRESS"`, or `"SAME_DAY"`).
- `length_cm` *(float, optional)*: Package length in cm.
- `width_cm` *(float, optional)*: Package width in cm.
- `height_cm` *(float, optional)*: Package height in cm.

#### `get_my_active_parcels`
- `status` *(string, optional)*: Filter by milestone (`"BOOKED"`, `"IN_TRANSIT"`, etc.).
- `limit` *(integer, optional)*: Max records to return (default: `10`, max: `50`).
- `authorization_token` *(string, optional)*: Bearer token (defaults to configured API key or gateway header).

#### `explain_company_policies`
- `query` *(string, required)*: Topic to query (e.g., `"prohibited items"`, `"packaging"`).

#### `lookup_postal_code`
- `city_or_area` *(string, required)*: City, town, or area name.

---

## 5. Context Resources

| Resource URI | Description | MIME Type |
| :--- | :--- | :--- |
| **`swiftdrop://policies`** | Official shipping guidelines and packaging policy. | `text/markdown` |
| **`swiftdrop://sla-matrix`** | Delivery service transit time SLAs. | `application/json` |

---

## 6. Connectivity Verification

Verify connection to the SSE server using `curl`:

```bash
curl -i -N http://127.0.0.1:8002/mcp
```

Expected response:
```http
HTTP/1.1 200 OK
content-type: text/event-stream
cache-control: no-cache
x-accel-buffering: no
```

---

## 7. Troubleshooting

- **`401 Unauthorized` on connection:** Ensure your configuration supplies a valid `X-API-Key` (e.g., `sd_live_...`).
- **`422 Unprocessable Entity` on fee estimation:** Ensure weight is $\ge 0.01\text{ kg}$, and if providing dimensions, supply all three (`length_cm`, `width_cm`, `height_cm`).
- **Tools not appearing in AI client:** Verify the SSE service is running on port `8002` or check your client's MCP configuration logs.

---

## 8. Local Contributor Setup (STDIO)

For contributors developing the Python service locally who prefer direct process execution without an SSE network daemon:

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
