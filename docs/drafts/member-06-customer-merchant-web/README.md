# Member 6 documentation

I prepared this folder to describe my customer and merchant frontend scope, planned design and integration needs for SwiftDrop.

## Documents

1. [Scope and responsibilities](01-scope-and-responsibilities.md)
2. [Functional flows and rules](02-functional-flows-and-rules.md)
3. [Frontend architecture](03-frontend-architecture.md)
4. [Interface and integration contract](04-interface-and-integration-contract.md)
5. [Data and persistence](05-data-and-persistence.md)
6. [Testing and acceptance](06-testing-and-acceptance.md)
7. [Security, failure and recovery](07-security-failure-and-recovery.md)
8. [Setup, deployment and operations](08-setup-deployment-and-operations.md)

`integration-inventory.json` is a supporting API inventory. It does not replace the backend OpenAPI schemas.

## Current status

These are design documents based on the assignment, supplied team baselines and repository text available on 24 October 2026. I have not verified source code, package scripts, deployed services or tests. Open decisions are marked in the relevant documents. The folder should be reviewed alongside the current repository contracts before implementation.

The suggested upload location is `docs/modules/customer-merchant-frontend/`, subject to the team's documentation structure. This folder does not replace the shared `web/README.md`; that file should continue to document the actual frontend implementation.

## Remaining handoffs

- M1: final session, roles and Google callback contracts.
- M2: quote path/schema, booking/idempotency, history and receipt contracts.
- M4: merchant application, approval, order and settings contracts.
- M5: public field policy, event schema, reconnect and polling behavior.
- M7: shared routes/components/client and frontend configuration.
- M9: Dockerfile/Nginx ownership, gateway routing and deployment checks.
- M10: coordinated end-to-end environment and test evidence.

The final submission will also need actual test reports/coverage and contribution evidence. I will add those after running the implementation rather than treating this plan as completed test evidence.
