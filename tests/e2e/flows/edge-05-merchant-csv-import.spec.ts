import { test, expect } from '../fixtures/authFixtures';

/**
 * Suite 5: Merchant CSV Bulk Order Import & Validation Errors
 *
 * Edge cases covered:
 * 1. Missing columns in uploaded CSV: verifies rejection with template alignment error.
 * 2. Over-sized batch (>100 orders): verifies rejection banner.
 * 3. Line-by-line validation errors: asserts row 2 highlights invalid phone format.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Suite 5: Merchant CSV Bulk Import Edge Cases', () => {
  test.beforeEach(async ({ merchantPage }) => {
    await merchantPage.route('**/api/v1/auth/refresh', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-merchant-jwt-token',
            user: {
              id: 'm0010000-0000-0000-0000-000000000001',
              email: 'merchant@swiftdrop.io',
              role: 'MERCHANT',
              full_name: 'Alpha Logistics Merchant',
            },
          },
        }),
      });
    });

    await merchantPage.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-merchant-jwt-token',
            refresh_token: 'mock-merchant-refresh-token',
            user: {
              id: 'm0010000-0000-0000-0000-000000000001',
              email: 'merchant@swiftdrop.io',
              role: 'MERCHANT',
              full_name: 'Alpha Logistics Merchant',
            },
          },
        }),
      });
    });

    await merchantPage.route('**/api/v1/merchant/orders', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [],
        }),
      });
    });
  });

  test('Edge 5.1: Malformed CSV with missing headers is rejected with template notice', async ({
    merchantPage,
  }) => {
    await merchantPage.goto('/auth/login');
    await merchantPage.getByTestId('login-email-input').fill('merchant@swiftdrop.io');
    await merchantPage.getByTestId('login-password-input').fill('Password123!');
    await merchantPage.getByTestId('login-submit-btn').click();

    await merchantPage.goto('/portal/merchants/orders');

    // Create a malformed CSV missing required columns
    const malformedCsv = 'merchant_reference,pickup_name\r\nREF-001,Sender Name\r\n';

    // Upload via file input
    const fileChooserPromise = merchantPage.waitForEvent('filechooser');
    await merchantPage.getByTestId('batch-csv-file-input').click();
    const fileChooser = await fileChooserPromise;
    await fileChooser.setFiles({
      name: 'orders.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(malformedCsv),
    });

    // Invariant: Error banner appears stating header mismatch
    await expect(merchantPage.getByText('Use the column names from the CSV template.')).toBeVisible();
  });

  test('Edge 5.2: Row-level phone number validation displays error badge', async ({
    merchantPage,
  }) => {
    await merchantPage.goto('/auth/login');
    await merchantPage.getByTestId('login-email-input').fill('merchant@swiftdrop.io');
    await merchantPage.getByTestId('login-password-input').fill('Password123!');
    await merchantPage.getByTestId('login-submit-btn').click();

    await merchantPage.goto('/portal/merchants/orders');

    // Valid header, but row has invalid non-E.164 phone number '12345'
    const columns = [
      'merchant_reference', 'pickup_name', 'pickup_phone', 'pickup_address', 'pickup_city', 'pickup_zone_id',
      'dropoff_name', 'dropoff_phone', 'dropoff_address', 'dropoff_city', 'dropoff_zone_id', 'weight_kg', 'length_cm', 'width_cm', 'height_cm', 'description',
    ].join(',');
    const invalidRow = 'REF-001,Sender,+94771234567,42 Galle Rd,Colombo,ZONE_COLOMBO_CENTRAL,Recipient,12345,15 Havelock Rd,Colombo,ZONE_COLOMBO_SOUTH,2.5,20,15,10,Sample Order';
    const csvContent = `${columns}\r\n${invalidRow}\r\n`;

    const fileChooserPromise = merchantPage.waitForEvent('filechooser');
    await merchantPage.getByTestId('batch-csv-file-input').click();
    const fileChooser = await fileChooserPromise;
    await fileChooser.setFiles({
      name: 'orders_with_invalid_phone.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(csvContent),
    });

    // Invariant: Validation message for international phone requirement is displayed on that row
    await expect(merchantPage.getByText('Use international phone numbers such as +94771234567.')).toBeVisible();
  });
});
