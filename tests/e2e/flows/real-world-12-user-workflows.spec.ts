/**
 * Canonical alias for real-system-12-user-workflows.spec.ts
 *
 * Implements the 12-user cross-module real-world workflow automation suite across:
 * - 3 Customers (existing + new sign-up via real Mailpit OTP + heavy shipment)
 * - 2 Merchants (API keys, webhooks, commercial orders, tenant boundary isolation)
 * - 3 Couriers (mobile shifts, delivery runs, digital POD signature, failure attempt exception, relief delivery)
 * - 2 Dispatchers (capacity allocation, routing, exception triage, rescheduling & re-allocation)
 * - 1 Admin (delivery zone auditing, operational metrics, tamper-proof audit log inspection)
 * - 1 Public Guest (public tracking lookup, SEC-009 PII suppression, SSE live projection stream)
 */
export * from './real-system-12-user-workflows.spec';
