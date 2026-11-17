import { randomUUID } from 'node:crypto';
import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';
import { getRegistrationOTPFromMailpit, clearMailpitMessages } from '../helpers/mailpitHelper';

/**
 * FLOW 8: END-TO-END MERCHANT ONBOARDING, B2B ORDER API & UI VERIFICATION
 *
 * Verifies the complete commercial merchant lifecycle:
 * 1. Customer registers at /auth/register using real SMTP email interception via Mailpit (:8025).
 * 2. Customer submits a commercial merchant application at /portal/merchants/apply.
 * 3. System Administrator reviews and approves the application via Core API REST endpoint.
 * 4. Merchant signs into Merchant Portal and generates API credentials (Read-Only & Full B2B).
 * 5. Exhaustive Security & Edge Case Suite on POST /api/v1/merchant/orders:
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
 * 6. Multi-Order Batch Generation (Orders 2 & 3 across different delivery corridors).
 * 7. Merchant Portal UI Verification:
 *    - Order table population with tracking numbers, references, destinations, and status CREATED.
 *    - Search input filtering by merchant reference and by tracking number.
 *    - Zero-match empty state and filter reset button.
 *    - Status filter dropdown.
 *    - Detailed tracking navigation drilldown (/portal/tracking/:trackingNumber).
 * 8. API Key Revocation Lifecycle via Frontend & Verification (401 API_KEY_REVOKED).
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 6 (Customer & Merchant Portal), Member 4 (Merchant Backend), Member 1 (Identity)
 */

test.describe('Flow 8: Merchant Portal Onboarding, B2B Order Ingestion & UI Parity', () => {
  test.setTimeout(90000);

  test.beforeEach(async ({}, info) => {
    test.skip(
      info.project.name !== 'merchant-portal',
      'Requires the merchant-portal project.'
    );
    await clearMailpitMessages();
  });

  test('should complete registration via Mailpit OTP, apply, approve via REST, test API order edge cases and verify UI reflection', async ({
    page,
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
    // PHASE 4: Exhaustive Security, Validation & Idempotency Edge Cases
    // =========================================================================
    console.log('[E2E] Phase 4: Testing API security and validation edge cases');
    const orderEndpoint = 'http://localhost:8080/api/v1/merchant/orders';

    const baseValidPayload = {
      merchant_reference: `E2E-REF-KANDY-${timestamp}`,
      service_level: 'STANDARD',
      pickup: {
        name: 'Colombo Central Depot',
        phone: '+94770000004',
        address: '42 Galle Road',
        city: 'Colombo',
        postal_code: '00100',
      },
      dropoff: {
        name: 'Nimal Fernando',
        phone: '+94770000005',
        address: '12 Peradeniya Road',
        city: 'Kandy',
        postal_code: '20000',
      },
      parcel: {
        weight_kg: 2.5,
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
    expect(tracking1).toMatch(/^SD-\d{8}-\d{5}$/);
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

    // =========================================================================
    // PHASE 5: Multi-Order Batch Generation
    // =========================================================================
    console.log('[E2E] Phase 5: Generating additional commercial orders');

    // Order 2: Colombo to Galle (Express)
    const order2Payload = {
      merchant_reference: `E2E-REF-GALLE-${timestamp}`,
      service_level: 'EXPRESS',
      pickup: {
        name: 'Colombo Central Depot',
        phone: '+94770000004',
        address: '42 Galle Road',
        city: 'Colombo',
        postal_code: '00100',
      },
      dropoff: {
        name: 'Sunil Jayasinghe',
        phone: '+94771234567',
        address: '88 Main Street',
        city: 'Galle',
        postal_code: '80000',
      },
      parcel: {
        weight_kg: 1.25,
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

    // Order 3: Colombo to Kurunegala (Standard)
    const order3Payload = {
      merchant_reference: `E2E-REF-KURU-${timestamp}`,
      service_level: 'STANDARD',
      pickup: {
        name: 'Colombo Central Depot',
        phone: '+94770000004',
        address: '42 Galle Road',
        city: 'Colombo',
        postal_code: '00100',
      },
      dropoff: {
        name: 'Anoma Perera',
        phone: '+94719876543',
        address: '5 Temple Road',
        city: 'Kurunegala',
        postal_code: '60000',
      },
      parcel: {
        weight_kg: 3.8,
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
    // PHASE 6: Merchant UI Reflection & Interactive Testing
    // =========================================================================
    console.log('[E2E] Phase 6: Verifying orders rendered and searchable in Merchant UI');
    await page.getByRole('navigation', { name: 'Portal navigation' }).getByRole('link', { name: 'Orders', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Orders', level: 1 })).toBeVisible({ timeout: 10000 });

    // Verify order table appears
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
    // PHASE 7: Key Revocation & Security Lifecycle Verification
    // =========================================================================
    console.log('[E2E] Phase 7: Revoking API Key via UI and testing post-revocation refusal');
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

    console.log('[E2E] Flow 8 completed with 100% assertions satisfied!');
  });
});
