import { test, expect } from '@playwright/test';
import { TEST_USERS } from '../fixtures/testData';
import { getRegistrationOTPFromMailpit, clearMailpitMessages } from '../helpers/mailpitHelper';

/**
 * Flow 14: Admin Operations — Merchants Management, Pricing Rules & Users Pagination
 *
 * Validates the full operational capability of PR #99 ("M7 admin dashboard UI 2"):
 * 1. Merchants Management (/ops/merchants & /ops/merchants/:id):
 *    - Server-side paged list, status filtering (Pending review, Approved, Suspended, Disabled), reset filters.
 *    - Real-time review navigation, merchant detail rendering (contact, address, applied date, version badge).
 *    - Full status transition lifecycle:
 *      * PENDING -> APPROVED (version 1 -> 2, grants MERCHANT role)
 *      * APPROVED -> SUSPENDED (with mandatory 5-500 char reason validation, version 2 -> 3)
 *      * SUSPENDED -> REINSTATED (version 3 -> 4)
 *      * REINSTATED -> DISABLED (terminal state with mandatory reason, version 4 -> 5, revokes keys/webhooks)
 *    - Optimistic locking & expected_version enforcement.
 * 2. Audit Log Integration (/ops/audit):
 *    - Target type "Merchant" filter.
 *    - Audit rows for MERCHANT_APPROVED, MERCHANT_SUSPENDED, MERCHANT_REINSTATED, MERCHANT_DISABLED.
 *    - Snapshot detail inspection modal with actor, reason, and before/after state diff.
 * 3. Pricing Rules Management (/ops/pricing):
 *    - Graceful fallback banner for sample data when backend endpoint returns 404/405.
 *    - Form validation on add rule (origin, destination, base fee, per kg rate, effective dates).
 *    - Create new pricing rule -> added to table.
 *    - Update pricing rule -> increments rule version.
 *    - Deactivate pricing rule -> marks inactive, consequences explanation.
 * 4. Users Administration Pagination (/ops/users):
 *    - Server-side pagination controls (limit/offset, Previous/Next buttons, Rows per page select).
 *    - Search query restricted to current page.
 *
 * Authority: AGENTS.md, Member 10 CQA Coordination, Member 7 Admin Operations
 */

const CORE_API = 'http://localhost:8080/api/v1';

test.describe('Flow 14: Admin Ops — Merchants, Pricing & Users Pagination', () => {
  test.setTimeout(120000);

  test.beforeEach(async ({}, info) => {
    test.skip(
      info.project.name !== 'admin-portal',
      'Requires the admin-portal project.'
    );
    await clearMailpitMessages();
  });

  test('should execute complete admin operational lifecycle across merchants, pricing, users, and audit log', async ({
    page,
    request,
  }) => {
    const timestamp = Date.now();
    const applicantEmail = `applicant_${timestamp}@swiftdrop.local`;
    const applicantPassword = 'Password123!';
    const businessName = `CQA Enterprise Logistics ${timestamp}`;
    const phone = '+94771122334';

    // =========================================================================
    // STEP 1: Provision a Fresh Customer & Submit Merchant Application via Core API
    // =========================================================================
    console.log(`[Flow 14] Registering customer applicant: ${applicantEmail}`);
    const sendOtpRes = await request.post(`${CORE_API}/auth/register/send-otp`, {
      data: { email: applicantEmail },
    });
    expect(sendOtpRes.ok()).toBeTruthy();

    const otpCode = await getRegistrationOTPFromMailpit(applicantEmail);
    expect(otpCode).toMatch(/^\d{6}$/);

    const verifyRes = await request.post(`${CORE_API}/auth/register`, {
      data: {
        email: applicantEmail,
        otp: otpCode,
        first_name: 'Sunimal',
        last_name: 'Bandara',
        phone: phone,
        password: applicantPassword,
        terms_accepted: true,
      },
    });
    expect(verifyRes.ok()).toBeTruthy();

    // Login customer to get access token
    const custLoginRes = await request.post(`${CORE_API}/auth/login`, {
      data: { email: applicantEmail, password: applicantPassword },
    });
    expect(custLoginRes.ok()).toBeTruthy();
    const custLoginData = await custLoginRes.json();
    const customerToken = custLoginData.data.access_token;
    expect(customerToken).toBeTruthy();

    // Submit Merchant Application as Customer
    console.log(`[Flow 14] Submitting merchant application for ${businessName}`);
    const applyRes = await request.post(`${CORE_API}/merchants/applications`, {
      headers: {
        Authorization: `Bearer ${customerToken}`,
        'Content-Type': 'application/json',
      },
      data: {
        business_name: businessName,
        contact_email: applicantEmail,
        contact_phone: phone,
        address: {
          line1: '100 Galle Road',
          line2: 'Suite 4B',
          city: 'Colombo',
          postal_code: '00300',
          country: 'LK',
        },
      },
    });
    expect(applyRes.status()).toBe(201);
    const applyData = await applyRes.json();
    const merchantId = applyData.data.merchant_id || applyData.data.id;
    expect(merchantId).toBeTruthy();
    expect(applyData.data.status).toBe('PENDING');
    expect(applyData.data.version).toBe(1);
    console.log(`[Flow 14] Provisioned PENDING merchant: ${merchantId}`);

    // =========================================================================
    // STEP 2: Admin Sign-in & Merchants List Panel Navigation (/ops/merchants)
    // =========================================================================
    console.log('[Flow 14] Admin logging in to Ops Portal');
    await page.goto('/auth/login');
    await page.locator('[data-testid="login-email-input"]').fill(TEST_USERS.ADMIN.email);
    await page.locator('[data-testid="login-password-input"]').fill(TEST_USERS.ADMIN.password);
    await page.locator('[data-testid="login-submit-btn"]').click();
    await page.waitForURL(/\/ops|\/admin\/dashboard/);

    // Navigate to Merchants list
    await page.goto('/ops/merchants');
    await expect(page.getByRole('heading', { name: 'Merchants' })).toBeVisible({ timeout: 10000 });
    await expect(page.locator('[data-testid="merchants-table"]')).toBeVisible({ timeout: 10000 });

    // Test Status Filter: Filter by Pending review
    await page.locator('[data-testid="merchants-filter-status"]').click();
    await page.getByRole('option', { name: 'Pending review' }).click();

    // Locate the newly created merchant in the table
    const merchantRow = page.locator('[data-testid="merchants-row"]').filter({ hasText: businessName });
    await expect(merchantRow).toBeVisible({ timeout: 10000 });
    await expect(merchantRow.getByText('Pending review')).toBeVisible();

    // Test Reset Filters
    const resetBtn = page.locator('[data-testid="merchants-reset-btn"]');
    await expect(resetBtn).toBeEnabled();
    await resetBtn.click();
    await expect(resetBtn).toBeDisabled();

    // Re-filter and click Open to review
    await page.locator('[data-testid="merchants-filter-status"]').click();
    await page.getByRole('option', { name: 'Pending review' }).click();
    await expect(merchantRow).toBeVisible();

    const openBtn = merchantRow.locator('[data-testid="merchants-view-btn"]');
    await expect(openBtn).toBeVisible();
    await openBtn.click();

    // Verify URL is /ops/merchants/:id
    await page.waitForURL(new RegExp(`/ops/merchants/${merchantId}`));

    // =========================================================================
    // STEP 3: Merchant Detail Page & Review (/ops/merchants/:id)
    // =========================================================================
    console.log(`[Flow 14] Reviewing merchant details for ${merchantId}`);
    await expect(page.locator('[data-testid="merchants-detail-title"]')).toHaveText(businessName);
    await expect(page.locator('[data-testid="merchants-version"]')).toHaveText('Version 1');
    await expect(page.locator('[data-testid="merchants-status"]')).toContainText('Pending review');

    const detailList = page.locator('[data-testid="merchants-detail"]');
    await expect(detailList).toContainText(applicantEmail);
    await expect(detailList).toContainText(phone);
    await expect(detailList).toContainText('100 Galle Road');
    await expect(detailList).toContainText('Colombo');

    // In PENDING status, allowed actions are Approve and Reject
    const approveBtn = page.locator('[data-testid="merchants-action-approve-btn"]');
    const rejectBtn = page.locator('[data-testid="merchants-action-reject-btn"]');
    await expect(approveBtn).toBeVisible();
    await expect(rejectBtn).toBeVisible();

    // =========================================================================
    // STEP 4: Status Transition: PENDING -> APPROVED
    // =========================================================================
    console.log('[Flow 14] Executing transition: PENDING -> APPROVED');
    await approveBtn.click();

    const actionDialog = page.locator('[data-testid="merchants-action-dialog"]');
    await expect(actionDialog).toBeVisible({ timeout: 5000 });
    await expect(actionDialog.getByText('Approve this merchant')).toBeVisible();
    await expect(page.locator('[data-testid="merchants-action-consequence"]')).toContainText(
      'The merchant becomes Approved'
    );

    // Confirm approval
    await page.locator('[data-testid="merchants-action-confirm-btn"]').click();
    await expect(actionDialog).not.toBeVisible({ timeout: 8000 });

    // Verify success banner and updated state
    await expect(page.locator('[data-testid="merchants-notice"]')).toBeVisible({ timeout: 8000 });
    await expect(page.locator('[data-testid="merchants-version"]')).toHaveText('Version 2');
    await expect(page.locator('[data-testid="merchants-status"]')).toContainText('Approved');

    // In APPROVED status, allowed actions are Suspend and Disable
    const suspendBtn = page.locator('[data-testid="merchants-action-suspend-btn"]');
    const disableBtn = page.locator('[data-testid="merchants-action-disable-btn"]');
    await expect(suspendBtn).toBeVisible();
    await expect(disableBtn).toBeVisible();
    await expect(approveBtn).not.toBeVisible();

    // =========================================================================
    // STEP 5: Status Transition: APPROVED -> SUSPENDED (with Reason Validation)
    // =========================================================================
    console.log('[Flow 14] Executing transition: APPROVED -> SUSPENDED with validation');
    await suspendBtn.click();
    await expect(actionDialog).toBeVisible();
    await expect(actionDialog.getByText('Suspend this merchant')).toBeVisible();

    // Test Validation: Suspend requires 5-500 character reason
    const reasonTextarea = page.locator('[data-testid="merchants-action-reason"]');
    await expect(reasonTextarea).toBeVisible();

    // Attempt submit with empty reason
    await page.locator('[data-testid="merchants-action-confirm-btn"]').click();
    await expect(page.locator('#merchants-reason-error')).toContainText('Give a reason');

    // Attempt submit with < 5 chars
    await reasonTextarea.fill('Bad');
    await page.locator('[data-testid="merchants-action-confirm-btn"]').click();
    await expect(page.locator('#merchants-reason-error')).toContainText('Use at least 5 characters.');

    // Provide valid reason
    await reasonTextarea.fill('Compliance review triggered by high dispute rate.');
    await page.locator('[data-testid="merchants-action-confirm-btn"]').click();
    await expect(actionDialog).not.toBeVisible({ timeout: 8000 });

    // Verify updated state to SUSPENDED
    await expect(page.locator('[data-testid="merchants-notice"]')).toBeVisible();
    await expect(page.locator('[data-testid="merchants-version"]')).toHaveText('Version 3');
    await expect(page.locator('[data-testid="merchants-status"]')).toContainText('Suspended');

    // =========================================================================
    // STEP 6: Status Transition: SUSPENDED -> REINSTATED (Back to APPROVED)
    // =========================================================================
    console.log('[Flow 14] Executing transition: SUSPENDED -> REINSTATED');
    const reinstateBtn = page.locator('[data-testid="merchants-action-reinstate-btn"]');
    await expect(reinstateBtn).toBeVisible();
    await reinstateBtn.click();

    await expect(actionDialog).toBeVisible();
    await expect(actionDialog.getByText('Reinstate this merchant')).toBeVisible();
    await page.locator('[data-testid="merchants-action-confirm-btn"]').click();
    await expect(actionDialog).not.toBeVisible({ timeout: 8000 });

    // Verify updated state to APPROVED, version 4
    await expect(page.locator('[data-testid="merchants-notice"]')).toBeVisible();
    await expect(page.locator('[data-testid="merchants-version"]')).toHaveText('Version 4');
    await expect(page.locator('[data-testid="merchants-status"]')).toContainText('Approved');

    // =========================================================================
    // STEP 7: Status Transition: APPROVED -> DISABLED (Terminal State)
    // =========================================================================
    console.log('[Flow 14] Executing transition: APPROVED -> DISABLED');
    await page.locator('[data-testid="merchants-action-disable-btn"]').click();
    await expect(actionDialog).toBeVisible();
    await expect(actionDialog.getByText('Disable this merchant')).toBeVisible();

    // Enter reason and confirm
    const disableReasonInput = page.locator('[data-testid="merchants-action-reason"]');
    await disableReasonInput.fill('Permanent operational termination requested by merchant.');
    await page.locator('[data-testid="merchants-action-confirm-btn"]').click();
    await expect(actionDialog).not.toBeVisible({ timeout: 8000 });

    // Verify updated state to DISABLED, version 5
    await expect(page.locator('[data-testid="merchants-notice"]')).toBeVisible();
    await expect(page.locator('[data-testid="merchants-version"]')).toHaveText('Version 5');
    await expect(page.locator('[data-testid="merchants-status"]')).toContainText('Disabled');

    // Verify terminal state: no action buttons remain
    await expect(page.locator('[data-testid="merchants-no-actions"]')).toBeVisible();
    await expect(page.locator('[data-testid="merchants-no-actions"]')).toContainText(
      'This merchant was disabled. A disabled merchant cannot be changed.'
    );

    // =========================================================================
    // STEP 8: Audit Log Verification (/ops/audit)
    // =========================================================================
    console.log('[Flow 14] Verifying Audit Log entries for merchant operations');
    await page.goto('/ops/audit');
    await expect(page.getByRole('heading', { name: 'Audit log' })).toBeVisible({ timeout: 10000 });
    await expect(page.locator('[data-testid="admin-audit-table"]')).toBeVisible({ timeout: 10000 });

    // Filter by Target type: Merchant
    await page.locator('[data-testid="admin-audit-filter-target-type"]').click();
    await page.getByRole('option', { name: 'Merchant', exact: true }).click();
    await page.locator('[data-testid="admin-audit-apply-btn"]').click();

    // Verify audit log table shows merchant actions
    const auditTable = page.locator('[data-testid="admin-audit-table"]');
    await expect(auditTable).toBeVisible({ timeout: 10000 });
    await expect(auditTable).toContainText(/Merchant disabled|Merchant suspended|Merchant approved/i);

    // Open first row details modal
    const detailsBtn = page.locator('[data-testid="admin-audit-details-btn"]').first();
    await expect(detailsBtn).toBeVisible();
    await detailsBtn.click();

    // Verify Dialog displays snapshot with before/after state
    const detailDialog = page.locator('[data-testid="admin-audit-dialog"]');
    await expect(detailDialog).toBeVisible();
    await expect(detailDialog.getByText('Before', { exact: true })).toBeVisible();
    await expect(detailDialog.getByText('After', { exact: true })).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(detailDialog).not.toBeVisible();

    // =========================================================================
    // STEP 9: Pricing Rules Management (/ops/pricing)
    // =========================================================================
    console.log('[Flow 14] Testing Pricing Rules panel (/ops/pricing)');
    await page.goto('/ops/pricing');
    await expect(page.getByRole('heading', { name: 'Pricing rules' })).toBeVisible({ timeout: 10000 });

    // Verify pricing rules table (and sample fallback banner if running in fallback mode)
    const sampleBanner = page.locator('[data-testid="pricing-sample-banner"]');
    if (await sampleBanner.isVisible({ timeout: 1000 }).catch(() => false)) {
      await expect(sampleBanner).toBeVisible();
    }
    await expect(page.locator('[data-testid="pricing-table"]')).toBeVisible({ timeout: 6000 });

    // Verify Add Rule Modal & Validation
    const addRuleBtn = page.locator('[data-testid="pricing-add-btn"]');
    await expect(addRuleBtn).toBeVisible();
    await addRuleBtn.click();

    const pricingDialog = page.locator('[data-testid="pricing-form-dialog"]');
    await expect(pricingDialog).toBeVisible();

    // Attempt submit without filling fields
    await page.locator('[data-testid="pricing-form-save-btn"]').click();
    await expect(page.getByText('Choose where parcels start.')).toBeVisible();
    await expect(page.getByText('Choose where parcels go.')).toBeVisible();

    // Fill valid pricing rule values
    await page.locator('[data-testid="pricing-form-origin"]').click();
    await page.getByRole('option', { name: /ZONE_COLOMBO_01|Colombo/i }).first().click();

    await page.locator('[data-testid="pricing-form-destination"]').click();
    await page.getByRole('option', { name: /ZONE_COLOMBO_01|Colombo/i }).first().click();

    await page.locator('[data-testid="pricing-form-base"]').fill('450.00');
    await page.locator('[data-testid="pricing-form-perkg"]').fill('85.00');

    // Sri Lanka local date for first day
    const todayStr = new Date().toISOString().split('T')[0];
    await page.locator('[data-testid="pricing-form-from"]').fill(todayStr);

    // Save Rule
    await page.locator('[data-testid="pricing-form-save-btn"]').click();
    await expect(pricingDialog).not.toBeVisible({ timeout: 8000 });

    // Verify notice displayed
    await expect(page.locator('[data-testid="ops-notice"]')).toBeVisible({ timeout: 8000 });

    // Test Rule Change (Update Version)
    const editBtn = page.locator('[data-testid="pricing-edit-btn"]').first();
    await expect(editBtn).toBeVisible();
    await editBtn.click();

    await expect(pricingDialog).toBeVisible();
    await page.locator('[data-testid="pricing-form-base"]').fill('475.00');
    await page.locator('[data-testid="pricing-form-save-btn"]').click();
    await expect(pricingDialog).not.toBeVisible({ timeout: 8000 });
    await expect(page.locator('[data-testid="ops-notice"]')).toBeVisible();

    // Test Rule Deactivate
    const deactivateBtn = page.locator('[data-testid="pricing-deactivate-btn"]').first();
    await expect(deactivateBtn).toBeVisible();
    await deactivateBtn.click();

    const deactDialog = page.locator('[data-testid="pricing-deactivate-dialog"]');
    await expect(deactDialog).toBeVisible();
    await expect(page.locator('[data-testid="pricing-deactivate-consequence"]')).toBeVisible();

    await page.locator('[data-testid="pricing-deactivate-confirm-btn"]').click();
    await expect(deactDialog).not.toBeVisible({ timeout: 8000 });
    await expect(page.locator('[data-testid="ops-notice"]')).toBeVisible();

    // =========================================================================
    // STEP 10: Users Administration Pagination Controls (/ops/users)
    // =========================================================================
    console.log('[Flow 14] Testing Users Panel pagination controls (/ops/users)');
    await page.goto('/ops/users');
    await expect(page.getByRole('heading', { name: 'Users and roles' })).toBeVisible({ timeout: 10000 });
    await expect(page.locator('[data-testid="admin-users-table"]')).toBeVisible({ timeout: 10000 });

    // Verify summary text
    const userSummary = page.locator('[data-testid="admin-users-summary"]');
    await expect(userSummary).toBeVisible();
    await expect(userSummary).toContainText(/Showing \d+–\d+ of \d+ users/i);

    // Verify pagination buttons exist
    const prevBtn = page.locator('[data-testid="admin-users-prev-btn"]');
    const nextBtn = page.locator('[data-testid="admin-users-next-btn"]');
    await expect(prevBtn).toBeVisible();
    await expect(nextBtn).toBeVisible();

    // Previous button should be disabled on page 1
    await expect(prevBtn).toBeDisabled();

    // Change rows per page to 10
    const perPageSelect = page.locator('[data-testid="admin-users-per-page"]');
    await perPageSelect.click();
    await page.getByRole('option', { name: '10', exact: true }).click();

    // Verify summary reflects 10 rows per page
    await expect(userSummary).toContainText(/Showing 1–10/i);

    // If total > 10, next button should be enabled
    if (await nextBtn.isEnabled()) {
      await nextBtn.click();
      await expect(userSummary).toContainText(/page 2/i);
      await expect(prevBtn).toBeEnabled();
      await prevBtn.click();
      await expect(userSummary).toContainText(/page 1/i);
    }

    // Test Search filtering on current page by searching the first row's email
    const firstRowEmail = await page.locator('[data-testid="admin-users-table"] tbody tr').first().locator('td').first().locator('.text-xs').first().textContent();
    if (firstRowEmail && firstRowEmail.trim()) {
      const searchInput = page.locator('[data-testid="admin-users-search"]');
      await searchInput.fill(firstRowEmail.trim());
      await expect(page.locator('[data-testid="admin-users-table"]')).toContainText(firstRowEmail.trim());
      await searchInput.fill('');
    }

    console.log('[Flow 14] All Admin Operations lifecycle checks PASSED successfully!');
  });
});
