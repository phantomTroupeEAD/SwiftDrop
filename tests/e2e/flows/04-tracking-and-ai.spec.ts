import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Flow 4: Public Tracking with SEC-009 PII Masking & AI Assistant Grounding
 *
 * Invariants:
 * - Public tracking lookup (/track) requires zero authentication.
 * - Enforces SEC-009: Only sanitized status and city/milestones returned; no customer PII.
 * - Embedded AI assistant (/portal/dashboard) answers operational fee and tariff queries.
 * - FastMCP tool grounding returns deep-link action with prefilled booking parameters.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Flow 4: Public Tracking & AI Assistant Grounding E2E Flow', () => {
  test('should lookup public tracking with sanitized milestones and query grounded AI assistant', async ({
    customerPage,
  }) => {
    // 1. Mock Public Tracking API
    await customerPage.route('**/api/v1/tracking/public/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            tracking_number: 'SD-20261114-99881',
            status: 'OUT_FOR_DELIVERY',
            destination: {
              city: 'Colombo',
            },
            milestones: [
              {
                status: 'BOOKED',
                timestamp: '2026-11-14T08:00:00Z',
              },
              {
                status: 'DISPATCHED',
                timestamp: '2026-11-14T09:30:00Z',
              },
              {
                status: 'OUT_FOR_DELIVERY',
                timestamp: '2026-11-14T11:00:00Z',
              },
            ],
          },
        }),
      });
    });

    // 2. Mock Customer Auth & AI Chat API
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

    await customerPage.route('**/api/v1/ai/chat', async (route) => {
      if (route.request().method() === 'POST') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            conversation_id: 'conv_mock_881',
            reply: 'Based on your parameters (2.5 kg from 00100 to 00400), the estimated Standard delivery fee is 500 LKR.',
            actions: [
              {
                action_type: 'NAVIGATE_BOOKING',
                target_url: '/portal/book',
                prefill_data: {
                  pickup_postal_code: '00100',
                  delivery_postal_code: '00400',
                  weight_kg: 2.5,
                  service_type: 'STANDARD',
                },
              },
            ],
          }),
        });
      } else {
        await route.fallback();
      }
    });

    // -------------------------------------------------------------------------
    // Part A: Public Tracking Lookup & PII Sanitization Check (SEC-009)
    // -------------------------------------------------------------------------
    await customerPage.goto('/track');

    const searchInput = customerPage.locator(testId(SELECTORS.TRACKING.SEARCH_INPUT));
    await expect(searchInput).toBeVisible();
    await searchInput.fill('SD-20261114-99881');

    const searchBtn = customerPage.locator(testId(SELECTORS.TRACKING.SEARCH_BTN));
    await searchBtn.click();

    // Verify Tracking Result
    await expect(customerPage.getByText('SD-20261114-99881')).toBeVisible();
    await expect(customerPage.getByText('Destination Colombo')).toBeVisible();

    // Invariant Check (SEC-009): Confirm no customer PII (phone number or street address) is present in the DOM
    const bodyText = await customerPage.innerText('body');
    expect(bodyText).not.toContain('+94 77 123 4567');
    expect(bodyText).not.toContain('42 Galle Road');

    // -------------------------------------------------------------------------
    // Part B: AI Assistant Grounding & Booking Navigation
    // -------------------------------------------------------------------------
    // Customer logs in and accesses customer portal dashboard
    await customerPage.goto('/auth/login');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.io');
    await customerPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await customerPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await customerPage.goto('/portal/dashboard');

    const assistantPanel = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PANEL));
    await expect(assistantPanel).toBeVisible();

    const promptInput = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PROMPT_INPUT));
    await promptInput.fill('What is the delivery fee for 2.5kg from 00100 to 00400?');

    const sendBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN));
    await sendBtn.click();

    // Verify Grounded AI Reply
    await expect(customerPage.getByText('estimated Standard delivery fee is 500 LKR')).toBeVisible();

    // Verify Deep-link to Booking
    const bookingLink = customerPage.getByRole('link', { name: 'Continue to booking' });
    await expect(bookingLink).toBeVisible();
  });
});
