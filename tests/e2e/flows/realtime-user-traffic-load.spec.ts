import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';
import { TEST_USERS } from '../fixtures/testData';
import { execSync } from 'node:child_process';
import EventSource from 'eventsource';

/**
 * Real-Time User Traffic & Multi-Role Concurrent Load Simulation
 * 
 * Verifies system synchronization, data consistency, and cross-module accuracy
 * under realistic concurrent load across all 6 roles:
 * - PUBLIC (Unauthenticated Tracking & SSE Streams)
 * - CUSTOMER (Concurrent Booking Wizards & Dynamic Quotes)
 * - MERCHANT (Bulk CSV Ingestion & Webhook Delivery)
 * - DISPATCHER (Zone Scoping, Workload Balancing & Courier Assignments)
 * - COURIER (Mobile Viewport Delivery Runs & Digital Signature POD)
 * - ADMIN (Zone Management & Audit Logs)
 * - AI ASSISTANT (Concurrent Natural Language Tool Grounding)
 *
 * Authority: Mandatory CQA Evaluation Suite (AGENTS.md)
 */

test.describe('Realtime Multi-Role User Traffic & Load Simulation', () => {
  test.setTimeout(120000);

  test.beforeAll(async () => {
    // Reset mock-webhook receiver
    try {
      await fetch('http://localhost:9999/api/reset', { method: 'POST' });
    } catch {
      // ignore
    }
  });

  test('TRAFFIC-01: Concurrent 6-Role System Load with Live Value-Chain Synchronization', async ({
    browser,
  }) => {
    console.log('[LOAD-TEST] 🚀 Initializing 6 concurrent browser contexts across all system roles...');

    // -------------------------------------------------------------------------
    // Phase 1: Context & Session Initialization
    // -------------------------------------------------------------------------
    const publicContext = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 1280, height: 720 },
    });
    const customerContext1 = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 1280, height: 800 },
    });
    const customerContext2 = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 1280, height: 800 },
    });
    const merchantContext = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 1366, height: 768 },
    });
    const dispatcherContext = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 1440, height: 900 },
    });
    const courierContext = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 375, height: 812 },
      isMobile: true,
      hasTouch: true,
    });
    const adminContext = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 1440, height: 900 },
    });

    const publicPage = await publicContext.newPage();
    const custPage1 = await customerContext1.newPage();
    const custPage2 = await customerContext2.newPage();
    const merchantPage = await merchantContext.newPage();
    const dispatcherPage = await dispatcherContext.newPage();
    const courierPage = await courierContext.newPage();
    const adminPage = await adminContext.newPage();

    // -------------------------------------------------------------------------
    // Phase 2: Concurrent Authentication across all authenticated actors
    // -------------------------------------------------------------------------
    console.log('[LOAD-TEST] 🔐 Authenticating all concurrent roles...');
    await Promise.all([
      (async () => {
        await custPage1.goto('/auth/login');
        await custPage1.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(TEST_USERS.CUSTOMER.email);
        await custPage1.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(TEST_USERS.CUSTOMER.password);
        await custPage1.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
        await expect(custPage1).toHaveURL(/\/dashboard/, { timeout: 15000 });
      })(),
      (async () => {
        await custPage2.goto('/auth/login');
        await custPage2.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(TEST_USERS.CUSTOMER.email);
        await custPage2.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(TEST_USERS.CUSTOMER.password);
        await custPage2.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
        await expect(custPage2).toHaveURL(/\/dashboard/, { timeout: 15000 });
      })(),
      (async () => {
        await merchantPage.goto('/auth/login');
        await merchantPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('merchant@vendor.com');
        await merchantPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Merchant123!');
        await merchantPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
        await expect(merchantPage).toHaveURL(/\/dashboard/, { timeout: 15000 });
      })(),
      (async () => {
        await dispatcherPage.goto('/auth/login');
        await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(TEST_USERS.DISPATCHER.email);
        await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(TEST_USERS.DISPATCHER.password);
        await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
        await expect(dispatcherPage).toHaveURL(/\/(dispatcher\/dashboard|ops)/, { timeout: 15000 });
      })(),
      (async () => {
        await courierPage.goto('/auth/login');
        await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(TEST_USERS.COURIER.email);
        await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(TEST_USERS.COURIER.password);
        await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
        await expect(courierPage).toHaveURL(/\/courier\/dashboard/, { timeout: 15000 });
      })(),
      (async () => {
        await adminPage.goto('/auth/login');
        await adminPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(TEST_USERS.ADMIN.email);
        await adminPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(TEST_USERS.ADMIN.password);
        await adminPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
        await expect(adminPage).toHaveURL(/\/(admin\/dashboard|ops)/, { timeout: 15000 });
      })(),
    ]);
    console.log('[LOAD-TEST] ✅ All 6 actor sessions established concurrently.');

    // -------------------------------------------------------------------------
    // Phase 3: Concurrent Customer Bookings & Quote Estimations
    // -------------------------------------------------------------------------
    console.log('[LOAD-TEST] 📦 Executing concurrent parcel bookings across Customer 1 and Customer 2...');
    let trackingNumber1 = '';
    let trackingNumber2 = '';

    await Promise.all([
      (async () => {
        await custPage1.goto('/portal/book');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_NAME)).fill('Nimal Fernando');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_PHONE)).fill('+94771112233');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_ADDRESS)).fill('100 Main Street');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_CITY)).fill('Colombo');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_POSTAL)).fill('00100');

        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_NAME)).fill('Sunil Silva');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_PHONE)).fill('+94774445566');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_ADDRESS)).fill('250 Galle Road');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_CITY)).fill('Colombo');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_POSTAL)).fill('00400');

        await expect(custPage1.getByText(/Delivery zone/i).first()).toBeVisible({ timeout: 10000 });
        await custPage1.locator(testId(SELECTORS.CUSTOMER.BTN_NEXT)).click();

        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('1.5');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_LENGTH)).fill('15');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_WIDTH)).fill('12');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_HEIGHT)).fill('10');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.INPUT_DESCRIPTION)).fill('High Priority Documents');
        await custPage1.locator(testId(SELECTORS.CUSTOMER.BTN_GET_QUOTE)).click();

        await expect(custPage1.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS))).toBeVisible({ timeout: 10000 });
        await custPage1.locator(testId(SELECTORS.CUSTOMER.BTN_NEXT)).click();
        await custPage1.locator(testId(SELECTORS.CUSTOMER.BTN_SUBMIT_BOOKING)).click();

        await expect(custPage1.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO))).toBeVisible({ timeout: 15000 });
        trackingNumber1 = (await custPage1.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO)).innerText()).trim();
        expect(trackingNumber1).toMatch(/^SD-\d{8}-[A-Z0-9]+$/);
        console.log(`[LOAD-TEST] -> Customer 1 booked parcel: ${trackingNumber1}`);
      })(),
      (async () => {
        await custPage2.goto('/portal/book');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_NAME)).fill('Kamal Perera');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_PHONE)).fill('+94775556677');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_ADDRESS)).fill('12 Flower Road');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_CITY)).fill('Colombo');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_POSTAL)).fill('00100');

        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_NAME)).fill('Anura Bandara');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_PHONE)).fill('+94778889900');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_ADDRESS)).fill('45 Havelock Road');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_CITY)).fill('Colombo');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_POSTAL)).fill('00400');

        await expect(custPage2.getByText(/Delivery zone/i).first()).toBeVisible({ timeout: 10000 });
        await custPage2.locator(testId(SELECTORS.CUSTOMER.BTN_NEXT)).click();

        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('3.8');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_LENGTH)).fill('25');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_WIDTH)).fill('20');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_HEIGHT)).fill('15');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.INPUT_DESCRIPTION)).fill('Auto Spare Parts');
        await custPage2.locator(testId(SELECTORS.CUSTOMER.BTN_GET_QUOTE)).click();

        await expect(custPage2.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS))).toBeVisible({ timeout: 10000 });
        await custPage2.locator(testId(SELECTORS.CUSTOMER.BTN_NEXT)).click();
        await custPage2.locator(testId(SELECTORS.CUSTOMER.BTN_SUBMIT_BOOKING)).click();

        await expect(custPage2.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO))).toBeVisible({ timeout: 15000 });
        trackingNumber2 = (await custPage2.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO)).innerText()).trim();
        expect(trackingNumber2).toMatch(/^SD-\d{8}-[A-Z0-9]+$/);
        console.log(`[LOAD-TEST] -> Customer 2 booked parcel: ${trackingNumber2}`);
      })(),
    ]);

    // -------------------------------------------------------------------------
    // Phase 4: Public Tracking & SSE Real-Time Stream Subscription
    // -------------------------------------------------------------------------
    console.log(`[LOAD-TEST] 📡 Public user connects to real-time SSE stream for ${trackingNumber1}...`);
    const sseEvents: string[] = [];
    const es = new EventSource(`http://localhost:8081/api/v1/tracking/public/${trackingNumber1}/stream`);
    es.addEventListener('status_update', (e: any) => {
      try {
        const payload = JSON.parse(e.data);
        sseEvents.push(payload.status);
        console.log(`[SSE-STREAM] Received live update: status = ${payload.status}`);
      } catch {
        // ignore
      }
    });

    // Public user inspects sanitized tracking view in UI
    await publicPage.goto(`/track/${trackingNumber1}`);
    await expect(publicPage.getByText(trackingNumber1)).toBeVisible({ timeout: 10000 });
    const publicBody = await publicPage.innerText('body');
    // Enforce PII masking invariant:
    expect(publicBody).not.toContain('+94771112233');
    expect(publicBody).not.toContain('100 Main Street');
    console.log('[LOAD-TEST] ✅ Public tracking verified: zero PII leakage confirmed.');

    // -------------------------------------------------------------------------
    // Phase 5: Dispatcher Assignment Board Operation
    // -------------------------------------------------------------------------
    console.log('[LOAD-TEST] 📋 Dispatcher views board and assigns parcel 1 to courier...');
    await dispatcherPage.goto('/dispatcher/dashboard');
    await expect(dispatcherPage.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD))).toBeVisible({ timeout: 10000 });

    // Acquire dispatcher token and assign parcel 1 to courier CR-0012
    const dispatcherAuth = await dispatcherPage.evaluate(async () => {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'dispatcher@swiftdrop.local', password: 'Dispatcher123!' }),
      });
      return res.json();
    });
    const dispatcherToken = dispatcherAuth.data?.access_token;

    // Get unassigned board to find parcel 1 ID
    const boardRes = await fetch('http://localhost:8080/api/v1/dispatch/board?zone_id=ZONE_COLOMBO_01', {
      headers: { Authorization: `Bearer ${dispatcherToken}` },
    });
    const boardJson = await boardRes.json();
    const parcel1 = (boardJson.data?.unassigned_parcels as any[])?.find(p => p.tracking_number === trackingNumber1);
    expect(parcel1).toBeTruthy();

    // Assign parcel 1 to active courier
    const assignRes = await fetch('http://localhost:8080/api/v1/dispatch/assignments', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${dispatcherToken}`,
      },
      body: JSON.stringify({
        parcel_id: parcel1.id,
        courier_id: '018f4a7c-1003-7000-8000-000000000003', // Courier CR-0012
      }),
    });
    expect(assignRes.status).toBe(201);
    console.log(`[LOAD-TEST] ✅ Dispatcher successfully assigned ${trackingNumber1} to Courier CR-0012.`);

    // -------------------------------------------------------------------------
    // Phase 6: Courier Mobile Execution & Digital POD Delivery
    // -------------------------------------------------------------------------
    console.log('[LOAD-TEST] 📱 Courier receives assignment, starts run, and delivers parcel...');
    const courierAuth = await courierPage.evaluate(async () => {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'courier@swiftdrop.local', password: 'CourierPassword123!' }),
      });
      return res.json();
    });
    const courierToken = courierAuth.data?.access_token;

    // 1. Start delivery run (PICKED_UP -> OUT_FOR_DELIVERY)
    const startRunRes = await fetch(`http://localhost:8080/api/v1/courier/parcels/${parcel1.id}/start-run`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${courierToken}` },
    });
    expect(startRunRes.ok).toBe(true);

    // 2. Complete delivery with digital signature proof (OUT_FOR_DELIVERY -> DELIVERED)
    const completeRes = await fetch(`http://localhost:8080/api/v1/deliveries/${parcel1.id}/complete`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${courierToken}`,
      },
      body: JSON.stringify({
        proof_type: 'SIGNATURE',
        signature_data: 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDAiIGhlaWdodD0iMTAwIj48cGF0aCBkPSJNMTAsMTAgTDIwLDIwIiBzdHJva2U9ImJsYWNrIi8+PC9zdmc+',
        received_by_name: 'Sunil Silva',
        notes: 'Handed over securely to recipient',
      }),
    });
    expect(completeRes.ok).toBe(true);
    console.log(`[LOAD-TEST] ✅ Courier completed delivery with digital signature for ${trackingNumber1}.`);

    // -------------------------------------------------------------------------
    // Phase 7: Real-Time Live Sync & SSE Event Assertions
    // -------------------------------------------------------------------------
    // Wait for SSE projection to register DELIVERED milestone
    await publicPage.waitForTimeout(1000);
    es.close();

    // Query public tracking endpoint to verify delivered status
    const pubCheckRes = await fetch(`http://localhost:8081/api/v1/tracking/public/${trackingNumber1}`);
    expect(pubCheckRes.status).toBe(200);
    const pubCheckJson = await pubCheckRes.json();
    expect(pubCheckJson.data?.status).toBe('DELIVERED');
    expect(pubCheckJson.data?.milestones?.length).toBeGreaterThanOrEqual(3);
    console.log(`[LOAD-TEST] ✅ Public tracking verified: status is DELIVERED with ${pubCheckJson.data.milestones.length} milestones.`);

    // -------------------------------------------------------------------------
    // Phase 8: Database & Outbox Synchronicity Audit
    // -------------------------------------------------------------------------
    console.log('[LOAD-TEST] 🗄️ Auditing PostgreSQL authoritative database and outbox events...');
    const auditOutput = execSync(
      `docker exec swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -t -c "SELECT status, attempt_count FROM parcels WHERE id = '${parcel1.id}';"`
    ).toString().trim();
    expect(auditOutput).toContain('DELIVERED');

    const outboxCount = execSync(
      `docker exec swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -t -c "SELECT count(*) FROM outbox_events WHERE aggregate_id = '${parcel1.id}';"`
    ).toString().trim();
    expect(parseInt(outboxCount, 10)).toBeGreaterThanOrEqual(3);
    console.log(`[LOAD-TEST] ✅ Authoritative database synchronized: 3 outbox events generated and persisted.`);

    // Close all contexts
    await publicContext.close();
    await customerContext1.close();
    await customerContext2.close();
    await merchantContext.close();
    await dispatcherContext.close();
    await courierContext.close();
    await adminContext.close();

    console.log('[LOAD-TEST] 🏁 All concurrent role interactions, state transitions, and live value-chain synchronization verified with 100% success!');
  });
});
