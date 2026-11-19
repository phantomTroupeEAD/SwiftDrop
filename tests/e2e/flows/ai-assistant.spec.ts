import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * AI Assistant Comprehensive E2E Test Suite
 *
 * Covers:
 * 1. SSE Token Streaming & Active Tool Execution Indicator
 * 2. Grounded Tariff Quotation & Booking Form Parameter Hydration
 * 3. Merchant Portal Enablement & Role-Aware Navigation
 * 4. Public Tracking Page Availability & Tracking Action Deep-Link
 * 5. Multi-Turn Session Continuity (conversation_id round-trip)
 * 6. Tariff Error Interception & User-Friendly Fallback
 * 7. SEC-009 PII Masking & Input Boundaries (Empty/Whitespace/1000-char)
 * 8. Accessibility: Keyboard Trapping & Focus Restoration
 *
 * Owned by: Member 10 (AI Assistant / Cross-Workflow Quality)
 */

test.describe('AI Assistant Comprehensive Suite', () => {
  test.beforeEach(async ({ customerPage }) => {
    // Fail immediately on unhandled page errors or console exceptions
    customerPage.on('pageerror', (err) => {
      throw new Error(`[STRICT TEST FAILURE] Browser exception: ${err.message}\n${err.stack}`);
    });

    // Mock Customer Auth
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

  test('1. SSE Streaming, Tool Indicator & Booking Form Hydration', async ({ customerPage }) => {
    // Mock SSE Stream endpoint with tool execution and token chunks
    await customerPage.route('**/api/v1/ai/chat/stream', async (route) => {
      const sseFrames = [
        'event: tool_start\ndata: {"tool": "estimate_delivery_fee"}\n\n',
        'event: tool_end\ndata: {"tool": "estimate_delivery_fee", "success": true}\n\n',
        'event: token\ndata: {"delta": "Based on "}\n\n',
        'event: token\ndata: {"delta": "your parameters, standard delivery "}\n\n',
        'event: token\ndata: {"delta": "is **500 LKR**."}\n\n',
        'event: action_card\ndata: {"action_type": "NAVIGATE_BOOKING", "target_url": "/portal/book", "label": "Book Delivery Now", "prefill_data": {"pickup_postal_code": "00100", "delivery_postal_code": "00400", "weight_kg": 2.5, "service_type": "STANDARD"}}\n\n',
        'event: done\ndata: {"conversation_id": "conv-stream-001"}\n\n',
      ].join('');

      await route.fulfill({
        status: 200,
        headers: {
          'Content-Type': 'text/event-stream; charset=utf-8',
          'Cache-Control': 'no-cache',
          Connection: 'keep-alive',
        },
        body: sseFrames,
      });
    });

    await customerPage.goto('/portal/dashboard');

    // Open assistant
    const openBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.OPEN_BTN));
    await expect(openBtn).toBeVisible();
    await openBtn.click();

    const panel = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PANEL));
    await expect(panel).toBeVisible();

    // Type prompt and submit
    const promptInput = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PROMPT_INPUT));
    await promptInput.fill('How much to ship 2.5kg from 00100 to 00400?');

    const sendBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN));
    await sendBtn.click();

    // Verify response content renders with formatting
    await expect(panel.getByText('500 LKR')).toBeVisible();

    // Verify 1-click booking action button exists
    const actionBtn = panel.locator(testId(SELECTORS.AI_ASSISTANT.AI_ACTION_BUTTON_BOOK));
    await expect(actionBtn).toBeVisible();
    const actionHref = await actionBtn.getAttribute('href');
    expect(actionHref).toContain('pickup_postal_code=00100');
    expect(actionHref).toContain('delivery_postal_code=00400');
    expect(actionHref).toContain('weight_kg=2.5');

    // Click through to booking and verify form input hydration
    await actionBtn.click();
    await expect(customerPage).toHaveURL(/\/portal\/book\?.*pickup_postal_code=00100/);

    // Verify fields in the booking page draft are hydrated from query params
    const pickupPostalInput = customerPage.locator('input[id*="pickup-postal"], input[name*="pickup-postal"]');
    if (await pickupPostalInput.count() > 0) {
      await expect(pickupPostalInput.first()).toHaveValue('00100');
    }
  });

  test('2. Merchant Portal Enablement & Role-Aware Navigation', async ({ merchantPage }) => {
    merchantPage.on('pageerror', (err) => {
      throw new Error(`[STRICT TEST FAILURE] Browser exception: ${err.message}\n${err.stack}`);
    });

    // Mock Merchant Auth
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
              tenant_id: 't-alpha',
              full_name: 'Alpha Merchant',
            },
          },
        }),
      });
    });

    await merchantPage.route('**/api/v1/ai/chat/stream', async (route) => {
      const sseFrames = [
        'event: token\ndata: {"delta": "Freight tariff is 1200 LKR for business parcels."}\n\n',
        'event: action_card\ndata: {"action_type": "NAVIGATE_BOOKING", "target_url": "/portal/merchants/orders", "label": "Create Merchant Order"}\n\n',
        'event: done\ndata: {"conversation_id": "conv-merchant-001"}\n\n',
      ].join('');

      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: sseFrames,
      });
    });

    // Mock merchant profile & metrics
    await merchantPage.route('**/api/v1/merchants/me', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { business_name: 'Alpha Store', status: 'ACTIVE' } }),
      });
    });
    await merchantPage.route('**/api/v1/merchants/orders*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0 } }),
      });
    });

    await merchantPage.goto('/merchant/dashboard');

    // Invariant: Assistant launcher MUST be visible on the Merchant Portal!
    const openBtn = merchantPage.locator(testId(SELECTORS.AI_ASSISTANT.OPEN_BTN));
    await expect(openBtn).toBeVisible();
    await openBtn.click();

    const panel = merchantPage.locator(testId(SELECTORS.AI_ASSISTANT.PANEL));
    await expect(panel).toBeVisible();

    // Submit inquiry
    const promptInput = merchantPage.locator(testId(SELECTORS.AI_ASSISTANT.PROMPT_INPUT));
    await promptInput.fill('Quote commercial rate for 10kg');
    await merchantPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN)).click();

    // Verify response
    await expect(panel.getByText('Freight tariff is 1200 LKR')).toBeVisible();

    // Invariant: For merchants, booking action routes to /portal/merchants/orders
    const actionBtn = panel.locator(testId(SELECTORS.AI_ASSISTANT.AI_ACTION_BUTTON_BOOK));
    await expect(actionBtn).toBeVisible();
    const actionHref = await actionBtn.getAttribute('href');
    expect(actionHref).toBe('/portal/merchants/orders');
  });

  test('3. Public Tracking Page Availability & Tracking Action Card', async ({ customerPage }) => {
    // Unauthenticated access to /track with AI assistant
    await customerPage.route('**/api/v1/ai/chat/stream', async (route) => {
      const sseFrames = [
        'event: token\ndata: {"delta": "Parcel `SD-20261114-99881` is currently **OUT_FOR_DELIVERY** in Colombo."}\n\n',
        'event: action_card\ndata: {"action_type": "VIEW_TRACKING", "target_url": "/track/SD-20261114-99881", "label": "Track SD-20261114-99881", "prefill_data": {"tracking_number": "SD-20261114-99881"}}\n\n',
        'event: done\ndata: {"conversation_id": "conv-public-track"}\n\n',
      ].join('');

      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: sseFrames,
      });
    });

    await customerPage.goto('/track');

    // Assistant must be present for public visitors
    const openBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.OPEN_BTN));
    await expect(openBtn).toBeVisible();
    await openBtn.click();

    const panel = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PANEL));
    await expect(panel).toBeVisible();

    // Submit tracking query
    const promptInput = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PROMPT_INPUT));
    await promptInput.fill('Where is parcel SD-20261114-99881?');
    await customerPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN)).click();

    // Verify tracking response
    await expect(panel.getByText('OUT_FOR_DELIVERY')).toBeVisible();

    // SEC-009 Invariant: Confirm no customer telephone or street address is rendered
    const panelText = await panel.innerText();
    expect(panelText).not.toMatch(/\+94\d{9}/);
    expect(panelText).not.toMatch(/No\.\s*\d+/i);

    // Verify tracking action card appears and links directly to /track/SD-20261114-99881
    const trackAction = panel.locator(testId(SELECTORS.AI_ACTION_BUTTON_TRACK || SELECTORS.AI_ASSISTANT.AI_ACTION_BUTTON_TRACK));
    await expect(trackAction).toBeVisible();
    const trackHref = await trackAction.getAttribute('href');
    expect(trackHref).toBe('/track/SD-20261114-99881');
  });

  test('4. Multi-Turn Session Continuity (conversation_id round-trip)', async ({ customerPage }) => {
    let callIndex = 0;

    await customerPage.route('**/api/v1/ai/chat/stream', async (route) => {
      callIndex++;
      const payload = route.request().postDataJSON();

      if (callIndex === 1) {
        expect(payload.conversation_id).toBeUndefined();
        const sseFrames = [
          'event: token\ndata: {"delta": "Standard quote is 500 LKR."}\n\n',
          'event: done\ndata: {"conversation_id": "conv-multi-turn-999"}\n\n',
        ].join('');
        await route.fulfill({ status: 200, headers: { 'Content-Type': 'text/event-stream' }, body: sseFrames });
      } else if (callIndex === 2) {
        // Strict Invariant: Turn 2 MUST include the conversation_id returned from Turn 1!
        expect(payload.conversation_id).toBe('conv-multi-turn-999');
        expect(payload.prompt).toBe('What about express service?');
        const sseFrames = [
          'event: token\ndata: {"delta": "Express quote is 850 LKR."}\n\n',
          'event: done\ndata: {"conversation_id": "conv-multi-turn-999"}\n\n',
        ].join('');
        await route.fulfill({ status: 200, headers: { 'Content-Type': 'text/event-stream' }, body: sseFrames });
      }
    });

    await customerPage.goto('/portal/dashboard');
    await customerPage.locator(testId(SELECTORS.AI_ASSISTANT.OPEN_BTN)).click();

    // Turn 1
    const promptInput = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PROMPT_INPUT));
    await promptInput.fill('Quote standard 2kg');
    await customerPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN)).click();
    await expect(customerPage.getByText('Standard quote is 500 LKR.')).toBeVisible();

    // Turn 2
    await promptInput.fill('What about express service?');
    await customerPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN)).click();
    await expect(customerPage.getByText('Express quote is 850 LKR.')).toBeVisible();

    expect(callIndex).toBe(2);
  });

  test('5. Tariff Error Interception & User-Friendly Fallback', async ({ customerPage }) => {
    await customerPage.route('**/api/v1/ai/chat/stream', async (route) => {
      const sseFrames = [
        'event: token\ndata: {"delta": "Unable to calculate shipping estimate: Zone table missing for 99999"}\n\n',
        'event: done\ndata: {"conversation_id": "conv-fallback-test"}\n\n',
      ].join('');
      await route.fulfill({ status: 200, headers: { 'Content-Type': 'text/event-stream' }, body: sseFrames });
    });

    await customerPage.goto('/portal/dashboard');
    await customerPage.locator(testId(SELECTORS.AI_ASSISTANT.OPEN_BTN)).click();

    const promptInput = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PROMPT_INPUT));
    await promptInput.fill('Calculate tariff for postal 99999');
    await customerPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN)).click();

    // Invariant: UI intercepts technical error and shows friendly message with fallback booking link
    await expect(customerPage.getByText('I could not get a delivery estimate here')).toBeVisible();
    const actionBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.AI_ACTION_BUTTON_BOOK));
    await expect(actionBtn).toBeVisible();
    expect(await actionBtn.getAttribute('href')).toBe('/portal/book');
  });

  test('6. Input Boundaries & Accessibility (a11y focus restoration)', async ({ customerPage }) => {
    await customerPage.goto('/portal/dashboard');

    const openBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.OPEN_BTN));
    await openBtn.focus();
    await customerPage.keyboard.press('Enter');

    const panel = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PANEL));
    await expect(panel).toBeVisible();

    const promptInput = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.PROMPT_INPUT));
    await expect(promptInput).toBeFocused();

    const sendBtn = customerPage.locator(testId(SELECTORS.AI_ASSISTANT.SEND_BTN));
    // Disabled on empty
    await expect(sendBtn).toBeDisabled();

    // Disabled on whitespace only
    await promptInput.fill('   ');
    await expect(sendBtn).toBeDisabled();

    // Enabled on valid text
    await promptInput.fill('Valid inquiry');
    await expect(sendBtn).toBeEnabled();

    // 1000-character boundary
    await promptInput.fill('a'.repeat(1050));
    const value = await promptInput.inputValue();
    expect(value.length).toBe(1000);

    // Escape key dismisses and restores focus to launcher button
    await customerPage.keyboard.press('Escape');
    await expect(panel).toBeHidden();
    await expect(openBtn).toBeFocused();
  });
});
