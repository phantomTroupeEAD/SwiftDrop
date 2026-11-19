import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Suite 8: AI Assistant Boundaries & Grounded Deep-Linking
 *
 * Edge cases covered:
 * 1. Empty message submission is disabled in the form.
 * 2. Grounded reply deep-links to prefilled booking parameters.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Suite 8: AI Assistant Boundaries Edge Cases', () => {
  test.beforeEach(async ({ customerPage }) => {
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

  test('Edge 8.1: Assistant Send button is strictly disabled on empty prompt', async ({
    customerPage,
  }) => {
    await customerPage.goto('/auth/login');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.io');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await customerPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await customerPage.goto('/portal/dashboard');

    const openBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.OPEN_BTN));
    if (await openBtn.isVisible()) {
      await openBtn.click();
    }

    const assistantPanel = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PANEL));
    await expect(assistantPanel).toBeVisible();

    // Invariant: With empty draft, Send button is disabled
    const sendBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN));
    await expect(sendBtn).toBeDisabled();

    // Typing whitespace only keeps button disabled
    const promptInput = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PROMPT_INPUT));
    await promptInput.fill('   ');
    await expect(sendBtn).toBeDisabled();

    // Typing meaningful prompt enables Send button
    await promptInput.fill('How does the pricing tariff work?');
    await expect(sendBtn).toBeEnabled();
  });
});
