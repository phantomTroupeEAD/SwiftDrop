# Identity Module API Specifications (`/api/v1/auth`, `/api/v1/admin`)

> **Module:** `internal/identity` | **Owner:** Member 1 | **Status:** Implemented & Verified

---

## 1. Registration — Step 1: Send Verification OTP
Checks email availability, generates a cryptographically secure 6-digit code, and dispatches it via email.

- **Method / Path:** `POST /api/v1/auth/register/send-otp`
- **Auth Required:** None (Public)
- **Headers:** `Content-Type: application/json`

### Request Body
```json
{
  "email": "alice@example.com"
}
```

### Success Response (`200 OK`)
```json
{
  "data": {
    "message": "Verification code sent to your email",
    "email": "alice@example.com",
    "expires_in": 600,
    "cooldown_seconds": 60
  }
}
```

### Error Responses
- `400 Bad Request`: `INVALID_EMAIL` — Email format is invalid.
- `409 Conflict`: `AUTH_EMAIL_ALREADY_EXISTS` — Email is already registered.
- `429 Too Many Requests`: `OTP_COOLDOWN_ACTIVE` — Resend cooldown active (wait 60 seconds).

---

## 2. Registration — Step 2: Complete Registration
Verifies the OTP code, enforces terms acceptance and password complexity, and creates the customer account.

- **Method / Path:** `POST /api/v1/auth/register`
- **Auth Required:** None (Public)
- **Headers:** `Content-Type: application/json`

### Request Body
```json
{
  "email": "alice@example.com",
  "otp": "481920",
  "first_name": "Alice",
  "last_name": "Johnson",
  "phone": "+94771234567",
  "password": "Password123!",
  "terms_accepted": true
}
```

### Success Response (`201 Created`)
```json
{
  "data": {
    "user": {
      "id": "usr_01HABC1234567890",
      "email": "alice@example.com",
      "first_name": "Alice",
      "last_name": "Johnson",
      "display_name": "Alice Johnson",
      "phone": "+94771234567",
      "role": "CUSTOMER",
      "tenant_id": null,
      "status": "ACTIVE",
      "email_verified": true,
      "requires_phone": false,
      "created_at": "2026-11-09T12:00:00Z"
    },
    "is_new_user": true
  }
}
```

---

## 3. Login User
Authenticates credentials and returns a short-lived signed JWT access token (15m) plus a rotated `swiftdrop_refresh_token` HttpOnly cookie (7d).

- **Method / Path:** `POST /api/v1/auth/login`
- **Auth Required:** None (Public)
- **Headers:** `Content-Type: application/json`

### Request Body
```json
{
  "email": "admin@swiftdrop.local",
  "password": "AdminPassword123!"
}
```

### Success Response (`200 OK`)
Sets `Set-Cookie: swiftdrop_refresh_token=...; HttpOnly; SameSite=Lax; Path=/api/v1/auth; Max-Age=604800`
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "usr_01HADMIN000000000000000001",
      "email": "admin@swiftdrop.local",
      "display_name": "System Administrator",
      "role": "ADMIN",
      "status": "ACTIVE",
      "tenant_id": null
    }
  }
}
```

---

## 4. Google OAuth 2.0 / Sign-in with Google
Exchanges a Google ID token for an authenticated session. Implements **Approach A (Progressive Onboarding)**: flags `requires_phone: true` when a mobile number is still needed.

- **Method / Path:** `POST /api/v1/auth/google`
- **Auth Required:** None (Public)
- **Headers:** `Content-Type: application/json`

### Request Body
```json
{
  "id_token": "eyJhbGciOiJSUzI1NiIs..."
}
```
*(In local development/tests, mock tokens like `mock_google_id_token_alice` or `mock_user@example.com` are accepted).*

### Success Response (`200 OK` or `201 Created` for new users)
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "usr_01HXYZ99999999999",
      "email": "alice@gmail.com",
      "display_name": "Alice Google",
      "role": "CUSTOMER",
      "status": "ACTIVE",
      "requires_phone": true
    },
    "is_new_user": true,
    "requires_phone": true
  }
}
```

---

## 5. Token Refresh (ADR-003 Single-Use Rotation)
Rotates the refresh token and mints a fresh JWT access token. Replay detection automatically invalidates the entire session family if a reused token is presented.

- **Method / Path:** `POST /api/v1/auth/refresh`
- **Auth Required:** Refresh Token Cookie (`swiftdrop_refresh_token`) or optional JSON body `{"refresh_token": "..."}`
- **Success Response (`200 OK`):** Rotates cookie and returns fresh `access_token`.
- **Error Responses:**
  - `401 Unauthorized`: `AUTH_TOKEN_REUSED` — Replay detected, family revoked.
  - `401 Unauthorized`: `AUTH_TOKEN_EXPIRED` — Session expired, re-login required.

---

## 6. Logout
Invalidates the session family and clears the refresh cookie.

- **Method / Path:** `POST /api/v1/auth/logout`
- **Auth Required:** None (reads cookie if present; idempotent).

---

## 7. Profile Management (`/api/v1/auth/me`)

### `GET /api/v1/auth/me`
Returns current security principal and profile details.
- **Headers:** `Authorization: Bearer <access_token>`

### `PUT /api/v1/auth/me`
Completes progressive onboarding (phone number + terms acceptance).
- **Headers:** `Authorization: Bearer <access_token>`
- **Request Body:**
```json
{
  "phone": "+94771234567",
  "terms_accepted": true
}
```

---

## 8. Administrative Governance (`/api/v1/admin`)
*Strictly guarded by `SecurityMiddleware` and `RequireRole(ADMIN)`.*

### `GET /api/v1/admin/users`
Lists users with optional role filtering and pagination.
- **Query Params:** `role=COURIER`, `limit=20`, `offset=0`

### `PUT /api/v1/admin/users/{id}/role`
Promotes or updates a user's role. If promoting to `MERCHANT`, `tenant_id` is required.
```json
{
  "role": "MERCHANT",
  "tenant_id": "mrc_colombo_vendor_01"
}
```

### `PUT /api/v1/admin/users/{id}/status`
Suspends or reactivates an account.
```json
{
  "status": "SUSPENDED"
}
```
*(Options: `"ACTIVE"`, `"SUSPENDED"`)*

---

## 9. Forgot Password / Password Recovery (OTP-Verified)

### Step 1: Request Password Reset OTP (`POST /api/v1/auth/forgot-password/send-otp`)
- **Auth Required:** None (Public)
- **Request Body:** `{"email": "alice@example.com"}`
- **Success Response (`200 OK`):** Always returns `200 OK` for valid email formats (even if not registered, to prevent account enumeration) with `expires_in: 600` and `cooldown_seconds: 60`.
- **Error Responses:** `400 INVALID_EMAIL`, `429 OTP_COOLDOWN_ACTIVE`.

### Step 2: Verify OTP & Reset Password (`POST /api/v1/auth/forgot-password/reset`)
- **Auth Required:** None (Public)
- **Request Body:**
```json
{
  "email": "alice@example.com",
  "otp": "481920",
  "new_password": "NewPassword123!"
}
```
- **Success Response (`200 OK`):** `{"data": {"message": "Password has been reset successfully. You can now sign in with your new password."}}`
- **Error Responses:** `400 INVALID_EMAIL`, `400 WEAK_PASSWORD`, `400 AUTH_OTP_INVALID`, `400 AUTH_OTP_EXPIRED`, `400 AUTH_OTP_MAX_ATTEMPTS`, `403 ACCOUNT_SUSPENDED`.
