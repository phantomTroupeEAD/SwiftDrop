# Identity, Access & Security Module (`internal/identity`)

> **Domain:** Authentication, Authorization, User Accounts & RBAC  
> **Primary Owner:** Member 1 — Identity, Access & Security  
> **Clean Architecture Boundary:** Modular Monolith Internal Module  
> **Current Status:** Login & RBAC Active | 5 Mock Role Accounts Ready

---

## 1. Module Overview & Responsibilities
The `identity` module serves as the authoritative security boundary for SwiftDrop. It manages user credentials, authentication lifecycles, JWT token issuance/validation, and role-based access control (RBAC).

### Key Responsibilities:
- User login authentication via email and secure password verification.
- Interactive Web Developer Login Portal UI (`GET /login` and `GET /`) with 1-click role autofill and live RBAC tester.
- Stateless JWT issuance with standard claims (`sub`, `email`, `role`, `tenant_id`, `exp`).
- HttpOnly refresh token session cookie handling (`swiftdrop_refresh_token`).
- Reusable authentication middleware (`SecurityMiddleware.Authenticate`) and authorization guards (`RequireRole`, `RequireTenant`).
- Tenant isolation helpers ensuring customers access only their shipments and merchants access only their tenant data.
- Embedded database auto-migrations applying schema tables and 5 mock accounts directly into PostgreSQL.

---

## 2. Pre-Seeded Mock Accounts (For Team Development)

| Role | Email | Password | Display Name | Tenant ID | User ID |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`ADMIN`** | `admin@swiftdrop.local` | `AdminPassword123!` | System Administrator | *None* (`null`) | `usr_01HADMIN000000000000000001` |
| **`DISPATCHER`** | `dispatcher@swiftdrop.local` | `Dispatcher123!` | Central Dispatcher | *None* (`null`) | `usr_01HDISPATCH00000000000002` |
| **`COURIER`** | `courier@swiftdrop.local` | `CourierPassword123!` | Kamal Perera | *None* (`null`) | `usr_01HCOURIER00000000000003` |
| **`MERCHANT`** | `merchant@vendor.com` | `Merchant123!` | Colombo Tech Stores | `mrc_01HMERCHANTTENANT00001` | `usr_01HMERCHANT00000000000004` |
| **`CUSTOMER`** | `customer@swiftdrop.local` | `CustomerPassword123!` | Nimal Fernando | *None* (`null`) | `usr_01HCUSTOMER00000000000005` |

---

## 3. How Other Team Members Integrate Auth (Middleware Guide)

Members 2, 3, 4, 5, 7, and 8 can protect any endpoint using Member 1's middleware:

### Step 1: Wrap routes with `security.Authenticate`
```go
// Enforces that an incoming request has a valid 'Authorization: Bearer <token>' header
mux.Handle("GET /api/v1/parcels", security.Authenticate(parcelHandler))
```

### Step 2: Restrict routes by Role using `transport.RequireRole(...)`
```go
// Only Couriers or Admins can access courier task assignments:
mux.Handle("GET /api/v1/courier/tasks", security.Authenticate(
    transport.RequireRole(domain.RoleCourier, domain.RoleAdmin)(courierTasksHandler),
))

// Only Admins can modify zones:
mux.Handle("POST /api/v1/admin/zones", security.Authenticate(
    transport.RequireRole(domain.RoleAdmin)(createZoneHandler),
))
```

### Step 3: Enforce Merchant Multi-Tenancy using `transport.RequireTenant()`
```go
// Guarantees caller has a valid tenant_id
mux.Handle("POST /api/v1/merchants/orders", security.Authenticate(
    transport.RequireRole(domain.RoleMerchant)(
        transport.RequireTenant()(createBulkOrdersHandler),
    ),
))
```

### Step 4: Access Context Helpers in Handlers
```go
func HandleCreateParcel(w http.ResponseWriter, r *http.Request) {
    userID := transport.GetUserID(r.Context())
    role := transport.GetUserRole(r.Context())
    tenantID := transport.GetTenantID(r.Context())
    // ...
}
```

---

## 4. Layer Breakdown
```
internal/identity/
├── domain/                  # User entity, Role enum, Password VO, Email VO, domain errors
├── application/             # LoginCommand, AuthService, ports (UserRepository, TokenProvider, etc.)
├── infrastructure/          # StandardPasswordHasher, JWTTokenProvider, MockUserRepository, PostgresUserRepository
└── transport/               # HTTP handlers (/api/v1/auth/login, /me), middleware (Auth, RequireRole), DTOs
```

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-29 | Implemented Login lifecycle, JWT provider, RBAC middlewares, 5 mock accounts & migrations | Member 1 |
