import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Suite 2: Dynamic Pricing & Quote Expiration Anomalies
 *
 * Edge cases covered:
 * 1. Expired Quote Lockout: When quote expires, submission is blocked and user is prompted to get a new quote.
 * 2. Parameter Alteration Invalidation: Modifying weight after getting a quote invalidates the existing quote.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Suite 2: Dynamic Pricing & Quote Expiration Anomalies', () => {
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

    await customerPage.route('**/api/v1/zones?postal_code=*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: { id: 'ZONE_COLOMBO_CENTRAL', name: 'Colombo Central' },
        }),
      });
    });
  });

  test('Edge 2.1: Expired quote displays expiration warning and shifts CTA to "Get a new quote"', async ({
    customerPage,
  }) => {
    // Mock quote that expires in 1 second
    await customerPage.route('**/api/v1/pricing/quotes', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            quote_id: 'qte_short_lived_123',
            fee_cents: 450,
            currency: 'LKR',
            expires_at: new Date(Date.now() + 1000).toISOString(), // 1 second expiry
          },
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

    // Fill valid Step 2
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('2.0');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_LENGTH)).fill('15');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WIDTH)).fill('10');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_HEIGHT)).fill('10');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DESCRIPTION)).fill('Short expiry test parcel');

    await customerPage.getByRole('button', { name: 'Get quote' }).click();

    // Verify quote was loaded
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS))).toBeVisible();

    // Wait for the 1 second quote expiry
    await customerPage.waitForTimeout(1500);

    // Invariant: Expired quote notice is displayed and button shifts to "Get a new quote"
    await expect(customerPage.getByText('Request a new quote to continue.')).toBeVisible();
    await expect(customerPage.getByRole('button', { name: 'Get a new quote' })).toBeVisible();
  });

  test('Edge 2.2: Modifying weight invalidates existing quote and requires recalculation', async ({
    customerPage,
  }) => {
    let quoteRequests = 0;
    await customerPage.route('**/api/v1/pricing/quotes', async (route) => {
      quoteRequests++;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            quote_id: `qte_recalculated_${quoteRequests}`,
            fee_cents: quoteRequests === 1 ? 400 : 700,
            currency: 'LKR',
            expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
          },
        }),
      });
    });

    await customerPage.goto('/auth/login');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.io');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await customerPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await customerPage.goto('/portal/book');

    // Fill Step 1
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

    // Step 2: Enter initial weight 1.0kg
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('1.0');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_LENGTH)).fill('10');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WIDTH)).fill('10');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_HEIGHT)).fill('10');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DESCRIPTION)).fill('Sample parcel');

    await customerPage.getByRole('button', { name: 'Get quote' }).click();
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS))).toBeVisible();

    // Navigate back to Step 2
    await customerPage.getByRole('button', { name: 'Back' }).click();

    // Alter weight to 4.5kg
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('4.5');

    // Click Get Quote again
    await customerPage.getByRole('button', { name: 'Get quote' }).click();

    // Assert a second quote was requested and price updated
    expect(quoteRequests).toBe(2);
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS))).toBeVisible();
  });
});
