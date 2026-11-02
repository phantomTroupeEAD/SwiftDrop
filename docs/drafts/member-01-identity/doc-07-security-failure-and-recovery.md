# Document 7 — Security, Failure & Recovery Specification

> **Module:** Identity, Access Control & Security  
> **Owning Member:** Member 1 — Identity, Access & Security  
> **Status:** Proposed Draft (For Review in Team Meeting 2)  
> **Target Production Location:** `core-api/internal/identity/`

---

## 1. Threat Modeling & Abuse Cases

| Threat ID | Threat Description | Attack Vector | Mitigation & Technical Countermeasure |
|:---|:---|:---|:---|
| **TH-01** | **Credential Stuffing & Brute Force** | Automated dictionary attacks against `/api/v1/auth/login`. | - Argon2id memory-hard hashing slows offline attacks.<br>- Traefik token-bucket rate limit: 5 req/min per IP.<br>- Uniform error responses to eliminate username enumeration. |
| **TH-02** | **Token Theft via XSS / Storage Scraping** | Malicious JavaScript stealing access or refresh tokens. | - Access token stored **strictly in-memory** (`AuthContext`).<br>- Refresh token stored in `HttpOnly`, `SameSite=Strict`, `Secure` cookie (unreadable by JavaScript). |
| **TH-03** | **Refresh Token Replay (Session Hijacking)** | Attacker intercepts a rotated refresh token and attempts exchange. | - Single-use Refresh Token Rotation (RTR).<br>- Token family lineage tracking: presenting a previously rotated token revokes all sessions in that `family_id` immediately (`SEC-004`). |
| **TH-04** | **Privilege Escalation (Vertical)** | Regular customer requests administrative endpoints or passes `role = 'ADMIN'` in payload. | - Registration endpoint ignores role fields and forces `role = 'CUSTOMER'`.<br>- Downstream endpoints protected by backend `RequireRole` middleware, not frontend button visibility. |
| **TH-05** | **Insecure Direct Object Reference (IDOR)** | Customer changes parcel UUID to inspect another user's parcel history. | - `RequireOwnership` middleware verifies `user_id == parcel.sender_id` or caller holds `ADMIN`/`DISPATCHER` role. |
| **TH-06** | **Google OIDC Nonce / State Forgery** | Man-in-the-middle or replay of Google authentication code. | - Cryptographic `state` and `nonce` stored in transient HttpOnly cookies and validated against Google ID token claims. |

---

## 2. Sensitive Data Classification & Protection Rules

| Data Element | Sensitivity | Storage Format | Logging & Transport Rules |
|:---|:---|:---|:---|
| **User Password** | Critical | Argon2id / bcrypt salted hash | **STRICTLY PROHIBITED** from all logs, queries, and API responses. Sanitized before transport serialization. |
| **Refresh Token Secret** | High | SHA-256 hash | Client holds raw token in cookie; DB holds hash only. Never logged. |
| **JWT Signing Key** | Critical | Kubernetes Secret (`JWT_PRIVATE_KEY_PEM`) | Kept in application memory; never exposed via API or committed to Git. |
| **User Email & Phone** | PII | Cleartext in PostgreSQL | Masked in public tracking queries; logged only as anonymous UUIDs. |

---

## 3. Failure Modes & Recovery Matrix

| Component / Dependency | Failure Scenario | Impact | System Behavior & Recovery Action |
|:---|:---|:---|:---|
| **PostgreSQL Database** | Database down or connection pool exhausted | New logins, registrations, and token refreshes fail. | - Returns `503 Service Unavailable`.<br>- Existing RS256 access tokens continue to authenticate in-memory at downstream services until 15-minute expiration. |
| **Google OIDC Provider** | Google Identity API unreachable or network timeout | Google login button fails. | - Returns structured error `AUTH_OIDC_PROVIDER_UNAVAILABLE`.<br>- Local email/password login operates completely unaffected. |
| **Concurrent Token Refresh** | Multi-tab browser triggers concurrent refresh requests. | Token reuse detection triggered. | - Client-side state managers must serialize token refresh calls.<br>- In accordance with ADR-003 (RTR), any reuse of an already rotated refresh token triggers instant invalidation of the entire `family_id` to prevent session hijacking. |
| **Leaked Database Dump** | Database compromised by external threat actor. | Password hashes and refresh hashes dumped. | - Argon2id parameters ($m=65536, t=3, p=2$) resist GPU dictionary cracking.<br>- All refresh tokens are hashed with SHA-256, rendering dumped values useless for session hijacking. |

---

## 4. Audit Logging & Security Event Tracing (SEC-015)

The Identity module emits structured JSON security audit events directly to `stdout` and commits authoritative records to the shared `audit_logs` table via `AuditWriter`:

```json
{
  "timestamp": "2026-10-25T11:20:00Z",
  "level": "WARN",
  "service": "core-api",
  "module": "identity",
  "event": "TOKEN_REUSE_DETECTED",
  "actor_id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
  "actor_role": "CUSTOMER",
  "family_id": "0192e4a8-f123-7c91-9e12-aabbccddeeff",
  "client_ip": "192.0.2.45",
  "user_agent": "Mozilla/5.0 ...",
  "request_id": "req_01HXYZ99",
  "action_taken": "REVOKE_TOKEN_FAMILY"
}
```

### Standard Audited Events:
1. `USER_LOGIN_SUCCESS` / `USER_LOGIN_FAILED`
2. `TOKEN_ROTATED` / `TOKEN_REUSE_DETECTED`
3. `USER_REGISTERED`
4. `USER_ROLE_ASSIGNED` (e.g. `MERCHANT_ROLE_GRANTED` via Member 4 `RoleAssigner`)
5. `USER_SUSPENDED` (Administrative lock via Member 7)
6. `USER_SESSIONS_REVOKED` (Session purge via Member 7)
