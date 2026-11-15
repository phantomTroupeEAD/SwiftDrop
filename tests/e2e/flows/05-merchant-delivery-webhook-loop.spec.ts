import { execSync } from 'child_process';
import { test, expect } from '../fixtures/orchestrationFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * FULL-CIRCLE MERCHANT WEBHOOK & DELIVERY RETRY E2E TEST (ZERO ROUTE MOCKS)
 *
 * Verifies the complete asynchronous event-driven loop:
 * 1. Merchant registers live webhook endpoint pointing to Mock Webhook (:9999/webhooks/merchant-alpha).
 * 2. Merchant books B2B parcel (Status: CREATED).
 * 3. Central Dispatcher assigns parcel to on-shift Courier (Status: ASSIGNED).
 * 4. Courier Attempt 1: Recipient unavailable (Status: ATTEMPT_FAILED, attempt_count: 1) — ADR-004.
 * 5. Courier Attempt 2: Successful doorstep delivery with digital signature POD (Status: DELIVERED, attempt_count: 2).
 * 6. Messaging Service Audit:
 *    - Outbox Relay (:8082) published events from PostgreSQL outbox table to RabbitMQ.
 *    - Event Worker (:8083) consumed events, resolved merchant webhook, and computed HMAC signature.
 *    - Mock Webhook (:9999) received all 4 events with 100% valid HMAC-SHA256 signatures.
 *
 * Governed by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Merchant Webhook & Delivery Failure/Retry Lifecycle (Zero Mocks)', () => {

  test.beforeEach(async () => {
    // 1. Reset mock webhook event store
    try {
      await fetch('http://localhost:9999/api/reset', { method: 'POST' });
    } catch {
      // ignore if server starting up
    }

    // 2. Ensure merchant account exists and truncate any webhook endpoints and delivery tables
    try {
      execSync('docker exec -i swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -c "TRUNCATE webhook_delivery_attempts, webhook_deliveries, webhook_endpoints CASCADE; UPDATE users SET role = \'MERCHANT\', tenant_id = \'91d10f4e-6a14-4598-a5a8-ea472a3ffcba\' WHERE email = \'applicant@swiftdrop.local\';"');
    } catch {
      // ignore
    }
  });

  test('should execute full-circle merchant delivery cycle and verify 4 webhook events on port 9999', async ({
    applicantPage: merchantPage,
    dispatcherPage,
    courierPage,
  }) => {
    // -------------------------------------------------------------------------
    // 1. Verify Messaging Daemons Health Probes
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
    // 2. Window 1: Merchant Signs In & Configures Live Webhook to Port 9999
    // -------------------------------------------------------------------------
    await merchantPage.goto('/auth/login');
    await merchantPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('applicant@swiftdrop.local');
    await merchantPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('CustomerPassword123!');
    await merchantPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(merchantPage).toHaveURL(/\/dashboard/, { timeout: 10000 });
    await merchantPage.getByRole('link', { name: 'Webhooks' }).click();
    await expect(merchantPage.getByRole('heading', { name: 'Webhooks', level: 1 })).toBeVisible({ timeout: 10000 });

    // Register live mock webhook receiver on port 9999
    await merchantPage.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-alpha');

    // Select all available event subscriptions
    const checkboxes = merchantPage.locator('input[name="event"]');
    const count = await checkboxes.count();
    for (let i = 0; i < count; i++) {
      await checkboxes.nth(i).check();
    }

    await merchantPage.getByTestId('merchant-credential-create-btn').click();

    // Verify AES-encrypted secret modal appears, capture generated secret, configure mock-webhook, and dismiss
    await expect(merchantPage.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const generatedSecret = (await merchantPage.locator('code').innerText()).trim();
    expect(generatedSecret).toMatch(/^whsec_[A-Za-z0-9_-]+$/);
    await fetch('http://localhost:9999/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant: 'alpha', secret: generatedSecret }),
    });
    await merchantPage.getByTestId('dismiss-key-modal-checkbox').check();
    await merchantPage.getByTestId('merchant-secret-close-btn').click();

    // -------------------------------------------------------------------------
    // 3. Window 1: Merchant Books B2B Parcel via Live Booking Wizard
    // -------------------------------------------------------------------------
    await merchantPage.getByRole('link', { name: 'Book a parcel' }).click();
    await expect(merchantPage.getByRole('heading', { name: 'Book a parcel', level: 1 })).toBeVisible({ timeout: 10000 });

    // Step 1: Addresses
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_NAME)).fill('Colombo Tech Central Depot');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_PHONE)).fill('+94770000004');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_ADDRESS)).fill('123 Galle Road');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_CITY)).fill('Colombo');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_POSTAL)).fill('00100');

    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_NAME)).fill('Kasun Wickrama');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_PHONE)).fill('+94719876543');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_ADDRESS)).fill('15/A Havelock Road');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_CITY)).fill('Colombo');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_POSTAL)).fill('00400');

    await expect(merchantPage.getByText(/Delivery zone/i).first()).toBeVisible({ timeout: 10000 });
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.BTN_NEXT)).click();

    // Step 2: Parcel Dimensions & Real Pricing
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('3.0');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_LENGTH)).fill('25');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WIDTH)).fill('20');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_HEIGHT)).fill('15');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DESCRIPTION)).fill('High-value electronics merchandise');
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.BTN_GET_QUOTE)).click();

    // Step 3: Verify dynamic quote and advance
    await expect(merchantPage.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS))).toBeVisible({ timeout: 10000 });
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.BTN_NEXT)).click();

    // Step 4: Submit Booking & Capture Tracking Number
    await merchantPage.locator(testId(SELECTORS.CUSTOMER.BTN_SUBMIT_BOOKING)).click();
    await expect(merchantPage.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO))).toBeVisible({ timeout: 15000 });

    const trackingNumber = (await merchantPage.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO)).innerText()).trim();
    expect(trackingNumber).toMatch(/^SD-\d{8}-[A-Z0-9]+$/);

    // -------------------------------------------------------------------------
    // 4. Window 2: Central Dispatcher Assigns Parcel to Active Courier
    // -------------------------------------------------------------------------
    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.local');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Dispatcher123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(dispatcherPage).toHaveURL(/\/dispatcher\/dashboard/, { timeout: 10000 });
    await expect(dispatcherPage.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD))).toBeVisible({ timeout: 10000 });

    // Assign parcel via Dispatch API
    const assignResult = await dispatcherPage.evaluate(async (trackingNo) => {
      const authRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'dispatcher@swiftdrop.local', password: 'Dispatcher123!' }),
      });
      const authData = await authRes.json();
      const token = authData.data?.access_token;

      const boardRes = await fetch('/api/v1/dispatch/board?zone_id=ZONE_COLOMBO_01', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      const boardData = await boardRes.json();
      const parcel = boardData.data?.unassigned_parcels?.find((p: any) => p.tracking_number === trackingNo);
      const courier = boardData.data?.courier_loads?.find((c: any) => c.courier_id === '018f4a7c-1003-7000-8000-000000000003')
        || boardData.data?.courier_loads?.find((c: any) => c.status === 'ON_SHIFT');

      if (!parcel || !courier) return { success: false, reason: 'unassigned parcel or courier missing', parcel, courier };

      const assignRes = await fetch('/api/v1/dispatch/assignments', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          parcel_id: parcel.id,
          courier_id: courier.courier_id,
          notes: 'B2B Priority Dispatch',
        }),
      });
      return { success: assignRes.ok, parcelId: parcel.id, courierId: courier.courier_id };
    }, trackingNumber);

    expect(assignResult.success).toBe(true);

    // -------------------------------------------------------------------------
    // 5. Window 3: Courier Mobile PWA — Attempt 1: Delivery Failure (ADR-004)
    // -------------------------------------------------------------------------
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('courier@swiftdrop.local');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('CourierPassword123!');
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(courierPage).toHaveURL(/\/courier\/dashboard/, { timeout: 10000 });
    await expect(courierPage.getByRole('heading', { name: 'Delivery tasks', level: 1 })).toBeVisible({ timeout: 10000 });

    const parcelId = assignResult.parcelId;

    // Execute Attempt 1 via Delivery API: Start Run -> Mark Failed
    const attempt1Result = await courierPage.evaluate(async (id) => {
      const authRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'courier@swiftdrop.local', password: 'CourierPassword123!' }),
      });
      const token = (await authRes.json()).data?.access_token;

      // Start run -> OUT_FOR_DELIVERY
      await fetch(`/api/v1/courier/parcels/${id}/start-run`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
      });

      // Submit Failure Attempt: RECIPIENT_UNAVAILABLE
      const failRes = await fetch(`/api/v1/deliveries/${id}/failed`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          failure_reason_code: 'RECIPIENT_UNAVAILABLE',
          notes: 'Attempt 1: Recipient not home, gate locked',
        }),
      });
      return { success: failRes.ok, status: failRes.status };
    }, parcelId);

    expect(attempt1Result.success).toBe(true);

    // -------------------------------------------------------------------------
    // 6. Window 2: Dispatcher Authorizes Reattempt per ADR-004
    // -------------------------------------------------------------------------
    const rescheduleResult = await dispatcherPage.evaluate(async ({ id, courierId }) => {
      const authRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'dispatcher@swiftdrop.local', password: 'Dispatcher123!' }),
      });
      const token = (await authRes.json()).data?.access_token;

      const reschedRes = await fetch(`/api/v1/dispatch/parcels/${id}/reschedule`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          courier_id: courierId,
          notes: 'ADR-004: Dispatcher authorizes retry attempt 2',
        }),
      });
      return { success: reschedRes.ok, status: reschedRes.status };
    }, { id: parcelId, courierId: assignResult.courierId });

    expect(rescheduleResult.success).toBe(true);

    // -------------------------------------------------------------------------
    // 7. Window 3: Courier Mobile PWA — Attempt 2: Delivery Success with POD
    // -------------------------------------------------------------------------
    const attempt2Result = await courierPage.evaluate(async (id) => {
      const authRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'courier@swiftdrop.local', password: 'CourierPassword123!' }),
      });
      const token = (await authRes.json()).data?.access_token;

      // Start run again -> OUT_FOR_DELIVERY (Attempt 2)
      await fetch(`/api/v1/courier/parcels/${id}/start-run`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
      });

      // Submit Complete Delivery with digital signature POD
      const completeRes = await fetch(`/api/v1/deliveries/${id}/complete`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          proof_type: 'SIGNATURE',
          signature_data: 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDAiIGhlaWdodD0iMTAwIj48cGF0aCBkPSJNMTAgMTBMMTAwIDEwMCIvPjwvc3ZnPg==',
          received_by_name: 'Kasun Wickrama',
          notes: 'Attempt 2: Successfully delivered on doorstep retry',
        }),
      });
      return { success: completeRes.ok, status: completeRes.status };
    }, parcelId);

    expect(attempt2Result.success).toBe(true);

    // -------------------------------------------------------------------------
    // 7. Verify Tracking Service (Port 8081) Reflects DELIVERED Status & History
    // -------------------------------------------------------------------------
    const trackingRes = await fetch(`http://localhost:8081/api/v1/tracking/public/${trackingNumber}`);
    expect(trackingRes.ok).toBe(true);
    const trackingJson = await trackingRes.json();
    expect(trackingJson.data?.tracking_number).toBe(trackingNumber);
    expect(trackingJson.data?.status).toBe('DELIVERED');
    expect(trackingJson.data?.milestones?.length).toBeGreaterThanOrEqual(4);

    // -------------------------------------------------------------------------
    // 8. Auditing Mock Webhook Receiver on Port 9999 (The Definitive Test)
    // -------------------------------------------------------------------------
    // Wait for asynchronous outbox relay + event worker dispatch loop (poll up to 15s)
    let deliveredEvents: any[] = [];
    const maxPolls = 30;

    for (let p = 0; p < maxPolls; p++) {
      try {
        const res = await fetch('http://localhost:9999/api/events?tenant=alpha');
        const json = await res.json();
        if (Array.isArray(json)) {
          deliveredEvents = json;
          if (json.length >= 4) {
            break;
          }
        }
      } catch {
        // retry
      }
      await new Promise(r => setTimeout(r, 500));
    }

    // Verify mock webhook received events with valid HMAC signatures
    console.log(`[E2E Audit] Total events received on mock webhook :9999 = ${deliveredEvents.length}`);
    for (const evt of deliveredEvents) {
      console.log(`[E2E Audit] -> Received Event: ${evt.type} (hmacValid: ${evt.hmacValid}, id: ${evt.id})`);
    }
    expect(deliveredEvents.length).toBeGreaterThanOrEqual(4);

    // Assert that every received event has cryptographic HMAC valid verification
    for (const evt of deliveredEvents) {
      expect(evt.tenant).toBe('alpha');
      expect(evt.hmacValid).toBe(true);
      expect(evt.signatureHeader).toBeDefined();
    }

    // Assert that the full lifecycle milestones were all received by the merchant webhook
    const receivedEventTypes = deliveredEvents.map(e => e.type);
    expect(receivedEventTypes).toContain('parcel.booked.v1');
    expect(receivedEventTypes).toContain('parcel.assigned.v1');
    expect(receivedEventTypes).toContain('parcel.out_for_delivery.v1');
    expect(receivedEventTypes).toContain('parcel.delivery_failed.v1');
    expect(receivedEventTypes).toContain('parcel.delivered.v1');

    // -------------------------------------------------------------------------
    // 9. Visual Mock Webhook Dashboard Inspection
    // -------------------------------------------------------------------------
    await merchantPage.goto('http://localhost:9999/dashboard');
    await expect(merchantPage.locator('#eventRows')).toBeVisible({ timeout: 10000 });
    await expect(merchantPage.locator('#countTotal')).toHaveText(/[1-9]/, { timeout: 10000 });
  });

});
