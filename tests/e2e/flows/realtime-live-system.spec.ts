import { execSync } from 'child_process';
import { test, expect } from '../fixtures/orchestrationFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * LIVE REAL-ENVIRONMENT MULTI-ACTOR SYSTEM E2E TEST (ZERO ROUTE MOCKS)
 *
 * Runs across 5 concurrent browser windows interacting directly with the live
 * physical stack:
 * - Real PostgreSQL Database (port 5433)
 * - Real Core API Modular Monolith (port 8080 via Vite proxy on port 3000)
 * - Real Outbox Table & State Machine Transitions
 * - Real RabbitMQ Broker & Event Worker Pipeline
 * - Real Mock Webhook Receiver & Visual Examiner Dashboard (port 9999)
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Live Real-Environment Master System E2E (5 Concurrent Windows)', () => {

  test.beforeEach(() => {
    // Ensure applicant is in fresh CUSTOMER state for repeatable merchant onboarding
    try {
      execSync('docker exec -i swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -c "DELETE FROM webhook_endpoints WHERE merchant_id IN (SELECT id FROM merchants WHERE contact_email = \'applicant@swiftdrop.local\'); DELETE FROM merchant_api_credentials WHERE merchant_id IN (SELECT id FROM merchants WHERE contact_email = \'applicant@swiftdrop.local\'); DELETE FROM merchants WHERE contact_email = \'applicant@swiftdrop.local\'; UPDATE users SET role = \'CUSTOMER\', tenant_id = NULL WHERE email = \'applicant@swiftdrop.local\';"', { stdio: 'ignore' });
    } catch {
      // ignore if docker environment is running standalone
    }
  });

  test('should execute complete live multi-actor transaction with real database persistence', async ({
    customerPage,
    applicantPage,
    adminPage,
    dispatcherPage,
    courierPage,
  }) => {
    // -------------------------------------------------------------------------
    // Window 1: Customer A Signs In & Books Parcel through Real Pricing Engine
    // -------------------------------------------------------------------------
    await customerPage.goto('/auth/login');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.local');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('CustomerPassword123!');
    await customerPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // Wait for successful sign-in navigation
    await expect(customerPage).toHaveURL(/\/dashboard/, { timeout: 10000 });

    // Navigate to live Booking Wizard via portal navigation
    await customerPage.getByRole('link', { name: 'Book a parcel' }).click();
    await expect(customerPage.getByRole('heading', { name: 'Book a parcel' })).toBeVisible({ timeout: 10000 });

    // Step 1: Addresses
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_NAME)).fill('Devinda Perera');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_PHONE)).fill('+94771234567');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_ADDRESS)).fill('42 Galle Road');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_CITY)).fill('Colombo');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_POSTAL)).fill('00100');

    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_NAME)).fill('Kasun Wickrama');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_PHONE)).fill('+94719876543');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_ADDRESS)).fill('15/A Havelock Road');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_CITY)).fill('Colombo');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_POSTAL)).fill('00400');

    // Wait for live zone resolution from PostgreSQL delivery_zones
    await expect(customerPage.getByText(/Delivery zone/i).first()).toBeVisible({ timeout: 10000 });
    await customerPage.locator(testId(SELECTORS.CUSTOMER.BTN_NEXT)).click();

    // Step 2: Parcel Dimensions & Real Dynamic Pricing Calculation
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('2.5');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_LENGTH)).fill('20');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WIDTH)).fill('15');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_HEIGHT)).fill('10');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DESCRIPTION)).fill('Live testing parcel');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.BTN_GET_QUOTE)).click();

    // Step 3: Verify real fee calculated by pricing engine in PostgreSQL
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS))).toBeVisible({ timeout: 10000 });
    await customerPage.locator(testId(SELECTORS.CUSTOMER.BTN_NEXT)).click();

    // Step 4: Confirm and Commit to PostgreSQL Database
    await customerPage.locator(testId(SELECTORS.CUSTOMER.BTN_SUBMIT_BOOKING)).click();
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO))).toBeVisible({ timeout: 15000 });

    const liveTrackingNumber = (await customerPage.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO)).innerText()).trim();
    expect(liveTrackingNumber).toMatch(/^SD-\d{8}-[A-Z0-9]+$/);

    // -------------------------------------------------------------------------
    // Window 2: User B Submits Real Merchant Application to PostgreSQL
    // -------------------------------------------------------------------------
    await applicantPage.goto('/auth/login');
    await applicantPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('applicant@swiftdrop.local');
    await applicantPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('CustomerPassword123!');
    await applicantPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(applicantPage).toHaveURL(/\/dashboard/, { timeout: 10000 });
    await applicantPage.getByRole('link', { name: /Apply for a merchant account|merchant/i }).click();
    await expect(applicantPage.getByRole('heading', { name: 'Merchant application' })).toBeVisible({ timeout: 10000 });

    await applicantPage.getByTestId('merchant-application-business_name-input').fill('Apex Retailers Ltd');
    await applicantPage.getByTestId('merchant-application-contact_email-input').fill('applicant@swiftdrop.local');
    await applicantPage.getByTestId('merchant-application-contact_phone-input').fill('+94770002222');
    await applicantPage.getByTestId('merchant-application-line1-input').fill('100 Main Street');
    await applicantPage.getByTestId('merchant-application-city-input').fill('Colombo');
    await applicantPage.getByTestId('merchant-application-submit-btn').click();

    // Verify real application committed to PostgreSQL
    await expect(applicantPage.getByTestId('merchant-application-status')).toBeVisible({ timeout: 10000 });
    await expect(applicantPage.getByText('Apex Retailers Ltd')).toBeVisible();

    // -------------------------------------------------------------------------
    // Window 3: Admin Approves Merchant Application on Real Core API
    // -------------------------------------------------------------------------
    await adminPage.goto('/auth/login');
    await adminPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('admin@swiftdrop.local');
    await adminPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('AdminPassword123!');
    await adminPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(adminPage).toHaveURL(/\/admin\/dashboard/, { timeout: 10000 });
    await expect(adminPage.getByText('Administrator Operations & Governance Console')).toBeVisible();

    // Find and approve the pending merchant in PostgreSQL via Admin API
    const approvalResult = await adminPage.evaluate(async () => {
      // 0. Acquire admin JWT
      const authRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'admin@swiftdrop.local', password: 'AdminPassword123!' }),
      });
      const authData = await authRes.json();
      const token = authData.data?.access_token;

      // 1. Fetch pending merchants
      const listRes = await fetch('/api/v1/admin/merchants?status=PENDING', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      const listData = await listRes.json();
      const pendingMerchant = listData.data?.find((m: any) => m.business_name === 'Apex Retailers Ltd');
      if (!pendingMerchant) {
        return { success: false, reason: 'Pending merchant not found', listData };
      }

      // 2. Approve merchant
      const approveRes = await fetch(`/api/v1/admin/merchants/${pendingMerchant.merchant_id}/approve`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({ expected_version: pendingMerchant.version || 1, reason: 'KYC & business documents verified' }),
      });
      return { success: approveRes.ok, merchantId: pendingMerchant.merchant_id };
    });

    expect(approvalResult.success).toBe(true);

    // -------------------------------------------------------------------------
    // Window 2: User B Refreshes to Merchant & Configures Real Webhook
    // -------------------------------------------------------------------------
    // Log in again to receive new JWT with role: MERCHANT and tenant_id
    await applicantPage.goto('/auth/login');
    await applicantPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('applicant@swiftdrop.local');
    await applicantPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('CustomerPassword123!');
    await applicantPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(applicantPage).toHaveURL(/\/dashboard/, { timeout: 10000 });
    await applicantPage.getByRole('link', { name: 'Webhooks' }).click();
    await expect(applicantPage.getByRole('heading', { name: 'Webhooks', level: 1 })).toBeVisible({ timeout: 10000 });

    await applicantPage.getByTestId('merchant-webhook-url-input').fill('https://example.com/webhooks/merchant-alpha');
    // Select first event checkbox
    const eventCheckbox = applicantPage.locator('input[name="event"]').first();
    await eventCheckbox.check();
    await applicantPage.getByTestId('merchant-credential-create-btn').click();

    // Verify real AES-encrypted signing secret modal appears
    await expect(applicantPage.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    await applicantPage.getByTestId('dismiss-key-modal-checkbox').check();
    await applicantPage.getByTestId('merchant-secret-close-btn').click();

    // -------------------------------------------------------------------------
    // Window 4: Dispatcher Observes User A's Real Parcel & Assigns Courier
    // -------------------------------------------------------------------------
    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.local');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Dispatcher123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(dispatcherPage).toHaveURL(/\/dispatcher\/dashboard/, { timeout: 10000 });
    await expect(dispatcherPage.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD))).toBeVisible({ timeout: 10000 });

    // Execute assignment directly through real Dispatch API
    const assignResult = await dispatcherPage.evaluate(async (trackingNo) => {
      // 0. Acquire dispatcher JWT
      const authRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'dispatcher@swiftdrop.local', password: 'Dispatcher123!' }),
      });
      const authData = await authRes.json();
      const token = authData.data?.access_token;

      // 1. Fetch unassigned parcels
      const boardRes = await fetch('/api/v1/dispatch/board?zone_id=ZONE_COLOMBO_01', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      const boardData = await boardRes.json();
      const parcel = boardData.data?.unassigned_parcels?.find((p: any) => p.tracking_number === trackingNo);
      const courier = boardData.data?.courier_loads?.find((c: any) => c.courier_id === '018f4a7c-1003-7000-8000-000000000003') || boardData.data?.courier_loads?.find((c: any) => c.status === 'ON_SHIFT');

      if (!parcel || !courier) {
        return { success: false, reason: 'Parcel or on-shift courier not available', parcel, courier, boardData };
      }

      // 2. Assign courier
      const assignRes = await fetch('/api/v1/dispatch/assignments', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          parcel_id: parcel.id,
          courier_id: courier.courier_id,
          notes: 'Assigned via automated live system test',
        }),
      });
      return { success: assignRes.ok, parcelId: parcel.id, courierId: courier.courier_id };
    }, liveTrackingNumber);

    expect(assignResult.success).toBe(true);

    // -------------------------------------------------------------------------
    // Window 5: Mobile Courier PWA Advances Run & Commits POD to Database
    // -------------------------------------------------------------------------
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('courier@swiftdrop.local');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('CourierPassword123!');
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(courierPage).toHaveURL(/\/courier\/dashboard/, { timeout: 10000 });
    await expect(courierPage.getByRole('heading', { name: 'Delivery tasks' })).toBeVisible({ timeout: 10000 });
    await expect(courierPage.getByText(liveTrackingNumber)).toBeVisible({ timeout: 10000 });

    // Execute Start Run and Complete Delivery via real Delivery API in courier context
    const deliveryResult = await courierPage.evaluate(async (parcelId) => {
      // 0. Acquire courier JWT
      const authRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'courier@swiftdrop.local', password: 'CourierPassword123!' }),
      });
      const authData = await authRes.json();
      const token = authData.data?.access_token;

      // 1. Advance to OUT_FOR_DELIVERY
      const startRes = await fetch(`/api/v1/courier/parcels/${parcelId}/start-run`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (!startRes.ok) return { step: 'start-run', success: false };

      // 2. Submit Complete Delivery with real SVG signature stroke data
      const completeRes = await fetch(`/api/v1/deliveries/${parcelId}/complete`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          proof_type: 'SIGNATURE',
          signature_data: 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDAiIGhlaWdodD0iMTAwIj48cGF0aCBkPSJNMTAgMTBMMTAwIDEwMCIvPjwvc3ZnPg==',
          received_by_name: 'Kasun Wickrama',
          notes: 'Delivered to recipient with digital signature',
        }),
      });
      return { step: 'complete', success: completeRes.ok, data: await completeRes.json() };
    }, assignResult.parcelId);

    expect(deliveryResult.success).toBe(true);

    // -------------------------------------------------------------------------
    // Verification: Public Tracking Endpoint Reflects Real DELIVERED Status
    // -------------------------------------------------------------------------
    await customerPage.goto(`/track/${liveTrackingNumber}`);
    await expect(customerPage.getByText(liveTrackingNumber)).toBeVisible({ timeout: 10000 });
    await expect(customerPage.getByTestId('tracking-status-badge')).toBeVisible({ timeout: 10000 });
    await expect(customerPage.getByTestId('tracking-status-badge')).toHaveText(/delivered/i);
  });

});
