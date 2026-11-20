import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Suite 4: Courier Mobile Delivery Exceptions & Failed Attempts
 *
 * Edge cases covered:
 * 1. Delivery Attempt Failure Classification: Driver logs failure reason (e.g. RECIPIENT_UNAVAILABLE),
 *    which records failed attempt and updates status notice.
 * 2. Cancellation of record delivery form without state loss: form closes cleanly on "Close details".
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Suite 4: Courier Mobile Delivery Exceptions Edge Cases', () => {
  test.beforeEach(async ({ courierPage }) => {
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

    await courierPage.route('**/api/v1/courier/parcels', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              parcel_id: 'p0010000-0000-0000-0000-000000000001',
              tracking_number: 'SD-20261030-99801',
              current_status: 'OUT_FOR_DELIVERY',
              recipient_name: 'Kasun Wickramasinghe',
              recipient_phone: '+94771234567',
              delivery_street: '42 Galle Road',
              delivery_city: 'Colombo',
              attempt_count: 0,
            },
          ],
        }),
      });
    });

    await courierPage.route('**/api/v1/deliveries/**/failed', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            parcel_status: 'ATTEMPT_FAILED',
            attempt_number: 1,
            is_terminal: false,
          },
        }),
      });
    });
  });

  test('Edge 4.1: Driver records delivery attempt failure with specific reason code', async ({
    courierPage,
  }) => {
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('courier@swiftdrop.io');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await courierPage.goto('/courier/dashboard');
    await expect(courierPage.getByRole('heading', { name: 'Delivery tasks' })).toBeVisible();

    // Click "Report failed attempt" on the parcel in OUT_FOR_DELIVERY status
    const failedAttemptBtn = courierPage.getByRole('button', { name: 'Report failed attempt' }).first();
    await expect(failedAttemptBtn).toBeVisible();
    await failedAttemptBtn.click();

    // Verify inline failure form appears
    await expect(courierPage.getByText('Reason')).toBeVisible();

    // Select failure reason
    const reasonSelect = courierPage.locator('select[name="reason"]');
    await expect(reasonSelect).toBeVisible();
    await reasonSelect.selectOption('ACCESS_RESTRICTED');

    // Fill notes
    await courierPage.locator('input[name="notes"]').fill('Gate security refused entry without gate pass');

    // Submit failure log
    await courierPage.getByRole('button', { name: 'Save attempt' }).click();

    // Invariant: Success notice displayed
    await expect(courierPage.getByRole('heading', { name: 'Failed attempt recorded' })).toBeVisible();
    await expect(courierPage.getByText(/Attempt #1 logged/i)).toBeVisible();
  });

  test('Edge 4.2: POD modal allows cancellation without corrupting stop status', async ({
    courierPage,
  }) => {
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('courier@swiftdrop.io');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await courierPage.goto('/courier/dashboard');
    await expect(courierPage.getByRole('heading', { name: 'Delivery tasks' })).toBeVisible();

    // Open delivery record panel
    const recordDeliveryBtn = courierPage.getByRole('button', { name: 'Record delivery' }).first();
    await expect(recordDeliveryBtn).toBeVisible();
    await recordDeliveryBtn.click();

    await expect(courierPage.getByText('Recipient name')).toBeVisible();

    // Click Close details
    await courierPage.getByRole('button', { name: 'Close details' }).click();

    // Invariant: Panel closes and parcel remains in OUT_FOR_DELIVERY status with buttons visible
    await expect(courierPage.getByText('Recipient name')).not.toBeVisible();
    await expect(recordDeliveryBtn).toBeVisible();
  });
});
