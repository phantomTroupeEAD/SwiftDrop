import { test as base, Page, BrowserContext } from '@playwright/test';

/**
 * Custom Playwright test fixture providing 5 concurrent browser contexts
 * for full-system real-time orchestration testing:
 *
 * 1. customerContext / customerPage (User A - Customer booking)
 * 2. applicantContext / applicantPage (User B - Customer applying for Merchant)
 * 3. adminContext / adminPage (System Administrator - Approvals & System Oversight)
 * 4. dispatcherContext / dispatcherPage (Central Dispatcher - Board & Courier assignment)
 * 5. courierContext / courierPage (Driver Mobile PWA - Stop run & Signature POD)
 *
 * INVARIANT: Each actor runs in a completely isolated browser context with zero session crosstalk.
 */

type SystemOrchestrationFixtures = {
  customerContext: BrowserContext;
  customerPage: Page;

  applicantContext: BrowserContext;
  applicantPage: Page;

  adminContext: BrowserContext;
  adminPage: Page;

  dispatcherContext: BrowserContext;
  dispatcherPage: Page;

  courierContext: BrowserContext;
  courierPage: Page;
};

export const test = base.extend<SystemOrchestrationFixtures>({
  // Window 1: User A (Customer)
  customerContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      baseURL: process.env.BASE_URL || 'http://localhost:3000',
      viewport: { width: 1280, height: 800 },
      userAgent: 'Mozilla/5.0 SwiftDrop-E2E-Window1-Customer',
    });
    await use(context);
    await context.close();
  },
  customerPage: async ({ customerContext }, use) => {
    const page = await customerContext.newPage();
    await use(page);
  },

  // Window 2: User B (Customer applying for Merchant)
  applicantContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      baseURL: process.env.BASE_URL || 'http://localhost:3000',
      viewport: { width: 1280, height: 800 },
      userAgent: 'Mozilla/5.0 SwiftDrop-E2E-Window2-ApplicantMerchant',
    });
    await use(context);
    await context.close();
  },
  applicantPage: async ({ applicantContext }, use) => {
    const page = await applicantContext.newPage();
    await use(page);
  },

  // Window 3: Admin Dashboard
  adminContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      baseURL: process.env.BASE_URL || 'http://localhost:3000',
      viewport: { width: 1440, height: 900 },
      userAgent: 'Mozilla/5.0 SwiftDrop-E2E-Window3-Admin',
    });
    await use(context);
    await context.close();
  },
  adminPage: async ({ adminContext }, use) => {
    const page = await adminContext.newPage();
    await use(page);
  },

  // Window 4: Dispatcher Board
  dispatcherContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      baseURL: process.env.BASE_URL || 'http://localhost:3000',
      viewport: { width: 1440, height: 900 },
      userAgent: 'Mozilla/5.0 SwiftDrop-E2E-Window4-Dispatcher',
    });
    await use(context);
    await context.close();
  },
  dispatcherPage: async ({ dispatcherContext }, use) => {
    const page = await dispatcherContext.newPage();
    await use(page);
  },

  // Window 5: Courier Mobile PWA (Touch-enabled Pixel 7)
  courierContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      baseURL: process.env.BASE_URL || 'http://localhost:3000',
      viewport: { width: 375, height: 812 },
      isMobile: true,
      hasTouch: true,
      userAgent: 'Mozilla/5.0 (Linux; Android 13; Pixel 7) SwiftDrop-E2E-Window5-Courier',
    });
    await use(context);
    await context.close();
  },
  courierPage: async ({ courierContext }, use) => {
    const page = await courierContext.newPage();
    await use(page);
  },
});

export { expect } from '@playwright/test';
