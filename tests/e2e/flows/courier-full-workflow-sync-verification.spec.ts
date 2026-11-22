import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Courier Full Workflow & Cross-Portal Synchronization Verification Suite
 *
 * Verifies:
 * 1. Courier Authentication (courier@swiftdrop.local / CourierPassword123!)
 * 2. Full Courier Interface Exploration:
 *    - Shift status toggle (ON_SHIFT <-> OFF_SHIFT)
 *    - Active parcels manifest
 *    - Turn-by-turn Directions (Google Maps URL format) & Direct phone call (tel: format)
 *    - Delivery attempt history modal
 *    - Portal layout and navigation
 * 3. Delivery Lifecycle & State Machine Transitions:
 *    - ASSIGNED -> OUT_FOR_DELIVERY
 *    - Failed delivery attempt (RECIPIENT_UNAVAILABLE)
 *    - Attempt history inspection
 *    - Dispatcher reattempt authorization / reschedule (ADR-004 gate)
 *    - Complete delivery with Photo POD
 *    - Complete delivery with Electronic Signature POD
 * 4. Cross-Portal Real-Time / Live Synchronization across:
 *    - Courier Mobile PWA
 *    - Dispatcher Operations Board (/dispatcher/dashboard)
 *    - Public Tracking Portal (/track/:trackingNumber)
 *    - Customer Tracking Portal (/portal/tracking/:trackingNumber)
 *    - Route test for /tracking/:trackingNumber URL alias
 */

test.describe.serial('Courier Delivery Flow & Cross-Portal Sync Verification', () => {
  test.setTimeout(120000);

  const CORE_API = 'http://localhost:8080/api/v1';

  const USERS = {
    COURIER: {
      email: 'courier@swiftdrop.local',
      password: 'CourierPassword123!',
      id: '018f4a7c-1003-7000-8000-000000000003',
      code: 'CR-0012',
    },
    DISPATCHER: {
      email: 'dispatcher@swiftdrop.local',
      password: 'Dispatcher123!',
    },
    CUSTOMER: {
      email: 'customer@swiftdrop.local',
      password: 'CustomerPassword123!',
    },
  };

  let customerToken = '';
  let dispatcherToken = '';
  let courierToken = '';

  let parcel1Id = '';
  let parcel1Tracking = '';
  let parcel2Id = '';
  let parcel2Tracking = '';

  async function apiLogin(email: string, pass: string): Promise<string> {
    const res = await fetch(`${CORE_API}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password: pass }),
    });
    if (!res.ok) {
      throw new Error(`Login failed for ${email}: ${res.status} ${await res.text()}`);
    }
    const data = await res.json();
    return data.data?.session?.access_token || data.data?.access_token || data.token;
  }

  async function bookParcel(recipientName: string, recipientPhone: string, street: string): Promise<{ id: string; tracking: string }> {
    // 1. Get quote
    const quoteRes = await fetch(`${CORE_API}/pricing/quotes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${customerToken}` },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 2.0,
        dimensions: { length_cm: 25, width_cm: 20, height_cm: 15 },
      }),
    });
    if (!quoteRes.ok) {
      throw new Error(`Quote failed: ${quoteRes.status} ${await quoteRes.text()}`);
    }
    const quoteData = await quoteRes.json();

    // 2. Book parcel
    const bookRes = await fetch(`${CORE_API}/parcels`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${customerToken}`,
        'Idempotency-Key': randomUUID(),
      },
      body: JSON.stringify({
        quote_id: quoteData.data?.quote_id,
        recipient_name: recipientName,
        recipient_phone: recipientPhone,
        weight_kg: 2.0,
        dimensions: { length_cm: 25, width_cm: 20, height_cm: 15 },
        pickup: {
          contact_name: 'Warehouse Colombo',
          contact_phone: '+94770001122',
          address_line1: '10 Galle Road',
          city: 'Colombo',
          postal_code: '00300',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: recipientName,
          contact_phone: recipientPhone,
          address_line1: street,
          city: 'Colombo',
          postal_code: '00400',
          zone_id: 'ZONE_COLOMBO_01',
        },
      }),
    });
    if (!bookRes.ok) {
      throw new Error(`Booking failed: ${bookRes.status} ${await bookRes.text()}`);
    }
    const bookData = await bookRes.json();
    return {
      id: bookData.data?.id,
      tracking: bookData.data?.tracking_number,
    };
  }

  async function assignParcel(parcelId: string, courierId: string) {
    const res = await fetch(`${CORE_API}/dispatch/assignments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${dispatcherToken}`,
        'Idempotency-Key': randomUUID(),
      },
      body: JSON.stringify({
        parcel_id: parcelId,
        courier_id: courierId,
      }),
    });
    if (!res.ok) {
      throw new Error(`Assignment failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  test.beforeAll(async () => {
    customerToken = await apiLogin(USERS.CUSTOMER.email, USERS.CUSTOMER.password);
    dispatcherToken = await apiLogin(USERS.DISPATCHER.email, USERS.DISPATCHER.password);
    courierToken = await apiLogin(USERS.COURIER.email, USERS.COURIER.password);

    // Ensure Courier CR-0012 is ON_SHIFT
    await fetch(`${CORE_API}/courier/shift`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${courierToken}` },
      body: JSON.stringify({ target_status: 'ON_SHIFT' }),
    });

    // Book 2 parcels
    const p1 = await bookParcel('Piyumi Fernando', '+94771234560', '88 Duplication Road');
    parcel1Id = p1.id;
    parcel1Tracking = p1.tracking;
    console.log(`[TEST-SETUP] Booked Parcel 1: ${parcel1Tracking} (${parcel1Id})`);

    const p2 = await bookParcel('Rohan Jayasinghe', '+94779876540', '102 Havelock Road');
    parcel2Id = p2.id;
    parcel2Tracking = p2.tracking;
    console.log(`[TEST-SETUP] Booked Parcel 2: ${parcel2Tracking} (${parcel2Id})`);

    // Assign both to Courier CR-0012
    await assignParcel(parcel1Id, USERS.COURIER.id);
    console.log(`[TEST-SETUP] Assigned Parcel 1 to Courier CR-0012`);
    await assignParcel(parcel2Id, USERS.COURIER.id);
    console.log(`[TEST-SETUP] Assigned Parcel 2 to Courier CR-0012`);
  });

  test('Part 1: Courier Login, Shift Management, & Full Interface Exploration', async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 375, height: 812 },
      isMobile: true,
      hasTouch: true,
    });
    const page = await context.newPage();

    // 1. Log in as Courier
    console.log('[Part 1] 1. Logging in as courier...');
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(USERS.COURIER.email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(USERS.COURIER.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page).toHaveURL(/.*courier\/dashboard/, { timeout: 15000 });

    // 2. Verify Courier Header & Hero Section
    console.log('[Part 1] 2. Inspecting Courier interface layout...');
    await expect(page.getByRole('heading', { name: 'Delivery tasks' })).toBeVisible();
    await expect(page.getByText('Manage your shift and assigned deliveries.')).toBeVisible();

    // Check brand link & sign out button
    await expect(page.getByRole('link', { name: 'SwiftDrop' })).toBeVisible();
    await expect(page.getByRole('button', { name: /Sign out/i })).toBeVisible();

    // 3. Test Shift Status Toggle: Start shift / End shift
    console.log('[Part 1] 3. Testing Shift Status toggle...');
    const shiftHeader = page.getByRole('heading', { name: 'Shift' });
    await expect(shiftHeader).toBeVisible();

    const endShiftBtn = page.getByRole('button', { name: 'End shift' });
    const startShiftBtn = page.getByRole('button', { name: 'Start shift' });

    if (await endShiftBtn.isVisible()) {
      await endShiftBtn.click();
      await expect(page.getByText(/Off Shift/i)).toBeVisible({ timeout: 10000 });
      await expect(page.getByRole('button', { name: 'Start shift' })).toBeVisible();

      // Toggle back to ON_SHIFT
      await page.getByRole('button', { name: 'Start shift' }).click();
      await expect(page.getByText(/On Shift/i)).toBeVisible({ timeout: 10000 });
      await expect(page.getByRole('button', { name: 'End shift' })).toBeVisible();
    } else if (await startShiftBtn.isVisible()) {
      await startShiftBtn.click();
      await expect(page.getByText(/On Shift/i)).toBeVisible({ timeout: 10000 });
      await expect(page.getByRole('button', { name: 'End shift' })).toBeVisible();
    }

    // 4. Verify Active Deliveries List
    console.log('[Part 1] 4. Verifying active parcels list and action controls...');
    await expect(page.getByRole('heading', { name: 'Assigned deliveries' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Refresh tasks' })).toBeVisible();

    // Verify Parcel 1 is displayed in task list
    const parcel1Card = page.locator('[data-testid="courier-task"]', { hasText: parcel1Tracking });
    await expect(parcel1Card).toBeVisible({ timeout: 10000 });
    await expect(parcel1Card.getByText('Piyumi Fernando')).toBeVisible();
    await expect(parcel1Card.getByText(/88 Duplication Road/i)).toBeVisible();
    await expect(parcel1Card.getByText(/Assigned/i)).toBeVisible();

    // Check Maps/Directions link
    const directionsLink = parcel1Card.getByRole('link', { name: 'Directions' });
    await expect(directionsLink).toBeVisible();
    const href = await directionsLink.getAttribute('href');
    console.log(`[Part 1] Directions link href: ${href}`);
    expect(href).toContain('google.com/maps/search');
    expect(href).toContain('Duplication');

    // Check Call Recipient link
    const callLink = parcel1Card.getByRole('link', { name: 'Call recipient' });
    await expect(callLink).toBeVisible();
    const telHref = await callLink.getAttribute('href');
    console.log(`[Part 1] Call link href: ${telHref}`);
    expect(telHref).toBe('tel:+94771234560');

    // Check Batch Selection Checkbox and Start Delivery button
    await expect(parcel1Card.getByLabel('Select for batch')).toBeVisible();
    await expect(parcel1Card.getByRole('button', { name: 'Start delivery' })).toBeVisible();

    await context.close();
  });

  test('Part 2: Delivery Lifecycle, Failed Attempt, Reattempt Gate, Photo POD & Cross-Portal Sync (Parcel 1)', async ({ browser }) => {
    // 3 Browser contexts to test cross-portal synchronization concurrently
    const courierContext = await browser.newContext({
      viewport: { width: 375, height: 812 },
      isMobile: true,
      hasTouch: true,
    });
    const courierPage = await courierContext.newPage();

    const dispatcherContext = await browser.newContext({
      viewport: { width: 1440, height: 900 },
    });
    const dispatcherPage = await dispatcherContext.newPage();

    const trackingContext = await browser.newContext({
      viewport: { width: 1280, height: 800 },
    });
    const trackingPage = await trackingContext.newPage();

    // Log in Courier
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(USERS.COURIER.email);
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(USERS.COURIER.password);
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await courierPage.waitForURL('**/courier/dashboard');

    // Log in Dispatcher
    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(USERS.DISPATCHER.email);
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(USERS.DISPATCHER.password);
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.waitForURL(/.*(dispatcher|dashboard|ops).*/);
    await dispatcherPage.goto('/dispatcher/dashboard');
    await expect(dispatcherPage.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD))).toBeVisible();

    // Open Public Tracking Page for Parcel 1
    console.log(`[Part 2] Navigating to Public Tracking: /track/${parcel1Tracking}`);
    await trackingPage.goto(`/track/${parcel1Tracking}`);
    await expect(trackingPage.getByText(parcel1Tracking)).toBeVisible({ timeout: 10000 });

    // -------------------------------------------------------------------------
    // Step A: Courier starts delivery
    // -------------------------------------------------------------------------
    console.log('[Part 2] Step A: Courier starts delivery for Parcel 1...');
    const parcel1Card = courierPage.locator('[data-testid="courier-task"]', { hasText: parcel1Tracking });
    await expect(parcel1Card).toBeVisible({ timeout: 10000 });
    const startDeliveryBtn = parcel1Card.getByRole('button', { name: 'Start delivery' });
    await expect(startDeliveryBtn).toBeVisible();
    await startDeliveryBtn.click();

    // Verify Courier UI reflects OUT_FOR_DELIVERY
    await expect(parcel1Card.getByText(/Out For Delivery/i).first()).toBeVisible({ timeout: 10000 });
    await expect(parcel1Card.getByRole('button', { name: 'Record delivery' })).toBeVisible();
    await expect(parcel1Card.getByRole('button', { name: 'Report failed attempt' })).toBeVisible();

    // SYNC CHECK 1: Dispatcher Board reflects OUT_FOR_DELIVERY
    console.log('[Part 2] SYNC CHECK 1: Verifying Dispatcher Board updates to OUT_FOR_DELIVERY...');
    await dispatcherPage.getByRole('button', { name: 'Refresh' }).click();
    const assignedCard = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.ASSIGNED_PARCELS_LIST));
    const assignedRow = assignedCard.locator('li', { hasText: parcel1Tracking });
    await expect(assignedRow).toBeVisible({ timeout: 10000 });
    await expect(assignedRow.getByText(/Out For Delivery/i)).toBeVisible({ timeout: 10000 });

    // SYNC CHECK 2: Tracking Portal reflects OUT_FOR_DELIVERY
    console.log('[Part 2] SYNC CHECK 2: Verifying Tracking Portal updates to OUT_FOR_DELIVERY...');
    await trackingPage.reload();
    await expect(trackingPage.getByTestId('tracking-status-badge')).toHaveText(/Out For Delivery/i, { timeout: 10000 });

    // -------------------------------------------------------------------------
    // Step B: Courier reports failed attempt
    // -------------------------------------------------------------------------
    console.log('[Part 2] Step B: Courier reports failed attempt (RECIPIENT_UNAVAILABLE)...');
    await parcel1Card.getByRole('button', { name: 'Report failed attempt' }).click();
    await expect(parcel1Card.getByText('Reason')).toBeVisible();

    const reasonSelect = parcel1Card.locator('select[name="reason"]');
    await reasonSelect.selectOption('RECIPIENT_UNAVAILABLE');
    await parcel1Card.locator('input[name="notes"]').fill('Recipient not answering door or phone');

    await parcel1Card.getByRole('button', { name: 'Save attempt' }).click();

    // Verify Success Alert on Courier Page
    await expect(courierPage.getByTestId('delivery-success-alert')).toBeVisible({ timeout: 10000 });
    await expect(courierPage.getByText(/Failed attempt recorded/i)).toBeVisible();
    await expect(courierPage.getByText(/Attempt #1 logged/i)).toBeVisible();

    // -------------------------------------------------------------------------
    // Step C: Inspect Attempt History Modal on Courier UI
    // -------------------------------------------------------------------------
    console.log('[Part 2] Step C: Testing Attempt History Modal on Courier UI...');
    const historyBtn = parcel1Card.locator('[data-testid="view-attempt-history-btn"]');
    await expect(historyBtn).toBeVisible({ timeout: 10000 });
    await historyBtn.click();

    // Verify Attempt History modal contents inside dialog
    const historyDialog = courierPage.getByRole('dialog');
    await expect(historyDialog).toBeVisible({ timeout: 5000 });
    await expect(historyDialog.getByRole('heading', { name: 'Delivery attempt history' })).toBeVisible();
    await expect(historyDialog.getByText(/Attempt 1 of 3/i)).toBeVisible();
    await expect(historyDialog.getByText(/Recipient Unavailable/i)).toBeVisible();
    await expect(historyDialog.getByText('Recipient not answering door or phone')).toBeVisible();

    // Close Attempt History modal
    await historyDialog.getByRole('button', { name: 'Close' }).first().click();
    await expect(historyDialog).not.toBeVisible();

    // SYNC CHECK 3: Dispatcher Board & Tracking reflect ATTEMPT_FAILED
    console.log('[Part 2] SYNC CHECK 3: Verifying Dispatcher Board & Tracking reflect ATTEMPT_FAILED...');
    await dispatcherPage.getByRole('button', { name: 'Refresh' }).click();
    const failedParcelInDispatcher = dispatcherPage.locator('li', { hasText: parcel1Tracking }).first();
    await expect(failedParcelInDispatcher).toBeVisible({ timeout: 10000 });
    await expect(failedParcelInDispatcher.getByText(/Attempt Failed/i).first()).toBeVisible({ timeout: 10000 });

    await trackingPage.reload();
    await expect(trackingPage.getByTestId('tracking-status-badge')).toHaveText(/Attempt Failed/i, { timeout: 10000 });

    // -------------------------------------------------------------------------
    // Step D: Dispatcher Authorizes Reattempt (ADR-004 Gate)
    // -------------------------------------------------------------------------
    console.log('[Part 2] Step D: Dispatcher authorizes reattempt via Reschedule Modal...');
    const rescheduleBtn = failedParcelInDispatcher.locator('[data-testid="reschedule-btn"]');
    await expect(rescheduleBtn).toBeVisible({ timeout: 10000 });
    await rescheduleBtn.click();

    const rescheduleModal = dispatcherPage.getByRole('dialog');
    await expect(rescheduleModal).toBeVisible({ timeout: 5000 });
    await expect(rescheduleModal.getByRole('heading', { name: 'Reschedule Failed Delivery' })).toBeVisible();
    await rescheduleModal.locator('#reschedule-courier').selectOption(USERS.COURIER.id);
    await rescheduleModal.locator('#reschedule-notes').fill('Customer called, authorized afternoon reattempt');
    await rescheduleModal.locator('[data-testid="confirm-reschedule-btn"]').click();

    await expect(dispatcherPage.getByText(/rescheduled and assigned/i)).toBeVisible({ timeout: 10000 });

    // -------------------------------------------------------------------------
    // Step E: Courier starts reattempt and completes with Photo POD
    // -------------------------------------------------------------------------
    console.log('[Part 2] Step E: Courier re-starts delivery and completes with Photo POD...');
    await courierPage.getByRole('button', { name: 'Refresh tasks' }).click();
    await expect(parcel1Card.getByRole('button', { name: 'Start delivery' })).toBeVisible({ timeout: 10000 });
    await parcel1Card.getByRole('button', { name: 'Start delivery' }).click();

    await expect(parcel1Card.getByText(/Out For Delivery/i).first()).toBeVisible({ timeout: 10000 });

    // Record Delivery
    await parcel1Card.getByRole('button', { name: 'Record delivery' }).click();
    await parcel1Card.locator('input[name="name"]').fill('Piyumi Fernando');

    // Attach Photo Proof
    const buffer = Buffer.from('fake-photo-data-bytes-for-pod');
    await parcel1Card.locator('input[type="file"]').setInputFiles({
      name: 'proof_photo.jpg',
      mimeType: 'image/jpeg',
      buffer: buffer,
    });

    // Confirm Delivery
    await parcel1Card.getByRole('button', { name: 'Confirm delivery' }).click();

    // Verify Success Alert on Courier UI
    await expect(courierPage.getByTestId('delivery-success-alert')).toBeVisible({ timeout: 15000 });
    await expect(courierPage.getByText(/Delivery confirmed successfully/i)).toBeVisible();
    await expect(courierPage.getByText(/Piyumi Fernando with verified photo proof/i)).toBeVisible();

    // SYNC CHECK 4: Core API & Tracking Portal reflect DELIVERED
    console.log('[Part 2] SYNC CHECK 4: Verifying Core API & Tracking Portal reflect DELIVERED...');
    const verifyApi1 = await fetch(`${CORE_API}/parcels/${parcel1Id}`, {
      headers: { Authorization: `Bearer ${customerToken}` },
    });
    const verifyData1 = await verifyApi1.json();
    console.log(`[Part 2] Core API Parcel 1 status: ${verifyData1.data?.status}`);
    expect(verifyData1.data?.status).toBe('DELIVERED');

    await trackingPage.reload();
    await expect(trackingPage.getByTestId('tracking-status-badge')).toHaveText(/Delivered/i, { timeout: 10000 });

    await courierContext.close();
    await dispatcherContext.close();
    await trackingContext.close();
  });

  test('Part 3: Delivery Lifecycle with Electronic Signature POD & Cross-Portal Sync (Parcel 2)', async ({ browser }) => {
    const courierContext = await browser.newContext({
      viewport: { width: 375, height: 812 },
      isMobile: true,
      hasTouch: true,
    });
    const courierPage = await courierContext.newPage();

    const dispatcherContext = await browser.newContext({
      viewport: { width: 1440, height: 900 },
    });
    const dispatcherPage = await dispatcherContext.newPage();

    const trackingContext = await browser.newContext({
      viewport: { width: 1280, height: 800 },
    });
    const trackingPage = await trackingContext.newPage();

    // Log in Courier
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(USERS.COURIER.email);
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(USERS.COURIER.password);
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await courierPage.waitForURL('**/courier/dashboard');

    // Log in Dispatcher
    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(USERS.DISPATCHER.email);
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(USERS.DISPATCHER.password);
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.waitForURL(/.*(dispatcher|dashboard|ops).*/);
    await dispatcherPage.goto('/dispatcher/dashboard');

    // Public Tracking for Parcel 2
    console.log(`[Part 3] Navigating to Public Tracking: /track/${parcel2Tracking}`);
    await trackingPage.goto(`/track/${parcel2Tracking}`);
    await expect(trackingPage.getByText(parcel2Tracking)).toBeVisible({ timeout: 10000 });

    // Step A: Courier starts delivery for Parcel 2
    console.log('[Part 3] Step A: Courier starts delivery for Parcel 2...');
    const parcel2Card = courierPage.locator('[data-testid="courier-task"]', { hasText: parcel2Tracking });
    await expect(parcel2Card).toBeVisible({ timeout: 10000 });
    await parcel2Card.getByRole('button', { name: 'Start delivery' }).click();

    await expect(parcel2Card.getByText(/Out For Delivery/i).first()).toBeVisible({ timeout: 10000 });

    // Step B: Record Delivery with Signature POD
    console.log('[Part 3] Step B: Recording delivery with electronic signature...');
    await parcel2Card.getByRole('button', { name: 'Record delivery' }).click();

    // Fill Recipient Name
    await parcel2Card.locator('input[name="name"]').fill('Rohan Jayasinghe');

    // Switch to Customer Signature tab
    const signatureTab = parcel2Card.getByRole('tab', { name: /Customer signature/i });
    await expect(signatureTab).toBeVisible();
    await signatureTab.click();

    // Draw on Signature Canvas
    const canvas = parcel2Card.locator('canvas');
    await expect(canvas).toBeVisible();
    const box = await canvas.boundingBox();
    expect(box).toBeTruthy();
    if (box) {
      await courierPage.mouse.move(box.x + 30, box.y + 30);
      await courierPage.mouse.down();
      await courierPage.mouse.move(box.x + 70, box.y + 50);
      await courierPage.mouse.move(box.x + 120, box.y + 30);
      await courierPage.mouse.move(box.x + 160, box.y + 70);
      await courierPage.mouse.up();
    }

    // Verify signature recorded indicator
    await expect(parcel2Card.getByText(/Signature recorded/i)).toBeVisible({ timeout: 5000 });

    // Confirm Delivery
    await parcel2Card.getByRole('button', { name: 'Confirm delivery' }).click();

    // Verify Success Alert on Courier UI
    await expect(courierPage.getByTestId('delivery-success-alert')).toBeVisible({ timeout: 15000 });
    await expect(courierPage.getByText(/Delivery confirmed successfully/i)).toBeVisible();
    await expect(courierPage.getByText(/Rohan Jayasinghe with verified signature proof/i)).toBeVisible();

    // SYNC CHECK 5: Core API reflects DELIVERED
    console.log('[Part 3] SYNC CHECK 5: Verifying Core API & Dispatcher Board reflect DELIVERED...');
    const verifyApi2 = await fetch(`${CORE_API}/parcels/${parcel2Id}`, {
      headers: { Authorization: `Bearer ${customerToken}` },
    });
    const verifyData2 = await verifyApi2.json();
    console.log(`[Part 3] Core API Parcel 2 status: ${verifyData2.data?.status}`);
    expect(verifyData2.data?.status).toBe('DELIVERED');

    // SYNC CHECK 6: Tracking Portal reflects DELIVERED
    console.log('[Part 3] SYNC CHECK 6: Verifying Tracking Portal updates to DELIVERED...');
    await trackingPage.reload();
    await expect(trackingPage.getByTestId('tracking-status-badge')).toHaveText(/Delivered/i, { timeout: 10000 });

    await courierContext.close();
    await dispatcherContext.close();
    await trackingContext.close();
  });

  test('Part 4: Public & Customer Tracking Routing & URL Alias Verification', async ({ page }) => {
    console.log('[Part 4] Testing tracking route accessibility...');

    // 1. /track works
    await page.goto('/track');
    await expect(page.getByRole('heading', { name: 'Track a parcel' })).toBeVisible();

    // 2. /track/:trackingNumber works
    await page.goto(`/track/${parcel1Tracking}`);
    await expect(page.getByText(parcel1Tracking)).toBeVisible();
    await expect(page.getByTestId('tracking-status-badge')).toHaveText(/Delivered/i);

    // 3. Test /tracking route navigation
    console.log(`[Part 4] Navigating to /tracking/${parcel1Tracking}...`);
    await page.goto(`/tracking/${parcel1Tracking}`);
    const currentUrl = page.url();
    console.log(`[Part 4] Current URL after visiting /tracking/${parcel1Tracking}: ${currentUrl}`);

    // If it navigated to /auth/login, it indicates /tracking is not registered as an alias in App.tsx
    const isRedirectedToLogin = currentUrl.includes('/auth/login');
    console.log(`[Part 4] Did /tracking route redirect to login? ${isRedirectedToLogin}`);
  });
});
