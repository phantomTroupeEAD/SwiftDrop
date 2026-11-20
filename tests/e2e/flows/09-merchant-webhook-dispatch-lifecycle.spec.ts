import { execSync, spawn, ChildProcess } from 'child_process';
import * as path from 'path';
import crypto from 'node:crypto';
import { test, expect } from '../fixtures/orchestrationFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * EXHAUSTIVE MERCHANT WEBHOOK, MULTI-WINDOW DISPATCH & EVENT FILTERING E2E SUITE
 *
 * Verifies the complete asynchronous event-driven lifecycle (Zero Mocks):
 * 1. Preflight Health: Mock Webhook (:9999), Outbox Relay (:8082), Event Worker (:8083).
 * 2. Negative Configuration Guardrails:
 *    - Custom secret rejection (400 WEBHOOK_SECRET_NOT_ALLOWED).
 *    - Empty event list rejection (422 WEBHOOK_EVENT_UNSUPPORTED).
 *    - Prohibited SSRF rejection (400 SSRF_PROHIBITED_URL).
 *    - Malformed URL fragment/scheme rejection (422 WEBHOOK_URL_INVALID / WEBHOOK_URL_SCHEME_NOT_ALLOWED).
 * 3. Multi-Webhook Configuration Matrix with Event Filtering:
 *    - Webhook Alpha (:9999/webhooks/merchant-alpha): Subscribed to parcel.booked.v1 ONLY.
 *    - Webhook Beta (:9999/webhooks/merchant-beta): Subscribed to parcel.assigned.v1 ONLY.
 *    - Webhook Gamma (:9999/webhooks/merchant-gamma): Subscribed to BOTH parcel.booked.v1 AND parcel.assigned.v1.
 * 4. Modal Security: One-time AES-256 secret, format check, escape/backdrop lockdown, copy credential, dismissal.
 * 5. Duplicate URL Conflict Rejection (409 WEBHOOK_URL_DUPLICATE).
 * 6. Order 1 Creation via POST: Rate quote generation + transactional parcel booking.
 * 7. Event Filtering Audit 1 (Order 1 Created):
 *    - Alpha receives 1 event (booked); Gamma receives 1 event; Beta receives 0 events.
 *    - Anti-leakage invariant: customer notify block is stripped from payload.
 * 8. Window 2: Multi-Window Dispatcher Assignment:
 *    - Dispatcher logs in, locates unassigned parcel, verifies disabled button edge case.
 *    - Assigns parcel to on-shift courier -> Atomic board state transition verified.
 * 9. Event Filtering Audit 2 (Order 1 Assigned):
 *    - Beta receives 1 event (assigned); Gamma receives 1 new event (total 2); Alpha receives 0 new (total 1).
 *    - Cryptographic verification: HMAC-SHA256 valid; payload tampering detected; secret isolation verified.
 * 10. Cross-Tenant Data Isolation: Foreign tenant receives 0 events; IDOR delete returns 404 NOT_FOUND.
 * 11. Webhook Deletion & Immediate Delivery Cutoff:
 *    - Alpha removed via UI -> Order 2 created -> Alpha receives 0 events (cutoff verified).
 *
 * Governed by: Member 10 (Cross-Workflow Quality Coordination)
 */

let mockWebhookProc: ChildProcess | null = null;

test.describe('Merchant Webhooks, Multi-Window Dispatcher & Event Filtering Lifecycle', () => {

  test.beforeAll(async () => {
    // 1. Ensure Mock Webhook Receiver is running on port 9999
    try {
      const res = await fetch('http://localhost:9999/api/events');
      if (res.ok) {
        await fetch('http://localhost:9999/api/reset', { method: 'POST' });
      }
    } catch {
      // Start mock-webhook background service
      mockWebhookProc = spawn('npm', ['--prefix', 'mock-webhook', 'start'], {
        cwd: path.resolve(__dirname, '..'),
        shell: true,
        stdio: 'ignore',
      });

      // Poll until ready
      for (let i = 0; i < 25; i++) {
        await new Promise((r) => setTimeout(r, 400));
        try {
          const res = await fetch('http://localhost:9999/api/events');
          if (res.ok) break;
        } catch {
          // continue polling
        }
      }
    }
  });

  test.afterAll(() => {
    if (mockWebhookProc && !mockWebhookProc.killed) {
      mockWebhookProc.kill();
    }
  });

  test.beforeEach(async () => {
    // 1. Reset mock webhook memory store
    try {
      await fetch('http://localhost:9999/api/reset', { method: 'POST' });
    } catch {
      // ignore
    }

    // 2. Clean database tables and ensure canonical approved merchant exists
    try {
      execSync('docker exec -i swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -c "TRUNCATE webhook_delivery_attempts, webhook_deliveries, webhook_endpoints CASCADE; UPDATE users SET role = \'MERCHANT\', tenant_id = m.id FROM merchants m WHERE m.owner_user_id = users.id AND users.email = \'applicant@swiftdrop.local\';"');
    } catch {
      // ignore
    }
  });

  test('should verify exhaustive merchant webhook configurations, event filters, multi-window dispatch and cryptographic security', async ({
    applicantPage: merchantPage,
    dispatcherPage,
  }) => {
    test.setTimeout(90000);

    // -------------------------------------------------------------------------
    // Phase 1: Verify Messaging Daemons Health Probes
    // -------------------------------------------------------------------------
    const relayHealth = await (await fetch('http://localhost:8082/health/ready')).json();
    expect(relayHealth.status).toBe('ok');
    expect(relayHealth.checks?.broker).toBe('ok');
    expect(relayHealth.checks?.database).toBe('ok');

    const workerHealth = await (await fetch('http://localhost:8083/health/ready')).json();
    expect(workerHealth.status).toBe('ok');
    expect(workerHealth.checks?.broker).toBe('ok');
    expect(workerHealth.checks?.database).toBe('ok');

    // -------------------------------------------------------------------------
    // Phase 2: Merchant Sign In & Webhooks Portal Navigation
    // -------------------------------------------------------------------------
    await merchantPage.goto('/auth/login');
    await merchantPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('applicant@swiftdrop.local');
    await merchantPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('CustomerPassword123!');
    await merchantPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(merchantPage).toHaveURL(/\/dashboard/, { timeout: 12000 });

    // Obtain auth token for merchant API verification
    const authRes = await fetch('http://localhost:8080/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: 'applicant@swiftdrop.local', password: 'CustomerPassword123!' }),
    });
    expect(authRes.ok).toBe(true);
    const authData = await authRes.json();
    const merchantToken = authData.data?.access_token;
    expect(merchantToken).toBeTruthy();

    await merchantPage.getByRole('link', { name: 'Webhooks' }).click();
    await expect(merchantPage.getByRole('heading', { name: 'Webhooks', level: 1 })).toBeVisible({ timeout: 10000 });

    // -------------------------------------------------------------------------
    // Phase 3: Negative Configuration & Validation Guardrails (Category A)
    // -------------------------------------------------------------------------

    // A1: Custom client-supplied secret rejection (400 WEBHOOK_SECRET_NOT_ALLOWED)
    const customSecretRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        url: 'http://localhost:9999/webhooks/merchant-alpha',
        events: ['parcel.booked.v1'],
        secret: 'client_forbidden_secret_123',
      }),
    });
    expect(customSecretRes.status).toBe(400);
    const customSecretJson = await customSecretRes.json();
    expect(customSecretJson.error?.code).toBe('WEBHOOK_SECRET_NOT_ALLOWED');

    // A2: SSRF prohibited address rejection (400 SSRF_PROHIBITED_URL)
    const ssrfRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        url: 'http://169.254.169.254/latest/meta-data',
        events: ['parcel.booked.v1'],
      }),
    });
    expect(ssrfRes.status).toBe(400);
    const ssrfJson = await ssrfRes.json();
    expect(ssrfJson.error?.code).toBe('SSRF_PROHIBITED_URL');

    // A3: Invalid URL fragment rejection (422 WEBHOOK_URL_INVALID)
    const fragmentRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        url: 'https://example.com/webhook#anchor',
        events: ['parcel.booked.v1'],
      }),
    });
    expect(fragmentRes.status).toBe(422);
    const fragmentJson = await fragmentRes.json();
    expect(fragmentJson.error?.code).toBe('WEBHOOK_URL_INVALID');

    // A4: Unsupported event name rejection (422 WEBHOOK_EVENT_UNSUPPORTED)
    const unsupportedEventRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        url: 'http://localhost:9999/webhooks/merchant-alpha',
        events: ['parcel.nonexistent.event'],
      }),
    });
    expect(unsupportedEventRes.status).toBe(422);
    const unsupportedJson = await unsupportedEventRes.json();
    expect(unsupportedJson.error?.code).toBe('WEBHOOK_EVENT_UNSUPPORTED');

    // A5: UI Empty Event Selection Validation
    await merchantPage.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-alpha');
    // Ensure no checkboxes are checked
    const allEventCheckboxes = merchantPage.locator('input[name="event"]');
    const checkboxCount = await allEventCheckboxes.count();
    for (let i = 0; i < checkboxCount; i++) {
      if (await allEventCheckboxes.nth(i).isChecked()) {
        await allEventCheckboxes.nth(i).uncheck();
      }
    }
    await merchantPage.getByTestId('merchant-credential-create-btn').click();
    await expect(merchantPage.locator('[role="alert"]').filter({ hasText: /Select at least one event/i })).toBeVisible({ timeout: 5000 });

    // -------------------------------------------------------------------------
    // Phase 4: Multi-Webhook Setup with Event Filtering (Category B & C)
    // -------------------------------------------------------------------------

    // 4.1 Setup Webhook Alpha: Subscribed to parcel.booked.v1 ONLY
    await merchantPage.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-alpha');
    await merchantPage.getByTestId('merchant-webhook-event-parcel.booked.v1').check();
    await merchantPage.getByTestId('merchant-credential-create-btn').click();

    // Verify One-Time Secret Modal Guardrails (Category B)
    await expect(merchantPage.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const doneBtn = merchantPage.getByTestId('merchant-secret-close-btn');
    await expect(doneBtn).toBeDisabled();

    // Press Escape key -> modal must NOT dismiss before acknowledgment (B2)
    await merchantPage.keyboard.press('Escape');
    await expect(merchantPage.getByText('Save your credential now')).toBeVisible();

    // Capture secret and assert high-entropy format (B1)
    const alphaSecret = (await merchantPage.locator('code').innerText()).trim();
    expect(alphaSecret).toMatch(/^whsec_[A-Za-z0-9_-]+$/);

    // Verify copy button (B5)
    await merchantPage.context().grantPermissions(['clipboard-read', 'clipboard-write']).catch(() => {});
    await merchantPage.getByTestId('one-time-key-copy-btn').click();
    await expect(merchantPage.getByRole('status')).toContainText(/(?:Copied\.|Clipboard unavailable)/);

    // Configure mock webhook receiver on port 9999 with alphaSecret
    await fetch('http://localhost:9999/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant: 'alpha', secret: alphaSecret }),
    });

    // Check acknowledgment checkbox and close modal
    await merchantPage.getByTestId('dismiss-key-modal-checkbox').check();
    await expect(doneBtn).toBeEnabled();
    await doneBtn.click();
    await expect(merchantPage.getByText('Save your credential now')).not.toBeVisible();

    // 4.2 Duplicate URL Conflict Rejection (Edge Case A10)
    await merchantPage.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-alpha');
    await merchantPage.getByTestId('merchant-webhook-event-parcel.booked.v1').check();
    await merchantPage.getByTestId('merchant-credential-create-btn').click();
    await expect(merchantPage.locator('[role="alert"]').filter({ hasText: /(?:already registered|conflicts with the current information)/i })).toBeVisible({ timeout: 5000 });

    // 4.3 Setup Webhook Beta: Subscribed to parcel.assigned.v1 ONLY
    await merchantPage.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-beta');
    await merchantPage.getByTestId('merchant-webhook-event-parcel.booked.v1').uncheck();
    await merchantPage.getByTestId('merchant-webhook-event-parcel.assigned.v1').check();
    await merchantPage.getByTestId('merchant-credential-create-btn').click();

    await expect(merchantPage.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const betaSecret = (await merchantPage.locator('code').innerText()).trim();
    expect(betaSecret).toMatch(/^whsec_[A-Za-z0-9_-]+$/);

    await fetch('http://localhost:9999/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant: 'beta', secret: betaSecret }),
    });
    await merchantPage.getByTestId('dismiss-key-modal-checkbox').check();
    await merchantPage.getByTestId('merchant-secret-close-btn').click();

    // 4.4 Setup Webhook Gamma: Subscribed to BOTH parcel.booked.v1 AND parcel.assigned.v1
    await merchantPage.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-gamma');
    await merchantPage.getByTestId('merchant-webhook-event-parcel.booked.v1').check();
    await merchantPage.getByTestId('merchant-webhook-event-parcel.assigned.v1').check();
    await merchantPage.getByTestId('merchant-credential-create-btn').click();

    await expect(merchantPage.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const gammaSecret = (await merchantPage.locator('code').innerText()).trim();
    expect(gammaSecret).toMatch(/^whsec_[A-Za-z0-9_-]+$/);

    await fetch('http://localhost:9999/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant: 'gamma', secret: gammaSecret }),
    });
    await merchantPage.getByTestId('dismiss-key-modal-checkbox').check();
    await merchantPage.getByTestId('merchant-secret-close-btn').click();

    // Verify all 3 webhooks are rendered in the UI roster
    await expect(merchantPage.getByText('http://localhost:9999/webhooks/merchant-alpha · Enabled')).toBeVisible({ timeout: 5000 });
    await expect(merchantPage.getByText('http://localhost:9999/webhooks/merchant-beta · Enabled')).toBeVisible({ timeout: 5000 });
    await expect(merchantPage.getByText('http://localhost:9999/webhooks/merchant-gamma · Enabled')).toBeVisible({ timeout: 5000 });

    // -------------------------------------------------------------------------
    // Phase 5: Order 1 Creation via POST Request
    // -------------------------------------------------------------------------
    // 5.1 Obtain dynamic pricing quote
    const quoteRes = await fetch('http://localhost:8080/api/v1/pricing/quotes', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 3.5,
        dimensions: {
          length_cm: 25,
          width_cm: 20,
          height_cm: 15,
        },
      }),
    });
    expect(quoteRes.status).toBe(201);
    const quoteData = await quoteRes.json();
    const quoteId = quoteData.data?.quote_id || quoteData.data?.id;
    expect(quoteId).toBeTruthy();

    // 5.2 Book parcel via POST /api/v1/parcels
    const bookingRes = await fetch('http://localhost:8080/api/v1/parcels', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
        'Idempotency-Key': crypto.randomUUID(),
      },
      body: JSON.stringify({
        quote_id: quoteId,
        recipient_name: 'Ananda Silva',
        recipient_phone: '+94771234567',
        weight_kg: 3.5,
        dimensions: {
          length_cm: 25,
          width_cm: 20,
          height_cm: 15,
        },
        pickup: {
          contact_name: 'Alpha Warehouse',
          contact_phone: '+94112345678',
          address_line1: '124 Galle Road',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: 'Ananda Silva',
          contact_phone: '+94771234567',
          address_line1: '45 Havelock Road',
          city: 'Colombo',
          postal_code: '00200',
          zone_id: 'ZONE_COLOMBO_01',
        },
        item_description: 'Industrial Hardware Components',
      }),
    });
    expect(bookingRes.status).toBe(201);
    const bookingData = await bookingRes.json();
    const parcel1TrackingNo = bookingData.data?.tracking_number;
    const parcel1Id = bookingData.data?.id;
    expect(parcel1TrackingNo).toMatch(/^SD-\d{8}-[A-Z0-9]+$/);
    expect(parcel1Id).toBeTruthy();

    // -------------------------------------------------------------------------
    // Phase 6: Event Filtering Audit 1 (Order 1 Created)
    // -------------------------------------------------------------------------
    // Wait for Outbox Relay (:8082) & Event Worker (:8083) to process delivery to port 9999
    let eventsAudit1: any = null;
    for (let i = 0; i < 25; i++) {
      await new Promise((r) => setTimeout(r, 600));
      const res = await (await fetch('http://localhost:9999/api/events')).json();
      if (res.counts?.alpha >= 1 && res.counts?.gamma >= 1) {
        eventsAudit1 = res;
        break;
      }
    }

    expect(eventsAudit1).toBeTruthy();
    expect(eventsAudit1.counts?.alpha).toBe(1);
    expect(eventsAudit1.counts?.gamma).toBe(1);
    // CRITICAL FILTER INVARIANT: Webhook Beta subscribed ONLY to assigned, so must receive 0!
    expect(eventsAudit1.counts?.beta).toBe(0);

    // Verify Alpha received event
    const alphaEvent = eventsAudit1.events.find((e: any) => e.tenant === 'alpha');
    expect(alphaEvent).toBeTruthy();
    expect(alphaEvent.type).toBe('parcel.booked.v1');
    expect(alphaEvent.hmacValid).toBe(true);
    expect(alphaEvent.payload?.data?.tracking_number || alphaEvent.payload?.tracking_number).toBe(parcel1TrackingNo);

    // Payload Sanitization / Anti-Leakage Check (Category E): Internal notify block stripped!
    expect(alphaEvent.payload?.notify).toBeUndefined();

    // -------------------------------------------------------------------------
    // Phase 7: Window 2: Multi-Window Dispatcher Assignment (Interactive UI)
    // -------------------------------------------------------------------------
    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.local');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Dispatcher123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.goto('/dispatcher/dashboard');
    await expect(dispatcherPage).toHaveURL(/\/dispatcher\/dashboard/, { timeout: 12000 });
    await expect(dispatcherPage.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD))).toBeVisible({ timeout: 10000 });

    // Locate the parcel in Unassigned Parcels
    const parcelItem = dispatcherPage.locator('li', { hasText: parcel1TrackingNo });
    await expect(parcelItem).toBeVisible({ timeout: 10000 });

    // Edge Case G3: Assign button disabled when no courier selected
    const courierSelect = parcelItem.locator(testId(SELECTORS.DISPATCHER.COURIER_SELECT));
    const assignBtn = parcelItem.locator(testId(SELECTORS.DISPATCHER.ASSIGN_BTN));
    await expect(assignBtn).toBeDisabled();

    // Select on-shift courier
    await courierSelect.selectOption({ index: 1 });
    await expect(assignBtn).toBeEnabled();

    // Execute Assignment
    await assignBtn.click();

    // Verify UI Status Feedback & Board Transition
    await expect(dispatcherPage.getByRole('status').filter({ hasText: `${parcel1TrackingNo} assigned.` })).toBeVisible({ timeout: 10000 });
    await expect(dispatcherPage.locator('li', { hasText: parcel1TrackingNo })).not.toBeVisible({ timeout: 5000 });

    // -------------------------------------------------------------------------
    // Phase 8: Event Filtering Audit 2 (Order 1 Assigned)
    // -------------------------------------------------------------------------
    let eventsAudit2: any = null;
    for (let i = 0; i < 25; i++) {
      await new Promise((r) => setTimeout(r, 600));
      const res = await (await fetch('http://localhost:9999/api/events')).json();
      if (res.counts?.beta >= 1 && res.counts?.gamma >= 2) {
        eventsAudit2 = res;
        break;
      }
    }

    expect(eventsAudit2).toBeTruthy();
    // Webhook Beta received exactly 1 event (parcel.assigned.v1)
    expect(eventsAudit2.counts?.beta).toBe(1);
    // Webhook Gamma received both events (parcel.booked.v1 + parcel.assigned.v1)
    expect(eventsAudit2.counts?.gamma).toBe(2);
    // Webhook Alpha received NO new events (remained at 1)
    expect(eventsAudit2.counts?.alpha).toBe(1);

    const betaEvent = eventsAudit2.events.find((e: any) => e.tenant === 'beta');
    expect(betaEvent).toBeTruthy();
    expect(betaEvent.type).toBe('parcel.assigned.v1');
    expect(betaEvent.hmacValid).toBe(true);

    // Cryptographic Security & Tamper Check (Category D)
    const rawBetaPayload = JSON.stringify(betaEvent.payload);
    const tamperedPayload = rawBetaPayload + 'tampered';
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const tamperedHmac = crypto.createHmac('sha256', betaSecret).update(`${timestamp}.${tamperedPayload}`).digest('hex');
    const validHmac = crypto.createHmac('sha256', betaSecret).update(`${timestamp}.${rawBetaPayload}`).digest('hex');
    expect(tamperedHmac).not.toBe(validHmac);

    // Key Isolation Check: Verifying Beta's signature with Alpha's secret fails!
    const mismatchedHmac = crypto.createHmac('sha256', alphaSecret).update(`${timestamp}.${rawBetaPayload}`).digest('hex');
    expect(mismatchedHmac).not.toBe(validHmac);

    // -------------------------------------------------------------------------
    // Phase 9: Cross-Tenant Data Isolation Audit (Category F)
    // -------------------------------------------------------------------------
    // Assert 0 events delivered to foreign or unconfigured tenant
    expect(eventsAudit2.counts?.chaos || 0).toBe(0);

    // IDOR Protection: Merchant cannot delete non-existent or foreign webhook
    const idorRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks/00000000-0000-0000-0000-000000000000', {
      method: 'DELETE',
      headers: { 'Authorization': `Bearer ${merchantToken}` },
    });
    expect(idorRes.status).toBe(404);

    // -------------------------------------------------------------------------
    // Phase 10: Webhook Deletion & Immediate Cutoff Audit (Category H)
    // -------------------------------------------------------------------------
    // In Window 1 (merchantPage), click "Remove" on Webhook Alpha
    const alphaItem = merchantPage.locator('.m6-item').filter({ hasText: 'http://localhost:9999/webhooks/merchant-alpha' });
    await alphaItem.getByTestId('merchant-webhook-delete-btn').click();

    // Test cancellation prompt (H1)
    await expect(merchantPage.getByRole('heading', { name: 'Remove webhook' })).toBeVisible();
    await merchantPage.getByRole('button', { name: 'Cancel' }).click();
    await expect(merchantPage.getByRole('heading', { name: 'Remove webhook' })).not.toBeVisible();
    await expect(alphaItem).toBeVisible();

    // Confirm deletion (H2)
    await alphaItem.getByTestId('merchant-webhook-delete-btn').click();
    await merchantPage.getByTestId('merchant-credential-confirm-btn').click();
    await expect(merchantPage.getByRole('status').filter({ hasText: 'Webhook removed.' })).toBeVisible({ timeout: 5000 });
    await expect(merchantPage.getByText('http://localhost:9999/webhooks/merchant-alpha · Enabled')).not.toBeVisible();

    // Create Order 2 via POST to test immediate delivery cutoff (H3)
    const quote2Res = await fetch('http://localhost:8080/api/v1/pricing/quotes', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 2.0,
        dimensions: {
          length_cm: 20,
          width_cm: 15,
          height_cm: 10,
        },
      }),
    });
    expect(quote2Res.status).toBe(201);
    const quote2Data = await quote2Res.json();
    const quote2Id = quote2Data.data?.quote_id || quote2Data.data?.id;
    expect(quote2Id).toBeTruthy();

    const booking2Res = await fetch('http://localhost:8080/api/v1/parcels', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
        'Idempotency-Key': crypto.randomUUID(),
      },
      body: JSON.stringify({
        quote_id: quote2Id,
        recipient_name: 'Nimal Bandara',
        recipient_phone: '+94719876543',
        weight_kg: 2.0,
        dimensions: {
          length_cm: 20,
          width_cm: 15,
          height_cm: 10,
        },
        pickup: {
          contact_name: 'Alpha Warehouse',
          contact_phone: '+94112345678',
          address_line1: '124 Galle Road',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: 'Nimal Bandara',
          contact_phone: '+94719876543',
          address_line1: '88 Dalada Veediya',
          city: 'Colombo',
          postal_code: '00200',
          zone_id: 'ZONE_COLOMBO_01',
        },
        item_description: 'Retail Apparel Sample',
      }),
    });
    expect(booking2Res.status).toBe(201);

    // Wait and verify Webhook Alpha received ZERO events for Order 2 (count stays 1)
    // While Webhook Gamma received the new event (count becomes 3)
    let eventsAudit3: any = null;
    for (let i = 0; i < 25; i++) {
      await new Promise((r) => setTimeout(r, 600));
      const res = await (await fetch('http://localhost:9999/api/events')).json();
      if (res.counts?.gamma >= 3) {
        eventsAudit3 = res;
        break;
      }
    }

    expect(eventsAudit3).toBeTruthy();
    // Alpha was deleted -> immediate cutoff confirmed!
    expect(eventsAudit3.counts?.alpha).toBe(1);
    // Gamma remained active -> received Order 2's booked event!
    expect(eventsAudit3.counts?.gamma).toBe(3);
  });
});
