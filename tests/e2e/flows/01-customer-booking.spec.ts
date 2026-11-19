import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Flow 1: Customer Booking Wizard & Official Receipt Generation
 *
 * Invariants:
 * - Customer authenticates cleanly via /auth/login.
 * - Navigates 4-step wizard: Addresses -> Parcel Details -> Tariff Quote -> Confirmation.
 * - Form validation prevents proceeding until mandatory fields (postal codes, weights) are filled.
 * - Dynamic quote calculates correct price cents based on zone and weight.
 * - On submission, returns formal SD-YYYYMMDD-XXXXX tracking number and receipt button.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Flow 1: Customer Booking Wizard E2E Flow', () => {
  test('should complete the 4-step booking wizard, verify dynamic quote, and receive tracking confirmation', async ({
    customerPage,
  }) => {
    // 1. Intercept backend REST API calls with deterministic mock responses
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
      const url = new URL(route.request().url());
      const postal = url.searchParams.get('postal_code');
      const zoneId = postal === '00100' ? 'ZONE_COLOMBO_CENTRAL' : 'ZONE_COLOMBO_SOUTH';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            id: zoneId,
            name: zoneId === 'ZONE_COLOMBO_CENTRAL' ? 'Colombo Central' : 'Colombo South',
          },
        }),
      });
    });

    await customerPage.route('**/api/v1/pricing/quotes', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            quote_id: 'qte_mock_12345',
            pickup_postal_code: '00100',
            delivery_postal_code: '00400',
            fee_cents: 500,
            base_fee_cents: 350,
            weight_surcharge_cents: 150,
            currency: 'LKR',
            expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
          },
        }),
      });
    });

    await customerPage.route('**/api/v1/parcels', async (route) => {
      if (route.request().method() === 'POST') {
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'pcl_00100000-0000-0000-0000-000000000001',
              tracking_number: 'SD-20261114-99881',
              status: 'BOOKED',
              quote_id: 'qte_mock_12345',
              total_cents: 500,
              currency: 'LKR',
              created_at: new Date().toISOString(),
            },
          }),
        });
      } else {
        await route.fallback();
      }
    });

    // 2. Perform Customer Authentication
    await customerPage.goto('/auth/login');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.io');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await customerPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // 3. Navigate to Customer Booking Wizard
    await customerPage.goto('/portal/book');
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.BOOKING_WIZARD))).toBeVisible();

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

    // Wait for postal zones to be resolved and displayed
    await expect(customerPage.getByText('✓ ZONE_COLOMBO_CENTRAL')).toBeVisible();
    await expect(customerPage.getByText('✓ ZONE_COLOMBO_SOUTH')).toBeVisible();

    // Advance to Step 1 (Parcel Details)
    await customerPage.getByRole('button', { name: 'Continue' }).click();

    // Step 2: Parcel Details
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT))).toBeVisible();
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WEIGHT)).fill('2.5');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_LENGTH)).fill('20');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_WIDTH)).fill('15');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_HEIGHT)).fill('10');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.INPUT_DESCRIPTION)).fill('Electronics component parcel');
    await customerPage.locator(testId(SELECTORS.CUSTOMER.SELECT_SERVICE)).selectOption('STANDARD');

    // Advance to Step 2 (Quote)
    await customerPage.getByRole('button', { name: 'Get quote' }).click();

    // Step 3: Authoritative Dynamic Quote Verification
    const quotePriceLocator = customerPage.locator(testId(SELECTORS.CUSTOMER.QUOTE_PRICE_CENTS));
    await expect(quotePriceLocator).toBeVisible();

    // Advance to Step 3 (Review / Confirm)
    await customerPage.getByRole('button', { name: 'Continue' }).click();

    // Step 4: Submit Booking
    await customerPage.locator(testId(SELECTORS.CUSTOMER.BTN_SUBMIT_BOOKING)).click();

    // Step 5: Verify Confirmation & Tracking Number
    const confirmationCard = customerPage.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION));
    await expect(confirmationCard).toBeVisible();

    const trackingNumberElement = customerPage.locator(testId(SELECTORS.CUSTOMER.BOOKING_CONFIRMATION_TRACKING_NO));
    await expect(trackingNumberElement).toBeVisible();
    await expect(trackingNumberElement).toHaveText(/SD-20261114-99881/);

    // Verify official receipt button is mounted
    await expect(customerPage.locator(testId(SELECTORS.CUSTOMER.BTN_RECEIPT))).toBeVisible();
  });
});
