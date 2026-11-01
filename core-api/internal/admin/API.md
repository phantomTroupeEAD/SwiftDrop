# Admin Module API Specifications (`/api/v1/admin`)

> **Module:** `internal/admin` | **Owners:** Member 7 (Admin & Gateway), Member 1 (Identity), Member 4 (Merchants)  
> **Status:** Authoritative Post-Submission Baseline (v2.0.0) | **Port:** Core API `:8080`

---

## 1. Manage Delivery Zones

### 1.1 Create Delivery Zone
Creates an operational geographic delivery zone with postal code prefix mappings.

- **Method / Path:** `POST /api/v1/admin/zones`
- **Auth Required:** Bearer JWT (`ADMIN` only)
- **Headers:** 
  - `Authorization: Bearer <jwt>`
  - `Content-Type: application/json`
  - `Idempotency-Key: <uuid-v4>`

#### Request Body
```json
{
  "id": "ZONE_COLOMBO_01",
  "code": "CMB-01",
  "name": "Colombo Central Commercial",
  "description": "Fort, Pettah, and Galle Face commercial districts",
  "base_surcharge_cents": 35000,
  "postal_code_prefixes": ["00100", "00200", "00300"],
  "boundary_polygon": {
    "type": "Polygon",
    "coordinates": [[[79.84, 6.92], [79.86, 6.92], [79.86, 6.94], [79.84, 6.94], [79.84, 6.92]]]
  }
}
```

#### Success Response (`201 Created`)
```json
{
  "data": {
    "id": "ZONE_COLOMBO_01",
    "code": "CMB-01",
    "name": "Colombo Central Commercial",
    "description": "Fort, Pettah, and Galle Face commercial districts",
    "base_surcharge_cents": 35000,
    "currency": "LKR",
    "enabled": true,
    "version": 1,
    "postal_code_prefixes": ["00100", "00200", "00300"],
    "created_at": "2026-10-25T10:00:00Z",
    "updated_at": "2026-10-25T10:00:00Z"
  }
}
```

#### Error Responses
- `400 Bad Request` (`INVALID_PAYLOAD`):
  ```json
  {
    "error": {
      "code": "INVALID_PAYLOAD",
      "message": "Validation failed on zone input attributes",
      "details": [
        { "field": "base_surcharge_cents", "issue": "surcharge cannot be negative" },
        { "field": "postal_code_prefixes", "issue": "postal code prefix must be 3 to 5 digits" }
      ]
    }
  }
  ```
- `401 Unauthorized` (`UNAUTHORIZED`): Missing or invalid Bearer token.
- `403 Forbidden` (`FORBIDDEN`): Authenticated user lacks `ADMIN` role.
- `409 Conflict` (`POSTAL_PREFIX_COLLISION`):
  ```json
  {
    "error": {
      "code": "POSTAL_PREFIX_COLLISION",
      "message": "Postal code prefix 00100 is already assigned to active zone ZONE_COLOMBO_02"
    }
  }
  ```

---

### 1.2 List Delivery Zones
Retrieves all configured delivery zones.

- **Method / Path:** `GET /api/v1/admin/zones`
- **Auth Required:** Bearer JWT (`ADMIN`, `DISPATCHER`)
- **Query Params:** `?enabled=true` (optional boolean filter)

#### Success Response (`200 OK`)
```json
{
  "data": [
    {
      "id": "ZONE_COLOMBO_01",
      "code": "CMB-01",
      "name": "Colombo Central Commercial",
      "base_surcharge_cents": 35000,
      "currency": "LKR",
      "enabled": true,
      "postal_code_prefixes": ["00100", "00200", "00300"],
      "version": 1,
      "updated_at": "2026-10-25T10:00:00Z"
    }
  ]
}
```

---

### 1.3 Update Delivery Zone
Modifies delivery zone parameters (name, surcharge, or postal code prefix associations).

- **Method / Path:** `PATCH /api/v1/admin/zones/{id}`
- **Auth Required:** Bearer JWT (`ADMIN` only)

#### Request Body
```json
{
  "name": "Colombo Central Commercial Extended",
  "base_surcharge_cents": 40000,
  "postal_code_prefixes": ["00100", "00200", "00300", "00400"],
  "version": 1
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "id": "ZONE_COLOMBO_01",
    "code": "CMB-01",
    "name": "Colombo Central Commercial Extended",
    "base_surcharge_cents": 40000,
    "currency": "LKR",
    "enabled": true,
    "version": 2,
    "postal_code_prefixes": ["00100", "00200", "00300", "00400"],
    "updated_at": "2026-10-25T10:15:00Z"
  }
}
```

#### Error Responses
- `409 Conflict` (`CONCURRENCY_CONFLICT`): Version mismatch; record was modified concurrently.

---

### 1.4 Enable / Disable Delivery Zone
Toggles operational activation status of a delivery zone.

- **Method / Path:** `PATCH /api/v1/admin/zones/{id}/status`
- **Auth Required:** Bearer JWT (`ADMIN` only)

#### Request Body
```json
{
  "enabled": false,
  "version": 2
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "id": "ZONE_COLOMBO_01",
    "enabled": false,
    "version": 3,
    "updated_at": "2026-10-25T10:30:00Z"
  }
}
```

---

## 2. System Settings Management

### 2.1 List Runtime System Settings
Retrieves configurable runtime flags and thresholds.

- **Method / Path:** `GET /api/v1/admin/system-settings`
- **Auth Required:** Bearer JWT (`ADMIN` only)

#### Success Response (`200 OK`)
```json
{
  "data": [
    {
      "key": "DISPATCH_AUTO_ASSIGN_ENABLED",
      "value": false,
      "value_type": "BOOLEAN",
      "description": "Enables automated background courier assignment",
      "updated_at": "2026-10-25T08:00:00Z"
    },
    {
      "key": "MAX_DELIVERY_ATTEMPTS",
      "value": 3,
      "value_type": "INTEGER",
      "description": "Maximum allowed delivery attempts before forced return",
      "updated_at": "2026-10-25T08:00:00Z"
    }
  ]
}
```

---

### 2.2 Update System Setting
Updates a runtime configuration toggle and records an immutable audit log entry.

- **Method / Path:** `PATCH /api/v1/admin/system-settings/{key}`
- **Auth Required:** Bearer JWT (`ADMIN` only)

#### Request Body
```json
{
  "value": true
}
```

#### Success Response (`200 OK`)
```json
{
  "data": {
    "key": "DISPATCH_AUTO_ASSIGN_ENABLED",
    "value": true,
    "value_type": "BOOLEAN",
    "updated_at": "2026-10-25T10:45:00Z"
  }
}
```

---

## 3. Generate Operational Reports

Retrieves delivery volume, success rates, failed attempts, and SLA compliance metrics.

- **Method / Path:** `GET /api/v1/admin/reports/operational`
- **Auth Required:** Bearer JWT (`ADMIN`, `DISPATCHER`)
- **Query Params:** `?from=2026-10-01&to=2026-10-25&zone_id=ZONE_COLOMBO_01`

#### Success Response (`200 OK`)
```json
{
  "data": {
    "period": { "from": "2026-10-01", "to": "2026-10-25" },
    "total_parcels": 2450,
    "delivered_successfully": 2380,
    "first_attempt_success_rate": "97.14%",
    "failed_attempts": 70,
    "returned_to_merchant": 12,
    "sla_compliance_rate": "98.50%",
    "average_delivery_time_hours": 3.4
  }
}
```

---

## 4. Cross-Domain Audit Trail Queries

Queries the immutable append-only audit trail for compliance and investigations.

- **Method / Path:** `GET /api/v1/admin/audit-logs`
- **Auth Required:** Bearer JWT (`ADMIN` only)
- **Query Params:** `?target_entity=delivery_zones&target_id=ZONE_COLOMBO_01&limit=50&offset=0`

#### Success Response (`200 OK`)
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "actor_id": "usr_01JABC9988",
      "action": "ZONE_UPDATED",
      "target_entity": "delivery_zones",
      "target_id": "ZONE_COLOMBO_01",
      "changes": {
        "base_surcharge_cents": [35000, 40000]
      },
      "ip_address": "192.168.1.100",
      "occurred_at": "2026-10-25T10:15:00Z"
    }
  ],
  "pagination": { "total": 1, "limit": 50, "offset": 0 }
}
```
