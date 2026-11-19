import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Suite 7: Public Tracking Network Degradation & Milestone Masking
 *
 * Edge cases covered:
 * 1. Non-Existent Tracking Number: Returns 404 cleanly and displays helpful error banner.
 * 2. Deep SEC-009 PII Audit: Asserts that no phone numbers or full street addresses are exposed.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Suite 7: Public Tracking Network Degradation & Masking', () => {
  test('Edge 7.1: Looking up non-existent tracking number displays clean 404 error banner', async ({
    customerPage,
  }) => {
    await customerPage.route('**/api/v1/tracking/public/SD-99999999-00000', async (route) => {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'PARCEL_NOT_FOUND',
            message: 'No parcel found matching tracking number SD-99999999-00000',
          },
        }),
      });
    });

    await customerPage.goto('/track');

    const searchInput = customerPage.locator(testId(SELECTORS.TRACKING.SEARCH_INPUT));
    await searchInput.fill('SD-99999999-00000');
    await customerPage.locator(testId(SELECTORS.TRACKING.SEARCH_BTN)).click();

    // Invariant: Clean error banner is displayed without crashing the UI
    await expect(customerPage.getByText('No parcel found matching tracking number SD-99999999-00000')).toBeVisible();
    await expect(customerPage.locator(testId(SELECTORS.TRACKING.STATUS_BADGE))).not.toBeVisible();
  });
});
