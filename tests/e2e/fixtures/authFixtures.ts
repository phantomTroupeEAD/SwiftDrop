import { test as base, Page, BrowserContext } from '@playwright/test';
import { TEST_USERS, TEST_ENDPOINTS } from './testData';

/**
 * Custom Playwright test fixtures providing strict multi-actor browser context isolation.
 *
 * INVARIANT: No two actors ever share cookies, local storage, or session state.
 * Each fixture provisions a fresh `browser.newContext()` ensuring zero cross-actor bleed.
 */

type ActorFixtures = {
  customerContext: BrowserContext;
  customerPage: Page;

  merchantContext: BrowserContext;
  merchantPage: Page;

  dispatcherContext: BrowserContext;
  dispatcherPage: Page;

  courierContext: BrowserContext;
  courierPage: Page;

  adminContext: BrowserContext;
  adminPage: Page;
};

export const test = base.extend<ActorFixtures>({
  // 1. Customer Context & Page (Desktop Chrome 1280x800)
  customerContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) SwiftDrop-E2E-Customer',
    });
    await use(context);
    await context.close();
  },
  customerPage: async ({ customerContext }, use) => {
    const page = await customerContext.newPage();
    await use(page);
  },

  // 2. Merchant Context & Page (Desktop Chrome 1366x768)
  merchantContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      viewport: { width: 1366, height: 768 },
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) SwiftDrop-E2E-Merchant',
      extraHTTPHeaders: {
        'X-Tenant-ID': TEST_USERS.MERCHANT_ALPHA.tenantId,
      },
    });
    await use(context);
    await context.close();
  },
  merchantPage: async ({ merchantContext }, use) => {
    const page = await merchantContext.newPage();
    await use(page);
  },

  // 3. Dispatcher Context & Page (Desktop Chrome 1440x900)
  dispatcherContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 900 },
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) SwiftDrop-E2E-Dispatcher',
    });
    await use(context);
    await context.close();
  },
  dispatcherPage: async ({ dispatcherContext }, use) => {
    const page = await dispatcherContext.newPage();
    await use(page);
  },

  // 4. Courier Context & Page (Mobile Viewport: 375x812, Touch Enabled)
  courierContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      viewport: { width: 375, height: 812 },
      isMobile: true,
      hasTouch: true,
      userAgent: 'Mozilla/5.0 (Linux; Android 13; Pixel 7) SwiftDrop-E2E-Courier-PWA',
    });
    await use(context);
    await context.close();
  },
  courierPage: async ({ courierContext }, use) => {
    const page = await courierContext.newPage();
    await use(page);
  },

  // 5. Admin Context & Page (Desktop Chrome 1440x900)
  adminContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 900 },
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) SwiftDrop-E2E-Admin',
    });
    await use(context);
    await context.close();
  },
  adminPage: async ({ adminContext }, use) => {
    const page = await adminContext.newPage();
    await use(page);
  },
});

export { expect } from '@playwright/test';
