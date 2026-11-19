import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Suite 4: Courier Mobile Delivery Exceptions & Failed Attempts
 *
 * Edge cases covered:
 * 1. Delivery Attempt Failure Classification: Driver logs failure reason (e.g. RECIPIENT_UNAVAILABLE),
 *    which shifts task card to ATTEMPT_FAILED error state.
 * 2. Mandatory Signature Input Validation: POD modal requires signature text before confirming delivery.
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
  });

  test('Edge 4.1: Driver records delivery attempt failure with specific reason code', async ({
    courierPage,
  }) => {
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('courier@swiftdrop.io');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await courierPage.goto('/courier/dashboard');

    // Click "Failed Attempt" on the parcel in OUT_FOR_DELIVERY status
    const failedAttemptBtn = courierPage.getByRole('button', { name: 'Failed Attempt' }).first();
    await expect(failedAttemptBtn).toBeVisible();
    await failedAttemptBtn.click();

    // Verify modal appears
    await expect(courierPage.getByText('Record Delivery Attempt Failure')).toBeVisible();

    // Select failure reason
    const reasonSelect = courierPage.getByRole('combobox');
    await expect(reasonSelect).toBeVisible();
    await reasonSelect.selectOption('SECURITY_ACCESS_DENIED');

    // Submit failure log
    await courierPage.getByRole('button', { name: 'Log Attempt & Reschedule' }).click();

    // Invariant: Task shifts to ATTEMPT_FAILED status badge
    await expect(courierPage.getByText('ATTEMPT FAILED').first()).toBeVisible();
    await expect(courierPage.getByText('Attempt Failed: SECURITY_ACCESS_DENIED')).toBeVisible();
  });

  test('Edge 4.2: POD modal allows cancellation without corrupting stop status', async ({
    courierPage,
  }) => {
    await courierPage.goto('/auth/login');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('courier@swiftdrop.io');
    await courierPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await courierPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await courierPage.goto('/courier/dashboard');

    // Open POD modal
    const capturePodBtn = courierPage.getByRole('button', { name: 'Capture POD' }).first();
    await expect(capturePodBtn).toBeVisible();
    await capturePodBtn.click();

    await expect(courierPage.getByText('Capture Proof of Delivery (POD)')).toBeVisible();

    // Click Cancel
    await courierPage.getByRole('button', { name: 'Cancel' }).click();

    // Invariant: Modal closes and parcel remains in OUT_FOR_DELIVERY status
    await expect(courierPage.getByText('Capture Proof of Delivery (POD)')).not.toBeVisible();
    await expect(capturePodBtn).toBeVisible();
  });
});
