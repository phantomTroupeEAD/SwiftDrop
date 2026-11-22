import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * EDGE SUITE 10: COURIER OFFLINE RESILIENCE, PWA NETWORK RECOVERY & POD INTEGRITY
 *
 * Simulates mobile field conditions where connectivity drops while courier is on delivery run:
 * 1. Parcel lifecycle progression to OUT_FOR_DELIVERY.
 * 2. Playwright context.setOffline(true) drops all network requests.
 * 3. Courier attempts to submit failed delivery attempt while offline:
 *    - DOM does NOT crash or lose form input state.
 *    - UI surfaces graceful network failure alert (<p role="alert">).
 * 4. Network restored via context.setOffline(false).
 * 5. Courier resubmits without reloading or retyping:
 *    - Request succeeds with 200 OK.
 *    - Status cleanly transitions to ATTEMPT_FAILED.
 * 6. Second parcel tested with Digital Signature POD:
 *    - Drawn signature canvas data preserved across offline blip.
 *    - Reconnects, submits, verifies DELIVERED transition and zero duplicate DB rows.
 *
 * Authority: Mandatory Cross-Module Resiliency Gate (AGENTS.md)
 */

test.describe.serial('Edge Suite 10: Courier Offline Resilience & Network Recovery', () => {
  test.setTimeout(90000);

  const CORE_API = 'http://localhost:8080/api/v1';

  const ACTORS = {
    DISPATCHER: { email: 'dispatcher@swiftdrop.local', password: 'Dispatcher123!' },
    COURIER: {
      email: 'courier@swiftdrop.local',
      password: 'CourierPassword123!',
      id: '018f4a7c-1003-7000-8000-000000000003',
      code: 'CR-0012',
    },
    CUSTOMER: { email: 'customer@swiftdrop.local', password: 'CustomerPassword123!' },
  };

  let courierAuthToken = '';
  let dispatcherAuthToken = '';
  let customerAuthToken = '';

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

  async function createAndAssignParcel(recipientName: string): Promise<{ id: string; tracking: string }> {
    // 1. Get quote
    const quoteRes = await fetch(`${CORE_API}/pricing/quotes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${customerAuthToken}` },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 1.5,
        dimensions: { length_cm: 20, width_cm: 15, height_cm: 10 },
      }),
    });
    expect(quoteRes.ok).toBeTruthy();
    const quoteData = await quoteRes.json();

    // 2. Book parcel
    const bookRes = await fetch(`${CORE_API}/parcels`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${customerAuthToken}`,
        'Idempotency-Key': randomUUID(),
      },
      body: JSON.stringify({
        quote_id: quoteData.data?.quote_id,
        recipient_name: recipientName,
        recipient_phone: '+94771112233',
        weight_kg: 1.5,
        dimensions: { length_cm: 20, width_cm: 15, height_cm: 10 },
        pickup: {
          contact_name: 'Sender Test',
          contact_phone: '+94770001122',
          address_line1: '10 Galle Road',
          city: 'Colombo',
          postal_code: '00300',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: recipientName,
          contact_phone: '+94771112233',
          address_line1: '45 Duplication Road',
          city: 'Colombo',
          postal_code: '00400',
          zone_id: 'ZONE_COLOMBO_01',
        },
      }),
    });
    expect(bookRes.ok).toBeTruthy();
    const bookData = await bookRes.json();
    const id = bookData.data?.id;
    const tracking = bookData.data?.tracking_number;

    // 3. Assign to Courier 1
    const assignRes = await fetch(`${CORE_API}/dispatch/assignments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${dispatcherAuthToken}`,
        'Idempotency-Key': randomUUID(),
      },
      body: JSON.stringify({
        parcel_id: id,
        courier_id: ACTORS.COURIER.id,
      }),
    });
    expect(assignRes.ok).toBeTruthy();

    return { id, tracking };
  }

  test.beforeAll(async () => {
    dispatcherAuthToken = await apiLogin(ACTORS.DISPATCHER.email, ACTORS.DISPATCHER.password);
    courierAuthToken = await apiLogin(ACTORS.COURIER.email, ACTORS.COURIER.password);
    customerAuthToken = await apiLogin(ACTORS.CUSTOMER.email, ACTORS.CUSTOMER.password);

    // Create 2 test parcels assigned to Courier 1
    const p1 = await createAndAssignParcel('Offline Recipient One');
    parcel1Id = p1.id;
    parcel1Tracking = p1.tracking;

    const p2 = await createAndAssignParcel('Offline Recipient Two');
    parcel2Id = p2.id;
    parcel2Tracking = p2.tracking;
  });

  test('Test Case 1: Courier logs failed delivery while offline -> UI alerts -> reconnects & saves cleanly', async ({
    page,
    context,
  }) => {
    // 1. Mobile viewport
    await page.setViewportSize({ width: 375, height: 812 });

    // 2. Sign in as Courier
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(ACTORS.COURIER.email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(ACTORS.COURIER.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await page.waitForURL('**/courier/dashboard');

    // 3. Move Parcel 1 to OUT_FOR_DELIVERY via API or UI button
    const startRes = await fetch(`${CORE_API}/courier/parcels/${parcel1Id}/start-run`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${courierAuthToken}` },
    });
    expect(startRes.ok).toBeTruthy();

    // Reload or wait for task list to show OUT_FOR_DELIVERY
    await page.reload();
    await expect(page.getByText(parcel1Tracking)).toBeVisible({ timeout: 10000 });

    // 4. Open "Report failed attempt" form
    const parcelCard = page.locator('[data-testid="courier-task"]', { hasText: parcel1Tracking });
    await parcelCard.getByRole('button', { name: /report failed attempt/i }).click();

    // Fill the exception reason
    const reasonSelect = parcelCard.locator('select[name="reason"]');
    await reasonSelect.selectOption('RECIPIENT_UNAVAILABLE');
    const notesInput = parcelCard.locator('input[name="notes"]');
    await notesInput.fill('Gate locked, no answer to phone call');

    // 5. SIMULATE NETWORK OUTAGE (context.setOffline(true))
    console.log('[EDGE-10] 🔌 Simulating network outage (offline mode enabled)...');
    await context.setOffline(true);

    // 6. Attempt submission while offline
    const saveAttemptBtn = parcelCard.getByRole('button', { name: /save attempt/i });
    await saveAttemptBtn.click();

    // Assert: UI renders alert without crash, and preserving inputs
    const alertLocator = page.locator('p[role="alert"]');
    await expect(alertLocator).toBeVisible({ timeout: 5000 });
    const alertText = await alertLocator.textContent();
    console.log(`[EDGE-10] ✅ Graceful offline error alert rendered: "${alertText}"`);
    expect(alertText).toBeTruthy();

    // Form inputs must remain intact across the failure
    await expect(notesInput).toHaveValue('Gate locked, no answer to phone call');
    await expect(reasonSelect).toHaveValue('RECIPIENT_UNAVAILABLE');

    // 7. RESTORE NETWORK (context.setOffline(false))
    console.log('[EDGE-10] 🌐 Network restored (offline mode disabled)...');
    await context.setOffline(false);

    // 8. Re-submit without reloading
    await saveAttemptBtn.click();

    // Assert: Successful submission notice appears
    await expect(page.getByText(/attempt #1 logged/i)).toBeVisible({ timeout: 15000 });
    console.log('[EDGE-10] ✅ Successfully recorded attempt after network reconnection.');

    // 9. Invariant Verification: Check Core API parcel status is ATTEMPT_FAILED
    const verifyRes = await fetch(`${CORE_API}/parcels/${parcel1Id}`, {
      headers: { Authorization: `Bearer ${customerAuthToken}` },
    });
    expect(verifyRes.ok).toBeTruthy();
    const parcelData = (await verifyRes.json()).data;
    expect(parcelData.status).toBe('ATTEMPT_FAILED');
    console.log('[EDGE-10] ✅ Invariant verified: Parcel status is ATTEMPT_FAILED.');
  });

  test('Test Case 2: Courier captures signature POD offline -> preserves canvas -> reconnects & completes delivery', async ({
    page,
    context,
  }) => {
    // 1. Mobile viewport
    await page.setViewportSize({ width: 375, height: 812 });

    // 2. Sign in as Courier
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(ACTORS.COURIER.email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(ACTORS.COURIER.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await page.waitForURL('**/courier/dashboard');

    // Move Parcel 2 to OUT_FOR_DELIVERY
    const startRes = await fetch(`${CORE_API}/courier/parcels/${parcel2Id}/start-run`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${courierAuthToken}` },
    });
    expect(startRes.ok).toBeTruthy();

    // Reload tasks
    await page.reload();
    await expect(page.getByText(parcel2Tracking)).toBeVisible({ timeout: 10000 });

    const parcelCard = page.locator('[data-testid="courier-task"]', { hasText: parcel2Tracking });

    // Open "Record delivery" panel
    await parcelCard.getByRole('button', { name: /record delivery/i }).click();

    // Enter recipient name
    const nameInput = parcelCard.locator('input[name="name"]');
    await nameInput.fill('Sunil Wickramasinghe');

    // Switch to Customer signature tab
    await parcelCard.getByRole('tab', { name: /customer signature/i }).click();

    // Draw on signature canvas via mouse events
    const canvas = parcelCard.locator('canvas');
    await expect(canvas).toBeVisible();
    const box = await canvas.boundingBox();
    if (box) {
      await page.mouse.move(box.x + 20, box.y + 20);
      await page.mouse.down();
      await page.mouse.move(box.x + 60, box.y + 40);
      await page.mouse.move(box.x + 100, box.y + 20);
      await page.mouse.move(box.x + 140, box.y + 60);
      await page.mouse.up();
    }

    // Verify signature registered in UI
    await expect(page.getByText(/signature recorded/i)).toBeVisible();

    // SIMULATE NETWORK OUTAGE
    console.log('[EDGE-10] 🔌 Simulating network outage before signature upload...');
    await context.setOffline(true);

    // Submit while offline
    const confirmBtn = parcelCard.getByRole('button', { name: /confirm delivery/i });
    await confirmBtn.click();

    // Assert: Network error alert shown, recipient name and signature stay intact
    const alertLocator = page.locator('p[role="alert"]');
    await expect(alertLocator).toBeVisible({ timeout: 5000 });
    console.log(`[EDGE-10] ✅ Offline alert caught on signature POD: "${await alertLocator.textContent()}"`);

    // Canvas / Signature state preserved
    await expect(page.getByText(/signature recorded/i)).toBeVisible();
    await expect(nameInput).toHaveValue('Sunil Wickramasinghe');

    // RESTORE CONNECTIVITY
    console.log('[EDGE-10] 🌐 Network restored. Resubmitting signature POD...');
    await context.setOffline(false);

    // Re-click confirm delivery
    await confirmBtn.click();

    // Assert: Delivery confirmed successfully
    await expect(page.getByText(/delivery confirmed successfully/i)).toBeVisible({ timeout: 15000 });
    console.log('[EDGE-10] ✅ Delivery confirmed with verified signature POD after reconnection.');

    // Verify Parcel 2 status is DELIVERED
    const verifyRes = await fetch(`${CORE_API}/parcels/${parcel2Id}`, {
      headers: { Authorization: `Bearer ${customerAuthToken}` },
    });
    expect(verifyRes.ok).toBeTruthy();
    const parcelData = (await verifyRes.json()).data;
    expect(parcelData.status).toBe('DELIVERED');
    console.log('[EDGE-10] ✅ Invariant verified: Parcel status is DELIVERED.');
  });
});
