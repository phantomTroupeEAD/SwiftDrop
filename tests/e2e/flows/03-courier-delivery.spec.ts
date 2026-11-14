import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Flow 3: Courier Mobile Run, Transit Milestones & Digital Proof of Delivery (POD)
 *
 * Invariants:
 * - Operates under mobile viewport (Pixel 7 / 375x812) with touch simulation.
 * - Transitions stop statuses: ASSIGNED -> PICKED_UP -> OUT_FOR_DELIVERY.
 * - Enforces mandatory Proof of Delivery before transitioning to DELIVERED.
 * - POD capture records recipient electronic signature and updates task item.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Flow 3: Courier Mobile Run & POD Capture E2E Flow', () => {
  test('should execute stop-by-stop mobile transit milestones and complete electronic signature POD', async ({
    courierPage,
  }) => {
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

    // 2. Perform Courier Sign-In
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('courier@swiftdrop.io');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // 3. Navigate to Courier Mobile Dashboard
    await courierPage.goto('/courier/dashboard');

    // Verify Mobile Route Header & Stop-by-Stop Run list
    await expect(courierPage.getByText("Today's Stop-by-Stop Run")).toBeVisible();
    await expect(courierPage.getByText('SD-20261030-99801')).toBeVisible();

    // Step A: Advance task from PICKED_UP to OUT_FOR_DELIVERY
    const outForDeliveryBtn = courierPage.getByRole('button', { name: 'Out for Delivery' });
    if (await outForDeliveryBtn.isVisible()) {
      await outForDeliveryBtn.click();
    }

    // Step B: Locate parcel in OUT_FOR_DELIVERY status and trigger "Capture POD"
    const capturePodBtn = courierPage.getByRole('button', { name: 'Capture POD' }).first();
    await expect(capturePodBtn).toBeVisible();
    await capturePodBtn.click();

    // Step C: Verify POD Modal is displayed
    await expect(courierPage.getByText('Capture Proof of Delivery (POD)')).toBeVisible();
    await expect(courierPage.getByText('Digital Signature Pad Active')).toBeVisible();

    // Enter recipient signature name
    const sigInput = courierPage.getByPlaceholder('e.g. Kasun W.');
    await expect(sigInput).toBeVisible();
    await sigInput.fill('Kasun Wickramasinghe');

    // Confirm Delivery with digital signature
    const confirmDeliveryBtn = courierPage.getByRole('button', { name: 'Confirm Delivery' });
    await expect(confirmDeliveryBtn).toBeVisible();
    await confirmDeliveryBtn.click();

    // Step D: Verify Drop Completed status and captured signature badge
    await expect(courierPage.getByText('Signed by Kasun Wickramasinghe')).toBeVisible();
    await expect(courierPage.getByText('Drop Completed').first()).toBeVisible();
  });
});
