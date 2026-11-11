import { defineConfig, devices } from '@playwright/test';
import * as dotenv from 'dotenv';
import * as path from 'path';

// Load optional .env.test if present
dotenv.config({ path: path.resolve(__dirname, '.env.test') });

/**
 * SwiftDrop Cross-Module E2E Test Suite Configuration
 * Governed by Member 10 (Cross-Workflow Quality Coordination)
 * Supports 5 distinct actor profiles:
 * - Customer Portal
 * - Merchant Portal
 * - Dispatcher Board
 * - Courier Mobile PWA
 * - Admin Portal
 */
export default defineConfig({
  testDir: './flows',
  timeout: 30000,
  expect: {
    timeout: 5000,
  },
  fullyParallel: false, // Run flows deterministically to protect transactional ordering
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : 1, // Single worker avoids database cross-test contention

  reporter: [
    ['list'],
    ['html', { outputFolder: 'artifacts/report', open: 'never' }],
  ],

  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:3000',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
    screenshot: 'only-on-failure',
    actionTimeout: 10000,
    navigationTimeout: 15000,
  },

  projects: [
    {
      name: 'customer-portal',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1280, height: 800 },
      },
    },
    {
      name: 'merchant-portal',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1366, height: 768 },
      },
    },
    {
      name: 'dispatcher-board',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1440, height: 900 },
      },
    },
    {
      name: 'courier-pwa',
      use: {
        ...devices['Pixel 7'],
        isMobile: true,
        hasTouch: true,
      },
    },
    {
      name: 'admin-portal',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1440, height: 900 },
      },
    },
  ],
});
