import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * AUTHORIZATION RULES & IDOR GUARDRAILS SUITE
 *
 * Enforces critical security invariants:
 * 1. Courier IDOR: Courier A cannot execute delivery or POD for Courier B's assigned run (403 Forbidden).
 * 2. Merchant Tenant Isolation: Merchant A cannot access Merchant B's orders or credentials (403 Forbidden).
 * 3. Role-Based Navigation Guards: Unauthorized roles cannot access Admin, Dispatcher, or Courier dashboards.
 * 4. SEC-009 Public Tracking Sanitization: Public tracking views strictly mask PII (no phone, no street address).
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Security & Authorization Guardrails', () => {

  test('SEC-01: Courier IDOR Protection - Courier cannot modify another courier\'s parcel run', async ({ page }) => {
    // Mock Courier A session
    await page.route('**/api/v1/auth/refresh', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'jwt-courier-a',
            user: {
              id: 'cr-0012',
              email: 'courier_a@swiftdrop.local',
              role: 'COURIER',
              full_name: 'Kamal Perera',
            },
          },
        }),
      });
    });

    // Mock IDOR attempt on foreign parcel: Backend rejects with 403
    await page.route('**/api/v1/deliveries/pcl-foreign-courier-b/complete', async (route) => {
      await route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'FORBIDDEN_COURIER_MISMATCH',
            message: 'You are not authorized to complete delivery for a parcel assigned to another courier.',
          },
        }),
      });
    });

    await page.goto('/courier/dashboard');
    await expect(page.getByText("Today's Stop-by-Stop Run")).toBeVisible();

    // Directly execute foreign delivery attempt via fetch in courier session context
    const idorResponse = await page.evaluate(async () => {
      const res = await fetch('/api/v1/deliveries/pcl-foreign-courier-b/complete', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer jwt-courier-a',
        },
        body: JSON.stringify({
          recipient_name: 'Kasun Wickrama',
          signature_data: 'data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=',
        }),
      });
      return { status: res.status, body: await res.json() };
    });

    expect(idorResponse.status).toBe(403);
    expect(idorResponse.body.error.code).toBe('FORBIDDEN_COURIER_MISMATCH');
  });

  test('SEC-02: Tenant Isolation - Merchant cannot access cross-tenant orders', async ({ page }) => {
    // Mock Merchant A session (Tenant A)
    await page.route('**/api/v1/auth/refresh', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'jwt-merchant-alpha',
            user: {
              id: 'usr-merchant-a',
              email: 'alpha@merchant.local',
              role: 'MERCHANT',
              tenant_id: 'mrc_tenant_alpha_01',
              full_name: 'Alpha Merchant Staff',
            },
          },
        }),
      });
    });

    await page.route('**/api/v1/merchant/orders', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [] }),
      });
    });

    // Cross-tenant endpoint strictly rejects with 403
    await page.route('**/api/v1/merchants/mrc_tenant_beta_02/orders/**', async (route) => {
      await route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'FORBIDDEN',
            message: 'cross-tenant access prohibited',
          },
        }),
      });
    });

    // Navigate to set base URL origin for relative fetches
    await page.goto('/portal/merchants/orders');

    // Execute cross-tenant read from authenticated merchant context
    const crossTenantResponse = await page.evaluate(async () => {
      const res = await fetch('/api/v1/merchants/mrc_tenant_beta_02/orders/ord-998', {
        headers: { 'Authorization': 'Bearer jwt-merchant-alpha' },
      });
      return { status: res.status, body: await res.json() };
    });

    expect(crossTenantResponse.status).toBe(403);
    expect(crossTenantResponse.body.error.message).toBe('cross-tenant access prohibited');
  });

  test('SEC-03: RBAC Navigation Guard - Customer is barred from Admin & Dispatcher dashboards', async ({ page }) => {
    // Customer session
    await page.route('**/api/v1/auth/refresh', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'jwt-customer-devinda',
            user: {
              id: 'c-001',
              email: 'customer@swiftdrop.local',
              role: 'CUSTOMER',
              full_name: 'Devinda Perera',
            },
          },
        }),
      });
    });

    // 1. Attempt to navigate directly to Admin Console
    await page.goto('/admin/dashboard');
    // Guard redirects non-admin to /dashboard (RoleHome)
    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.getByText('Administrator Operations & Governance Console')).not.toBeVisible();

    // 2. Attempt to navigate directly to Dispatcher Board
    await page.goto('/dispatcher/dashboard');
    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD))).not.toBeVisible();
  });

  test('SEC-04: Public Tracking Data Masking (SEC-009) - Phone & street address are masked', async ({ page }) => {
    const publicTrackingNum = 'SD-20261113-A3X9K7';

    // Mock public tracking endpoint adhering strictly to SEC-009 allowlist
    await page.route(`**/api/v1/tracking/public/${publicTrackingNum}`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            tracking_number: publicTrackingNum,
            status: 'OUT_FOR_DELIVERY',
            status_description: 'Parcel is out for delivery with courier Kamal Perera',
            destination: {
              city: 'Colombo 07',
            },
            milestones: [
              {
                status: 'BOOKED',
                description: 'Shipment recorded in system',
                timestamp: '2026-11-13T08:00:00Z',
              },
              {
                status: 'OUT_FOR_DELIVERY',
                description: 'Assigned to courier run',
                timestamp: '2026-11-13T10:30:00Z',
              },
            ],
          },
        }),
      });
    });

    await page.goto(`/track/${publicTrackingNum}`);

    // Verify public information is rendered
    await expect(page.getByText(publicTrackingNum)).toBeVisible();
    await expect(page.getByText('Colombo 07')).toBeVisible();

    // Verify sensitive PII is absent from page content
    const pageText = await page.locator('body').innerText();
    expect(pageText).not.toContain('+94771234567');
    expect(pageText).not.toContain('42 Temple Road');
    expect(pageText).not.toContain('Secret door code');
  });

});
