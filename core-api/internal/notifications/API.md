# In-App Notifications API Specifications (`/api/v1/notifications`)

> **Module:** `internal/notifications` | **Owner:** Member 8 / Member 5 — Notifications & Real-Time Projections | **Status:** Active (Phase 2)

---

## 1. List User Notifications

Retrieves a paginated list of notifications for the authenticated user ordered from newest to oldest using keyset (cursor-based) pagination. If the authenticated user holds the `DISPATCHER` role, notifications targeted to `role:DISPATCHER` are included.

- **Method / Path:** `GET /api/v1/notifications`
- **Auth Required:** Bearer JWT (Any authenticated role: `CUSTOMER`, `MERCHANT`, `COURIER`, `DISPATCHER`, `ADMIN`)
- **Headers:**
  - `Authorization: Bearer <token>`
- **Query Parameters:**
  - `limit` (integer, optional, default: `20`, min: `1`, max: `100`): Maximum notifications to return per page.
  - `cursor` (string, optional): Base64URL-encoded pagination cursor returned from `meta.next_cursor` of a previous page.

### Success Response (`200 OK`)
```json
{
  "data": [
    {
      "id": "e0b821cf-3091-4cfd-b2b9-e16e6f11c750",
      "user_id": "usr_9988223344",
      "tenant_id": "tenant_apex_retail",
      "event_type": "parcel.status.in_transit",
      "aggregate_id": "pcl_01HXYZ998877",
      "title": "Parcel Out for Delivery",
      "body": "Your shipment pcl_01HXYZ998877 is out for delivery with courier Kasun Perera.",
      "metadata": {
        "courier_name": "Kasun Perera",
        "tracking_number": "SWIFT-202611-9988"
      },
      "is_read": false,
      "created_at": "2026-11-20T10:15:30Z",
      "read_at": null
    }
  ],
  "meta": {
    "limit": 20,
    "has_more": false,
    "next_cursor": null
  }
}
```

### Error Responses
- `400 Bad Request`: `INVALID_LIMIT` — Limit query parameter is not an integer between 1 and 100.
- `400 Bad Request`: `INVALID_CURSOR` — Pagination cursor is invalid or malformed.
- `401 Unauthorized`: `AUTH_UNAUTHORIZED` — Missing or invalid bearer authorization token.
- `500 Internal Server Error`: `INTERNAL_ERROR` — Database query or serialization error.

---

## 2. Get Unread Notification Count

Returns the count of unread notifications for the authenticated user. Includes `role:DISPATCHER` notifications if the caller holds the `DISPATCHER` role.

- **Method / Path:** `GET /api/v1/notifications/unread-count`
- **Auth Required:** Bearer JWT (Any authenticated role)
- **Headers:**
  - `Authorization: Bearer <token>`

### Success Response (`200 OK`)
```json
{
  "data": {
    "unread_count": 3
  }
}
```

### Error Responses
- `401 Unauthorized`: `AUTH_UNAUTHORIZED` — Missing or invalid bearer authorization token.
- `500 Internal Server Error`: `INTERNAL_ERROR` — Database query failure.

---

## 3. Mark Notification as Read

Marks a single notification identified by its UUID as read. The notification must belong to the authenticated caller or be addressed to `role:DISPATCHER` when invoked by a dispatcher.

- **Method / Path:** `PATCH /api/v1/notifications/{id}/read`
- **Auth Required:** Bearer JWT (Any authenticated role)
- **Headers:**
  - `Authorization: Bearer <token>`
- **Path Parameters:**
  - `id` (string, required): Unique UUID of the notification.

### Success Response (`200 OK`)
```json
{
  "data": {
    "id": "e0b821cf-3091-4cfd-b2b9-e16e6f11c750",
    "user_id": "usr_9988223344",
    "tenant_id": "tenant_apex_retail",
    "event_type": "parcel.status.in_transit",
    "aggregate_id": "pcl_01HXYZ998877",
    "title": "Parcel Out for Delivery",
    "body": "Your shipment pcl_01HXYZ998877 is out for delivery with courier Kasun Perera.",
    "metadata": {
      "courier_name": "Kasun Perera",
      "tracking_number": "SWIFT-202611-9988"
    },
    "is_read": true,
    "created_at": "2026-11-20T10:15:30Z",
    "read_at": "2026-11-20T10:20:00Z"
  }
}
```

### Error Responses
- `400 Bad Request`: `INVALID_NOTIFICATION_ID` — Notification ID is missing or invalid in URL path.
- `401 Unauthorized`: `AUTH_UNAUTHORIZED` — Missing or invalid bearer authorization token.
- `403 Forbidden`: `NOTIFICATION_FORBIDDEN` — Caller does not have permission to access or modify this notification.
- `404 Not Found`: `NOTIFICATION_NOT_FOUND` — Notification ID does not exist.
- `500 Internal Server Error`: `INTERNAL_ERROR` — Database execution failure.

---

## 4. Mark All Notifications as Read

Marks all unread notifications for the authenticated caller as read in a single batch operation.

- **Method / Path:** `PATCH /api/v1/notifications/read-all`
- **Auth Required:** Bearer JWT (Any authenticated role)
- **Headers:**
  - `Authorization: Bearer <token>`

### Success Response (`200 OK`)
```json
{
  "data": {
    "updated_count": 5
  }
}
```

### Error Responses
- `401 Unauthorized`: `AUTH_UNAUTHORIZED` — Missing or invalid bearer authorization token.
- `500 Internal Server Error`: `INTERNAL_ERROR` — Database execution failure.
