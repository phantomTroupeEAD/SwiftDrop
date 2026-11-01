# Assignment Guideline: Parcel Delivery & Courier Management System

> **Coursework / Module:** Enterprise Application Development  
> **Weightage:** 20 marks  
> **Group Size:** 10 members  
> **Source Document:** `Group Assignment.pdf` (Transcribed to Markdown)

---

## 1. Objective

This assignment tests your knowledge of enterprise application development by designing and implementing a fully functional **Parcel Delivery & Courier Management System**. The application should let customers and merchants send and track parcels, let dispatchers assign deliveries, and let couriers update delivery progress in real time. It should also integrate with external systems, as enterprise applications rarely operate in isolation. You will design, implement, secure, containerize, test, and deploy a modern enterprise application.

---

## 2. Scenario

**SwiftDrop** is a growing courier company operating island-wide. It currently manages bookings through phone calls and spreadsheets, and customers have no visibility into where their parcels are. SwiftDrop also wants partner online stores (merchants) to create delivery orders automatically from their own e-commerce platforms. Your team has been hired to build the new platform.

---

## 3. Assignment Overview & Functional Scope

### 3.1 Customer Functionality
- **Secure Signup & Login:** Including "Login with Google" (OAuth 2.0 / OpenID Connect).
- **Delivery Request Creation:** Specify pickup address, drop-off address, and parcel size/weight.
- **Delivery Fee Estimation:** View an estimated delivery fee before confirming booking.
- **Real-Time Parcel Tracking:** Track status across lifecycle stages:
  $$\text{Created} \longrightarrow \text{Picked Up} \longrightarrow \text{In Transit} \longrightarrow \text{Out for Delivery} \longrightarrow \text{Delivered / Failed}$$
- **Delivery History & Receipts:** View past delivery history and download delivery receipts.
- **Public Tracking:** Mobile-friendly tracking page accessible via a public tracking number.

### 3.2 Merchant (Business) Functionality
- **Merchant Registration:** Registration with an administrative approval step.
- **Automated Order API:** Create delivery orders via a secured API using API keys or OAuth 2.0 client credentials.
- **Webhook Updates:** Receive automated status updates through webhooks when parcel status changes (with retry on failure).
- **Merchant Dashboard:** Dashboard displaying all orders and their current delivery states.

### 3.3 Courier Functionality
- **Login & Authentication:** Dedicated courier access.
- **Daily Deliveries View:** View assigned deliveries for the day.
- **Status Updates & Proof of Delivery:** Update parcel status at each stage with delivery notes or proof (e.g., recipient name or photo upload).
- **Failed Deliveries:** Mark failed delivery attempts with a mandatory reason.

### 3.4 Dispatcher / Admin Functionality
- **Parcel Assignment:** View all unassigned parcels and assign them to couriers.
- **Workload Monitoring:** Monitor courier workloads (number of active deliveries per courier).
- **System Management:** Manage users, merchants, delivery zones, and tariff/pricing rules.
- **Operational Reports:** Basic reports (deliveries per day, success/failure rates).

### 3.5 System Requirements
- **Real-Time Updates:** WebSockets, Server-Sent Events (SSE), or polling.
- **Role-Based Access Control (RBAC):** Strict isolation for Customer, Merchant, Courier, and Dispatcher/Admin.
- **Authentication:** JWT-based authentication for application users, plus OAuth 2.0 for social login and merchant API access.
- **Asynchronous Integration Pattern:** Publish/subscribe via a message broker (RabbitMQ or Kafka) for parcel status change events triggering notifications and merchant webhooks.
- **Notification Service:** Email or mock SMS service consuming status events.
- **Containerized Deployment:** Dockerized frontend, backend services, database, and message broker orchestrated with Docker Compose.
- **Security & Privacy:** Secure handling of personal data (addresses, phone numbers), input validation, hashed passwords, zero sensitive data in error messages or logs.

---

## 4. Assessment & Grading Criteria

### 1. Design Documentation (20 marks)
- **Use Case Diagrams:** Interactions for customer, merchant, courier, and dispatcher.
- **ER Diagram:** Covering users, roles, merchants, parcels, status history, couriers, zones, and pricing.
- **Architecture Proposal:** Modular monolith, microservices, or layered architecture with BFF and clear justification (presentation, business logic, and data layers).
- **Integration Design:** Clear documentation of integration patterns (request/reply, publish/subscribe, message broker) and rationale.
- **Sequence / Activity Diagrams:** Key flows including:
  1. Creating a delivery
  2. Merchant API order creation
  3. Courier status update triggering notifications
  4. OAuth 2.0 login flow
- **Design Patterns & Principles:** SOLID, DRY, Repository pattern, Strategy pattern (pricing), Observer pattern (events).

### 2. Implementation (50 marks)
- **Frontend:** Responsive UI using React, Angular, or Vue.
- **Backend:** Core logic using a suitable framework (Spring Boot, Node.js/Express, Go, Django, etc.).
- **APIs:** Secure REST or GraphQL APIs with proper versioning, input validation, and centralized error handling.
- **Integration:** Message broker for status events, notification consumer, and merchant webhooks with retry on failure.
- **Real-Time Updates:** Live parcel tracking via WebSockets / SSE.
- **Code Quality:** Meaningful exception handling, zero duplicated logic, configuration via environment variables (no hardcoded secrets).
- **Containerization:** Dockerize all services; use `docker-compose` for local orchestration.

### 3. Testing (15 marks)
- **Unit & Integration Tests:** For backend services, including at least one test verifying the asynchronous event/integration flow.
- **Authorization Tests:** Strict verification of authorization rules (e.g., couriers cannot access another courier's deliveries; tenant isolation).
- **Code Coverage:** Measurable code coverage with reports included in the final submission.

### 4. Presentation / Demo (15 marks)
- **Live Demo of Core Features:**
  - Customer login (including Google OAuth), delivery creation, and real-time tracking.
  - Merchant creating an order via API and receiving a webhook.
  - Dispatcher assigning a parcel; courier updating status.
  - Notification triggered by a status change.
- **Architecture Walkthrough:** Diagrams, integration choices, and security decisions.
- **Individual Contribution:** Each member must be able to explain the part they contributed.

---

## 5. Bonus Marks (+10 marks total)

### Bonus 1: Kubernetes Deployment (+5 marks)
- Deploy the full system (frontend, backend services, database, and broker) on Kubernetes.
- Use ConfigMaps, Secrets, Service definitions, and Ingress.
- Helm charts are a plus.

### Bonus 2: AI Delivery Assistant (+5 marks)
- Integrate a Generative AI chatbot capable of answering queries such as:
  - *"Where is my parcel?"*
  - *"How much to send 2 kg to Kandy?"*
- Powered by calling backend APIs (tool calling / function calling).
- Exposing these capabilities as **MCP (Model Context Protocol)** tools is encouraged.

---

## 6. Submission Requirements

1. **Source Code Repository Link:** Including a comprehensive `README` with local setup instructions.
2. **Design Documentation (PDF):** Containing all required architectural and design diagrams.
3. **Test Reports & Coverage Summary:** Documenting automated test execution and coverage statistics.
4. **Contribution Sheet:** Explicitly listing each member's specific responsibilities and deliverables.
