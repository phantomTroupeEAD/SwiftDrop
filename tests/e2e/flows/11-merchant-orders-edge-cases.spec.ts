import { test, expect } from '@playwright/test';
import { randomUUID } from 'crypto';
import { getRegistrationOTPFromMailpit } from '../helpers/mailpitHelper';

/**
 * Flow 11: Exhaustive Merchant Portal Orders Page Edge Cases Suite
 *
 * Covers every critical Senior QA surface across the Merchant Orders page (/portal/merchants/orders):
 * 1. Bulk CSV Ingestion & Parsing Engine Edge Cases (Excel BOM, Quoted Commas, File Boundaries, Limits)
 * 2. Partial Failure Handling, Selective Retry Idempotency & Batch State Clearing
 * 3. Orders Table Multi-Criteria Filtering, Search Variations, Empty States & Pagination Boundaries
 * 4. Tracking Detail Drilldown & Public PII Masking Parity
 * 5. Cross-Tenant IDOR Isolation across multiple merchant tenants
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 4 (Merchants), Member 6 (Customer & Merchant Portal), Member 2 (Parcels)
 */

const SELECTORS = {
  AUTH: {
    INPUT_EMAIL: 'login-email-input',
    INPUT_PASSWORD: 'login-password-input',
    BTN_LOGIN: 'login-submit-btn',
    REGISTER_EMAIL: 'register-email-input',
    REGISTER_SEND_OTP_BTN: 'register-send-otp-btn',
    REGISTER_OTP: 'register-otp-input',
    REGISTER_FIRST_NAME: 'register-first-name-input',
    REGISTER_LAST_NAME: 'register-last-name-input',
    REGISTER_PHONE: 'register-phone-input',
    REGISTER_PASSWORD: 'register-password-input',
    REGISTER_TERMS_CHECKBOX: 'register-terms-checkbox',
    REGISTER_SUBMIT_BTN: 'register-submit-btn',
  },
};
const testId = (id: string) => `[data-testid="${id}"]`;

const CSV_HEADER =
  'merchant_reference,pickup_name,pickup_phone,pickup_address,pickup_city,pickup_zone_id,dropoff_name,dropoff_phone,dropoff_address,dropoff_city,dropoff_zone_id,weight_kg,length_cm,width_cm,height_cm,description';

test.describe('Flow 11: Merchant Orders Page & Bulk CSV Ingestion Edge Cases Suite', () => {
  test('should verify exhaustive CSV parsing, partial failure retries, table filtering, and multi-tenant isolation', async ({
    page,
    request,
    browser,
  }) => {
    test.setTimeout(120000);
    const timestamp = Date.now();
    const merchantAEmail = `e2e_orders_qa_a_${timestamp}@swiftdrop.local`;
    const merchantBEmail = `e2e_orders_qa_b_${timestamp}@swiftdrop.local`;
    const merchantPassword = 'Password123!';
    const phone = '+94770000004';
    const businessNameA = `Apex Commercial Logistics ${timestamp}`;
    const businessNameB = `Ceylon Retail Traders ${timestamp}`;

    // =========================================================================
    // SECTION 1: Provisioning Merchant Tenant A & Merchant Tenant B
    // =========================================================================
    console.log(`[QA-ORDERS] Provisioning Merchant Tenant A (${merchantAEmail})`);

    // Register Merchant A
    await page.goto('/auth/register');
    await expect(page.getByText('Create a customer account')).toBeVisible({ timeout: 10000 });
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(merchantAEmail);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    const otpCodeA = await getRegistrationOTPFromMailpit(merchantAEmail);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(otpCodeA);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Sunil');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill(phone);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();
    await expect(page.getByText('Account created')).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /Continue to Sign in/i }).click();

    // Login and apply as Merchant A
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(merchantAEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10000 });

    await page.goto('/portal/merchants/apply');
    await page.getByTestId('merchant-application-business_name-input').fill(businessNameA);
    await page.getByTestId('merchant-application-contact_email-input').fill(merchantAEmail);
    await page.getByTestId('merchant-application-contact_phone-input').fill(phone);
    await page.getByTestId('merchant-application-line1-input').fill('100 Warehouse Way');
    await page.getByTestId('merchant-application-city-input').fill('Colombo');

    const appPromiseA = page.waitForResponse(
      res => new URL(res.url()).pathname === '/api/v1/merchants/applications' && res.request().method() === 'POST'
    );
    await page.getByTestId('merchant-application-submit-btn').click();
    const appResA = await appPromiseA;
    const merchantAId = (await appResA.json()).data?.merchant_id;

    // Admin approves Merchant A
    const adminLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: 'admin@swiftdrop.local', password: 'AdminPassword123!' },
    });
    const adminToken = (await adminLoginRes.json()).data.access_token;
    await request.post(`http://localhost:8080/api/v1/admin/merchants/${merchantAId}/approve`, {
      headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
      data: { expected_version: 1, reason: 'Approved for Orders QA Edge Cases Suite' },
    });

    // Re-login as approved Merchant A
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(merchantAEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(merchantPassword);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/portal\/merchants\/orders|\/dashboard/, { timeout: 10000 });
    if (!page.url().includes('/portal/merchants/orders')) {
      await page.goto('/portal/merchants/orders');
    }
    await expect(page.getByRole('heading', { name: 'Orders', level: 2 })).toBeVisible({ timeout: 10000 });

    // =========================================================================
    // SECTION 2: CSV Parsing & Client Validation Boundaries
    // =========================================================================
    console.log('[QA-ORDERS] Section 2: Testing CSV Parsing & Client-Side Validation Boundaries');
    const fileInput = page.getByTestId('batch-csv-file-input');

    // CSV-01: File Size Boundary (> 256 KB)
    console.log('[CSV-01] Testing file size > 256 KB limit');
    const largeContent = CSV_HEADER + '\n' + 'A'.repeat(260_000);
    await fileInput.setInputFiles({
      name: 'too-large.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(largeContent, 'utf-8'),
    });
    await expect(page.getByText('Choose a CSV file smaller than 256 KB.')).toBeVisible({ timeout: 5000 });

    // CSV-02: Header Validation & Missing Column Names
    console.log('[CSV-02] Testing header mismatch rejection');
    const badHeaderCsv = 'merchant_ref,pickup,dropoff\nREF1,Colombo,Kandy';
    await fileInput.setInputFiles({
      name: 'bad-header.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(badHeaderCsv, 'utf-8'),
    });
    await expect(page.getByText('Use the column names from the CSV template.')).toBeVisible({ timeout: 5000 });

    // CSV-03: Row Volume Limits (0 data rows and > 100 data rows)
    console.log('[CSV-03] Testing empty rows limit');
    const emptyRowsCsv = CSV_HEADER + '\r\n';
    await fileInput.setInputFiles({
      name: 'empty.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(emptyRowsCsv, 'utf-8'),
    });
    await expect(page.getByText('Upload between 1 and 100 orders at a time.')).toBeVisible({ timeout: 5000 });

    console.log('[CSV-03] Testing > 100 rows limit');
    const validRowTemplate = 'REF-{i},Colombo Depot,+94770000004,42 Galle Rd,Colombo,ZONE_COLOMBO_01,Kasun,+94771234567,15 Kandy Rd,Kandy,ZONE_COLOMBO_01,2.0,10,10,10,Sample Item';
    const overflowRows = [CSV_HEADER];
    for (let i = 1; i <= 101; i++) {
      overflowRows.push(validRowTemplate.replace('{i}', String(i)));
    }
    await fileInput.setInputFiles({
      name: 'overflow.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(overflowRows.join('\r\n'), 'utf-8'),
    });
    await expect(page.getByText('Upload between 1 and 100 orders at a time.')).toBeVisible({ timeout: 5000 });

    // CSV-04: Unclosed Quotes Rejection
    console.log('[CSV-04] Testing unclosed quotes error handling');
    const unclosedQuoteCsv = CSV_HEADER + '\r\n' +
      'REF-UNCLOSED,Colombo Depot,+94770000004,"42 Galle Rd, Suite 2B,Colombo,ZONE_COLOMBO_01,Kasun,+94771234567,15 Kandy Rd,Kandy,ZONE_COLOMBO_01,2.0,10,10,10,Item';
    await fileInput.setInputFiles({
      name: 'unclosed.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(unclosedQuoteCsv, 'utf-8'),
    });
    await expect(page.getByText('A quoted CSV field is missing its closing quote.')).toBeVisible({ timeout: 5000 });

    // CSV-05: Cell-Level Validation Matrix (Phone, Weight, Dimension, Missing Fields)
    console.log('[CSV-05] Testing client-side row-level validation errors');
    const invalidRowsCsv = [
      CSV_HEADER,
      // Bad phone (non-E.164: local 0771234567)
      'REF-BAD-PHONE,Colombo Depot,0771234567,42 Galle Rd,Colombo,ZONE_COLOMBO_01,Kasun,+94771234567,15 Kandy Rd,Kandy,ZONE_COLOMBO_01,2.0,10,10,10,Item 1',
      // Excessive weight (> 30 kg)
      'REF-BAD-WEIGHT,Colombo Depot,+94770000004,42 Galle Rd,Colombo,ZONE_COLOMBO_01,Kasun,+94771234567,15 Kandy Rd,Kandy,ZONE_COLOMBO_01,35.0,10,10,10,Item 2',
      // Excessive dimension (> 150 cm)
      'REF-BAD-DIM,Colombo Depot,+94770000004,42 Galle Rd,Colombo,ZONE_COLOMBO_01,Kasun,+94771234567,15 Kandy Rd,Kandy,ZONE_COLOMBO_01,2.0,180,10,10,Item 3',
      // Missing required dropoff_city
      'REF-MISSING-FIELD,Colombo Depot,+94770000004,42 Galle Rd,Colombo,ZONE_COLOMBO_01,Kasun,+94771234567,15 Kandy Rd,,ZONE_COLOMBO_01,2.0,10,10,10,Item 4',
    ].join('\r\n');

    await fileInput.setInputFiles({
      name: 'invalid-matrix.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(invalidRowsCsv, 'utf-8'),
    });

    // Invariant: Preview table flags each row as Invalid with precise validation reasons
    await expect(page.getByRole('table', { name: 'CSV order preview' })).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('Use international phone numbers such as +94771234567.')).toBeVisible();
    await expect(page.getByText('Weight must be 0.01 to 30 kg.')).toBeVisible();
    await expect(page.getByText('Each dimension must be 0.01 to 150 cm.')).toBeVisible();
    await expect(page.getByText('Fill in every required field.')).toBeVisible();

    // Invariant: When 0 rows are valid, the Submit button is strictly disabled
    const submitBtn = page.getByTestId('batch-csv-submit-btn');
    await expect(submitBtn).toBeDisabled();

    // CSV-06: Excel BOM & Quoted Address with Commas & Escaped Quotes
    console.log('[CSV-06] Testing Excel UTF-8 BOM, embedded commas and escaped quotes parsing');
    const bomCsv = '\uFEFF' + [
      CSV_HEADER,
      'REF-BOM-01,Colombo Depot,+94770000004,"No. 42, Galle Road, Suite 3B",Colombo,ZONE_COLOMBO_01,Nimal Perera,+94771234567,"Apt 4, Temple Road",Kandy,ZONE_COLOMBO_01,2.5,10,10,10,"Box with ""Fragile"" Stamp"',
      'REF-BOM-02,Colombo Depot,+94770000004,100 Sea Street,Colombo,ZONE_COLOMBO_01,Sunil Jayasuriya,+94779876543,88 Main St,Galle,ZONE_COLOMBO_01,1.8,12,12,12,Standard Samples',
    ].join('\r\n');

    await fileInput.setInputFiles({
      name: 'excel-bom.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(bomCsv, 'utf-8'),
    });

    await expect(page.getByText('0 of 2 valid orders submitted.')).toBeVisible({ timeout: 5000 });
    await expect(submitBtn).toBeEnabled();

    // Submit the BOM batch
    await submitBtn.click();
    await expect(page.getByText('2 of 2 valid orders submitted.')).toBeVisible({ timeout: 10000 });

    // =========================================================================
    // SECTION 3: Partial Failure Handling, Retries & Batch Clear
    // =========================================================================
    console.log('[QA-ORDERS] Section 3: Testing Partial Failure, Selective Retry & Idempotency');

    // Create mixed batch: Row 1 valid, Row 2 will be intercepted to fail, Row 3 valid
    const mixedBatchCsv = [
      CSV_HEADER,
      `REF-MIXED-01-${timestamp},Colombo Depot,+94770000004,Galle Rd,Colombo,ZONE_COLOMBO_01,Customer One,+94771234567,Main St,Colombo,ZONE_COLOMBO_01,1.0,10,10,10,Item 1`,
      `REF-MIXED-FAIL-${timestamp},Colombo Depot,+94770000004,Galle Rd,Colombo,ZONE_COLOMBO_01,Customer Two,+94771234567,Main St,Colombo,ZONE_COLOMBO_01,1.0,10,10,10,Item 2`,
      `REF-MIXED-03-${timestamp},Colombo Depot,+94770000004,Galle Rd,Colombo,ZONE_COLOMBO_01,Customer Three,+94771234567,Main St,Colombo,ZONE_COLOMBO_01,1.0,10,10,10,Item 3`,
    ].join('\r\n');

    await fileInput.setInputFiles({
      name: 'mixed-batch.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(mixedBatchCsv, 'utf-8'),
    });
    await expect(page.getByText('0 of 3 valid orders submitted.')).toBeVisible({ timeout: 5000 });

    // Set up intercept to fail the request matching REF-MIXED-FAIL on first attempt
    let failAttempted = false;
    await page.route('**/api/v1/merchant/orders', async route => {
      const body = route.request().postData();
      if (body && body.includes(`REF-MIXED-FAIL-${timestamp}`) && !failAttempted) {
        failAttempted = true;
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: { code: 'INTERNAL_ERROR', message: 'Simulated network drop' } }),
        });
      } else {
        await route.continue();
      }
    });

    // Click submit
    await page.getByTestId('batch-csv-submit-btn').click();

    // Verify Row 1 and Row 3 are Submitted, while Row 2 shows Failed
    await expect(page.getByText('2 of 3 valid orders submitted.')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Failed')).toBeVisible();

    // Invariant: Button text shifts to "Retry unsuccessful orders"
    const retryBtn = page.getByRole('button', { name: 'Retry unsuccessful orders' });
    await expect(retryBtn).toBeVisible();

    // Now click Retry: Row 1 & 3 MUST NOT be re-sent; Row 2 should now succeed
    console.log('[QA-ORDERS] Retrying unsuccessful orders selectively');
    await retryBtn.click();
    await expect(page.getByText('3 of 3 valid orders submitted.')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Failed')).toBeHidden();

    // Test Clear Batch
    console.log('[QA-ORDERS] Testing Clear Batch');
    await page.getByRole('button', { name: 'Clear batch' }).click();
    await expect(page.getByRole('table', { name: 'CSV order preview' })).toBeHidden();

    // =========================================================================
    // SECTION 4: Orders Table Search, Filtering, Pagination & Drilldown
    // =========================================================================
    console.log('[QA-ORDERS] Section 4: Testing Table Search, Filter, Pagination and Tracking Link');
    const orderTable = page.getByTestId('merchant-order-table');
    await expect(orderTable).toBeVisible({ timeout: 10000 });

    const searchInput = page.getByTestId('merchant-order-search');
    const filterSelect = page.getByTestId('merchant-order-filter');

    // 1. Search by Reference
    console.log('[QA-ORDERS] Search by merchant reference');
    await searchInput.fill(`REF-MIXED-01-${timestamp}`);
    await expect(orderTable.getByText(`REF-MIXED-01-${timestamp}`)).toBeVisible();
    await expect(orderTable.getByText(`REF-MIXED-03-${timestamp}`)).toBeHidden();

    // 2. Search Negative Case & Reset
    console.log('[QA-ORDERS] Search negative query & reset filters');
    await searchInput.fill('NON_EXISTENT_REF_99999');
    await expect(page.getByTestId('merchant-order-empty')).toHaveText('No orders match your filters.');
    await page.getByTestId('merchant-order-reset-btn').click();
    await expect(searchInput).toHaveValue('');
    await expect(orderTable.getByText(`REF-MIXED-01-${timestamp}`)).toBeVisible();

    // 3. Status Filter Dropdown
    console.log('[QA-ORDERS] Testing status filter dropdown');
    await filterSelect.selectOption('CREATED');
    await expect(orderTable.getByText(`REF-MIXED-01-${timestamp}`)).toBeVisible();
    await filterSelect.selectOption(''); // All statuses

    // 4. Ingest orders until we exceed 20 to verify pagination boundaries (ordersPerPage = 20)
    console.log('[QA-ORDERS] Generating orders to test pagination boundary (> 20 orders)');
    const paginationRows = [CSV_HEADER];
    for (let i = 1; i <= 16; i++) {
      paginationRows.push(
        `REF-PAGE-${i}-${timestamp},Colombo Depot,+94770000004,Galle Rd,Colombo,ZONE_COLOMBO_01,Customer ${i},+94771234567,Main St,Colombo,ZONE_COLOMBO_01,1.0,10,10,10,Page Item ${i}`
      );
    }
    await fileInput.setInputFiles({
      name: 'pagination-batch.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(paginationRows.join('\r\n'), 'utf-8'),
    });
    await page.getByTestId('batch-csv-submit-btn').click();
    await expect(page.getByText('16 of 16 valid orders submitted.')).toBeVisible({ timeout: 15000 });

    // Verify pagination controls appear
    const navPagination = page.getByRole('navigation', { name: 'Order pages' });
    await expect(navPagination).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(/Page 1 of 2/i)).toBeVisible();

    const nextBtn = navPagination.getByRole('button', { name: 'Next' });
    const prevBtn = navPagination.getByRole('button', { name: 'Previous' });

    // Invariant: On Page 1, Previous is disabled, Next is enabled
    await expect(prevBtn).toBeDisabled();
    await expect(nextBtn).toBeEnabled();

    // Navigate to Page 2
    await nextBtn.click();
    await expect(page.getByText(/Page 2 of 2/i)).toBeVisible();
    await expect(nextBtn).toBeDisabled();
    await expect(prevBtn).toBeEnabled();

    // Invariant: Filtering to single result hides pagination controls, and clearing restores them
    await searchInput.fill(`REF-BOM-01`);
    await expect(orderTable.getByText('REF-BOM-01')).toBeVisible();
    await expect(navPagination).toBeHidden();
    await searchInput.fill('');
    await expect(navPagination).toBeVisible();

    // 5. Tracking Link Drilldown & Public Masking Parity
    console.log('[QA-ORDERS] Testing tracking link drilldown');
    const firstRowLink = orderTable.getByRole('link').first();
    const trackingNumber = (await firstRowLink.innerText()).trim();
    await firstRowLink.click();

    await expect(page).toHaveURL(new RegExp(`/portal/tracking/${trackingNumber}`));
    await expect(page.getByTestId('tracking-search-input')).toHaveValue(trackingNumber);
    // Public tracking must never display recipient phone number or street address
    await expect(page.getByText('+94771234567')).toBeHidden();
    await expect(page.getByText('No. 42, Galle Road')).toBeHidden();

    // =========================================================================
    // SECTION 5: Multi-Tenant IDOR Protection
    // =========================================================================
    console.log('[QA-ORDERS] Section 5: Testing Cross-Tenant IDOR Isolation with Merchant Tenant B');
    // Open a second isolated browser context for Merchant B
    const contextB = await browser.newContext();
    const pageB = await contextB.newPage();

    // Register Merchant B
    await pageB.goto('http://localhost:3000/auth/register');
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(merchantBEmail);
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();
    const otpCodeB = await getRegistrationOTPFromMailpit(merchantBEmail);
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(otpCodeB);
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Anura');
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Kumara');
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94770000005');
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill(merchantPassword);
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await pageB.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();
    await expect(pageB.getByText('Account created')).toBeVisible({ timeout: 10000 });

    // Login and apply as Merchant B
    await pageB.goto('http://localhost:3000/auth/login');
    await pageB.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(merchantBEmail);
    await pageB.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(merchantPassword);
    await pageB.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(pageB).toHaveURL(/\/dashboard/, { timeout: 10000 });

    await pageB.goto('http://localhost:3000/portal/merchants/apply');
    await pageB.getByTestId('merchant-application-business_name-input').fill(businessNameB);
    await pageB.getByTestId('merchant-application-contact_email-input').fill(merchantBEmail);
    await pageB.getByTestId('merchant-application-contact_phone-input').fill('+94770000005');
    await pageB.getByTestId('merchant-application-line1-input').fill('200 Kandy High Street');
    await pageB.getByTestId('merchant-application-city-input').fill('Kandy');

    const appPromiseB = pageB.waitForResponse(
      res => new URL(res.url()).pathname === '/api/v1/merchants/applications' && res.request().method() === 'POST'
    );
    await pageB.getByTestId('merchant-application-submit-btn').click();
    const appResB = await appPromiseB;
    const merchantBId = (await appResB.json()).data?.merchant_id;

    // Admin approves Merchant B
    await request.post(`http://localhost:8080/api/v1/admin/merchants/${merchantBId}/approve`, {
      headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
      data: { expected_version: 1, reason: 'Approved Merchant B for IDOR tests' },
    });

    // Login as approved Merchant B and inspect Orders view
    await pageB.goto('http://localhost:3000/auth/login');
    await pageB.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(merchantBEmail);
    await pageB.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(merchantPassword);
    await pageB.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await pageB.goto('http://localhost:3000/portal/merchants/orders');

    // CRITICAL SECURITY INVARIANT: Merchant B's orders view must be completely empty
    await expect(pageB.getByTestId('merchant-order-empty')).toHaveText('No orders yet.');
    // None of Merchant A's references must leak into Merchant B's DOM
    await expect(pageB.getByText(`REF-MIXED-01-${timestamp}`)).toBeHidden();
    await expect(pageB.getByText(`REF-BOM-01`)).toBeHidden();

    await contextB.close();
    console.log('[QA-ORDERS] All Senior QA test scenarios on Merchant Orders page completed with 100% assertions satisfied!');
  });

  test('CHAIN-01: CSV Ingested Order MUST traverse the full value-chain (DB -> Dispatcher -> Tracking -> Webhook)', async ({
    page,
    request,
  }) => {
    // Invariant: This test directly audits the cross-module value chain across DB, Dispatcher, Tracking and Webhooks.
    // It verifies that Issue #92 (@NagithaDS) & #93 (@HansiAshara) are fully resolved.
    const timestamp = Date.now();
    const chainRef = `REF-CHAIN-${timestamp}`;

    // 1. Log in as canonical merchant
    const merchantLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: 'merchant@vendor.com', password: 'Merchant123!' },
    });
    expect(merchantLoginRes.ok()).toBeTruthy();
    const merchantToken = (await merchantLoginRes.json()).data.access_token;

    await request.post('http://localhost:8080/api/v1/merchants/me/webhooks', {
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${merchantToken}`,
      },
      data: {
        url: `http://localhost:9999/webhooks/chain-${timestamp}`,
        events: ['parcel.booked.v1'],
      },
    });

    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('merchant@vendor.com');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Merchant123!');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await page.goto('/portal/merchants/orders');
    await expect(page.getByRole('heading', { name: 'Orders', level: 2 })).toBeVisible({ timeout: 10000 });

    // 2. Upload CSV order via Merchant Portal
    const csvContent = [
      CSV_HEADER,
      `${chainRef},Colombo Depot,+94770000004,"42 Galle Rd",Colombo,ZONE_COLOMBO_01,Nimal Perera,+94771234567,"15 Kandy Rd",Kandy,ZONE_COLOMBO_01,2.5,10,10,10,Chain Fragile Electronics`,
    ].join('\r\n');

    await page.getByTestId('batch-csv-file-input').setInputFiles({
      name: 'value-chain-order.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(csvContent, 'utf-8'),
    });
    await page.getByTestId('batch-csv-submit-btn').click();
    await expect(page.getByText('1 of 1 valid orders submitted.')).toBeVisible({ timeout: 10000 });

    // Extract generated tracking number from the submitted preview row
    const rowText = await page.locator('table[aria-label="CSV order preview"] td').nth(2).innerText();
    const trackingMatch = rowText.match(/SD-\d{8}-[A-Za-z0-9_-]+/);
    expect(trackingMatch).toBeTruthy();
    const trackingNumber = trackingMatch![0];
    console.log(`[VALUE-CHAIN-AUDIT] Testing downstream stations for CSV tracking number: ${trackingNumber}`);

    // =========================================================================
    // STATION 1: Central Dispatcher Board Visibility (Member 3)
    // Queries: SELECT * FROM parcels WHERE status IN ('CREATED', 'ATTEMPT_FAILED')
    // =========================================================================
    console.log('[VALUE-CHAIN-AUDIT] Station 1: Checking Central Dispatcher Board unassigned parcels');
    const dispatcherLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: 'dispatcher@swiftdrop.local', password: 'Dispatcher123!' },
    });
    expect(dispatcherLoginRes.ok()).toBeTruthy();
    const dispatcherToken = (await dispatcherLoginRes.json()).data.access_token;

    const dispatchRes = await request.get('http://localhost:8080/api/v1/dispatch/board?zone_id=ZONE_COLOMBO_01', {
      headers: { Authorization: `Bearer ${dispatcherToken}` },
    });
    expect(dispatchRes.status()).toBe(200);
    const dispatchJson = await dispatchRes.json();
    const unassignedList = dispatchJson.data.unassigned_parcels as Array<{ tracking_number: string }>;

    // FAILS HERE TODAY because BookingStub does not write to PostgreSQL parcels table!
    const isVisibleOnDispatchBoard = unassignedList.some(p => p.tracking_number === trackingNumber);
    expect(
      isVisibleOnDispatchBoard,
      `[BLOCKED BY #92 & #93] CSV parcel ${trackingNumber} is MISSING from the Central Dispatcher Board! BookingStub must be replaced with PostgreSQL adapter.`
    ).toBe(true);

    // =========================================================================
    // STATION 2: Public Tracking API Visibility (Member 5)
    // Queries: SELECT * FROM parcels WHERE tracking_number = $1
    // =========================================================================
    console.log('[VALUE-CHAIN-AUDIT] Station 2: Checking Public Tracking API');
    const trackingRes = await request.get(`http://localhost:8081/api/v1/tracking/public/${trackingNumber}`);
    expect(
      trackingRes.status(),
      `[BLOCKED BY #92 & #93] Public Tracking returned 404 for CSV parcel ${trackingNumber}!`
    ).toBe(200);

    // =========================================================================
    // STATION 3: Outbox & Webhook Delivery (Member 8)
    // Verifies parcel.booked.v1 was published and received by third-party receiver
    // =========================================================================
    console.log('[VALUE-CHAIN-AUDIT] Station 3: Checking Webhook Delivery');
    let webhookReceived = false;
    for (let i = 0; i < 20; i++) {
      await page.waitForTimeout(500);
      const webhookRes = await request.get('http://localhost:9999/api/events');
      const events = (await webhookRes.json()).events as Array<any>;
      webhookReceived = events.some(e => 
        e.data?.tracking_number === trackingNumber || 
        e.payload?.tracking_number === trackingNumber || 
        e.payload?.data?.tracking_number === trackingNumber ||
        JSON.stringify(e).includes(trackingNumber)
      );
      if (webhookReceived) break;
    }
    expect(
      webhookReceived,
      `Event Worker never received outbox event for CSV parcel ${trackingNumber}!`
    ).toBe(true);
  });
});
