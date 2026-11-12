# SwiftDrop OpenAPI & Swagger Guide

> **Primary Maintainer:** Member 08 (Event-Driven Backend & Messaging / Core API Foundation)  
> **Audience:** All SwiftDrop Developers (Members 01 through 10)

---

## 1. Quick Start: Accessing Swagger UI

When running Core API locally (`APP_ENV=local` or `SWAGGER_ENABLED=true`), the interactive Swagger UI and OpenAPI directory are available out of the box:

- **OpenAPI Directory Index:** [http://localhost:8080/swagger/](http://localhost:8080/swagger/)
- **Events & Outbox Ops UI:** [http://localhost:8080/swagger/events/](http://localhost:8080/swagger/events/)
- **Raw OpenAPI JSON Spec:** [http://localhost:8080/api/v1/events/openapi.json](http://localhost:8080/api/v1/events/openapi.json)

### Alternative: Standalone Swagger UI Docker Container
You can also view any module spec without running the Go application:
```bash
docker run --rm -p 8089:8080 \
  -e SWAGGER_JSON=/spec/openapi.json \
  -v $(pwd)/core-api/internal/events/transport/openapi.json:/spec/openapi.json \
  swaggerapi/swagger-ui
```
Then open [http://localhost:8089](http://localhost:8089).

---

## 2. Authentication in Swagger UI

Protected endpoints require a valid JWT Bearer token:
1. Obtain an access token by logging into the Auth API:
   ```bash
   curl -X POST http://localhost:8080/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email": "admin@swiftdrop.local", "password": "AdminPassword123!"}'
   ```
2. Copy the `access_token` string from the JSON response.
3. In Swagger UI, click the green **Authorize** button at the top right.
4. Enter the token in the `bearerAuth` field and click **Authorize**. All subsequent requests from the Swagger UI will automatically include the `Authorization: Bearer <token>` header.

---

## 3. How Other Members Register Their Module OpenAPI Specs

Every domain module owner (Members 01, 02, 03, 04, 07) can register their module's OpenAPI specification in **one line of code** inside their composition wiring:

```go
package wiring

import (
    _ "embed"
    "github.com/swiftdrop/core-api/internal/events/swagger"
)

//go:embed transport/openapi.json
var parcelsOpenAPISpec []byte

func init() {
    _ = swagger.Register("parcels", parcelsOpenAPISpec)
}
```

Once registered:
- The module automatically appears in the `/swagger/` directory index.
- Dedicated interactive Swagger UI is served at `/swagger/parcels/`.
- The raw spec is available at `/swagger/parcels/spec.json`.

---

## 4. Specification Guidelines for Module Contributors

To maintain contract consistency across 10 developers:
1. **Format:** Author specifications in valid OpenAPI 3.0.3 JSON or YAML.
2. **Security Scheme:** Use the standard `bearerAuth` HTTP Bearer scheme:
   ```json
   "securitySchemes": {
     "bearerAuth": {
       "type": "http",
       "scheme": "bearer",
       "bearerFormat": "JWT"
     }
   }
   ```
3. **Response Envelopes:** All responses must conform to `docs/contracts/api-conventions.md`:
   - Single item: `{"data": { ... }}`
   - Paginated list: `{"data": [ ... ], "meta": { ... }}`
   - Errors: `{"error": { "code": "...", "message": "...", "correlation_id": "...", "timestamp": "..." }}`
4. **Concrete Examples:** Always include realistic values in request and response schemas so mock servers and clients can generate accurate tests.

---

## 5. Generating Typed Clients for Frontends (Members 06, 07 & 10)

Frontend teams (React/TypeScript) and AI Assistant developers (Python/FastAPI) can generate strongly typed API clients directly from the OpenAPI specifications without manual mapping:

### TypeScript / React
```bash
npx openapi-typescript http://localhost:8080/api/v1/events/openapi.json -o web/src/types/events-api.ts
```

### Python / FastAPI
```bash
datamodel-codegen --url http://localhost:8080/api/v1/events/openapi.json --output services/ai-assistant/app/models/events_gen.py
```

