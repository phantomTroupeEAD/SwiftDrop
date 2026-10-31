# AI Review Prompt — Check Your Drafts Against the Member 9 Agreements

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 9 — Minidu Dhananjana (DevOps, Kubernetes & Observability + Database Coordinator)  
> **What this is:** a copy-paste prompt for **your own AI assistant** (Claude Code, Cursor, GitHub Copilot Chat, ChatGPT, …). It reads the shared agreements and **your** draft folder, then tells you exactly what to change. It takes about 5 minutes.

---

## How to use it (3 steps)

1. **Get the latest drafts:** `git checkout docs/module-spec-reviews && git pull`
2. **Open your AI in the repo folder.**
3. **Paste the prompt below**, replacing `FOLDER` with your folder name from this table:

| Member | `FOLDER` |
| :-: | :--- |
| 1 | `member-01-identity` |
| 2 | `member-02-parcels-pricing` |
| 3 | `member-03-dispatch-delivery` |
| 4 | `member-04-merchants` |
| 5 | `member-05-tracking` |
| 6 | `member-06-customer-merchant-web` |
| 7 | `member-07-dispatcher-admin-web-gateway` |
| 8 | `member-08-events-messaging` |
| 10 | `member-10-ai-assistant-quality` |

---

## The prompt (copy everything in the box)

```text
You are reviewing my SwiftDrop module drafts for agreement with the shared DevOps and database rules.

READ THESE FILES FIRST (all on branch docs/module-spec-reviews):
1. AGENTS.md                         (project rules for AI assistants)
2. docs/drafts/README.md             (drafts process: only edit your own folder)
3. docs/drafts/member-09-devops-k8s/team-agreements-with-member-09.md   (THE RULES to check)
4. Every file in my folder: docs/drafts/FOLDER/   (including sub-folders such as doc8-local/, migrations-draft/)

CHECK my files against:
- every rule A1–A13 (section 1) and D1–D12 (section 2) of the agreements file,
- every K-item in section 4 that names my member number,
- my row in section 5.

Search my files especially for:
- ports: core-api 8080, tracking-api 8081, outbox-relay 8082, event-worker 8083, ai-assistant 8000, web 3000
- health paths: must be /health/live, /health/ready, /metrics  (flag /livez, /readyz, /healthz, /health)
- image tags ":latest" (not allowed) and Dockerfile USER (must be 10001:10001)
- Dockerfile HEALTHCHECK using curl/wget in slim or distroless images (they don't have them)
- Kubernetes secret names (postgres-secrets, postgres-migrator-secrets, rabbitmq-secrets, jwt-secrets,
  integrations-secrets, <service>-<module>-secrets) — flag postgres-credentials, *-owner-credentials, others
- DB URL: every service reads only the env var DATABASE_URL; in Kubernetes it comes from its own key in
  postgres-secrets (DATABASE_URL, TRACKING_/RELAY_/WORKER_/RECONCILER_DATABASE_URL, rule A5)
- DB login roles (swiftdrop_migrator, swiftdrop_app, swiftdrop_tracking_ro, swiftdrop_relay, swiftdrop_worker,
  swiftdrop_reconciler) — flag swiftdrop_owner, swiftdrop_core_api, swiftdrop_tracking, or apps using the postgres superuser
- "sslmode=require" (in-cluster must be sslmode=disable, rule D11)
- Compose file location (must be deploy/docker-compose.yml + root Makefile; flag root docker-compose.yml)
- migration files: naming must be 6-digit 000NNN_name.up.sql / .down.sql (rule D2, flag timestamps),
  and my first-wave files must use the exact numbers in rule D3 (000001–000011),
  stub tables that belong to other members (rule D4), seed SQL inside migrations folders (rule D12)
- /health/ready that checks optional/external services (object storage, LLM, email, webhooks) — rule A2
- env var names: VITE_API_BASE_URL, RABBITMQ_URL, S3_* / STORAGE_DRIVER, and any new variable
  that is not listed in docs/operations/environment-variables.md (rule A4)
- custom Prometheus metric names must start with swiftdrop_ (rule A11); request ID header X-Request-ID (A10)

OUTPUT exactly these 3 parts:

PART 1 — Table of problems:
| Rule/ID | File:line | What I wrote (exact quote) | What the agreement says | Suggested fix | Needs team vote? |

PART 2 — Rules I already follow (short list of IDs).

PART 3 — Draft reply for my row in the sign-off table (section 6):
"Yes" / "Yes with changes: ..." / "No: <rule ID> because ..."

RULES FOR YOU (the AI):
- Only suggest changes to files in MY folder (docs/drafts/FOLDER/). Never edit Member 9's files or
  any other member's folder.
- Do NOT commit, push, or open pull requests. I will review and commit myself.
- Quote exact lines with file:line. If you can't find something, write "not found" — never guess.
- Items still open (hosting, the A5 DB URL amendment, anything marked ⏳) are NOT yours to decide: report them only.
- If I think an agreement is wrong, help me write the reason (Part 3 "No: ..."); don't silently ignore it.
- If my folder has no doc-0X files yet, skip Part 1 and instead give me a checklist of the agreements
  I must follow while writing my 8 documents (doc-01 ... doc-08).
```

---

## Tool tips

| Tool | How |
| :--- | :--- |
| **Claude Code** | In the repo folder run `claude`, then paste the prompt. *Optional:* save the prompt (with your folder filled in) as `~/.claude/commands/m9-review.md` on your own computer; then type `/m9-review` any time. Don't commit it. |
| **Cursor** | Open Chat (Ctrl+L), paste the prompt, and `@`-mention the agreements file and your folder. |
| **GitHub Copilot Chat (VS Code)** | Open Copilot Chat, paste the prompt, and add the files with `#file:` (agreements file + your docs). |
| **ChatGPT / web AI** | Upload `team-agreements-with-member-09.md` and your 8 docs, then paste the prompt. |

> ⚠️ AI can be wrong. **Check every quoted line yourself** before changing anything. Never paste passwords or keys into an AI chat.

---

## After the review

1. **Fix your own docs** and push to `docs/module-spec-reviews` (**never** to `main`).
2. **Reply to Minidu only with disagreements**, using the rule IDs (e.g. *"D6: I prefer swiftdrop_core_api because…"*). Or write your answer in your row of the sign-off table (agreements §6).
3. Questions about a rule? Ask in the group chat with the rule ID, so everyone sees the answer.
