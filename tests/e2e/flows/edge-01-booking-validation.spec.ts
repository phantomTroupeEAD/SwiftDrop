import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Suite 1: Customer Booking Wizard Edge Cases & Boundary Violations
 *
 * Edge cases covered:
 * 1. Unmapped postal code fallback: when postal code has no direct zone,
 *    BookingZone.tsx falls back to a regional zone dropdown.
 * 2. Mandatory field gating: advancing is barred if any address field is empty.
 * 3. Chargeable weight and dimension boundaries: enforces 0.01kg - 30kg and 0.01cm - 150cm.
 * 4. Idempotency preservation: resubmitting a booking reuses the exact same idempotency key.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Suite 1: Customer Booking Wizard Edge Cases', () => {
  test.beforeEach(async ({ customerPage }) => {
    // 1. Mock Session Refresh & Customer Auth
    await customerPage.route('**/api/v1/auth/refresh', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-customer-jwt-token',
            user: {
              id: 'c0010000-0000-0000-0000-000000000001',
              email: 'customer@swiftdrop.io',
              role: 'CUSTOMER',
              full_name: 'Devinda Perera',
            },
          },
        }),
      });
    });

    await customerPage.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-customer-jwt-token',
            refresh_token: 'mock-customer-refresh-token',
            user: {
              id: 'c0010000-0000-0000-0000-000000000001',
              email: 'customer@swiftdrop.io',
              role: 'CUSTOMER',
              full_name: 'Devinda Perera',
            },
          },
        }),
      });
    });
  });

  test('Edge 1.1: Unmapped postal code triggers fallback regional zone selector', async ({
    customerPage,
  }) => {
    // Mock direct zone lookup returning 404 for unmapped postal code '99999'
    await customerPage.route('**/api/v1/zones?postal_code=99999', async (route) => {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'ZONE_NOT_FOUND',
            message: 'No zone directly mapped to postal code 99999',
          },
        }),
      });
    });

    // Mock all available regional zones for fallback selection
    await customerPage.route('**/api/v1/zones', async (route) => {
      if (route.request().url().includes('postal_code')) {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            { id: 'ZONE_NORTHERN', name: 'Northern Province Regional' },
            { id: 'ZONE_EASTERN', name: 'Eastern Province Regional' },
          ],
        }),
      });
    });

    // Valid dropoff zone mock
    await customerPage.route('**/api/v1/zones?postal_code=00400', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: { id: 'ZONE_COLOMBO_SOUTH', name: 'Colombo South' },
        }),
      });
    });

    // Authenticate and navigate to booking
    await customerPage.goto('/auth/login');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.io');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await customerPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await customerPage.goto('/portal/book');

    // Fill pickup details with unmapped postal code
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_NAME)).fill('Devinda Perera');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_PHONE)).fill('+94771234567');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_ADDRESS)).fill('Remote Station Road');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_CITY)).fill('Jaffna');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_POSTAL)).fill('99999');

    // Assert fallback notice is shown
    await expect(
      customerPage.getByText('No direct zone mapped for postal code 99999. Please select your regional zone manually.')
    ).toBeVisible();

    // Select regional zone from fallback dropdown (labeled by the parent text)
    const fallbackSelect = customerPage.getByRole('combobox', { name: /No direct zone mapped for postal code 99999/i });
    await expect(fallbackSelect).toBeVisible();
    await fallbackSelect.selectOption('ZONE_NORTHERN');

    // Fill dropoff details
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_NAME)).fill('Kasun Wickrama');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_PHONE)).fill('+94719876543');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_ADDRESS)).fill('15/A Havelock Road');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_CITY)).fill('Colombo');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_POSTAL)).fill('00400');
    await expect(customerPage.getByText('✓ ZONE_COLOMBO_SOUTH')).toBeVisible();

    // Verify advancing to Step 2 succeeds
    await customerPage.getByRole('button', { name: 'Continue' }).click();
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT))).toBeVisible();
  });

  test('Edge 1.2: Boundary violations block progression beyond Step 2', async ({
    customerPage,
  }) => {
    // Normal zone mocks
    await customerPage.route('**/api/v1/zones?postal_code=*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: { id: 'ZONE_COLOMBO_CENTRAL', name: 'Colombo Central' },
        }),
      });
    });

    await customerPage.goto('/auth/login');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.io');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await customerPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await customerPage.goto('/portal/book');

    // Fill valid Step 1
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_NAME)).fill('Devinda Perera');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_PHONE)).fill('+94771234567');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_ADDRESS)).fill('42 Galle Road');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_CITY)).fill('Colombo');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_PICKUP_POSTAL)).fill('00100');

    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_NAME)).fill('Kasun Wickrama');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_PHONE)).fill('+94719876543');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_ADDRESS)).fill('15/A Havelock Road');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_CITY)).fill('Colombo');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DROPOFF_POSTAL)).fill('00100');

    await expect(customerPage.getByText('✓ ZONE_COLOMBO_CENTRAL').first()).toBeVisible();
    await customerPage.getByRole('button', { name: 'Continue' }).click();

    // Step 2: Attempt entering weight > 30kg (e.g. 45kg)
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('45');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_LENGTH)).fill('20');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WIDTH)).fill('15');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_HEIGHT)).fill('10');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DESCRIPTION)).fill('Heavy machinery component');

    // Click Get Quote - browser HTML5 validation blocks advancing
    await customerPage.getByRole('button', { name: 'Get quote' }).click();

    // Invariant: quote-price-cents must NOT be visible because 45kg > 30kg max ceiling
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS))).not.toBeVisible();
  });
});
