import { randomUUID } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';

const email = process.env.M6_MERCHANT_EMAIL;
const password = process.env.M6_MERCHANT_PASSWORD;

async function signIn(page: Page) {
  await page.goto('/auth/login');
  await page.getByTestId('login-email-input').fill(email!);
  await page.getByTestId('login-password-input').fill(password!);
  await page.getByTestId('login-submit-btn').click();
  await expect(page.getByRole('heading', { name: 'Merchant portal' })).toBeVisible();
}

test.describe('Member 6 merchant integration', () => {
  test.beforeEach(({}, info) => {
    test.skip(info.project.name !== 'merchant-portal' || !email || !password,
    'Requires the merchant-portal project and an approved test merchant.');
  });

  test('loads the merchant profile and orders through the frontend', async ({ page }) => {
    const profile = page.waitForResponse(response =>
      new URL(response.url()).pathname === '/api/v1/merchants/me' && response.request().method() === 'GET');
    await signIn(page);
    expect((await profile).status()).toBe(200);
    await expect(page.getByRole('heading', { name: 'Merchant profile' })).toBeVisible();

    const orders = page.waitForResponse(response =>
      new URL(response.url()).pathname === '/api/v1/merchant/orders' && response.request().method() === 'GET');
    await page.getByRole('navigation', { name: 'Portal navigation' }).getByRole('link', { name: 'Orders', exact: true }).click();
    expect((await orders).status()).toBe(200);
    await expect(page.getByTestId('merchant-order-table').or(page.getByTestId('merchant-order-empty'))).toBeVisible();
  });

  test('creates an API key, requires acknowledgment, and revokes it', async ({ page }) => {
    await signIn(page);
    await page.getByRole('navigation', { name: 'Portal navigation' }).getByRole('link', { name: 'API keys' }).click();
    const label = 'M6 integration ' + randomUUID();
    await page.getByTestId('merchant-key-label-input').fill(label);
    await page.getByTestId('merchant-key-read-scope').check();
    await page.getByTestId('merchant-credential-create-btn').click();

    const done = page.getByTestId('merchant-secret-close-btn');
    await expect(page.getByTestId('one-time-key-copy-btn')).toBeVisible();
    await expect(done).toBeDisabled();
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await done.click();
    await expect(done).toBeHidden();

    const entry = page.locator('.m6-item').filter({ hasText: label });
    await entry.getByTestId('merchant-key-revoke-btn').click();
    await page.getByTestId('merchant-credential-confirm-btn').click();
    await expect(entry.getByTestId('merchant-key-revoke-btn')).toBeHidden();
  });
});
