# Customer and Merchant Frontend: Setup and Operations

Owner: Sajini, Member 6  
Date: 24 October 2026  
Status: Planned setup; package scripts and deployment are not verified

## Workspace

My code belongs to the unified `web/` application. The proposed stack is React, TypeScript, Vite, Tailwind CSS, TanStack Query and React Router. The frontend plan also calls for Vitest, React Testing Library and Playwright. Exact versions must come from the committed package manifest and lockfile; I have not verified those files.

My feature entry points will be the customer/merchant route components. Shared application startup, routing, auth providers and the HTTP client are coordinated with M7. The feature folder is not built as a separate application.

## Prerequisites

Use the Node/npm versions agreed in the repository. The overview currently lists Node 20 or later; the selected Vite version's actual engine requirement must be checked when dependencies are pinned. Docker/Compose are needed for container testing. Real integration also needs the gateway, Core API and Tracking API with seeded test users.

## Planned commands

Run from the repository root:

```sh
cd web
npm install
npm run dev
```

After a lockfile is committed, CI and repeatable clean installs should use `npm ci`. The planned development port is 3000. Vite must be configured accordingly; this is not established just by documenting the port.

```sh
npm run lint
npm run test:unit
npm run test:unit:coverage
npm run test:e2e
npm run build
npm run preview
```

`npm run build` is intended to produce `dist/`. Confirm that the actual Vite configuration does not override it. `npm run preview` is a local build check, not the production web server.

## Environment configuration

Proposed safe frontend example:

```dotenv
VITE_API_BASE_URL=/api/v1
```

This variable is public and is baked into the Vite build. Changing only a running container's environment will not change the existing bundle. Using the relative base URL allows the same image to call the gateway on its own origin across environments.

Do not put database credentials, JWT signing keys, OAuth client secrets or webhook secrets in frontend environment variables.

During local Vite development, M7/M9 and I need a development proxy for `/api/v1` or an agreed alternative so relative API calls reach the gateway. SSE proxying must remain streaming.

## Container handoff to M9

| Setting | Proposed value |
| --- | --- |
| Build | `npm run build` |
| Static output | `dist/`, pending configuration verification |
| API base | `/api/v1` |
| Runtime | Nginx serving static files |
| Container port | 3000 |
| Dockerfile/Nginx ownership | Proposed M9, coordinated with M6/M7 |

The Docker build should use a builder stage and copy only the built assets into the runtime image. Install from the committed lockfile. Select/pin runtime images after checking compatibility; no image versions are claimed here.

Nginx must listen on the agreed container port and provide SPA fallback for browser routes such as `/portal/book`. Missing assets should not silently return HTML. API and health paths must not fall through to the SPA.

The gateway routes `/api/v1/tracking` to Tracking API, other agreed API paths to their services, and frontend paths to the web container. M7/M9 must also account for any assistant route. Tracking streams need correct proxy timeout/buffering settings.

## Docker and Kubernetes checks

M9 maintains Compose/Kubernetes integration. If Compose remains at `deploy/docker-compose.yml`, run from the root with `docker compose -f deploy/docker-compose.yml ...` unless an agreed Makefile wraps it. Service names and final commands must be checked against the real file.

For Kubernetes, the web deployment needs a Service targeting container port 3000, gateway routing, probes and resources agreed with M9. A static web health probe checks that Nginx can serve; it does not prove backend APIs are healthy. Health endpoints must return an intentional result, not an SPA fallback page. The frontend itself has no queue-driven scaling need; replica/resource choices belong to deployment configuration and measurement.

## Debugging

| Symptom | First check |
| --- | --- |
| Blank page | Browser errors and missing static assets |
| API 404 | Base URL, duplicated prefix and gateway route |
| Refreshing a portal URL fails | Nginx SPA fallback |
| Login refresh fails | Cookie settings, origin, auth response and M1 contract |
| Tracking stays stale | Stream response, connection state, proxy buffering and re-fetch |
| Old data after account switch | Query keys and logout cache cleanup |
| Container env change has no effect | Vite value was compiled at build time |

I will use browser developer tools without copying sensitive tokens or customer data into screenshots/reports. Operational request logs and infrastructure dashboards are coordinated with M9. Frontend monitoring must not capture private form values.

## Startup, shutdown and limitations

The static container can start independently, but meaningful workflows require reachable APIs. On navigation/unmount, the application must stop timers and streams; on logout it clears private state. The server must handle normal container termination through its runtime configuration.

Known limitations: no actual build/container execution has been verified for this package, API conflicts remain open, no frontend performance figures exist, and no test results are included. These notes must be updated as implementation progresses.
