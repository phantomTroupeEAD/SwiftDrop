# Member 08 Design Report — Event-Driven Backend & Messaging

This directory contains the formal Design Documentation report section for **Member 08 (Event-Driven Backend & Messaging)** of the SwiftDrop Parcel Delivery & Courier Management System. Designed for submission in the system design report (due November 1), these documents detail the use case models, architecture proposals, integration patterns, sequence flows, state machines, data structures, and design pattern justifications in full alignment with the project requirements, approved SRS (`SwiftDrop_SRS_v1.md`), and repository standards (`AGENTS.md`).

---

## 1. Document Index

| Document | Topic | Description & Core Focus |
| :--- | :--- | :--- |
| [**`01-use-cases.md`**](01-use-cases.md) | Use Case Diagrams | High-level system use cases, Core API producer views, operator maintenance flows, and requirement mapping. |
| [**`02-architecture.md`**](02-architecture.md) | System & Subsystem Architecture | System context, container topology, layered package structure, vertical RabbitMQ broker routing, Kubernetes deployment, and ~250-word justification. |
| [**`03-integration-design.md`**](03-integration-design.md) | Integration Design & Rationale | Enterprise Integration Patterns (EIP) map, vertical delivery guarantee pipeline, failure recovery matrix, pattern comparison tables, and detailed architectural rationale. |
| [**`04-sequence-flows.md`**](04-sequence-flows.md) | Sequence Diagrams | Assignment-required courier status update flow, outbox relay publish loop, customer email delivery, signed merchant webhooks with SSRF defense, worker lease recovery, and DLQ replay. |
| [**`05-flowcharts-and-state-machines.md`**](05-flowcharts-and-state-machines.md) | Flowcharts & State Machines | Lifecycle state machines for outbox events and deliveries, retry decision tree with exact jitter schedule, HTTP status code classification, SSRF IP validation, reconciler flow, and graceful drain. |
| [**`06-data-model.md`**](06-data-model.md) | Data Model & Persistence | Entity-Relationship diagram, complete data dictionary for all five persistence tables adhering to sequential migrations, and architectural rationale for omitting foreign keys to `outbox_events`. |
| [**`07-patterns-and-justifications.md`**](07-patterns-and-justifications.md) | Patterns, Principles & Trade-offs | SOLID and EIP pattern catalog, vertical core Go interface structure (`classDiagram`), architectural trade-offs matrix (DEC-01 to DEC-06), and documented system limitations. |

---

## 2. Complete Figure Index

| Figure | Title | Document Location |
| :--- | :--- | :--- |
| **Figure 1.1** | Overall Messaging System Use Cases | [`01-use-cases.md`](01-use-cases.md#figure-11--overall-messaging-system-use-cases) |
| **Figure 1.2** | Core API Producer View | [`01-use-cases.md`](01-use-cases.md#figure-12--core-api-producer-view) |
| **Figure 1.3** | Platform Operator & Maintenance View | [`01-use-cases.md`](01-use-cases.md#figure-13--platform-operator--maintenance-view) |
| **Figure 2.1** | System Context | [`02-architecture.md`](02-architecture.md#figure-21--system-context) |
| **Figure 2.2** | Container View | [`02-architecture.md`](02-architecture.md#figure-22--container-view) |
| **Figure 2.3** | Layered Internal Package Structure | [`02-architecture.md`](02-architecture.md#figure-23--layered-internal-package-structure) |
| **Figure 2.4** | Messaging Broker Topology | [`02-architecture.md`](02-architecture.md#figure-24--messaging-broker-topology) |
| **Figure 2.5** | Kubernetes Deployment View | [`02-architecture.md`](02-architecture.md#figure-25--kubernetes-deployment-view) |
| **Figure 3.1** | Enterprise Integration Patterns Map | [`03-integration-design.md`](03-integration-design.md#figure-31--enterprise-integration-patterns-map) |
| **Figure 3.2** | Delivery Guarantee Chain | [`03-integration-design.md`](03-integration-design.md#figure-32--delivery-guarantee-chain) |
| **Figure 4.1** | Courier Status Update Triggering Notifications *(Assignment Required Flow)* | [`04-sequence-flows.md`](04-sequence-flows.md#figure-41--courier-status-update-triggering-notifications-assignment-required-flow) |
| **Figure 4.2** | Outbox Relay Publication Loop | [`04-sequence-flows.md`](04-sequence-flows.md#figure-42--outbox-relay-publication-loop) |
| **Figure 4.3** | Customer Email Notification Delivery Flow | [`04-sequence-flows.md`](04-sequence-flows.md#figure-43--customer-email-notification-delivery-flow) |
| **Figure 4.4** | Merchant Webhook Delivery & SSRF Defense Flow | [`04-sequence-flows.md`](04-sequence-flows.md#figure-44--merchant-webhook-delivery--ssrf-defense-flow) |
| **Figure 4.5** | Worker Crash and Automated Lease Recovery Flow | [`04-sequence-flows.md`](04-sequence-flows.md#figure-45--worker-crash-and-automated-lease-recovery-flow) |
| **Figure 4.6** | Dead-Letter Queue (DLQ) Routing & Operator Replay Flow | [`04-sequence-flows.md`](04-sequence-flows.md#figure-46--dead-letter-queue-dlq-routing--operator-replay-flow) |
| **Figure 5.1** | Outbox Event Lifecycle State Machine | [`05-flowcharts-and-state-machines.md`](05-flowcharts-and-state-machines.md#figure-51--outbox-event-lifecycle-state-machine) |
| **Figure 5.2** | Delivery Task Lifecycle State Machine | [`05-flowcharts-and-state-machines.md`](05-flowcharts-and-state-machines.md#figure-52--delivery-task-lifecycle-state-machine) |
| **Figure 5.3** | Delivery Retry Decision Flowchart | [`05-flowcharts-and-state-machines.md`](05-flowcharts-and-state-machines.md#figure-53--delivery-retry-decision-flowchart) |
| **Figure 5.4** | Webhook Response Classification Flowchart | [`05-flowcharts-and-state-machines.md`](05-flowcharts-and-state-machines.md#figure-54--webhook-response-classification-flowchart) |
| **Figure 5.5** | SSRF Defense Flowchart (`SafeDialContext`) | [`05-flowcharts-and-state-machines.md`](05-flowcharts-and-state-machines.md#figure-55--ssrf-defense-flowchart-safedialcontext) |
| **Figure 5.6** | Worker Message Ingestion Flowchart | [`05-flowcharts-and-state-machines.md`](05-flowcharts-and-state-machines.md#figure-56--worker-message-ingestion-flowchart) |
| **Figure 5.7** | Reconciler Audit & Maintenance Flowchart | [`05-flowcharts-and-state-machines.md`](05-flowcharts-and-state-machines.md#figure-57--reconciler-audit--maintenance-flowchart) |
| **Figure 5.8** | Graceful Startup & Shutdown Flowchart | [`05-flowcharts-and-state-machines.md`](05-flowcharts-and-state-machines.md#figure-58--graceful-startup--shutdown-flowchart) |
| **Figure 6.1** | Entity-Relationship Diagram (Messaging Subsystem) | [`06-data-model.md`](06-data-model.md#figure-61--entity-relationship-diagram-messaging-subsystem) |
| **Figure 7.1** | Core Interfaces and Class Structure | [`07-patterns-and-justifications.md`](07-patterns-and-justifications.md#figure-71--core-interfaces-and-class-structure) |

---

## 3. Guide to PDF Export

To compile these Markdown files and Mermaid diagrams into the final design report PDF:

### Option A: Mermaid CLI Pre-Rendering (Recommended)
1. Install Mermaid CLI globally:
   ```bash
   npm install -g @mermaid-js/mermaid-cli
   ```
2. Render diagrams directly to SVG or PNG:
   ```bash
   mmdc -i report/04-sequence-flows.md -o report/images/04.svg
   ```
3. Use a markdown-to-pdf generator (such as `pandoc` or `md-to-pdf`) to compile the documents with embedded vector images:
   ```bash
   pandoc report/*.md -o member-08-design-report.pdf --pdf-engine=xelatex
   ```

### Option B: VS Code Extensions
1. Install the **Markdown Preview Mermaid Support** extension (by Matt Bierner).
2. Install the **Markdown PDF** extension (by yzane).
3. Open any file in `report/`, right-click in the editor, and select **Markdown PDF: Export (pdf)**.
4. *Formatting check:* All wide diagrams have been oriented vertically (`flowchart TD`, `direction TB`) and message texts streamlined to ensure comfortable readability on standard A4 portrait pages without shrunken text.

---

## 4. Open Points for Cross-Member Coordination (Meeting 2)

1. **FR-REC-003 Scope Alignment (DEC-04):**  
   Detecting a parcel missing expected status-history or assignment records requires read access to Member 02's `parcels` table. In compliance with Layer 1 domain boundaries, Member 08 marks FR-REC-003 as *partially covered* until Member 02 exposes an approved read-only view.
2. **PostgreSQL Roles Provisioning (Agreement D6):**  
   Provisioning the dedicated `swiftdrop_reconciler` database role with scoped `UPDATE`/`DELETE` permissions on `outbox_events` is currently pending confirmation by Member 09.
3. **DLQ Replay Job Secret Configuration:**  
   The standalone replay Job (`reconcile --replay-dlq`) must mount both `outbox-reconciler-secrets` (PostgreSQL credentials to stage events) and `rabbitmq-secrets` (AMQP credentials to consume from `swiftdrop.dlq`), resolving the missing broker credential gap in earlier draft manifests.
4. **Domain Event Payload Enrichment (CR-M02-01 & CR-M03-02):**  
   Confirm that Member 02 and Member 03 include the minimal `notify` contact block (`recipient_name`, `recipient_email`) and `merchant_id` in `parcel.*` domain event payloads to ensure zero-lookup notification delivery.
