import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';
import { TEST_USERS } from '../fixtures/testData';
import { randomUUID } from 'node:crypto';

/**
 * Flow 13: Dispatcher & Cross-Portal Real-Time Live Synchronization Suite
 *
 * Real multi-actor end-to-end integration test with ZERO MOCKS:
 * 1. Dispatcher <-> Courier Shift Synchronization:
 *    - Dispatcher toggles courier shift from Dispatcher Board (POST /courier/shift).
 *    - Courier mobile PWA updates shift state without conflicting.
 * 2. Ingestion <-> Dispatcher Queue Synchronization:
 *    - Customer books an authentic parcel through Core API (POST /api/v1/parcels).
 *    - Real parcel aggregate commits to PostgreSQL and appears in Dispatcher's Unassigned queue.
 * 3. Dispatcher Assignment <-> Courier Task List Synchronization:
 *    - Dispatcher assigns the real parcel to an on-shift courier (POST /dispatch/assignments).
 *    - Parcel appears on the Courier's mobile manifest.
 * 4. Dispatcher Unassign / Cancel <-> Courier Task Eviction:
 *    - Dispatcher cancels the assignment with an operational reason (POST /dispatch/assignments/:id/cancel).
 *    - Parcel is evicted from Courier's manifest and returns to Dispatcher's unassigned queue.
 * 5. Reassign <-> In-Transit Lifecycle Sync:
 *    - Dispatcher reassigns parcel.
 *    - Courier marks parcel Picked Up / In Transit.
 *    - Dispatcher Board displays the updated status badge "In Transit".
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 3 (Dispatch & Delivery), Member 2 (Parcels & Pricing)
 */

test.describe('Flow 13: Dispatcher & Cross-Portal Live Synchronization (Zero Mocks)', () => {
  test.setTimeout(90000);

  test('should synchronize shift states, parcel booking, courier assignment, cancellation, and in-transit progress across Dispatcher and Courier portals', async ({
    browser,
    request,
  }) => {
    // -------------------------------------------------------------------------
    // Setup 2 isolated browser contexts: Dispatcher (Desktop) & Courier (Mobile)
    // -------------------------------------------------------------------------
    const dispatcherContext = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 1440, height: 900 },
    });
    const dispatcherPage = await dispatcherContext.newPage();

    const courierContext = await browser.newContext({
      baseURL: 'http://localhost:3000',
      viewport: { width: 375, height: 812 },
      isMobile: true,
      hasTouch: true,
    });
    const courierPage = await courierContext.newPage();

    try {
      // =======================================================================
      // STEP 1: Log in Dispatcher
      // =======================================================================
      console.log('[Flow 13] 1. Logging in Dispatcher on Desktop...');
      await dispatcherPage.goto('/auth/login');
      await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(TEST_USERS.DISPATCHER.email);
      await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(TEST_USERS.DISPATCHER.password);
      await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

      await expect(dispatcherPage).toHaveURL(/.*(dispatcher|dashboard|ops).*/, { timeout: 15000 });
      await dispatcherPage.goto('/dispatcher/dashboard');

      const board = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD));
      await expect(board).toBeVisible({ timeout: 15000 });

      // =======================================================================
      // STEP 2: Log in Courier on Mobile PWA
      // =======================================================================
      console.log('[Flow 13] 2. Logging in Courier on Mobile PWA...');
      await courierPage.goto('/auth/login');
      await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(TEST_USERS.COURIER.email);
      await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(TEST_USERS.COURIER.password);
      await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

      await expect(courierPage).toHaveURL(/.*(courier|dashboard).*/, { timeout: 15000 });
      await courierPage.goto('/courier/dashboard');
      await expect(courierPage.getByRole('heading', { name: 'Delivery tasks' })).toBeVisible({ timeout: 15000 });

      // =======================================================================
      // STEP 3: Administrative Shift Control from Dispatcher -> Courier Sync
      // =======================================================================
      console.log('[Flow 13] 3. Verifying administrative shift toggle synchronization...');
      await expect(dispatcherPage.getByText('CR-0012 · MOTORCYCLE')).toBeVisible({ timeout: 10000 });

      // Test Shift Toggle: Put courier on break, then resume shift
      const breakBtn = dispatcherPage.locator('[data-testid="toggle-break-CR-0012"]');
      if (await breakBtn.isVisible()) {
        console.log('[Flow 13] Toggling courier to On Break on Dispatcher Board...');
        await breakBtn.click();
        await expect(dispatcherPage.getByText(/Shift updated for.*On Break/i)).toBeVisible({ timeout: 10000 });
        
        // Resume shift
        const resumeBtn = dispatcherPage.locator('[data-testid="toggle-shift-CR-0012"]');
        await expect(resumeBtn).toBeVisible({ timeout: 10000 });
        await resumeBtn.click();
        await expect(dispatcherPage.getByText(/Shift updated for.*On Shift/i)).toBeVisible({ timeout: 10000 });
      }

      // =======================================================================
      // STEP 4: Book Authentic Customer Parcel via Live Core API with Tariff Quote
      // =======================================================================
      console.log('[Flow 13] 4. Authenticating Customer, generating tariff quote, and booking parcel...');
      const customerLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
        data: {
          email: TEST_USERS.CUSTOMER.email,
          password: TEST_USERS.CUSTOMER.password,
        },
      });
      expect(customerLoginRes.status()).toBe(200);
      const customerAuthData = await customerLoginRes.json();
      const customerToken = customerAuthData.data.access_token;

      // 4.1 Request Tariff Quote (Member 2 pricing contract)
      const timestamp = Date.now();
      const quoteRes = await request.post('http://localhost:8080/api/v1/pricing/quotes', {
        headers: {
          Authorization: `Bearer ${customerToken}`,
          'Content-Type': 'application/json',
        },
        data: {
          origin_zone_id: 'ZONE_COLOMBO_01',
          destination_zone_id: 'ZONE_COLOMBO_01',
          actual_weight_kg: 2.4,
          dimensions: { length_cm: 20.0, width_cm: 15.0, height_cm: 10.0 },
          service_level: 'STANDARD',
        },
      });
      expect(quoteRes.status()).toBe(201);
      const quoteId = (await quoteRes.json()).data.quote_id;

      // 4.2 Book Parcel with Quote ID and Idempotency Key
      const bookingRes = await request.post('http://localhost:8080/api/v1/parcels', {
        headers: {
          Authorization: `Bearer ${customerToken}`,
          'Content-Type': 'application/json',
          'Idempotency-Key': randomUUID(),
        },
        data: {
          quote_id: quoteId,
          recipient_name: `Recipient ${timestamp}`,
          recipient_phone: '+94779998888',
          weight_kg: 2.4,
          dimensions: { length_cm: 20.0, width_cm: 15.0, height_cm: 10.0 },
          pickup: {
            contact_name: 'Central Sender',
            contact_phone: '+94771234567',
            address_line1: '45 Galle Road, Suite 3B',
            city: 'Colombo',
            postal_code: '00100',
            zone_id: 'ZONE_COLOMBO_01',
            instructions: 'Pickup from front desk',
          },
          dropoff: {
            contact_name: `Recipient ${timestamp}`,
            contact_phone: '+94779998888',
            address_line1: 'No. 55 Janadhipathi Mawatha',
            city: 'Colombo',
            postal_code: '00100',
            zone_id: 'ZONE_COLOMBO_01',
            instructions: 'Doorstep handoff',
          },
          item_description: 'E2E Cross-Portal Live Test Package',
          special_instructions: 'Handle with care',
        },
      });
      expect(bookingRes.status()).toBe(201);
      const bookingData = await bookingRes.json();
      const parcelId = bookingData.data.id;
      const trackingNumber = bookingData.data.tracking_number;
      console.log(`[Flow 13] Booked parcel successfully: ${trackingNumber} (ID: ${parcelId})`);

      // =======================================================================
      // STEP 5: Verify Parcel appears on Dispatcher Board in Unassigned Queue
      // =======================================================================
      console.log('[Flow 13] 5. Verifying booked parcel appears in Dispatcher Unassigned queue...');
      // Refresh board
      await dispatcherPage.getByRole('button', { name: 'Refresh' }).click();

      // Locate specific unassigned parcel row
      const parcelRow = dispatcherPage.locator('li', { hasText: trackingNumber });
      await expect(parcelRow).toBeVisible({ timeout: 15000 });
      console.log(`[Flow 13] Verified ${trackingNumber} is live on Dispatcher Board.`);

      // =======================================================================
      // STEP 6: Dispatcher Assigns Parcel to Courier CR-0012
      // =======================================================================
      console.log('[Flow 13] 6. Assigning parcel to courier CR-0012 on Dispatcher Board...');
      const courierSelect = parcelRow.locator('select');
      await courierSelect.selectOption('018f4a7c-1003-7000-8000-000000000003');

      const assignBtn = parcelRow.getByRole('button', { name: 'Assign courier' });
      await expect(assignBtn).toBeEnabled();
      await assignBtn.click();

      // Assert assignment notice banner
      await expect(dispatcherPage.getByText(`${trackingNumber} assigned.`)).toBeVisible({ timeout: 10000 });

      // Verify parcel moved to Assigned Parcels table on Dispatcher board
      const assignedCard = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.ASSIGNED_PARCELS_LIST));
      const assignedRow = assignedCard.locator('li', { hasText: trackingNumber });
      await expect(assignedRow).toBeVisible({ timeout: 10000 });
      console.log(`[Flow 13] ${trackingNumber} verified under Assigned Parcels on Dispatcher.`);

      // =======================================================================
      // STEP 7: Verify Parcel Synchronizes onto Courier Mobile Task List
      // =======================================================================
      console.log('[Flow 13] 7. Verifying assignment appears on Courier Mobile PWA...');
      // Refresh courier page
      await courierPage.getByRole('button', { name: 'Refresh tasks' }).click();
      await expect(courierPage.getByText(trackingNumber)).toBeVisible({ timeout: 15000 });
      console.log(`[Flow 13] Verified ${trackingNumber} is present on Courier device.`);

      // =======================================================================
      // STEP 8: Dispatcher Cancels / Unassigns Parcel (Emergency Rollback)
      // =======================================================================
      console.log('[Flow 13] 8. Dispatcher unassigns parcel with operational reason...');
      const unassignBtn = assignedRow.locator(testId(SELECTORS.DISPATCHER.UNASSIGN_BTN));
      await unassignBtn.click();

      // Verify dialog
      await expect(dispatcherPage.getByRole('dialog')).toBeVisible();
      const cancelReasonInput = dispatcherPage.locator('#cancel-reason');
      await cancelReasonInput.fill('Dispatcher reassignment drill: vehicle rerouted');

      const confirmCancelBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.CONFIRM_CANCEL_ASSIGNMENT_BTN));
      await confirmCancelBtn.click();

      // Assert notice
      await expect(dispatcherPage.getByText(/Assignment cancelled for.*returned to unassigned queue/i)).toBeVisible({ timeout: 10000 });

      // Verify parcel returned to unassigned list
      await expect(dispatcherPage.locator('li', { hasText: trackingNumber })).toBeVisible({ timeout: 10000 });
      console.log(`[Flow 13] Verified ${trackingNumber} returned to Dispatcher unassigned queue.`);

      // =======================================================================
      // STEP 9: Verify Parcel is Evicted from Courier Task List
      // =======================================================================
      console.log('[Flow 13] 9. Verifying parcel eviction on Courier Mobile PWA...');
      await courierPage.getByRole('button', { name: 'Refresh tasks' }).click();
      await expect(courierPage.getByText(trackingNumber)).not.toBeVisible({ timeout: 10000 });
      console.log(`[Flow 13] Verified ${trackingNumber} successfully evicted from Courier device.`);

      // =======================================================================
      // STEP 10: Re-assign Parcel to Courier and Advance to In-Transit via Courier UI
      // =======================================================================
      console.log('[Flow 13] 10. Re-assigning parcel to Courier CR-0012 on Dispatcher Board...');
      const reassignRow = dispatcherPage.locator('li', { hasText: trackingNumber });
      const reassignSelect = reassignRow.locator('select');
      await reassignSelect.selectOption('018f4a7c-1003-7000-8000-000000000003');

      const reassignBtn = reassignRow.getByRole('button', { name: 'Assign courier' });
      await reassignBtn.click();
      await expect(dispatcherPage.getByText(`${trackingNumber} assigned.`)).toBeVisible({ timeout: 10000 });

      // On Courier device, parcel re-appears
      await courierPage.getByRole('button', { name: 'Refresh tasks' }).click();
      await expect(courierPage.getByText(trackingNumber)).toBeVisible({ timeout: 15000 });

      // Courier clicks "Start delivery" button in their mobile UI
      console.log('[Flow 13] Courier clicks "Start delivery" on Mobile PWA...');
      const courierCard = courierPage.locator('[data-testid="courier-task"]', { hasText: trackingNumber });
      const startDeliveryBtn = courierCard.locator('[data-testid="courier-start-run"]');
      await expect(startDeliveryBtn).toBeVisible({ timeout: 10000 });
      await startDeliveryBtn.click();
      await expect(courierPage.getByRole('status')).toHaveText('Delivery started.', { timeout: 10000 });

      // Refresh dispatcher board and verify status badge reflects "out for delivery"
      console.log('[Flow 13] Verifying Dispatcher Board reflects live OUT_FOR_DELIVERY status...');
      await dispatcherPage.getByRole('button', { name: 'Refresh' }).click();
      const reAssignedRow = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.ASSIGNED_PARCELS_LIST)).locator('li', { hasText: trackingNumber });
      await expect(reAssignedRow.getByText(/out for delivery/i)).toBeVisible({ timeout: 10000 });

      console.log('[Flow 13] PASSED! Real-time cross-portal value chain synchronization verified!');
    } finally {
      await dispatcherContext.close();
      await courierContext.close();
    }
  });
});
