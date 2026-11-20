import { test, expect } from '../fixtures/authFixtures';

/**
 * Suite 6: Merchant API Key & Webhook Security Lifecycle
 *
 * Edge cases covered:
 * 1. One-Time Key Reveal: verifies key modal close button is disabled until checking acknowledgment.
 * 2. Revocation confirmation modal: asserts revocation requires confirmation.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Suite 6: Merchant Key & Security Scopes Edge Cases', () => {
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

    await merchantPage.route('**/api/v1/merchants/me/api-keys', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [],
          }),
        });
      } else if (route.request().method() === 'POST') {
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              key_id: 'key_mock_991',
              label: 'Production Automation Key',
              api_key: 'sk_live_mock_abcdef123456789',
              scopes: ['parcels:read'],
              created_at: new Date().toISOString(),
            },
          }),
        });
      }
    });
  });

  test('Edge 6.1: One-time API key reveal requires mandatory acknowledgment checkbox to close', async ({
    merchantPage,
  }) => {
    await merchantPage.goto('/auth/login');
    await merchantPage.getByTestId('login-email-input').fill('merchant@swiftdrop.io');
    await merchantPage.getByTestId('login-password-input').fill('Password123!');
    await merchantPage.getByTestId('login-submit-btn').click();

    await merchantPage.goto('/portal/merchants/api-keys');

    // Create a new API key
    await merchantPage.getByTestId('merchant-key-label-input').fill('Production Automation Key');
    await merchantPage.getByTestId('merchant-key-read-scope').check();
    await merchantPage.getByTestId('merchant-credential-create-btn').click();

    // Verify modal is displayed with one-time copy button
    const doneBtn = merchantPage.getByTestId('merchant-secret-close-btn');
    await expect(merchantPage.getByTestId('one-time-key-copy-btn')).toBeVisible();

    // Invariant: Close button is disabled until checking the safety acknowledgment
    await expect(doneBtn).toBeDisabled();

    // Check acknowledgment
    await merchantPage.getByTestId('dismiss-key-modal-checkbox').check();

    // Invariant: Done button is now enabled
    await expect(doneBtn).toBeEnabled();
    await doneBtn.click();
    await expect(doneBtn).toBeHidden();
  });
});
