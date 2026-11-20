import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Flow 3: Courier Mobile Run, Transit Milestones & Digital Proof of Delivery (POD)
 *
 * Invariants:
 * - Operates under mobile viewport (Pixel 7 / 375x812) with touch simulation.
 * - Transitions stop statuses: ASSIGNED -> OUT_FOR_DELIVERY -> DELIVERED.
 * - Enforces mandatory Proof of Delivery before transitioning to DELIVERED.
 * - POD capture records recipient electronic signature and updates task item.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Flow 3: Courier Mobile Run & POD Capture E2E Flow', () => {
  test('should execute stop-by-stop mobile transit milestones and complete electronic signature POD', async ({
    courierPage,
  }) => {
    let taskStatus = 'ASSIGNED';

    // 1. Mock Courier Authentication & Session Refresh
    await courierPage.route('**/api/v1/auth/refresh', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-courier-jwt-token',
            user: {
              id: 'cr010000-0000-0000-0000-000000000001',
              email: 'courier@swiftdrop.io',
              role: 'COURIER',
              full_name: 'Saman Kumara',
            },
          },
        }),
      });
    });

    await courierPage.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-courier-jwt-token',
            refresh_token: 'mock-courier-refresh-token',
            user: {
              id: 'cr010000-0000-0000-0000-000000000001',
              email: 'courier@swiftdrop.io',
              role: 'COURIER',
              full_name: 'Saman Kumara',
            },
          },
        }),
      });
    });

    // 2. Mock Courier Tasks & Operations Endpoints
    await courierPage.route('**/api/v1/courier/parcels', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              parcel_id: 'pcl-courier-001',
              tracking_number: 'SD-20261030-99801',
              current_status: taskStatus,
              recipient_name: 'Kasun Wickramasinghe',
              recipient_phone: '+94771234567',
              delivery_street: '45 Galle Road',
              delivery_city: 'Colombo 03',
              handling_instructions: 'Fragile electronics',
            },
          ],
        }),
      });
    });

    await courierPage.route('**/api/v1/courier/parcels/**/start-run', async (route) => {
      taskStatus = 'OUT_FOR_DELIVERY';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { success: true, status: 'OUT_FOR_DELIVERY' } }),
      });
    });

    await courierPage.route('**/api/v1/deliveries/**/complete', async (route) => {
      taskStatus = 'DELIVERED';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { success: true, parcel_status: 'DELIVERED' } }),
      });
    });

    // 3. Perform Courier Sign-In
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('courier@swiftdrop.io');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // 4. Navigate to Courier Mobile Dashboard
    await courierPage.goto('/courier/dashboard');

    // Verify Mobile Route Header & Assigned Tasks list
    await expect(courierPage.getByRole('heading', { name: 'Delivery tasks' })).toBeVisible();
    await expect(courierPage.getByText('SD-20261030-99801')).toBeVisible();

    // Step A: Advance task from ASSIGNED to OUT_FOR_DELIVERY
    const startDeliveryBtn = courierPage.getByRole('button', { name: 'Start delivery' });
    await expect(startDeliveryBtn).toBeVisible();
    await startDeliveryBtn.click();

    // Step B: Trigger "Record delivery" to open Proof of Delivery modal/panel
    const recordDeliveryBtn = courierPage.getByRole('button', { name: 'Record delivery' });
    await expect(recordDeliveryBtn).toBeVisible();
    await recordDeliveryBtn.click();

    // Step C: Verify POD Form is displayed and select Signature tab
    const nameInput = courierPage.locator('input[name="name"]');
    await expect(nameInput).toBeVisible();
    await nameInput.fill('Kasun Wickramasinghe');

    const signatureTab = courierPage.getByRole('tab', { name: /Customer signature/i });
    await expect(signatureTab).toBeVisible();
    await signatureTab.click();

    // Draw electronic signature on the canvas
    const canvas = courierPage.locator('canvas');
    await expect(canvas).toBeVisible();
    const box = await canvas.boundingBox();
    if (box) {
      await courierPage.mouse.move(box.x + 20, box.y + 20);
      await courierPage.mouse.down();
      await courierPage.mouse.move(box.x + 60, box.y + 40);
      await courierPage.mouse.move(box.x + 100, box.y + 20);
      await courierPage.mouse.move(box.x + 140, box.y + 60);
      await courierPage.mouse.up();
    }

    await expect(courierPage.getByText(/Signature recorded/i)).toBeVisible();

    // Confirm Delivery with digital signature
    const confirmDeliveryBtn = courierPage.getByRole('button', { name: 'Confirm delivery' });
    await expect(confirmDeliveryBtn).toBeVisible();
    await confirmDeliveryBtn.click();

    // Step D: Verify Delivery confirmed alert/notice
    await expect(courierPage.getByText('Delivery confirmed successfully')).toBeVisible();
    await expect(courierPage.getByTestId('delivery-success-alert')).toContainText('Kasun Wickramasinghe');
  });
});
