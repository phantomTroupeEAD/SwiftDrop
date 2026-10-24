# Document 7 — Security, Failure & Recovery Specification

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 9 — DevOps, Kubernetes & Observability + Database Coordinator (Minidu Dhananjana)  
> **Target Module / Location:** `deploy/` (Docker Compose, Kubernetes, Helm, monitoring), `.github/` (CI/CD), root `Makefile`; database coordination of `core-api/migrations/`  
> **Key Upstream Dependencies:** every service owner (Dockerfile, health endpoints, env vars), Member 7 (gateway routes), Member 8 (queues, reconciler), Member 2 (seed, migrations)  
> **Open Questions / Needs Clarification:** Backup storage location and retention; who is the backup cluster admin; Member 1 review of the per-service database login roles.  
> **Answers:** What can go wrong, how can it be abused, and how does the module recover?  
> **Scope note:** this **supplements** `docs/security/security-notes.md`, SRS §8 and ADR-002 §8. It does **not** redefine the system security policy.  

---

## 1. Authentication, authorization, ownership and isolation (platform level)

The platform does no user login. Its security is about **who can change, deploy and reach infrastructure**.

| Area | Rule | Source |
| :--- | :--- | :--- |
| Git `main` branch | PR required, at least 1 review, required CI checks, **code-owner review for migrations** (DB coordinator) | Work Division §4; Engineering Standards §3.3; election 2026-10-24 |
| GitHub Actions | least-privilege `GITHUB_TOKEN` (`contents: read` by default; `packages: write` only in `deploy.yml`); no `pull_request_target` with untrusted code; pinned action versions | Engineering Standards §4.1 (pinning) |
| Kubernetes API | kubeconfig held by Minidu (+ one named backup admin, TBD); never committed; VPS SSH is key-only | ADR-002 §3.1 |
| Workload identity | one **ServiceAccount per workload**, `automountServiceAccountToken: false` unless needed; no cluster-admin for apps | ADR-002 §8.2; SRS SEC-017 |
| Network | only Traefik is exposed. **NetworkPolicy:** Postgres and RabbitMQ reachable only from `core-api`, `tracking-api`, `outbox-relay`, `event-worker` and the reconciler/migrate jobs. Core API egress restricted; worker egress allowed for webhooks (with M8's SSRF guard) | ADR-002 §8.4 |
| VPS firewall | ports 80/443 only from **Cloudflare IP ranges** (UFW); everything else closed except SSH | ADR-002 §3.1 |
| Database | per-service roles; the tracking role is **read-only**; the AI service has **no** DB access; the superuser is never used by apps | Doc 5 §2.2; SRS §3.2, FR-AI-005 |
| Admin UIs | Traefik dashboard `api.insecure: false` in K8s; RabbitMQ management, Grafana and Prometheus reached only via `kubectl port-forward` | ADR-002 §8.1 |
| Tenant isolation | not a platform concern; enforced in the backend (M1/M4). The platform must not break it (e.g. no shared admin DB role for apps) | AGENTS.md §8 |

## 2. Sensitive data the platform handles

| Secret / sensitive data | Where it lives | Must never appear in |
| :--- | :--- | :--- |
| DB passwords, `DATABASE_URL` | `postgres-secrets` | git, images, logs, CI output |
| RabbitMQ credentials, `RABBITMQ_URL` | `rabbitmq-secrets` | same |
| JWT private key | `jwt-secrets` | same, and never in ConfigMaps |
| Google client secret, LLM key, SendGrid/Twilio, object storage keys, AI service token | `integrations-secrets` | same |
| Cloudflare Origin certificate key | `traefik-origin-tls` (C5) | same |
| kubeconfig, VPS SSH key, GHCR token | the admin's machine / GitHub secrets | the repo, chat messages, screenshots |
| DB backups (contain **PII**) | encrypted, off-site | laptops, git, public buckets |
| Logs | stdout → cluster | must not contain passwords, tokens, API keys, webhook secrets or PII (SEC-008) |

**Rule:** only `.env.example` and `secrets.example.yml` with **placeholders** are committed (SRS §11.1). `.env`, `*.pem` and `*.key` are already gitignored ✅.

## 3. Threats and abuse cases

| # | Threat | Impact | Prevention | Detection |
| :-: | :--- | :--- | :--- | :--- |
| T1 | Secret committed to git | credential theft | `.gitignore`, placeholder-only templates, review | TruffleHog in CI, `verify.sh` |
| T2 | Attacker skips Cloudflare and hits the VPS IP | bypasses WAF/DDoS protection | UFW allows only Cloudflare ranges | firewall logs |
| T3 | Spoofed `X-Forwarded-For` to dodge rate limits | rate limit bypass | `trustedIPs` = Cloudflare + private ranges only (`traefik.yml`) | Traefik access logs |
| T4 | Database or RabbitMQ port exposed | data theft | ClusterIP only, NetworkPolicy, no host ports | `kubectl get svc` review in PLT tests |
| T5 | Public Traefik dashboard or RabbitMQ UI | config leak or takeover | `insecure: false`, port-forward only | route review |
| T6 | Vulnerable or malicious base image | container compromise | pinned minimal bases, `trivy` scan | CI scan report |
| T7 | Container breakout or privilege escalation | node compromise | non-root 10001, `allowPrivilegeEscalation: false`, read-only root FS where possible, no privileged pods | manifest check |
| T8 | Supply chain via an unpinned CI action | CI secret theft | pin action versions (C25); minimal token permissions | review `.github/` changes |
| T9 | Secret theft through a PR from a fork | token leak | never run untrusted code with secrets (`pull_request`, not `pull_request_target`) | workflow review |
| T10 | kubeconfig leak | full cluster control | key-only SSH, no shared kubeconfig in chat, rotate on leak | K3s audit log (optional) |
| T11 | DoS on a single VPS | outage | Cloudflare, Traefik rate limits (M7), resource limits, upload limit 10 MB | Grafana, alerts |
| T12 | Disk fills up (PVCs, logs, images) | DB crash | disk alert at **80%** (ADR-002 §9), log rotation, image cleanup | alert |
| T13 | Backup theft | PII leak | encrypted backups, private off-site bucket | access logs |
| T14 | Harmful migration (drops data, wrong FK) | data loss | DB coordinator review + checklist, CI up/down/up, backup before deploy | migration log |
| T15 | App uses the superuser DB role | blast radius | per-service roles (Doc 5 §2.2) | review of Secrets |

## 4. Validation, rate limiting and resource limits

| Control | Value / rule | Owner |
| :--- | :--- | :--- |
| CPU/memory requests and limits | every workload (existing: postgres and rabbitmq 100m/256Mi → 500m/512Mi; reconciler 50m/64Mi → 200m/128Mi) | M9 |
| Namespace guard rails *(proposed)* | `LimitRange` defaults + `ResourceQuota` on `swiftdrop`, so a mistake can't starve the VPS | M9 |
| Upload size | 10 MB at the gateway (`dynamic-routes.yml`) | M7 |
| Rate limits | Traefik middlewares (C19) | M7 |
| Config validation | CI schema and lint checks (Doc 6) | M9 |
| DB connections | per-service pool budget ≤ `max_connections` (Doc 5 §5) | M9 |

## 5. Dependency timeouts and retries

| Dependency | Behaviour |
| :--- | :--- |
| Postgres down at start | Compose: dependants wait for `service_healthy`. K8s: readiness stays false and the apps retry connecting (app code) |
| Postgres down while running | readiness becomes `NOT_READY`, so the pod leaves the endpoints. **Not** restarted (liveness doesn't check dependencies) |
| RabbitMQ down | relay and worker readiness false; the outbox keeps events safe in Postgres (runbook Playbook B) |
| Migration Job fails | `backoffLimit: 2`, then the deploy stops; the old version keeps running |
| Image pull fails | `ImagePullBackOff`; the rollout doesn't progress; check the tag and GHCR access |
| Slow shutdown | SIGTERM, then the grace period (core-api 20s), then SIGKILL |

## 6. Duplicate requests, partial failures and concurrency conflicts

| Case | Handling |
| :--- | :--- |
| Two deploys at once | GitHub Actions `concurrency: deploy-main` (proposed) |
| Two migration runs at once | golang-migrate advisory lock; one Job per deploy |
| Migration fails half-way | `dirty` flag stops everything; fix → `force` → `up` (Doc 5 §3) |
| Rollout half done | rolling update keeps old pods until new ones are Ready; `rollout undo` restores |
| Overlapping CronJob runs | `concurrencyPolicy: Forbid` ✅ |
| Duplicate event publishing | relay singleton (`replicas: 1`); idempotent consumers (M8) |

## 7. Failure-mode matrix (recovery)

| Failure | Detect | Impact | Recovery | Data loss? |
| :--- | :--- | :--- | :--- | :--- |
| App pod crash | restarts count, alert | brief; other replicas serve | Kubernetes restarts it | no |
| Bad release | probes fail / errors spike | new pods not Ready | `kubectl rollout undo` (runbook §7) | no |
| Failed migration | migrate Job failed; `dirty = true` | deploy stopped; old app runs | fix SQL → `force` → re-run; restore from backup if data was damaged | no, if caught |
| `postgres-0` restart | pod event | short API outage | StatefulSet restarts it on the same PVC | no |
| Postgres disk lost / corrupted | pod fails, I/O errors | full outage | new PVC → `pg_restore` latest backup → re-run migrations to the expected version | **since the last backup** (up to 6h at the ADR-002 schedule) |
| RabbitMQ crash | relay/worker not ready | async delayed | `kubectl rollout restart statefulset/rabbitmq`; the relay resumes from the outbox | no (outbox in Postgres) |
| Outbox Relay crash | no publishing; outbox age grows | async delayed | Kubernetes restarts it (still 1 replica) | no |
| Worker crash mid-message | consumer failures | message redelivered | restart; idempotent consumers; DLQ after retries (M8) | no |
| Messages stuck in the DLQ | DLQ depth > 0 alert | some notifications/webhooks not sent | runbook §4.2 replay via the reconciler (M8) | no |
| VPS reboot | everything down, then up | outage during boot | K3s and pods restart automatically (ADR-002 §9) | no |
| Cloudflare outage | external checks fail | public access lost | wait, or temporarily point DNS directly (emergency only) | no |
| Origin certificate expired / secret missing | TLS errors | public access lost | re-create `traefik-origin-tls` (15-year Origin CA cert) | no |
| Secret leaked | scan alert / report | security risk | **rotate** the secret, update the K8s Secret, restart the pods, remove it from git history if it was committed | — |
| CI broken | red checks | merges blocked | fix the workflow; never skip checks (Engineering Standards §3.3) | no |
| GHCR unavailable | image pull errors | new deploys blocked | running pods unaffected; retry later | no |

**Reconciliation:** the outbox and delivery reconciliation **logic** is M8's (FR-REC-001..005). I deploy it as a CronJob every `*/10` minutes (C10), with no overlapping runs.

## 8. Audit logging

| What | Where |
| :--- | :--- |
| Who changed infrastructure | git history + PR reviews (`deploy/`, `.github/`) |
| Who changed the DB schema | migration log + CODEOWNERS approvals + `git log -- core-api/migrations/` |
| What was deployed and when | `kubectl rollout history`; CI deploy run logs; image git SHA |
| Admin actions on the cluster | K3s API audit log *(optional; enable if VPS resources allow)* |
| App-level security audits | `audit_logs` table (M7/M1), not platform |

## 9. Security test scenarios

| ID | Scenario | Expected |
| :--- | :--- | :--- |
| SEC-PLT-01 | Commit a fake AWS-style key in a test branch | CI secret scan fails |
| SEC-PLT-02 | `docker inspect` user on every image | `10001:10001` |
| SEC-PLT-03 | `curl` Postgres/RabbitMQ from outside the cluster | connection refused |
| SEC-PLT-04 | Pod in another namespace connects to `postgres:5432` | blocked by NetworkPolicy |
| SEC-PLT-05 | Open the Traefik dashboard via the public URL | not reachable |
| SEC-PLT-06 | Read health/metrics via the gateway | not routed |
| SEC-PLT-07 | `trivy image` on all images | no critical vulnerabilities, or documented exceptions |
| SEC-PLT-08 | tracking-api role tries `UPDATE parcels` | permission denied |
| SEC-PLT-09 | Merge a migration PR without the DB coordinator's approval | blocked by branch protection |
| SEC-PLT-10 | Spoof `X-Forwarded-For` from a non-Cloudflare IP | header ignored for rate limiting |

## 10. Module security checklist (tick before each milestone)

- [ ] No real secrets in git; only placeholders in `.env.example` / `secrets.example.yml`
- [ ] All images pinned, non-root, scanned
- [ ] Every workload has a ServiceAccount, resources and probes; no privileged pods
- [ ] Only Traefik is exposed; DB, broker and admin UIs are internal
- [ ] NetworkPolicies applied for Postgres and RabbitMQ
- [ ] Per-service DB roles; the tracking role is read-only; the AI service has no DB credentials
- [ ] CI token permissions are minimal; actions pinned
- [ ] Branch protection + CODEOWNERS active for migrations
- [ ] Backups encrypted and off-site; a restore drill has been done
- [ ] Disk usage alert at 80%
- [ ] Secret rotation steps written in the runbook
