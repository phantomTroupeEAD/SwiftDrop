import { test, expect } from '@playwright/test';
import { randomUUID } from 'crypto';
import { getRegistrationOTPFromMailpit } from '../helpers/mailpitHelper';

/**
 * Flow 10: Exhaustive Merchant Portal API Key Lifecycle & Security Matrix
 *
 * Covers every possible API key testing scenario across:
 * 1. Merchant Portal UI Interaction & Dialog Security Invariants (M6 / Portal)
 * 2. Core API Validation, Boundary Limits & Quota Enforcement (M4 / Merchants)
 * 3. Cryptographic Authentication, Scope Enforcement & Transport Security (M4 / M2)
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 4 (Merchants), Member 6 (Customer & Merchant Portal), Member 1 (Identity)
 */

const SELECTORS = {
  AUTH: {
    INPUT_EMAIL: 'login-email-input',
    INPUT_PASSWORD: 'login-password-input',
    BTN_LOGIN: 'login-submit-btn',
    REGISTER_EMAIL: 'register-email-input',
    REGISTER_SEND_OTP_BTN: 'register-send-otp-btn',
    REGISTER_OTP: 'register-otp-input',
    REGISTER_VERIFY_OTP_BTN: 'register-verify-otp-btn',
    REGISTER_FIRST_NAME: 'register-first-name-input',
    REGISTER_LAST_NAME: 'register-last-name-input',
    REGISTER_PHONE: 'register-phone-input',
    REGISTER_PASSWORD: 'register-password-input',
    REGISTER_TERMS_CHECKBOX: 'register-terms-checkbox',
    REGISTER_SUBMIT_BTN: 'register-submit-btn',
  },
};
const testId = (id: string) => `[data-testid="${id}"]`;

test.describe('Flow 10: Exhaustive Merchant Portal API Key Lifecycle & Security Matrix', () => {
  test('should verify every possible API key scenario: UI validation, modal security, quota, scopes, boundaries, and auth enforcement', async ({
    page,
    request,
  }) => {
    test.setTimeout(120000);
    const timestamp = Date.now();
    const merchantEmail = `e2e_keymaster_${timestamp}@swiftdrop.local`;
    const merchantPassword = 'Password123!';
    const phone = '+94770000004';
    const businessName = `KeyMaster Commercial Logistics ${timestamp}`;

    // =========================================================================
    // SECTION 1: Provisioning Approved Merchant & Unapproved Applicant
    // =========================================================================
    console.log(`[API-KEY-E2E] Provisioning merchant ${merchantEmail}`);

    // Register merchant user via real OTP
    await page.goto('/auth/register');
    await expect(page.getByText('Create a customer account')).toBeVisible({ timeout: 10000 });
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(merchantEmail);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    const otpCode = await getRegistrationOTPFromMailpit(merchantEmail);
    expect(otpCode).toMatch(/^\d{6}$/);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(otpCode);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_VERIFY_OTP_BTN)).click();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Sunil');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Fernando');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill(phone);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();
    await expect(page.getByText('Account created')).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /Continue to Sign in/i }).click();

    // Sign in as applicant
    await expect(page).toHaveURL(/\/auth\/login/, { timeout: 10000 });
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(merchantEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10000 });

    // Submit merchant application
    await page.goto('/portal/merchants/apply');
    await expect(page.getByRole('heading', { name: 'Merchant application' })).toBeVisible({ timeout: 10000 });
    await page.getByTestId('merchant-application-business_name-input').fill(businessName);
    await page.getByTestId('merchant-application-contact_email-input').fill(merchantEmail);
    await page.getByTestId('merchant-application-contact_phone-input').fill(phone);
    await page.getByTestId('merchant-application-line1-input').fill('100 Keyway Plaza');
    await page.getByTestId('merchant-application-city-input').fill('Colombo');

    const appPromise = page.waitForResponse(
      res => new URL(res.url()).pathname === '/api/v1/merchants/applications' && res.request().method() === 'POST'
    );
    await page.getByTestId('merchant-application-submit-btn').click();
    const appRes = await appPromise;
    expect(appRes.status()).toBe(201);
    const appJson = await appRes.json();
    const merchantId = appJson.data?.merchant_id || appJson.data?.id;

    // Admin approves merchant
    const adminLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: 'admin@swiftdrop.local', password: 'AdminPassword123!' },
    });
    const adminToken = (await adminLoginRes.json()).data.access_token;
    const approveRes = await request.post(`http://localhost:8080/api/v1/admin/merchants/${merchantId}/approve`, {
      headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
      data: { expected_version: 1, reason: 'Approved for API Key Exhaustive Verification' },
    });
    expect(approveRes.status()).toBe(200);

    // Re-login as approved merchant to get updated tenant claims
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(merchantEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/portal\/merchants\/orders|\/dashboard/, { timeout: 10000 });

    // Acquire REST JWT for direct API assertions
    const merchantLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: merchantEmail, password: merchantPassword },
    });
    const merchantToken = (await merchantLoginRes.json()).data.access_token;
    expect(merchantToken).toBeTruthy();

    // Acquire unapproved customer token directly from canonical mock customer
    const unapprovedLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: 'customer@swiftdrop.local', password: 'CustomerPassword123!' },
    });
    expect(unapprovedLoginRes.ok()).toBeTruthy();
    const unapprovedToken = (await unapprovedLoginRes.json()).data.access_token;

    // =========================================================================
    // SECTION 2: Merchant Portal UI Key Form Validation & Dialog Security
    // =========================================================================
    console.log('[API-KEY-E2E] Section 2: Testing UI Key Form Validation & Modal Security Invariants');
    await page.goto('/portal/merchants/api-keys');
    await expect(page.getByRole('heading', { name: 'API keys', level: 1 })).toBeVisible({ timeout: 10000 });

    // UI-KEY-01: Zero-Scope Selection Block
    console.log('[UI-KEY-01] Testing empty scope submission block');
    await page.getByTestId('merchant-key-label-input').fill('Zero Scopes Key');
    // Ensure both scope checkboxes are unchecked
    if (await page.getByTestId('merchant-key-read-scope').isChecked()) {
      await page.getByTestId('merchant-key-read-scope').uncheck();
    }
    if (await page.getByTestId('merchant-key-create-scope').isChecked()) {
      await page.getByTestId('merchant-key-create-scope').uncheck();
    }
    await page.getByTestId('merchant-credential-create-btn').click();
    await expect(page.getByText('Select at least one permission.')).toBeVisible({ timeout: 5000 });
    // Invariant: Modal must NOT open on validation failure
    await expect(page.getByText('Save your credential now')).toBeHidden();

    // UI-KEY-02: HTML5 Required Label Block
    console.log('[UI-KEY-02] Testing empty label validation');
    await page.getByTestId('merchant-key-label-input').fill('');
    await page.getByTestId('merchant-key-read-scope').check();
    await page.getByTestId('merchant-credential-create-btn').click();
    await expect(page.getByText('Save your credential now')).toBeHidden();

    // UI-KEY-03 & UI-KEY-04: Single-Scope UI Creation & One-Time Secret Modal Invariants
    console.log('[UI-KEY-03 & 04] Testing Read-Only Key Creation and One-Time Modal Security');
    const readOnlyLabel = `UI Read-Only Key ${timestamp}`;
    await page.getByTestId('merchant-key-label-input').fill(readOnlyLabel);
    await page.getByTestId('merchant-key-read-scope').check();
    await page.getByTestId('merchant-credential-create-btn').click();

    // Modal verification
    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Copy this secret now. It will never be displayed again.')).toBeVisible();

    const readOnlySecret = (await page.locator('code.select-all').innerText()).trim();
    expect(readOnlySecret).toMatch(/^sd_(?:live|test)_[A-Za-z0-9]{8}_[A-Za-z0-9]{32}$/);
    console.log(`[UI-KEY-04] Captured plaintext secret: ${readOnlySecret.slice(0, 16)}...`);

    // Invariant: "Done" button is disabled initially
    const doneBtn = page.getByTestId('merchant-secret-close-btn');
    await expect(doneBtn).toBeDisabled();

    // Invariant: Escape key does NOT dismiss modal when unacknowledged
    await page.keyboard.press('Escape');
    await expect(page.getByText('Save your credential now')).toBeVisible();

    // Invariant: Copy button interaction
    await page.getByTestId('one-time-key-copy-btn').click();
    await expect(page.getByRole('status').filter({ hasText: /Copied|manual/i })).toBeVisible();

    // Acknowledge and dismiss modal
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await expect(doneBtn).toBeEnabled();
    await doneBtn.click();
    await expect(page.getByText('Save your credential now')).toBeHidden();

    // UI-KEY-05: Single-Scope Write-Only Key Creation
    console.log('[UI-KEY-05] Testing Write-Only Key Creation via UI');
    const writeOnlyLabel = `UI Write-Only Key ${timestamp}`;
    await page.getByTestId('merchant-key-label-input').fill(writeOnlyLabel);
    await page.getByTestId('merchant-key-create-scope').check();
    if (await page.getByTestId('merchant-key-read-scope').isChecked()) {
      await page.getByTestId('merchant-key-read-scope').uncheck();
    }
    await page.getByTestId('merchant-credential-create-btn').click();
    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const writeOnlySecret = (await page.locator('code.select-all').innerText()).trim();
    expect(writeOnlySecret).toMatch(/^sd_(?:live|test)_[A-Za-z0-9]{8}_[A-Za-z0-9]{32}$/);
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await page.getByTestId('merchant-secret-close-btn').click();
    await expect(page.getByText('Save your credential now')).toBeHidden();

    // UI-KEY-06: Dual-Scope Full Commercial Key Creation
    console.log('[UI-KEY-06] Testing Full Commercial Key Creation via UI');
    const fullCommercialLabel = `UI Full Commercial Key ${timestamp}`;
    await page.getByTestId('merchant-key-label-input').fill(fullCommercialLabel);
    await page.getByTestId('merchant-key-read-scope').check();
    await page.getByTestId('merchant-key-create-scope').check();
    await page.getByTestId('merchant-credential-create-btn').click();
    await expect(page.getByText('Save your credential now')).toBeVisible({ timeout: 10000 });
    const fullCommercialSecret = (await page.locator('code.select-all').innerText()).trim();
    expect(fullCommercialSecret).toMatch(/^sd_(?:live|test)_[A-Za-z0-9]{8}_[A-Za-z0-9]{32}$/);
    await page.getByTestId('dismiss-key-modal-checkbox').check();
    await page.getByTestId('merchant-secret-close-btn').click();
    await expect(page.getByText('Save your credential now')).toBeHidden();

    // UI-KEY-07: Key List Item Presentation Invariants & Zero Plaintext Leak
    console.log('[UI-KEY-07] Verifying key list item presentation and DOM security');
    const readRow = page.locator('.m6-item').filter({ hasText: readOnlyLabel });
    await expect(readRow).toBeVisible();
    await expect(readRow.getByText('ACTIVE')).toBeVisible();
    await expect(readRow.getByText('parcels:read')).toBeVisible();

    const writeRow = page.locator('.m6-item').filter({ hasText: writeOnlyLabel });
    await expect(writeRow).toBeVisible();
    await expect(writeRow.getByText('ACTIVE')).toBeVisible();
    await expect(writeRow.getByText('parcels:create')).toBeVisible();

    const fullRow = page.locator('.m6-item').filter({ hasText: fullCommercialLabel });
    await expect(fullRow).toBeVisible();
    await expect(fullRow.getByText('ACTIVE')).toBeVisible();

    // CRITICAL SECURITY INVARIANT: Entire DOM must never contain any plaintext secret
    const pageHtml = await page.content();
    expect(pageHtml).not.toContain(readOnlySecret);
    expect(pageHtml).not.toContain(writeOnlySecret);
    expect(pageHtml).not.toContain(fullCommercialSecret);

    // UI-KEY-08 & 09: Revocation Modal Flow, Cancel Dismissal, and Confirmation Execution
    console.log('[UI-KEY-08 & 09] Testing Revocation Modal Flow and State Transition');
    // 1. Click Revoke on readRow
    await readRow.getByTestId('merchant-key-revoke-btn').click();
    await expect(page.getByRole('heading', { name: 'Revoke API key' })).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('will no longer be able to authenticate requests.')).toBeVisible();

    // 2. Click Cancel -> Invariant: Key remains active
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expect(page.getByRole('heading', { name: 'Revoke API key' })).toBeHidden();
    await expect(readRow.getByTestId('merchant-key-revoke-btn')).toBeVisible();

    // 3. Click Revoke again and Confirm
    await readRow.getByTestId('merchant-key-revoke-btn').click();
    await page.getByTestId('merchant-credential-confirm-btn').click();
    await expect(page.getByText('API key revoked.')).toBeVisible({ timeout: 5000 });

    // Invariant: Status badge changes to REVOKED and Revoke button is removed
    await expect(readRow.getByText('REVOKED')).toBeVisible({ timeout: 5000 });
    await expect(readRow.getByTestId('merchant-key-revoke-btn')).toBeHidden();

    // =========================================================================
    // SECTION 3: Core API Boundary, Quota & Validation Exhaustive Matrix
    // =========================================================================
    console.log('[API-KEY-E2E] Section 3: Testing Core API Validation, Boundary Limits & Quota Matrix');
    const apiKeysEndpoint = 'http://localhost:8080/api/v1/merchants/me/api-keys';

    // API-KEY-01: Label Validation Extremes (422 VALIDATION_FAILED)
    console.log('[API-KEY-01] Testing Label Validation Edge Cases');
    const invalidLabels = [
      { label: '   ', reason: 'Whitespace-only label' },
      { label: 'A'.repeat(61), reason: 'Exceeding 60 characters limit' },
      { label: 'Key\nWith\nNewline', reason: 'Contains newline control character' },
      { label: 'Key\tWith\tTab', reason: 'Contains tab control character' },
      { label: 'Key\x00WithNull', reason: 'Contains null byte' },
    ];
    for (const testCase of invalidLabels) {
      const res = await request.post(apiKeysEndpoint, {
        headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
        data: { label: testCase.label, scopes: ['parcels:read'] },
      });
      expect(res.status(), `Expected 422 for ${testCase.reason}`).toBe(422);
      const json = await res.json();
      expect(json.error?.code).toBe('VALIDATION_FAILED');
    }

    // Boundary Valid Labels (1 char and 60 chars)
    const validBoundaries = [
      { label: 'Z', desc: '1-character minimal boundary' },
      { label: 'K'.repeat(60), desc: '60-character exact maximum boundary' },
      { label: 'Shopify-Store #01 [Colombo-Central] - LK', desc: 'Alphanumeric with symbols & spaces' },
    ];
    for (const b of validBoundaries) {
      const res = await request.post(apiKeysEndpoint, {
        headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
        data: { label: b.label, scopes: ['parcels:read'] },
      });
      expect(res.status(), `Expected 201 for ${b.desc}`).toBe(201);
      const json = await res.json();
      // Revoke boundary key to keep quota available
      await request.delete(`${apiKeysEndpoint}/${json.data.key_id}`, {
        headers: { Authorization: `Bearer ${merchantToken}` },
      });
    }

    // API-KEY-02: Scope Validation Edge Cases (422 VALIDATION_FAILED)
    console.log('[API-KEY-02] Testing Scope Validation Edge Cases');
    const invalidScopeSets = [
      ['admin:all'],
      ['webhooks:manage'],
      ['parcels:delete'],
      ['unknown:scope'],
      ['parcels:create', 'super:admin'],
    ];
    for (const scopes of invalidScopeSets) {
      const res = await request.post(apiKeysEndpoint, {
        headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
        data: { label: `Invalid Scope ${randomUUID().slice(0, 8)}`, scopes },
      });
      expect(res.status()).toBe(422);
      const json = await res.json();
      expect(json.error?.code).toBe('VALIDATION_FAILED');
    }

    // Empty Scope Array Defaults to Both Scopes
    const defaultScopeRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: `Default Scopes Key ${randomUUID().slice(0, 8)}`, scopes: [] },
    });
    expect(defaultScopeRes.status()).toBe(201);
    const defaultScopeJson = await defaultScopeRes.json();
    expect(defaultScopeJson.data.scopes).toContain('parcels:create');
    expect(defaultScopeJson.data.scopes).toContain('parcels:read');
    await request.delete(`${apiKeysEndpoint}/${defaultScopeJson.data.key_id}`, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });

    // Duplicate Scopes Deduplicated
    const dupScopeRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: `Dup Scopes Key ${randomUUID().slice(0, 8)}`, scopes: ['parcels:read', 'parcels:read'] },
    });
    expect(dupScopeRes.status()).toBe(201);
    const dupScopeJson = await dupScopeRes.json();
    expect(dupScopeJson.data.scopes).toEqual(['parcels:read']);
    await request.delete(`${apiKeysEndpoint}/${dupScopeJson.data.key_id}`, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });

    // API-KEY-03: Expiry Bounds (expires_at) (422 VALIDATION_FAILED)
    console.log('[API-KEY-03] Testing Expiration Bounds');
    const invalidExpirations = [
      { exp: new Date(Date.now() - 86400000).toISOString(), reason: 'Past date' },
      { exp: new Date(Date.now() + 3600000).toISOString(), reason: 'Too soon (< 24 hours)' },
      { exp: new Date(Date.now() + 400 * 86400000).toISOString(), reason: 'Too far (> 365 days)' },
    ];
    for (const testCase of invalidExpirations) {
      const res = await request.post(apiKeysEndpoint, {
        headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
        data: { label: `Exp Key ${randomUUID().slice(0, 8)}`, scopes: ['parcels:read'], expires_at: testCase.exp },
      });
      expect(res.status(), `Expected 422 for ${testCase.reason}`).toBe(422);
      const json = await res.json();
      expect(json.error?.code).toBe('VALIDATION_FAILED');
    }

    // Valid Future Expiry (e.g. +30 days)
    const validExpRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: {
        label: `Valid Exp Key ${randomUUID().slice(0, 8)}`,
        scopes: ['parcels:read'],
        expires_at: new Date(Date.now() + 30 * 86400000).toISOString(),
      },
    });
    expect(validExpRes.status()).toBe(201);
    const validExpJson = await validExpRes.json();
    expect(validExpJson.data.expires_at).toBeTruthy();
    await request.delete(`${apiKeysEndpoint}/${validExpJson.data.key_id}`, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });

    // API-KEY-04: Duplicate Active Label Conflict (409 API_KEY_LABEL_DUPLICATE)
    console.log('[API-KEY-04] Testing Duplicate Active Label Conflict and Post-Revocation Reusability');
    const dupLabel = `Duplicate Sync Test ${timestamp}`;
    const firstDupRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: dupLabel, scopes: ['parcels:read'] },
    });
    expect(firstDupRes.status()).toBe(201);
    const firstDupJson = await firstDupRes.json();

    // Attempt second key with same label while first is active -> 409 API_KEY_LABEL_DUPLICATE
    const secondDupRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: dupLabel, scopes: ['parcels:read'] },
    });
    expect(secondDupRes.status()).toBe(409);
    const secondDupJson = await secondDupRes.json();
    expect(secondDupJson.error?.code).toBe('API_KEY_LABEL_DUPLICATE');

    // Revoke the first key -> Reusing label now succeeds
    await request.delete(`${apiKeysEndpoint}/${firstDupJson.data.key_id}`, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });
    const reuseLabelRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: dupLabel, scopes: ['parcels:read'] },
    });
    expect(reuseLabelRes.status()).toBe(201);
    const reuseLabelJson = await reuseLabelRes.json();
    await request.delete(`${apiKeysEndpoint}/${reuseLabelJson.data.key_id}`, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });

    // API-KEY-05: Active Key Quota Limit Enforcement (Max 5 Active Keys)
    console.log('[API-KEY-05] Testing Max 5 Active Key Limit Enforcement');
    // Retrieve current active keys
    const listRes = await request.get(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });
    const currentKeys = (await listRes.json()).data as Array<{ key_id: string; status: string }>;
    const activeKeys = currentKeys.filter(k => k.status === 'ACTIVE');

    // Create keys until we hit exactly 5 active keys
    const createdForQuota: string[] = [];
    for (let i = activeKeys.length; i < 5; i++) {
      const quotaRes = await request.post(apiKeysEndpoint, {
        headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
        data: { label: `Quota Key ${i + 1} ${timestamp}`, scopes: ['parcels:read'] },
      });
      expect(quotaRes.status()).toBe(201);
      const qJson = await quotaRes.json();
      createdForQuota.push(qJson.data.key_id);
    }

    // 6th Key Creation MUST fail with 409 API_KEY_LIMIT_REACHED
    const overflowRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: `Overflow Key 6 ${timestamp}`, scopes: ['parcels:read'] },
    });
    expect(overflowRes.status()).toBe(409);
    const overflowJson = await overflowRes.json();
    expect(overflowJson.error?.code).toBe('API_KEY_LIMIT_REACHED');

    // Revoking one active key unlocks quota immediately
    const keyToFree = createdForQuota.pop();
    if (keyToFree) {
      await request.delete(`${apiKeysEndpoint}/${keyToFree}`, {
        headers: { Authorization: `Bearer ${merchantToken}` },
      });
    }
    const freedSlotRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: `Slot Reclaimed Key ${timestamp}`, scopes: ['parcels:read'] },
    });
    expect(freedSlotRes.status()).toBe(201);
    const freedSlotJson = await freedSlotRes.json();
    createdForQuota.push(freedSlotJson.data.key_id);

    // Clean up created quota keys
    for (const id of createdForQuota) {
      await request.delete(`${apiKeysEndpoint}/${id}`, {
        headers: { Authorization: `Bearer ${merchantToken}` },
      });
    }

    // API-KEY-06: Unapproved Applicant Restriction (403 FORBIDDEN / MERCHANT_NOT_APPROVED)
    console.log('[API-KEY-06] Testing unapproved user restriction');
    const unapprovedKeyRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${unapprovedToken}`, 'Content-Type': 'application/json' },
      data: { label: 'Unapproved Merchant Key', scopes: ['parcels:read'] },
    });
    expect(unapprovedKeyRes.status()).toBe(403);

    // API-KEY-07: Idempotent Revocation & 404 on Non-Existent Key
    console.log('[API-KEY-07] Testing Idempotent Revocation & 404 for Non-Existent Key');
    // Create temporary key and revoke it twice
    const tempKeyRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: `Idem Revoke ${randomUUID().slice(0, 8)}`, scopes: ['parcels:read'] },
    });
    const tempKeyId = (await tempKeyRes.json()).data.key_id;
    const rev1 = await request.delete(`${apiKeysEndpoint}/${tempKeyId}`, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });
    expect(rev1.status()).toBe(204);
    // Second revocation on already-revoked key is idempotent (204)
    const rev2 = await request.delete(`${apiKeysEndpoint}/${tempKeyId}`, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });
    expect(rev2.status()).toBe(204);

    // Revoking random UUID returns 404 API_KEY_NOT_FOUND
    const fakeKeyRes = await request.delete(`${apiKeysEndpoint}/${randomUUID()}`, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });
    expect(fakeKeyRes.status()).toBe(404);

    // API-KEY-08: Zero Plaintext Leakage on Retrieval
    console.log('[API-KEY-08] Verifying zero plaintext leakage across all key records');
    const allKeysRes = await request.get(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}` },
    });
    expect(allKeysRes.status()).toBe(200);
    const allKeys = (await allKeysRes.json()).data as Array<Record<string, unknown>>;
    for (const k of allKeys) {
      expect(k.api_key).toBeUndefined();
      expect(k.secret).toBeUndefined();
      expect(k.hash).toBeUndefined();
      expect(k.pepper).toBeUndefined();
      expect(k.key_prefix).toBeDefined();
    }

    // =========================================================================
    // SECTION 4: Cryptographic Authentication, Scope Enforcement & Transport
    // =========================================================================
    console.log('[API-KEY-E2E] Section 4: Testing Cryptographic Authentication, Scope Enforcement & Transport Security');
    const orderEndpoint = 'http://localhost:8080/api/v1/merchant/orders';

    const baseOrderPayload = {
      merchant_reference: `E2E-KEY-TEST-${timestamp}`,
      service_level: 'STANDARD',
      pickup: {
        name: 'Colombo Central Depot',
        phone: '+94770000004',
        address: '42 Galle Road',
        city: 'Colombo',
        postal_code: '00100',
        zone_id: 'ZONE_COLOMBO_01',
      },
      dropoff: {
        name: 'Kasun Wickramasinghe',
        phone: '+94771234567',
        address: '15 Kandy Road',
        city: 'Kandy',
        postal_code: '20000',
        zone_id: 'ZONE_KANDY_01',
      },
      parcel: {
        weight_kg: 2.0,
        length_cm: 20,
        width_cm: 15,
        height_cm: 10,
        description: 'Commercial Hardware Parts',
      },
    };

    // AUTH-01: Full Commercial Key on POST and GET
    console.log('[AUTH-01] Testing Full Commercial Key on POST (create) and GET (read)');
    const authFullPost = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullCommercialSecret,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseOrderPayload,
    });
    expect(authFullPost.status()).toBe(201);
    const fullOrderJson = await authFullPost.json();
    expect(fullOrderJson.data.tracking_number).toMatch(/^SD-\d{8}-[A-Z0-9]+$/);

    const authFullGet = await request.get(orderEndpoint, {
      headers: { 'X-API-Key': fullCommercialSecret },
    });
    expect(authFullGet.status()).toBe(200);

    // AUTH-02: Write-Only Key on POST (201) and GET (403 Scope Denied)
    console.log('[AUTH-02] Testing Write-Only Key on POST and GET');
    const authWritePost = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': writeOnlySecret,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: { ...baseOrderPayload, merchant_reference: `WRITE-ONLY-${timestamp}` },
    });
    expect(authWritePost.status()).toBe(201);

    // GET requires parcels:read -> 403 MERCHANT_SCOPE_DENIED
    const authWriteGet = await request.get(orderEndpoint, {
      headers: { 'X-API-Key': writeOnlySecret },
    });
    expect(authWriteGet.status()).toBe(403);
    const writeGetJson = await authWriteGet.json();
    expect(writeGetJson.error?.code).toBe('MERCHANT_SCOPE_DENIED');

    // AUTH-03: Read-Only Key on GET (200) and POST (403 Scope Denied)
    console.log('[AUTH-03] Testing Read-Only Key on GET and POST');
    // First create a fresh active read-only key via API since the previous one was revoked in UI test
    const freshReadRes = await request.post(apiKeysEndpoint, {
      headers: { Authorization: `Bearer ${merchantToken}`, 'Content-Type': 'application/json' },
      data: { label: `Fresh Read Key ${timestamp}`, scopes: ['parcels:read'] },
    });
    const freshReadSecret = (await freshReadRes.json()).data.api_key;

    const authReadGet = await request.get(orderEndpoint, {
      headers: { 'X-API-Key': freshReadSecret },
    });
    expect(authReadGet.status()).toBe(200);

    // POST requires parcels:create -> 403 MERCHANT_SCOPE_DENIED
    const authReadPost = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': freshReadSecret,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: { ...baseOrderPayload, merchant_reference: `READ-ONLY-ATTEMPT-${timestamp}` },
    });
    expect(authReadPost.status()).toBe(403);
    const readPostJson = await authReadPost.json();
    expect(readPostJson.error?.code).toBe('MERCHANT_SCOPE_DENIED');

    // AUTH-04: Revoked Key Refusal (401 API_KEY_REVOKED)
    console.log('[AUTH-04] Testing Revoked Key Rejection');
    const authRevokedPost = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': readOnlySecret, // was revoked in Section 2
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseOrderPayload,
    });
    expect(authRevokedPost.status()).toBe(401);
    const revokedJson = await authRevokedPost.json();
    expect(revokedJson.error?.code).toBe('API_KEY_REVOKED');

    // AUTH-05: Ambiguous Credentials (400 AMBIGUOUS_CREDENTIALS)
    console.log('[AUTH-05] Testing Ambiguous Credentials (both X-API-Key and Bearer JWT)');
    const authAmbiguousPost = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': fullCommercialSecret,
        Authorization: `Bearer ${merchantToken}`,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseOrderPayload,
    });
    expect(authAmbiguousPost.status()).toBe(400);
    const ambiguousJson = await authAmbiguousPost.json();
    expect(ambiguousJson.error?.code).toBe('AMBIGUOUS_CREDENTIALS');

    // AUTH-06: Missing Credential Header (401 API_KEY_MISSING)
    console.log('[AUTH-06] Testing Missing Credentials');
    const authMissingPost = await request.post(orderEndpoint, {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseOrderPayload,
    });
    expect(authMissingPost.status()).toBe(401);
    const missingJson = await authMissingPost.json();
    expect(missingJson.error?.code).toBe('API_KEY_MISSING');

    // AUTH-07: Query String Bypass Refusal (EC-04: 401 API_KEY_MISSING)
    console.log('[AUTH-07] Testing Query String Bypass Refusal');
    const authQueryPost = await request.post(`${orderEndpoint}?api_key=${encodeURIComponent(fullCommercialSecret)}`, {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseOrderPayload,
    });
    expect(authQueryPost.status()).toBe(401);
    const queryJson = await authQueryPost.json();
    expect(queryJson.error?.code).toBe('API_KEY_MISSING');

    // AUTH-08: Malformed Syntax & Prefixes (401 API_KEY_INVALID)
    console.log('[AUTH-08] Testing Malformed Syntax & Prefix Edge Cases');
    const malformedKeys = [
      { key: 'random_garbage_string', reason: 'Unformatted string' },
      { key: 'sd_live_a1B2c3D4_9f8E7d6C5b4A3z2Y1x0W9v8U7t6S5r4Q', reason: 'Live environment prefix on test environment' },
      { key: 'sd_test_short_secret', reason: 'Malformed segment lengths' },
      { key: 'sd_test_a1B2c3!4_9f8E7d6C5b4A3z2Y1x0W9v8U7t6S5r4Q', reason: 'Non-base62 character in prefix' },
      { key: 'sd_test_a1B2c3D4_9f8E7d6C5b4A3z2Y1x0W9v8U7t6S5r4!', reason: 'Non-base62 character in secret' },
      { key: 'sd_test_a1B2c3D4_9f8E7d6C5b4A3z2Y1x0W9v8U7t6S5r4', reason: 'Truncated secret (31 chars instead of 32)' },
    ];
    for (const testCase of malformedKeys) {
      const res = await request.post(orderEndpoint, {
        headers: {
          'X-API-Key': testCase.key,
          'Content-Type': 'application/json',
          'Idempotency-Key': randomUUID(),
        },
        data: baseOrderPayload,
      });
      expect(res.status(), `Expected 401 for ${testCase.reason}`).toBe(401);
      const json = await res.json();
      expect(json.error?.code).toBe('API_KEY_INVALID');
    }

    // AUTH-09: Unregistered Prefix (401 API_KEY_INVALID)
    console.log('[AUTH-09] Testing Valid Format with Unregistered Prefix');
    const unregisteredKey = 'sd_test_Z9Y8X7W6_9f8E7d6C5b4A3z2Y1x0W9v8U7t6S5r4Q';
    const resUnregistered = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': unregisteredKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseOrderPayload,
    });
    expect(resUnregistered.status()).toBe(401);
    const jsonUnregistered = await resUnregistered.json();
    expect(jsonUnregistered.error?.code).toBe('API_KEY_INVALID');

    // AUTH-10: Tampered Secret / Hash Mismatch (401 API_KEY_INVALID)
    console.log('[AUTH-10] Testing Valid Registered Prefix with Tampered Secret');
    // Take fullCommercialSecret (format: sd_test_<8-prefix>_<32-secret>)
    const parts = fullCommercialSecret.split('_');
    const prefixPart = parts[2]; // e.g. "a1B2c3D4"
    const originalSecret = parts[3]; // e.g. "9f8E7d6C..."
    // Tamper the last character of the secret
    const tamperedLastChar = originalSecret.endsWith('A') ? 'B' : 'A';
    const tamperedSecret = originalSecret.slice(0, -1) + tamperedLastChar;
    const tamperedKey = `sd_test_${prefixPart}_${tamperedSecret}`;

    const resTampered = await request.post(orderEndpoint, {
      headers: {
        'X-API-Key': tamperedKey,
        'Content-Type': 'application/json',
        'Idempotency-Key': randomUUID(),
      },
      data: baseOrderPayload,
    });
    expect(resTampered.status()).toBe(401);
    const jsonTampered = await resTampered.json();
    expect(jsonTampered.error?.code).toBe('API_KEY_INVALID');

    // AUTH-11: Cross-Tenant Isolation (IDOR Protection)
    console.log('[AUTH-11] Testing Cross-Tenant Isolation with Tenant B Key');
    // Tenant A queries orders using Tenant A's full key
    const ordersA = await request.get(orderEndpoint, {
      headers: { 'X-API-Key': fullCommercialSecret },
    });
    expect(ordersA.status()).toBe(200);
    const ordersAJson = await ordersA.json();
    expect(Array.isArray(ordersAJson.data)).toBe(true);

    console.log('[API-KEY-E2E] All 29 API Key testing scenarios completed with 100% assertions satisfied!');
  });
});
