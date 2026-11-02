# Compose needs from Member 7 (proposal for Member 9)

**Status:** Proposal. Member 9 owns the one local stack file, `deploy/docker-compose.yml`, run with
`make up` (agreement A7). Members do not keep their own Compose files (K10), so this folder has
**no** `docker-compose.m7.yml` any more. Below is what my module needs; Member 9 merges it in.

| Need | Value | Why |
| --- | --- | --- |
| Database roles before migrations | `scripts/db-bootstrap.sql` (creates `swiftdrop_migrator` and `swiftdrop_app`, D6, D7) mounted into `/docker-entrypoint-initdb.d/` | Migrations run as the migrator; core-api runs as the app role |
| Migrations | `migrate/migrate:v4.17.1`, `-path /migrations`, URL with `swiftdrop_migrator` | Files `000002_create_delivery_zones`, `000011_create_audit_logs` |
| core-api env | `DATABASE_URL` with `swiftdrop_app`, `JWT_PUBLIC_KEY_PEM=/run/keys/jwt_public.pem` | Least privilege; public key only |
| core-api health check | `/core-api -healthcheck` (calls `/health/live`, A2) | Distroless image has no curl |
| core-api stop grace | `stop_grace_period: 30s` | 5 s drain + 20 s grace + margin |
| Traefik ports | `80:80`, dashboard `127.0.0.1:8090:8090` only | Dashboard never public (R-10) |
| Traefik files | `deploy/gateway/traefik/traefik.yml` and `dynamic-routes.yml` mounted read-only | My gateway config |

Snippet in the shape of the shared file (service names as in `deploy/docker-compose.yml`):

```yaml
services:
  postgres:
    volumes:
      - ./scripts/db-bootstrap.sql:/docker-entrypoint-initdb.d/10-bootstrap.sql:ro

  migrate:
    image: migrate/migrate:v4.17.1
    command: ["-path", "/migrations", "-database",
              "postgres://swiftdrop_migrator:migrator_dev_only@postgres:5432/swiftdrop_db?sslmode=disable", "up"]
    volumes: ["./core-api/migrations:/migrations:ro"]
    depends_on: {postgres: {condition: service_healthy}}
    restart: "no"

  core-api:
    environment:
      DATABASE_URL: postgres://swiftdrop_app:app_dev_only@postgres:5432/swiftdrop_db?sslmode=disable
      JWT_PUBLIC_KEY_PEM: /run/keys/jwt_public.pem
    read_only: true
    security_opt: ["no-new-privileges:true"]
    depends_on: {migrate: {condition: service_completed_successfully}}
    stop_grace_period: 30s

  traefik:
    image: traefik:v3.1.7
    command: ["--configFile=/etc/traefik/traefik.yml"]
    volumes:
      - ./deploy/gateway/traefik/traefik.yml:/etc/traefik/traefik.yml:ro
      - ./deploy/gateway/traefik/dynamic-routes.yml:/etc/traefik/dynamic/dynamic-routes.yml:ro
    ports: ["80:80", "127.0.0.1:8090:8090"]
```

Local passwords above are demo values only. Real ones come from `postgres-secrets` and
`postgres-migrator-secrets` (A5).
