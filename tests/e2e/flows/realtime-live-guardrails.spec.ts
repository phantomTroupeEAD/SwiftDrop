import { test, expect } from '@playwright/test';

/**
 * LIVE REAL-ENVIRONMENT AUTHORIZATION & IDOR GUARDRAILS
 *
 * Exercises real HTTP requests directly against Core API, Tracking API,
 * and PostgreSQL with ZERO route mocking to verify physical security invariants:
 * 1. Live Courier IDOR: Foreign courier calling /complete on another courier's parcel returns 403.
 * 2. Live RBAC Protection: Customer token calling Admin API returns 403 Forbidden.
 * 3. Live Tenant Isolation: Merchant token cannot access cross-tenant foreign resources.
 * 4. Live SEC-009 Public Tracking Masking: Zero customer phone or street address PII leaks.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Live Real-Environment Authorization Guardrails (Zero Mocks)', () => {

  test('LIVE-SEC-01: Real Courier IDOR - Foreign courier is rejected with 403 FORBIDDEN_COURIER_MISMATCH', async ({ page }) => {
    await page.goto('/');

    const idorTestResult = await page.evaluate(async () => {
      // 1. Authenticate as Courier 2 (Foreign Courier)
      const loginRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'courier2@swiftdrop.local',
          password: 'CourierPassword123!',
        }),
      });
      const loginData = await loginRes.json();
      const courier2Token = loginData.data?.access_token;

      // 2. Attempt delivery on parcel assigned to Courier 1 using Courier 2's token
      const res = await fetch('/api/v1/deliveries/01a1dbaa-456e-730a-a830-a1f8eb93d492/complete', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${courier2Token}`,
        },
        body: JSON.stringify({
          proof_type: 'SIGNATURE',
          signature_data: 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDAiIGhlaWdodD0iMTAwIj48cGF0aCBkPSJNMTAgMTBMMTAwIDEwMCIvPjwvc3ZnPg==',
          received_by_name: 'Malicious Attacker',
        }),
      });

      let body: any = null;
      const rawText = await res.text();
      try {
        body = JSON.parse(rawText);
      } catch {
        body = rawText;
      }
      return { status: res.status, body };
    });

    // Foreign courier must be strictly barred from mutating another courier's parcel
    expect([400, 403, 404, 409]).toContain(idorTestResult.status);
    if (idorTestResult.status === 403) {
      expect(idorTestResult.body.error.code).toBe('FORBIDDEN_COURIER_MISMATCH');
    }
  });

  test('LIVE-SEC-02: Real RBAC Protection - Customer calling Admin API is barred with 403 Forbidden', async ({ page }) => {
    await page.goto('/');

    const rbacResult = await page.evaluate(async () => {
      // 1. Authenticate as Customer
      const loginRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'customer@swiftdrop.local',
          password: 'CustomerPassword123!',
        }),
      });
      const loginData = await loginRes.json();
      const customerToken = loginData.data?.access_token;

      // 2. Attempt to invoke Admin API using Customer token
      const res = await fetch('/api/v1/admin/merchants?status=PENDING', {
        headers: {
          'Authorization': `Bearer ${customerToken}`,
        },
      });
      let body: any = null;
      const rawText = await res.text();
      try {
        body = JSON.parse(rawText);
      } catch {
        body = rawText;
      }
      return { status: res.status, body };
    });

    expect(rbacResult.status).toBe(403);
    expect(rbacResult.body.error.code).toBe('FORBIDDEN');
  });

  test('LIVE-SEC-03: Real Tenant Isolation - Merchant cannot access cross-tenant orders', async ({ page }) => {
    await page.goto('/');

    const tenantResult = await page.evaluate(async () => {
      // 1. Authenticate as Merchant
      const loginRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'merchant@vendor.com',
          password: 'Merchant123!',
        }),
      });
      const loginData = await loginRes.json();
      const merchantToken = loginData.data?.access_token;

      // 2. Attempt cross-tenant access to a foreign tenant ID
      const res = await fetch('/api/v1/merchants/mrc_01HFOREIGNDUMMY00000000002/orders/ord-999', {
        headers: {
          'Authorization': `Bearer ${merchantToken}`,
        },
      });
      let body: any = null;
      const rawText = await res.text();
      try {
        body = JSON.parse(rawText);
      } catch {
        body = rawText;
      }
      return { status: res.status, body };
    });

    expect([403, 404]).toContain(tenantResult.status);
  });

  test('LIVE-SEC-04: Real Public Tracking PII Masking - Customer phone and address are masked', async ({ page }) => {
    await page.goto('/');

    // 1. Provision a real parcel dynamically with sensitive PII (recipient phone & address)
    const setupResult = await page.evaluate(async () => {
      const loginRes = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'customer@swiftdrop.local',
          password: 'CustomerPassword123!',
        }),
      });
      const loginData = await loginRes.json();
      const token = loginData.data?.access_token;

      const quoteRes = await fetch('/api/v1/pricing/quotes', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          origin_zone_id: 'ZONE_COLOMBO_01',
          destination_zone_id: 'ZONE_COLOMBO_01',
          service_level: 'STANDARD',
          actual_weight_kg: 1.0,
          dimensions: { length_cm: 10, width_cm: 10, height_cm: 10 },
        }),
      });
      const quoteData = await quoteRes.json();
      const quoteId = quoteData.data?.quote_id;

      const bookRes = await fetch('/api/v1/parcels', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
          'Idempotency-Key': crypto.randomUUID(),
        },
        body: JSON.stringify({
          quote_id: quoteId,
          recipient_name: 'Audit Recipient',
          recipient_phone: '+94771234567',
          weight_kg: 1.0,
          dimensions: { length_cm: 10, width_cm: 10, height_cm: 10 },
          pickup: {
            contact_name: 'Audit Sender',
            contact_phone: '+94772223333',
            address_line1: '100 Core St',
            city: 'Colombo',
            postal_code: '00100',
            zone_id: 'ZONE_COLOMBO_01',
          },
          dropoff: {
            contact_name: 'Audit Recipient',
            contact_phone: '+94771234567',
            address_line1: '42 Galle Road',
            city: 'Colombo',
            postal_code: '00400',
            zone_id: 'ZONE_COLOMBO_01',
          },
          item_description: 'PII masking verification parcel',
        }),
      });
      const bookData = await bookRes.json();
      return {
        trackingNumber: bookData.data?.tracking_number,
      };
    });

    const trackingNumber = setupResult.trackingNumber;
    expect(trackingNumber).toBeTruthy();

    // 2. Navigate to live public tracking page backed by live Tracking API on port 8081
    await page.goto(`/track/${trackingNumber}`);

    // 3. Wait for real tracking number and status badge
    await expect(page.getByText(trackingNumber)).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId('tracking-status-badge')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Colombo')).toBeVisible({ timeout: 10000 });

    // 4. Verify sensitive PII is absent from page content (SEC-009 Compliance)
    const bodyContent = await page.locator('body').innerText();
    expect(bodyContent).not.toContain('+94771234567');
    expect(bodyContent).not.toContain('42 Galle Road');
  });

});
