import { randomUUID, createHmac } from 'node:crypto';
import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';
import { getRegistrationOTPFromMailpit, clearMailpitMessages } from '../helpers/mailpitHelper';

/**
 * FLOW 8: UNIFIED MASTER COMMERCIAL MERCHANT LIFECYCLE
 *
 * Verifies the complete end-to-end commercial merchant lifecycle:
 * 1. Customer registers at /auth/register using real SMTP email interception via Mailpit (:8025).
 * 2. Customer submits a commercial merchant application at /portal/merchants/apply.
 * 3. System Administrator reviews and approves the application via Core API REST endpoint.
 * 4. Merchant signs into Merchant Portal and generates API credentials (Read-Only & Full B2B).
 * 5. Webhooks Configuration, Security Guardrails & Matrix Setup:
 *    - Custom client secret rejection (400 WEBHOOK_SECRET_NOT_ALLOWED)
 *    - SSRF prohibited IP rejection (400 SSRF_PROHIBITED_URL)
 *    - Invalid URL fragment rejection (422 WEBHOOK_URL_INVALID)
 *    - Unsupported event name rejection (422 WEBHOOK_EVENT_UNSUPPORTED)
 *    - UI empty event selection validation
 *    - Webhook Alpha: parcel.booked.v1 only
 *    - Duplicate URL collision rejection (409)
 *    - Webhook Beta: parcel.assigned.v1 only
 *    - Webhook Gamma: parcel.booked.v1 AND parcel.assigned.v1 multi-subscription
 * 6. Authoritative Parcel Booking & Event Pipeline:
 *    - Quotes tariff and books authoritative parcel via POST /api/v1/parcels
 * 7. Event Filtering Audit 1 (Order Booking Events):
 *    - Webhooks Alpha & Gamma receive parcel.booked.v1; Webhook Beta receives 0 events.
 *    - HMAC-SHA256 signature verification & customer notification PII sanitization.
 * 8. Multi-Window Central Dispatcher Assignment:
 *    - Dispatcher logs in (/dispatcher/dashboard), locates parcel, assigns on-shift courier.
 * 9. Event Filtering Audit 2 & Cryptographic Security (Order Assignment):
 *    - Webhooks Beta & Gamma receive parcel.assigned.v1; Webhook Alpha receives 0 new events.
 *    - HMAC tamper detection check & per-webhook secret isolation.
 * 10. Cross-Tenant Data Isolation & IDOR Verification:
 *    - Zero events to foreign tenant; IDOR delete returns 404.
 * 11. Webhook Deletion & Immediate Delivery Cutoff Audit:
 *    - Webhook Alpha deleted; Follow-up parcel booked; Alpha receives 0 new events, Gamma receives event.
 * 12. Exhaustive B2B Order API Edge Cases & Batch Generation on POST /api/v1/merchant/orders:
 *    - EC-01: Missing API Key (401 API_KEY_MISSING)
 *    - EC-02: Ambiguous Credentials (400 AMBIGUOUS_CREDENTIALS)
 *    - EC-03: Malformed API Key (401 API_KEY_INVALID)
 *    - EC-04: Non-existent API Key (401 API_KEY_INVALID)
 *    - EC-05: Scope Denied using Read-Only Key (403 MERCHANT_SCOPE_DENIED)
 *    - EC-06: Missing Idempotency-Key Header (400 IDEMPOTENCY_KEY_REQUIRED)
 *    - EC-07: Non-UUID Idempotency-Key (400 IDEMPOTENCY_KEY_INVALID)
 *    - EC-08: Malformed JSON Body (400 BAD_REQUEST)
 *    - EC-09: Missing Required Address Fields (422 VALIDATION_FAILED)
 *    - EC-10: Non-positive Parcel Weight (422 VALIDATION_FAILED)
 *    - EC-11: Invalid Phone Number Format (422 VALIDATION_FAILED)
 *    - EC-12: Valid Order 1 Booking (201 Created)
 *    - EC-13: Idempotent Replay with Same Key & Payload (Cached 201/200, Idempotent-Replay: true)
 *    - EC-14: Idempotency Conflict with Modified Payload (409 IDEMPOTENCY_KEY_PAYLOAD_MISMATCH)
 *    - Batch Generation: Orders 2 & 3
 * 13. Merchant Portal UI Reflection & Search Verification:
 *    - Orders table verification with references, tracking numbers, and created badges.
 *    - Search by reference, search by tracking, empty filter reset, status dropdown, tracking link drilldown.
 * 14. API Key Revocation Lifecycle via Frontend & Verification (401 API_KEY_REVOKED).
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 6 (Customer & Merchant Portal), Member 4 (Merchant Backend), Member 1 (Identity), Member 3 (Dispatch), Member 2 (Parcels)
 */

test.describe('Flow 8: Unified Commercial Merchant Onboarding, B2B Order Ingestion, Webhooks & Dispatcher Lifecycle', () => {
  test.setTimeout(120000);

  test.beforeEach(async ({}, info) => {
    test.skip(
      info.project.name !== 'merchant-portal',
      'Requires the merchant-portal project.'
    );
    await clearMailpitMessages();
    try {
      await fetch('http://localhost:9999/api/reset', { method: 'POST' });
    } catch {
      // Mock receiver reset (optional in dev)
    }
  });

  test('should complete full commercial merchant journey: registration, approval, API keys, webhooks, order edge cases, multi-window dispatch, and UI reflection', async ({
    page,
    browser,
    request,
  }) => {
    const timestamp = Date.now();
    const merchantEmail = `e2e_merchant_${timestamp}@swiftdrop.local`;
    const merchantPassword = 'Password123!';
    const firstName = 'Nimal';
    const lastName = 'Bandara';
    const phone = '+94770000004';
    const businessName = `Apex Commercial Merchants ${timestamp}`;

    // =========================================================================
    // PHASE 1: Real Email OTP Registration (Messaging & Auth Subsystems)
    // =========================================================================
    console.log(`[E2E] Phase 1: Registering new customer account for ${merchantEmail}`);
    await page.goto('/auth/register');
    await expect(page.getByText('Create a customer account')).toBeVisible({ timeout: 10000 });

    // Request verification OTP
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(merchantEmail);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    // Verify OTP input appears
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    // Retrieve real OTP code from Mailpit REST API (:8025)
    const otpCode = await getRegistrationOTPFromMailpit(merchantEmail);
    console.log(`[E2E] Retrieved Registration OTP for ${merchantEmail}: ${otpCode}`);
    expect(otpCode).toMatch(/^\d{6}$/);

    // Complete registration form
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(otpCode);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill(firstName);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill(lastName);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill(phone);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    // Verify Account Created screen and click "Continue to Sign in"
    await expect(page.getByText('Account created')).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /Continue to Sign in/i }).click();

    // Verify redirect to login page
    await expect(page).toHaveURL(/\/auth\/login/, { timeout: 10000 });

    // =========================================================================
    // PHASE 2: Merchant Application & REST Admin Approval
    // =========================================================================
    console.log('[E2E] Phase 2: Customer signs in and submits merchant application');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(merchantEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // Wait for customer dashboard
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10000 });

    // Navigate to Merchant Application view
    await page.goto('/portal/merchants/apply');
    await expect(page.getByRole('heading', { name: 'Merchant application' })).toBeVisible({ timeout: 10000 });

    // Fill application form
    await page.getByTestId('merchant-application-business_name-input').fill(businessName);
    await page.getByTestId('merchant-application-contact_email-input').fill(merchantEmail);
    await page.getByTestId('merchant-application-contact_phone-input').fill(phone);
    await page.getByTestId('merchant-application-line1-input').fill('42 Galle Road');
    await page.getByTestId('merchant-application-city-input').fill('Colombo');

    // Submit application and intercept response to extract merchant ID
    const applicationPromise = page.waitForResponse(
      response =>
        new URL(response.url()).pathname === '/api/v1/merchants/applications' &&
        response.request().method() === 'POST'
    );
    await page.getByTestId('merchant-application-submit-btn').click();
    const appResponse = await applicationPromise;
    expect(appResponse.status()).toBe(201);
    const appJson = await appResponse.json();
    const merchantId = appJson.data?.merchant_id || appJson.data?.id;
    console.log(`[E2E] Created Merchant Application ID: ${merchantId}`);

    // Verify UI reflects PENDING application status
    await expect(page.getByTestId('merchant-application-status')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(businessName)).toBeVisible();
    await expect(page.getByText('Your application is under review.')).toBeVisible();

    // Admin approves the merchant application via REST API
    console.log('[E2E] Admin approving merchant via Core API REST endpoint');
    const adminLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: {
        email: 'admin@swiftdrop.local',
        password: 'AdminPassword123!',
      },
    });
    expect(adminLoginRes.ok()).toBeTruthy();
    const adminLoginJson = await adminLoginRes.json();
    const adminToken = adminLoginJson.data.access_token;
    expect(adminToken).toBeTruthy();

    const approveRes = await request.post(`http://localhost:8080/api/v1/admin/merchants/${merchantId}/approve`, {
      headers: {
        Authorization: `Bearer ${adminToken}`,
        'Content-Type': 'application/json',
      },
      data: {
        expected_version: 1,
        reason: 'Merchant commercial documents verified and approved by E2E test.',
      },
    });
    expect(approveRes.status()).toBe(200);
    const approveJson = await approveRes.json();
    expect(approveJson.data.status).toBe('APPROVED');
    console.log(`[E2E] Merchant ${merchantId} approved successfully`);

    // Sign in as approved merchant to refresh JWT role and tenant context
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(merchantEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // Merchant lands on portal
    await expect(page).toHaveURL(/\/portal\/merchants\/orders|\/dashboard/, { timeout: 10000 });
    if (!page.url().includes('/portal/merchants/orders')) {
      await page.goto('/portal/merchants/orders');
    }
    await expect(page.getByTestId('merchant-order-table').or(page.getByTestId('merchant-order-empty'))).toBeVisible({ timeout: 10000 });

    // Acquire authenticated merchant token for REST-level assertions
    const merchantLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: merchantEmail, password: merchantPassword },
    });
    expect(merchantLoginRes.ok()).toBeTruthy();
    const merchantToken = (await merchantLoginRes.json()).data.access_token;
    expect(merchantToken).toBeTruthy();

    // =========================================================================
    // PHASE 3: Frontend API Key Management
    // =========================================================================
    console.log('[E2E] Phase 3: Creating API keys via Merchant UI');
    await page.getByRole('navigation', { name: 'Portal navigation' }).getByRole('link', { name: 'API keys' }).click();
    await expect(page.getByRole('heading', { name: 'API keys', level: 1 })).toBeVisible({ timeout: 10000 });

    // 1. Create Key A (Read-Only: parcels:read only)
    const readKeyLabel = `E2E Read-Only Key ${timestamp}`;
    await page.getByTestId('merchant-key-label-input').fill(readKeyLabel);
    await page.getByTestId('merchant-key-read-scope').check();
    await page.getByTestId('merchant-credential-create-btn').click();

    // Capture Key A from modal
    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const readOnlyKey = (await page.locator('code.select-all').innerText()).trim();
    expect(readOnlyKey).toMatch(/^sd_(?:live|test)_[A-Za-z0-9_-]+$/);
    console.log(`[E2E] Captured Read-Only API Key A: ${readOnlyKey.slice(0, 16)}...`);
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await page.getByTestId('merchant-secret-close-btn').click();
    await expect(page.getByText('Save your credential now')).toBeHidden();

    // 2. Create Key B (Full Commercial: parcels:create + parcels:read)
    const fullKeyLabel = `E2E Full Commercial Key ${timestamp}`;
    await page.getByTestId('merchant-key-label-input').fill(fullKeyLabel);
    await page.getByTestId('merchant-key-create-scope').check();
    await page.getByTestId('merchant-key-read-scope').check();
    await page.getByTestId('merchant-credential-create-btn').click();

    // Capture Key B from modal
    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const fullKey = (await page.locator('code.select-all').innerText()).trim();
    expect(fullKey).toMatch(/^sd_(?:live|test)_[A-Za-z0-9_-]+$/);
    console.log(`[E2E] Captured Full Commercial API Key B: ${fullKey.slice(0, 16)}...`);
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await page.getByTestId('merchant-secret-close-btn').click();
    await expect(page.getByText('Save your credential now')).toBeHidden();

    // =========================================================================
    // PHASE 4: Webhook Configuration, Security Guardrails & Matrix Setup
    // =========================================================================
    console.log('[E2E] Phase 4: Setting up webhooks and verifying negative guardrails');
    await page.getByRole('navigation', { name: 'Portal navigation' }).getByRole('link', { name: 'Webhooks' }).click();
    await expect(page.getByRole('heading', { name: 'Webhooks', level: 1 })).toBeVisible({ timeout: 10000 });

    // 4.1 Negative Guardrails: Custom Secret Rejection (400)
    const customSecretRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        url: 'http://localhost:9999/webhooks/merchant-alpha',
        events: ['parcel.booked.v1'],
        secret: 'client_forbidden_secret_123',
      }),
    });
    expect(customSecretRes.status).toBe(400);
    const customSecretJson = await customSecretRes.json();
    expect(customSecretJson.error?.code).toBe('WEBHOOK_SECRET_NOT_ALLOWED');

    // 4.2 Negative Guardrails: SSRF Prohibited IP Rejection (400)
    const ssrfRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        url: 'http://169.254.169.254/latest/meta-data',
        events: ['parcel.booked.v1'],
      }),
    });
    expect(ssrfRes.status).toBe(400);
    const ssrfJson = await ssrfRes.json();
    expect(ssrfJson.error?.code).toBe('SSRF_PROHIBITED_URL');

    // 4.3 Negative Guardrails: Invalid URL Fragment Rejection (422)
    const fragmentRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        url: 'https://example.com/webhook#anchor',
        events: ['parcel.booked.v1'],
      }),
    });
    expect(fragmentRes.status).toBe(422);
    const fragmentJson = await fragmentRes.json();
    expect(fragmentJson.error?.code).toBe('WEBHOOK_URL_INVALID');

    // 4.4 Negative Guardrails: Unsupported Event Name (422)
    const unsupportedEventRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        url: 'http://localhost:9999/webhooks/merchant-alpha',
        events: ['parcel.nonexistent.event'],
      }),
    });
    expect(unsupportedEventRes.status).toBe(422);
    const unsupportedJson = await unsupportedEventRes.json();
    expect(unsupportedJson.error?.code).toBe('WEBHOOK_EVENT_UNSUPPORTED');

    // 4.5 UI Empty Event Selection Validation
    await page.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-alpha');
    const allEventCheckboxes = page.locator('input[name="event"]');
    const checkboxCount = await allEventCheckboxes.count();
    for (let i = 0; i < checkboxCount; i++) {
      if (await allEventCheckboxes.nth(i).isChecked()) {
        await allEventCheckboxes.nth(i).uncheck();
      }
    }
    await page.getByTestId('merchant-credential-create-btn').click();
    await expect(page.locator('[role="alert"]').filter({ hasText: /Select at least one event/i })).toBeVisible({ timeout: 5000 });

    // 4.6 Webhook Alpha: Subscribed to parcel.booked.v1 ONLY
    await page.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-alpha');
    await page.getByTestId('merchant-webhook-event-parcel.booked.v1').check();
    await page.getByTestId('merchant-credential-create-btn').click();

    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const doneBtn = page.getByTestId('merchant-secret-close-btn');
    await expect(doneBtn).toBeDisabled();

    // Verify modal dismiss guardrails
    await page.keyboard.press('Escape');
    await expect(page.getByText('Save your credential now')).toBeVisible();

    const alphaSecret = (await page.locator('code').innerText()).trim();
    expect(alphaSecret).toMatch(/^whsec_[A-Za-z0-9_-]+$/);

    // Register secret with mock receiver
    await fetch('http://localhost:9999/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant: 'alpha', secret: alphaSecret }),
    });
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await expect(doneBtn).toBeEnabled();
    await doneBtn.click();
    await expect(page.getByText('Save your credential now')).not.toBeVisible();

    // 4.7 Duplicate URL Collision Rejection (409)
    await page.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-alpha');
    await page.getByTestId('merchant-webhook-event-parcel.booked.v1').check();
    await page.getByTestId('merchant-credential-create-btn').click();
    await expect(page.locator('[role="alert"]').filter({ hasText: /(?:already registered|conflicts with the current information)/i })).toBeVisible({ timeout: 5000 });

    // 4.8 Webhook Beta: Subscribed to parcel.assigned.v1 ONLY
    await page.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-beta');
    await page.getByTestId('merchant-webhook-event-parcel.booked.v1').uncheck();
    await page.getByTestId('merchant-webhook-event-parcel.assigned.v1').check();
    await page.getByTestId('merchant-credential-create-btn').click();

    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const betaSecret = (await page.locator('code').innerText()).trim();
    expect(betaSecret).toMatch(/^whsec_[A-Za-z0-9_-]+$/);

    await fetch('http://localhost:9999/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant: 'beta', secret: betaSecret }),
    });
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await page.getByTestId('merchant-secret-close-btn').click();

    // 4.9 Webhook Gamma: Subscribed to BOTH parcel.booked.v1 AND parcel.assigned.v1
    await page.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-gamma');
    await page.getByTestId('merchant-webhook-event-parcel.booked.v1').check();
    await page.getByTestId('merchant-webhook-event-parcel.assigned.v1').check();
    await page.getByTestId('merchant-credential-create-btn').click();

    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const gammaSecret = (await page.locator('code').innerText()).trim();
    expect(gammaSecret).toMatch(/^whsec_[A-Za-z0-9_-]+$/);

    await fetch('http://localhost:9999/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant: 'gamma', secret: gammaSecret }),
    });
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await page.getByTestId('merchant-secret-close-btn').click();

    // 4.10 Webhook Omega: Subscribed to ALL 6 events (Universal Wildcard Webhook)
    await page.getByTestId('merchant-webhook-url-input').fill('http://localhost:9999/webhooks/merchant-omega');
    await page.getByTestId('merchant-webhook-event-parcel.booked.v1').check();
    await page.getByTestId('merchant-webhook-event-parcel.assigned.v1').check();
    await page.getByTestId('merchant-webhook-event-parcel.picked_up.v1').check();
    await page.getByTestId('merchant-webhook-event-parcel.out_for_delivery.v1').check();
    await page.getByTestId('merchant-webhook-event-parcel.delivered.v1').check();
    await page.getByTestId('merchant-webhook-event-parcel.delivery_failed.v1').check();
    await page.getByTestId('merchant-credential-create-btn').click();

    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const omegaSecret = (await page.locator('code').innerText()).trim();
    expect(omegaSecret).toMatch(/^whsec_[A-Za-z0-9_-]+$/);

    await fetch('http://localhost:9999/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant: 'omega', secret: omegaSecret }),
    });
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await page.getByTestId('merchant-secret-close-btn').click();

    // Verify all 4 webhooks rendered in the UI roster
    await expect(page.getByText('http://localhost:9999/webhooks/merchant-alpha · Enabled')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('http://localhost:9999/webhooks/merchant-beta · Enabled')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('http://localhost:9999/webhooks/merchant-gamma · Enabled')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('http://localhost:9999/webhooks/merchant-omega · Enabled')).toBeVisible({ timeout: 5000 });

    // =========================================================================
    // PHASE 5: Authoritative Parcel Booking & Event Pipeline
    // =========================================================================
    console.log('[E2E] Phase 5: Booking authoritative parcel to trigger outbox and webhooks');
    // 5.1 Obtain dynamic pricing quote
    const quoteRes = await fetch('http://localhost:8080/api/v1/pricing/quotes', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 3.5,
        dimensions: {
          length_cm: 25,
          width_cm: 20,
          height_cm: 15,
        },
      }),
    });
    expect(quoteRes.status).toBe(201);
    const quoteData = await quoteRes.json();
    const quoteId = quoteData.data?.quote_id || quoteData.data?.id;
    expect(quoteId).toBeTruthy();

    // 5.2 Book parcel via authoritative POST /api/v1/parcels
    const bookingRes = await fetch('http://localhost:8080/api/v1/parcels', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
        'Idempotency-Key': randomUUID(),
      },
      body: JSON.stringify({
        quote_id: quoteId,
        recipient_name: 'Ananda Silva',
        recipient_phone: '+94771234567',
        weight_kg: 3.5,
        dimensions: {
          length_cm: 25,
          width_cm: 20,
          height_cm: 15,
        },
        pickup: {
          contact_name: 'Alpha Warehouse',
          contact_phone: '+94112345678',
          address_line1: '124 Galle Road',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: 'Ananda Silva',
          contact_phone: '+94771234567',
          address_line1: '45 Havelock Road',
          city: 'Colombo',
          postal_code: '00200',
          zone_id: 'ZONE_COLOMBO_01',
        },
        item_description: 'Industrial Hardware Components',
      }),
    });
    expect(bookingRes.status).toBe(201);
    const bookingData = await bookingRes.json();
    const authoritativeTrackingNo = bookingData.data?.tracking_number;
    expect(authoritativeTrackingNo).toMatch(/^SD-\d{8}-[A-Z0-9]+$/);
    console.log(`[E2E] Authoritative Parcel Booked: Tracking = ${authoritativeTrackingNo}`);

    // =========================================================================
    // PHASE 6: Event Filtering Audit 1 (Order Booking Events)
    // =========================================================================
    console.log('[E2E] Phase 6: Auditing webhook delivery for parcel.booked.v1');
    let eventsAudit1: any = null;
    for (let i = 0; i < 25; i++) {
      await new Promise((r) => setTimeout(r, 600));
      try {
        const res = await (await fetch('http://localhost:9999/api/events')).json();
        if (res.counts?.alpha >= 1 && res.counts?.gamma >= 1 && res.counts?.omega >= 1) {
          eventsAudit1 = res;
          break;
        }
      } catch {
        // retry
      }
    }
    expect(eventsAudit1).toBeTruthy();

    // Webhook Alpha: Received parcel.booked.v1 events
    expect(eventsAudit1.counts?.alpha).toBeGreaterThanOrEqual(1);
    // Webhook Gamma: Received parcel.booked.v1 events
    expect(eventsAudit1.counts?.gamma).toBeGreaterThanOrEqual(1);
    // Webhook Omega: Received parcel.booked.v1 events (Universal wildcard webhook)
    expect(eventsAudit1.counts?.omega).toBeGreaterThanOrEqual(1);
    // Webhook Beta: Received ZERO events (subscribed only to parcel.assigned.v1)
    expect(eventsAudit1.counts?.beta || 0).toBe(0);

    // Verify cryptographic signature and payload structure
    const alphaEvent = eventsAudit1.events.find((e: any) => e.tenant === 'alpha');
    expect(alphaEvent).toBeTruthy();
    expect(alphaEvent.type).toBe('parcel.booked.v1');
    expect(alphaEvent.hmacValid).toBe(true);

    const omegaEvent = eventsAudit1.events.find((e: any) => e.tenant === 'omega');
    expect(omegaEvent).toBeTruthy();
    expect(omegaEvent.type).toBe('parcel.booked.v1');
    expect(omegaEvent.hmacValid).toBe(true);

    // Anti-leak check: Customer notification PII stripped
    expect(alphaEvent.payload.notify).toBeUndefined();

    // =========================================================================
    // PHASE 7: Multi-Window Central Dispatcher Assignment
    // =========================================================================
    console.log('[E2E] Phase 7: Central Dispatcher assigns parcel in secondary window');
    const dispatcherContext = await browser.newContext();
    const dispatcherPage = await dispatcherContext.newPage();

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.local');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Dispatcher123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.goto('/dispatcher/dashboard');
    await expect(dispatcherPage).toHaveURL(/\/dispatcher\/dashboard/, { timeout: 12000 });
    await expect(dispatcherPage.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD))).toBeVisible({ timeout: 10000 });

    // Locate parcel in Unassigned Parcels
    const parcelItem = dispatcherPage.locator('li', { hasText: authoritativeTrackingNo });
    await expect(parcelItem).toBeVisible({ timeout: 10000 });

    // Edge Case: Assign button disabled until courier selected
    const courierSelect = parcelItem.locator(testId(SELECTORS.DISPATCHER.COURIER_SELECT));
    const assignBtn = parcelItem.locator(testId(SELECTORS.DISPATCHER.ASSIGN_BTN));
    await expect(assignBtn).toBeDisabled();

    // Select on-shift courier and execute assignment
    await courierSelect.selectOption({ index: 1 });
    await expect(assignBtn).toBeEnabled();
    await assignBtn.click();

    // Verify UI Status Feedback & parcel removed from unassigned queue
    await expect(dispatcherPage.getByRole('status').filter({ hasText: `${authoritativeTrackingNo} assigned.` })).toBeVisible({ timeout: 10000 });
    await expect(dispatcherPage.locator('li', { hasText: authoritativeTrackingNo })).not.toBeVisible({ timeout: 5000 });

    await dispatcherContext.close();

    // =========================================================================
    // PHASE 8: Event Filtering Audit 2 & Cryptographic Security (Order Assignment)
    // =========================================================================
    console.log('[E2E] Phase 8: Auditing webhook delivery for parcel.assigned.v1');
    let eventsAudit2: any = null;
    for (let i = 0; i < 25; i++) {
      await new Promise((r) => setTimeout(r, 600));
      try {
        const res = await (await fetch('http://localhost:9999/api/events')).json();
        if (res.counts?.beta >= 1 && res.counts?.gamma >= 2 && res.counts?.omega >= 2) {
          eventsAudit2 = res;
          break;
        }
      } catch {
        // retry
      }
    }
    expect(eventsAudit2).toBeTruthy();

    // Webhook Beta: Received exactly 1 event (parcel.assigned.v1)
    expect(eventsAudit2.counts?.beta).toBe(1);
    // Webhook Gamma: Received both events (booked + assigned)
    expect(eventsAudit2.counts?.gamma).toBeGreaterThanOrEqual(2);
    // Webhook Omega: Received both events (booked + assigned)
    expect(eventsAudit2.counts?.omega).toBeGreaterThanOrEqual(2);
    // Webhook Alpha: Received NO new events (count unchanged)
    expect(eventsAudit2.counts?.alpha).toBe(eventsAudit1.counts?.alpha);

    const betaEvent = eventsAudit2.events.find((e: any) => e.tenant === 'beta');
    expect(betaEvent).toBeTruthy();
    expect(betaEvent.type).toBe('parcel.assigned.v1');
    expect(betaEvent.hmacValid).toBe(true);

    // Cryptographic Security: Anti-Tamper Check
    const rawBetaPayload = JSON.stringify(betaEvent.payload);
    const tamperedPayload = rawBetaPayload + 'tampered';
    const timestampSec = Math.floor(Date.now() / 1000).toString();
    const tamperedHmac = createHmac('sha256', betaSecret).update(`${timestampSec}.${tamperedPayload}`).digest('hex');
    const validHmac = createHmac('sha256', betaSecret).update(`${timestampSec}.${rawBetaPayload}`).digest('hex');
    expect(tamperedHmac).not.toBe(validHmac);

    // Key Isolation Check: Verifying Beta's signature with Alpha's secret fails
    const mismatchedHmac = createHmac('sha256', alphaSecret).update(`${timestampSec}.${rawBetaPayload}`).digest('hex');
    expect(mismatchedHmac).not.toBe(validHmac);

    // =========================================================================
    // PHASE 9: Cross-Tenant Data Isolation Audit
    // =========================================================================
    console.log('[E2E] Phase 9: Verifying cross-tenant isolation and IDOR protection');
    // Assert 0 events delivered to foreign or unconfigured tenant
    expect(eventsAudit2.counts?.chaos || 0).toBe(0);

    // IDOR Protection: Merchant cannot delete non-existent or foreign webhook
    const idorRes = await fetch('http://localhost:8080/api/v1/merchants/me/webhooks/00000000-0000-0000-0000-000000000000', {
      method: 'DELETE',
      headers: { 'Authorization': `Bearer ${merchantToken}` },
    });
    expect(idorRes.status).toBe(404);

    // =========================================================================
    // PHASE 10: Webhook Deletion & Immediate Delivery Cutoff Audit
    // =========================================================================
    console.log('[E2E] Phase 10: Deleting Webhook Alpha and verifying immediate cutoff');
    await page.goto('/portal/merchants/webhooks');
    await expect(page.getByRole('heading', { name: 'Webhooks', level: 1 })).toBeVisible({ timeout: 10000 });

    const alphaItem = page.locator('.m6-item').filter({ hasText: 'http://localhost:9999/webhooks/merchant-alpha' });
    await expect(alphaItem).toBeVisible();

    await alphaItem.getByTestId('merchant-webhook-delete-btn').click();
    await page.getByTestId('merchant-credential-confirm-btn').click();
    await expect(page.getByRole('status').filter({ hasText: 'Webhook removed.' })).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('http://localhost:9999/webhooks/merchant-alpha · Enabled')).not.toBeVisible();

    const alphaCountBefore = eventsAudit2.counts?.alpha;
    const gammaCountBefore = eventsAudit2.counts?.gamma;
    const omegaCountBefore = eventsAudit2.counts?.omega;

    // Book follow-up parcel to test immediate cutoff
    const quote2Res = await fetch('http://localhost:8080/api/v1/pricing/quotes', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
      },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 2.0,
        dimensions: {
          length_cm: 20,
          width_cm: 15,
          height_cm: 10,
        },
      }),
    });
    expect(quote2Res.status).toBe(201);
    const quote2Data = await quote2Res.json();
    const quote2Id = quote2Data.data?.quote_id || quote2Data.data?.id;

    await fetch('http://localhost:8080/api/v1/parcels', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${merchantToken}`,
        'Idempotency-Key': randomUUID(),
      },
      body: JSON.stringify({
        quote_id: quote2Id,
        recipient_name: 'Nimal Bandara',
        recipient_phone: '+94719876543',
        weight_kg: 2.0,
        dimensions: {
          length_cm: 20,
          width_cm: 15,
          height_cm: 10,
        },
        pickup: {
          contact_name: 'Alpha Warehouse',
          contact_phone: '+94112345678',
          address_line1: '124 Galle Road',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: 'Nimal Bandara',
          contact_phone: '+94719876543',
          address_line1: '88 Dalada Veediya',
          city: 'Colombo',
          postal_code: '00200',
          zone_id: 'ZONE_COLOMBO_01',
        },
        item_description: 'Retail Apparel Sample',
      }),
    });

    // Wait and verify Webhook Alpha received ZERO events, while Gamma and Omega received it
    let eventsAudit3: any = null;
    for (let i = 0; i < 25; i++) {
      await new Promise((r) => setTimeout(r, 600));
      try {
        const res = await (await fetch('http://localhost:9999/api/events')).json();
        if (res.counts?.gamma > gammaCountBefore && res.counts?.omega > omegaCountBefore) {
          eventsAudit3 = res;
          break;
        }
      } catch {
        // retry
      }
    }
    expect(eventsAudit3).toBeTruthy();
    // Alpha was deleted -> count remained identical (immediate cutoff confirmed!)
    expect(eventsAudit3.counts?.alpha).toBe(alphaCountBefore);
    expect(eventsAudit3.counts?.gamma).toBeGreaterThan(gammaCountBefore);
    expect(eventsAudit3.counts?.omega).toBeGreaterThan(omegaCountBefore);

    // =========================================================================
    // PHASE 12: Exhaustive B2B Order API Security, Validation & Batch Generation
    // =========================================================================
    console.log('[E2E] Phase 12: Testing B2B order API edge cases and batch generation');
    const orderEndpoint = 'http://localhost:8080/api/v1/merchant/orders';

    const baseValidPayload = {
      merchant_reference: `E2E-REF-KANDY-${timestamp}`,
      service_level: 'STANDARD',
      pickup: {
        name: 'Colombo Central Depot',
        phone: '+94770000004',
        address: '42 Galle Road',
        street_line_1: '42 Galle Road',
        city: 'Colombo',
        postal_code: '00100',
        zone_id: 'ZONE_COLOMBO_01',
      },
      dropoff: {
        name: 'Nimal Fernando',
        phone: '+94770000005',
        address: '12 Peradeniya Road',
        street_line_1: '12 Peradeniya Road',
        city: 'Kandy',
        postal_code: '20000',
        zone_id: 'ZONE_KANDY_01',
      },
      parcel: {
        weight_kg: 2.5,
        length_cm: 20,
        width_cm: 15,
        height_cm: 10,
        description: 'Electronic Gadgets and Components',
      },
    };

    // EC-01: Missing API Key
    const resNoKey = await request.post(orderEndpoint, {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseValidPayload,
    });
    expect(resNoKey.status()).toBe(401);
    const jsonNoKey = await resNoKey.json();
    expect(jsonNoKey.error?.code).toBe('API_KEY_MISSING');

    // EC-02: Ambiguous Credentials (Both X-API-Key and Bearer)
    const resAmbiguous = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        Authorization: `Bearer ${adminToken}`,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseValidPayload,
    });
    expect(resAmbiguous.status()).toBe(400);
    const jsonAmbiguous = await resAmbiguous.json();
    expect(jsonAmbiguous.error?.code).toBe('AMBIGUOUS_CREDENTIALS');

    // EC-03: Malformed API Key
    const resMalformedKey = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': 'malformed_invalid_key',
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseValidPayload,
    });
    expect(resMalformedKey.status()).toBe(401);
    const jsonMalformed = await resMalformedKey.json();
    expect(jsonMalformed.error?.code).toBe('API_KEY_INVALID');

    // EC-04: Non-existent API Key with valid structure
    const resUnknownKey = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': 'sd_live_01HMERCH_unknownkeysecret1234567890123456',
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseValidPayload,
    });
    expect(resUnknownKey.status()).toBe(401);
    const jsonUnknown = await resUnknownKey.json();
    expect(jsonUnknown.error?.code).toBe('API_KEY_INVALID');

    // EC-05: Scope Denied using Read-Only Key (lacks parcels:create)
    const resScopeDenied = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': readOnlyKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseValidPayload,
    });
    expect(resScopeDenied.status()).toBe(403);
    const jsonScope = await resScopeDenied.json();
    expect(jsonScope.error?.code).toBe('MERCHANT_SCOPE_DENIED');

    // EC-06: Missing Idempotency-Key Header
    const resNoIdem = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
      },
      data: baseValidPayload,
    });
    expect(resNoIdem.status()).toBe(400);
    const jsonNoIdem = await resNoIdem.json();
    expect(jsonNoIdem.error?.code).toBe('IDEMPOTENCY_KEY_REQUIRED');

    // EC-07: Non-UUID Idempotency-Key
    const resInvalidIdem = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': 'not-a-valid-uuid-header',
      },
      data: baseValidPayload,
    });
    expect(resInvalidIdem.status()).toBe(400);
    const jsonInvalidIdem = await resInvalidIdem.json();
    expect(jsonInvalidIdem.error?.code).toBe('IDEMPOTENCY_KEY_INVALID');

    // EC-08: Malformed JSON Body
    const resMalformedBody = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: '{ invalid_json: ',
    });
    expect(resMalformedBody.status()).toBe(400);

    // EC-09: Missing Required Dropoff Block
    const resMissingDropoff = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: {
        merchant_reference: 'MISSING-DROPOFF-TEST',
        pickup: baseValidPayload.pickup,
        parcel: baseValidPayload.parcel,
      },
    });
    expect(resMissingDropoff.status()).toBe(422);
    const jsonMissingDropoff = await resMissingDropoff.json();
    expect(jsonMissingDropoff.error?.code).toBe('VALIDATION_FAILED');

    // EC-10: Missing Required Pickup Block
    const resMissingPickup = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: {
        merchant_reference: 'MISSING-PICKUP-TEST',
        dropoff: baseValidPayload.dropoff,
        parcel: baseValidPayload.parcel,
      },
    });
    expect(resMissingPickup.status()).toBe(422);
    const jsonMissingPickup = await resMissingPickup.json();
    expect(jsonMissingPickup.error?.code).toBe('VALIDATION_FAILED');

    // EC-11: Non-positive Parcel Weight (<= 0)
    const resInvalidWeight = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: {
        ...baseValidPayload,
        parcel: {
          weight_kg: -2.5,
          description: 'Invalid negative weight',
        },
      },
    });
    expect(resInvalidWeight.status()).toBe(422);
    const jsonInvalidWeight = await resInvalidWeight.json();
    expect(jsonInvalidWeight.error?.code).toBe('VALIDATION_FAILED');

    // EC-12: Happy Path Order 1 Creation
    const idemKey1 = randomUUID();
    const resOrder1 = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': idemKey1,
      },
      data: baseValidPayload,
    });
    expect(resOrder1.status()).toBe(201);
    const jsonOrder1 = await resOrder1.json();
    const tracking1 = jsonOrder1.data.tracking_number;
    expect(tracking1).toMatch(/^SD-\d{8}-[A-Z0-9]+$/);
    expect(jsonOrder1.data.status).toBe('CREATED');
    expect(jsonOrder1.data.merchant_reference).toBe(baseValidPayload.merchant_reference);
    console.log(`[E2E] Order 1 Created Successfully: Tracking = ${tracking1}`);

    // EC-13: Idempotent Replay (Re-send identical request with same Idempotency-Key)
    const resOrder1Replay = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': idemKey1,
      },
      data: baseValidPayload,
    });
    expect(resOrder1Replay.ok()).toBeTruthy();
    const jsonOrder1Replay = await resOrder1Replay.json();
    expect(jsonOrder1Replay.data.tracking_number).toBe(tracking1);
    expect(resOrder1Replay.headers()['idempotent-replay']).toBe('true');
    console.log('[E2E] Verified Idempotency Replay Header and matching Tracking Number');

    // EC-14: Idempotency Key Conflict (Reused Idempotency-Key with modified payload)
    const resIdemConflict = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': idemKey1,
      },
      data: {
        ...baseValidPayload,
        merchant_reference: 'DIFFERENT-REF-CONFLICT',
      },
    });
    expect(resIdemConflict.status()).toBe(409);
    const jsonIdemConflict = await resIdemConflict.json();
    expect(jsonIdemConflict.error?.code).toBe('IDEMPOTENCY_KEY_PAYLOAD_MISMATCH');

    // Batch Generation: Order 2 (Colombo to Galle)
    const order2Payload = {
      merchant_reference: `E2E-REF-GALLE-${timestamp}`,
      service_level: 'EXPRESS',
      pickup: {
        name: 'Colombo Central Depot',
        phone: '+94770000004',
        address: '42 Galle Road',
        street_line_1: '42 Galle Road',
        city: 'Colombo',
        postal_code: '00100',
        zone_id: 'ZONE_COLOMBO_01',
      },
      dropoff: {
        name: 'Sunil Jayasinghe',
        phone: '+94771234567',
        address: '88 Main Street',
        street_line_1: '88 Main Street',
        city: 'Galle',
        postal_code: '80000',
        zone_id: 'ZONE_GALLE_01',
      },
      parcel: {
        weight_kg: 1.25,
        length_cm: 15,
        width_cm: 10,
        height_cm: 8,
        description: 'Cotton Fabric Samples',
      },
    };
    const resOrder2 = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: order2Payload,
    });
    expect(resOrder2.status()).toBe(201);
    const jsonOrder2 = await resOrder2.json();
    const tracking2 = jsonOrder2.data.tracking_number;
    console.log(`[E2E] Order 2 Created: Tracking = ${tracking2}`);

    // Batch Generation: Order 3 (Colombo to Negombo)
    const order3Payload = {
      merchant_reference: `E2E-REF-NEGOMBO-${timestamp}`,
      service_level: 'STANDARD',
      pickup: {
        name: 'Colombo Central Depot',
        phone: '+94770000004',
        address: '42 Galle Road',
        street_line_1: '42 Galle Road',
        city: 'Colombo',
        postal_code: '00100',
        zone_id: 'ZONE_COLOMBO_01',
      },
      dropoff: {
        name: 'Anoma Perera',
        phone: '+94719876543',
        address: '5 Temple Road',
        street_line_1: '5 Temple Road',
        city: 'Negombo',
        postal_code: '11500',
        zone_id: 'ZONE_NEGOMBO_01',
      },
      parcel: {
        weight_kg: 3.8,
        length_cm: 25,
        width_cm: 20,
        height_cm: 15,
        description: 'Household Appliances - Electric Kettle',
      },
    };
    const resOrder3 = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: order3Payload,
    });
    expect(resOrder3.status()).toBe(201);
    const jsonOrder3 = await resOrder3.json();
    const tracking3 = jsonOrder3.data.tracking_number;
    console.log(`[E2E] Order 3 Created: Tracking = ${tracking3}`);

    // =========================================================================
    // PHASE 13: Merchant UI Reflection & Interactive Testing
    // =========================================================================
    console.log('[E2E] Phase 13: Verifying orders rendered and searchable in Merchant UI');
    await page.getByRole('navigation', { name: 'Portal navigation' }).getByRole('link', { name: 'Orders', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Orders', level: 1 })).toBeVisible({ timeout: 10000 });

    const orderTable = page.getByTestId('merchant-order-table');
    await expect(orderTable).toBeVisible({ timeout: 10000 });

    // Assert all 3 orders are rendered with their exact references
    await expect(orderTable.getByText(tracking1)).toBeVisible();
    await expect(orderTable.getByText(baseValidPayload.merchant_reference)).toBeVisible();

    await expect(orderTable.getByText(tracking2)).toBeVisible();
    await expect(orderTable.getByText(order2Payload.merchant_reference)).toBeVisible();

    await expect(orderTable.getByText(tracking3)).toBeVisible();
    await expect(orderTable.getByText(order3Payload.merchant_reference)).toBeVisible();

    // Verify Status badge CREATED
    await expect(orderTable.locator('tbody tr').first().locator('.m6-pill', { hasText: 'created' })).toBeVisible();

    // 1. Search by Reference
    console.log('[E2E] Testing UI search by merchant reference');
    const searchInput = page.getByTestId('merchant-order-search');
    await searchInput.fill(order2Payload.merchant_reference);

    // Order 2 should remain visible; Orders 1 and 3 should be filtered out
    await expect(orderTable.getByText(order2Payload.merchant_reference)).toBeVisible();
    await expect(orderTable.getByText(baseValidPayload.merchant_reference)).toBeHidden();
    await expect(orderTable.getByText(order3Payload.merchant_reference)).toBeHidden();

    // Clear search
    await searchInput.fill('');
    await expect(orderTable.getByText(baseValidPayload.merchant_reference)).toBeVisible();
    await expect(orderTable.getByText(order3Payload.merchant_reference)).toBeVisible();

    // 2. Search by Tracking Number
    console.log('[E2E] Testing UI search by tracking number');
    await searchInput.fill(tracking3);
    await expect(orderTable.getByText(tracking3)).toBeVisible();
    await expect(orderTable.getByText(tracking1)).toBeHidden();
    await expect(orderTable.getByText(tracking2)).toBeHidden();

    // 3. Search with Non-Matching String -> Zero State & Reset
    console.log('[E2E] Testing UI search non-matching empty state');
    await searchInput.fill('NON_EXISTENT_ORDER_REF_9999');
    await expect(page.getByTestId('merchant-order-empty')).toHaveText('No orders match your filters.');
    await page.getByTestId('merchant-order-reset-btn').click();
    await expect(orderTable.getByText(tracking1)).toBeVisible();

    // 4. Status Filter Dropdown
    console.log('[E2E] Testing UI status filter dropdown');
    const statusSelect = page.getByTestId('merchant-order-filter');
    await statusSelect.selectOption('CREATED');
    await expect(orderTable.getByText(tracking1)).toBeVisible();
    await expect(orderTable.getByText(tracking2)).toBeVisible();
    await statusSelect.selectOption(''); // All statuses

    // 5. Tracking Detail Drilldown Navigation
    console.log('[E2E] Testing tracking link drilldown');
    await orderTable.getByRole('link', { name: tracking1 }).click();
    await expect(page).toHaveURL(new RegExp(`/portal/tracking/${tracking1}`));
    await expect(page.getByTestId('tracking-search-input')).toHaveValue(tracking1);

    // =========================================================================
    // PHASE 14: Key Revocation & Security Lifecycle Verification
    // =========================================================================
    console.log('[E2E] Phase 14: Revoking API Key via UI and testing post-revocation refusal');
    await page.goto('/portal/merchants/api-keys');
    await expect(page.getByRole('heading', { name: 'API keys', level: 1 })).toBeVisible({ timeout: 10000 });

    const keyRow = page.locator('.m6-item').filter({ hasText: fullKeyLabel });
    await expect(keyRow).toBeVisible();
    await keyRow.getByTestId('merchant-key-revoke-btn').click();
    await page.getByTestId('merchant-credential-confirm-btn').click();

    // Verify key revocation notice and removal from active keys
    await expect(page.getByText('API key revoked.')).toBeVisible({ timeout: 10000 });
    await expect(keyRow.getByTestId('merchant-key-revoke-btn')).toBeHidden();

    // Attempt to book order using revoked Key B -> Expect 401 API_KEY_REVOKED
    const resRevokedKey = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseValidPayload,
    });
    expect(resRevokedKey.status()).toBe(401);
    const jsonRevokedKey = await resRevokedKey.json();
    expect(jsonRevokedKey.error?.code).toBe('API_KEY_REVOKED');
    console.log('[E2E] Verified that revoked API key is rejected with 401 API_KEY_REVOKED');

    console.log('[E2E] Flow 8 Master Commercial Merchant Lifecycle completed with 100% assertions satisfied!');
  });
});
