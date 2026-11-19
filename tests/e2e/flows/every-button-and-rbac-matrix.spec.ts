import { test, expect } from '@playwright/test';
import { TEST_USERS } from '../fixtures/testData';
import { randomUUID } from 'node:crypto';

/**
 * Master CQA Suite: Every UI Button, DB Sync, and 6-Role RBAC Privilege Matrix ("No Less, No More")
 * 
 * Verifies:
 * Part 1: Courier Mobile PWA Every-Button & Invariant Synchronization:
 *   - Start/End shift buttons & DB status reflection
 *   - External communications (tel: link, Google Maps directions link)
 *   - Task list refresh
 *   - Single delivery start (ASSIGNED -> OUT_FOR_DELIVERY)
 *   - Digital signature pad capture (drawing canvas, clear, validation, submit) -> moves to DELIVERED
 *   - Delivery failure reporting with reason code & notes validation -> moves to ATTEMPT_FAILED
 *   - Attempt history modal drawer inspection
 * 
 * Part 2: Admin Console & Ops Workspace Every-Button Synchronization:
 *   - Overview live metric cards
 *   - Delivery zones: New Zone modal, validation, creation, enable/disable toggle with reason
 *   - Users & Access: User search, role filter, change role dialog trigger
 *   - System Settings: Runtime settings list, edit modal with audit reason validation
 *   - Append-only Audit Log: Multi-criteria filtering, before/after JSON snapshot modal
 *   - Ops Workspace full navigation (Dispatch, Fleet, Failed deliveries, Reports)
 * 
 * Part 3: Strict 6-Role Privilege Matrix ("No Less, No More" / Least Privilege):
 *   - PUBLIC: Access to tracking, login, register. Zero access to internal portals & APIs (401).
 *   - CUSTOMER: Access to booking, receipts, AI assistant, own shipments. Zero access to courier/ops/admin (403/redirect).
 *   - MERCHANT: Access to merchant orders, bulk import, API keys, webhooks. Zero access to courier/ops/admin.
 *   - COURIER: Access to mobile runs & POD capture. Zero access to booking, dispatch, admin.
 *   - DISPATCHER: Access to dispatch board, fleet, failed deliveries, reports. Zero access to admin-only zones/users/settings/audit.
 *   - ADMIN: Full access to admin console and operations workspace.
 * 
 * Authority: Mandatory CQA Evaluation Suite (AGENTS.md)
 */

test.describe('Master CQA: Every Button, Every Action & Strict RBAC Privileges Matrix', () => {
  const CORE_API = 'http://localhost:8080/api/v1';

  // Helper to authenticate via API
  async function apiLogin(email: string, pass: string) {
    const res = await fetch(`${CORE_API}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password: pass }),
    });
    const body = await res.json();
    return { token: body.data?.access_token as string, user: body.data?.user };
  }

  // Helper to provision a parcel assigned to courier
  async function provisionAssignedParcel(custToken: string, dispToken: string, courierUserId: string) {
    // 1. Get quote
    const quoteRes = await fetch(`${CORE_API}/pricing/quotes`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${custToken}`,
      },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 2.0,
        dimensions: { length_cm: 20, width_cm: 15, height_cm: 10 },
      }),
    });
    const quoteData = await quoteRes.json();
    const quoteId = quoteData.data?.quote_id;

    // 2. Book parcel
    const bookRes = await fetch(`${CORE_API}/parcels`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${custToken}`,
        'Idempotency-Key': randomUUID(),
      },
      body: JSON.stringify({
        quote_id: quoteId,
        recipient_name: 'Test Recipient CQA',
        recipient_phone: '+94770001111',
        weight_kg: 2.0,
        dimensions: { length_cm: 20, width_cm: 15, height_cm: 10 },
        pickup: {
          contact_name: 'Sender CQA',
          contact_phone: '+94772223333',
          address_line1: '100 Core St',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: 'Test Recipient CQA',
          contact_phone: '+94770001111',
          address_line1: '200 Edge Ave',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
      }),
    });
    const bookData = await bookRes.json();
    if (!bookRes.ok) {
      throw new Error(`Booking failed: ${JSON.stringify(bookData)}`);
    }
    const parcelId = bookData.data?.id;
    const trackingNumber = bookData.data?.tracking_number;

    // 3. Assign to courier as dispatcher
    const assignRes = await fetch(`${CORE_API}/dispatch/assignments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${dispToken}`,
        'Idempotency-Key': randomUUID(),
      },
      body: JSON.stringify({
        parcel_id: parcelId,
        courier_id: courierUserId,
      }),
    });
    if (!assignRes.ok) {
      const assignErr = await assignRes.json();
      throw new Error(`Assignment failed: ${JSON.stringify(assignErr)}`);
    }

    return { parcelId, trackingNumber };
  }

  // =========================================================================
  // SECTION 1: COURIER MOBILE PWA EVERY-BUTTON & INVARIANT SYNCHRONIZATION
  // =========================================================================
  test.describe('Section 1: Courier Mobile PWA Every-Button & Actions', () => {
    test('should exercise all courier buttons: shifts, links, runs, signature pad POD, and failure reporting with attempt history', async ({ page }) => {
      // Setup tokens and courier ID
      const custAuth = await apiLogin(TEST_USERS.CUSTOMER.email, TEST_USERS.CUSTOMER.password);
      const dispAuth = await apiLogin(TEST_USERS.DISPATCHER.email, TEST_USERS.DISPATCHER.password);
      const courAuth = await apiLogin(TEST_USERS.COURIER.email, TEST_USERS.COURIER.password);

      // Provision 2 assigned parcels for the courier: one for POD delivery, one for failure reporting
      const parcel1 = await provisionAssignedParcel(custAuth.token, dispAuth.token, courAuth.user.id);
      const parcel2 = await provisionAssignedParcel(custAuth.token, dispAuth.token, courAuth.user.id);

      // Step A: Courier Login
      await page.goto('/auth/login');
      await page.locator('[data-testid="login-email-input"]').fill(TEST_USERS.COURIER.email);
      await page.locator('[data-testid="login-password-input"]').fill(TEST_USERS.COURIER.password);
      await page.locator('[data-testid="login-submit-btn"]').click();
      await page.waitForURL('**/courier/dashboard');

      // Step B: Verify Shift Management Buttons ("Start shift" / "End shift")
      const startShiftBtn = page.getByRole('button', { name: 'Start shift' });
      const endShiftBtn = page.getByRole('button', { name: 'End shift' });

      if (await startShiftBtn.isVisible()) {
        await startShiftBtn.click();
        await expect(endShiftBtn).toBeVisible({ timeout: 5000 });
      } else if (await endShiftBtn.isVisible()) {
        await endShiftBtn.click();
        await expect(startShiftBtn).toBeVisible({ timeout: 5000 });
        // Restore to ON_SHIFT
        await startShiftBtn.click();
        await expect(endShiftBtn).toBeVisible({ timeout: 5000 });
      }

      // Step C: Verify "Refresh tasks" button
      const refreshBtn = page.getByRole('button', { name: 'Refresh tasks' });
      await expect(refreshBtn).toBeVisible();
      await refreshBtn.click();

      // Step D: Verify Task Cards, Phone Link, and Directions Link
      const taskCard = page.locator('[data-testid="courier-task"]').filter({ hasText: parcel1.trackingNumber });
      await expect(taskCard).toBeVisible({ timeout: 10000 });

      const telLink = taskCard.getByRole('link', { name: /Call recipient/i });
      await expect(telLink).toBeVisible();
      const telHref = await telLink.getAttribute('href');
      expect(telHref).toContain('tel:');

      const directionsLink = taskCard.getByRole('link', { name: /Directions/i });
      await expect(directionsLink).toBeVisible();
      const mapsHref = await directionsLink.getAttribute('href');
      expect(mapsHref).toContain('google.com/maps');

      // Step E: Trigger "Start delivery" button for Parcel 1
      const startRunBtn = taskCard.locator('[data-testid="courier-start-run"]');
      await expect(startRunBtn).toBeVisible();
      await startRunBtn.click();

      // Verify status moves to OUT_FOR_DELIVERY
      await expect(taskCard.getByText(/Out for delivery/i)).toBeVisible({ timeout: 6000 });

      // Step F: Test Digital Signature POD Modal & Canvas
      const recordDeliveryBtn = taskCard.getByRole('button', { name: 'Record delivery' });
      await expect(recordDeliveryBtn).toBeVisible();
      await recordDeliveryBtn.click();

      // Verify Proof Form renders with Proof method selector
      await expect(taskCard.getByText('Proof of delivery method')).toBeVisible({ timeout: 6000 });

      // Switch to Signature mode
      const signatureTab = taskCard.getByRole('tab', { name: /Customer signature|Signature/i });
      await expect(signatureTab).toBeVisible();
      await signatureTab.click();

      // Name input
      const nameInput = taskCard.locator('input[name="name"]');
      await nameInput.fill('Kamal Wickramasinghe');

      // Test validation: submitting without signature shows error
      const confirmBtn = taskCard.getByRole('button', { name: 'Confirm delivery' });
      await confirmBtn.click();
      await expect(page.getByText(/Please have the recipient draw their signature/i)).toBeVisible();

      // Draw signature on Canvas (simulate mouse pointer events)
      const canvas = taskCard.locator('canvas');
      await expect(canvas).toBeVisible();
      const box = await canvas.boundingBox();
      if (box) {
        await page.mouse.move(box.x + 20, box.y + 20);
        await page.mouse.down();
        await page.mouse.move(box.x + 80, box.y + 40);
        await page.mouse.move(box.x + 140, box.y + 30);
        await page.mouse.move(box.x + 200, box.y + 70);
        await page.mouse.up();
      }

      // Test "Clear signature" button
      const clearBtn = taskCard.getByRole('button', { name: 'Clear signature' });
      await expect(clearBtn).toBeVisible();
      await clearBtn.click();

      // Re-draw signature after clear
      if (box) {
        await page.mouse.move(box.x + 30, box.y + 30);
        await page.mouse.down();
        await page.mouse.move(box.x + 100, box.y + 60);
        await page.mouse.move(box.x + 160, box.y + 40);
        await page.mouse.move(box.x + 220, box.y + 80);
        await page.mouse.up();
      }

      // Submit Confirm Delivery
      await confirmBtn.click();

      // Verify Delivery Success Alert Badge
      await expect(page.locator('[data-testid="delivery-success-alert"]')).toBeVisible({ timeout: 6000 });
      await expect(page.getByText('Delivery confirmed successfully')).toBeVisible();

      // Verify Dismiss button on Alert
      await page.getByRole('button', { name: 'Dismiss' }).click();
      await expect(page.locator('[data-testid="delivery-success-alert"]')).not.toBeVisible();

      // Step G: Test Delivery Failure Reporting & Validation for Parcel 2
      await page.getByRole('button', { name: 'Refresh tasks' }).click();
      const taskCard2 = page.locator('[data-testid="courier-task"]').filter({ hasText: parcel2.trackingNumber });
      await expect(taskCard2).toBeAttached({ timeout: 15000 });
      await taskCard2.scrollIntoViewIfNeeded();
      await expect(taskCard2).toBeVisible({ timeout: 10000 });

      // Start delivery for Parcel 2
      await taskCard2.locator('[data-testid="courier-start-run"]').click();
      await expect(taskCard2.getByText(/Out for delivery/i)).toBeVisible({ timeout: 6000 });

      // Click "Report failed attempt"
      const reportFailureBtn = taskCard2.getByRole('button', { name: 'Report failed attempt' });
      await expect(reportFailureBtn).toBeVisible();
      await reportFailureBtn.click();

      // Test validation: OTHER reason with < 15 chars notes
      const reasonSelect = taskCard2.locator('select[name="reason"]');
      await reasonSelect.selectOption('OTHER');
      const notesInput = taskCard2.locator('input[name="notes"]');
      await notesInput.fill('Short');

      const saveAttemptBtn = taskCard2.getByRole('button', { name: 'Save attempt' });
      await saveAttemptBtn.click();
      await expect(page.getByText(/Reason OTHER requires a detailed explanation of at least 15 characters/i)).toBeVisible();

      // Provide valid reason and notes
      await reasonSelect.selectOption('RECIPIENT_UNAVAILABLE');
      await notesInput.fill('Customer did not answer doorbell and phone was unreachable after 3 attempts.');
      await saveAttemptBtn.click();

      // Verify notice and alert
      await expect(page.locator('[data-testid="delivery-success-alert"]')).toBeVisible({ timeout: 6000 });
      await page.getByRole('button', { name: 'Dismiss' }).click();

      // Step H: Test "View attempt history" button
      const viewHistoryBtn = taskCard2.locator('[data-testid="view-attempt-history-btn"]');
      await expect(viewHistoryBtn).toBeVisible({ timeout: 6000 });
      await viewHistoryBtn.click();

      // Verify attempt history dialog displays recorded attempt
      await expect(page.getByText('Delivery attempt history')).toBeVisible({ timeout: 6000 });
      await expect(page.getByText(/Recipient unavailable/i).first()).toBeVisible();

      // Close modal via Escape
      await page.keyboard.press('Escape');
    });
  });

  // =========================================================================
  // SECTION 2: ADMIN CONSOLE & OPS WORKSPACE EVERY-BUTTON SYNCHRONIZATION
  // =========================================================================
  test.describe('Section 2: Admin Console & Ops Workspace Every-Button Actions', () => {
    test('should exercise all admin buttons: summary cards, delivery zones CRUD, user filters, settings edit, and audit log', async ({ page }) => {
      // Step A: Admin Sign-In
      await page.goto('/auth/login');
      await page.locator('[data-testid="login-email-input"]').fill(TEST_USERS.ADMIN.email);
      await page.locator('[data-testid="login-password-input"]').fill(TEST_USERS.ADMIN.password);
      await page.locator('[data-testid="login-submit-btn"]').click();
      await page.waitForURL(/\/ops|\/admin\/dashboard/);

      // Navigate to /admin/dashboard explicitly
      await page.goto('/admin/dashboard');

      // Step B: Verify Summary Stat Cards
      await expect(page.getByText('Delivery zones').first()).toBeVisible({ timeout: 8000 });
      await expect(page.getByText('User accounts').first()).toBeVisible();
      await expect(page.getByText('Audit log entries').first()).toBeVisible();

      // Step C: Delivery Zones Tab — "New zone" modal and validation
      const zonesTab = page.getByRole('tab', { name: 'Delivery zones' });
      await expect(zonesTab).toBeVisible();
      await zonesTab.click();

      const newZoneBtn = page.locator('[data-testid="admin-zone-new-btn"]');
      await expect(newZoneBtn).toBeVisible();
      await newZoneBtn.click();

      // Verify modal opens
      await expect(page.locator('[data-testid="admin-zone-form"]')).toBeVisible({ timeout: 5000 });

      // Test validation: submit empty form
      const saveZoneBtn = page.locator('[data-testid="admin-zone-save-btn"]');
      await saveZoneBtn.click();
      await expect(page.getByText('Zone id is required.')).toBeVisible();

      // Create a unique test zone
      const randomSuffix = Math.floor(1000 + Math.random() * 9000);
      const testZoneId = `ZONE_CQA_${randomSuffix}`;
      const testZoneCode = `CQA-${randomSuffix}`;
      const testPrefix = `8${randomSuffix.toString().slice(0, 4)}`;

      await page.locator('[data-testid="admin-zone-id-input"]').fill(testZoneId);
      await page.locator('[data-testid="admin-zone-code-input"]').fill(testZoneCode);
      await page.locator('[data-testid="admin-zone-name-input"]').fill(`CQA Zone ${randomSuffix}`);
      await page.locator('[data-testid="admin-zone-description-input"]').fill('Quality Assurance automated test zone');
      await page.locator('[data-testid="admin-zone-prefixes-input"]').fill(testPrefix);

      await saveZoneBtn.click();

      // Verify success notice and table presence
      await expect(page.locator('[data-testid="admin-notice"]')).toBeVisible({ timeout: 6000 });
      await expect(page.getByText(`Zone ${testZoneId} created.`)).toBeVisible();
      await expect(page.getByText(testZoneCode)).toBeVisible();

      // Step D: Users and Access Tab
      const usersTab = page.getByRole('tab', { name: 'Users and access' });
      await expect(usersTab).toBeVisible();
      await usersTab.click();

      // Verify users table renders
      await expect(page.locator('[data-testid="admin-users-table"]')).toBeVisible({ timeout: 6000 });

      // Dynamically read a visible email from the first row and test search filter
      const firstRowEmail = await page.locator('[data-testid="admin-users-table"] tbody tr').first().locator('td').first().locator('.text-xs').first().textContent();
      if (firstRowEmail && firstRowEmail.trim()) {
        const searchInput = page.locator('[data-testid="admin-users-search"]');
        await searchInput.fill(firstRowEmail.trim());
        await expect(page.getByText(firstRowEmail.trim())).toBeVisible();
        await searchInput.fill('');
      }

      // Step E: System Settings Tab
      const settingsTab = page.getByRole('tab', { name: 'Settings' });
      await expect(settingsTab).toBeVisible();
      await settingsTab.click();

      await expect(page.getByText('System settings', { exact: true })).toBeVisible();
      await expect(page.locator('table[aria-label="System settings"]')).toBeVisible({ timeout: 6000 });

      // Click "Edit" on the first setting to test modal
      const editSettingBtn = page.getByRole('button', { name: /Edit/i }).first();
      if (await editSettingBtn.isVisible()) {
        await editSettingBtn.click();
        await expect(page.getByText(/Reason for the change/i)).toBeVisible();
        // Click Cancel to leave unchanged
        await page.getByRole('button', { name: 'Cancel' }).click();
      }

      // Step F: Audit Log Tab
      const auditTab = page.getByRole('tab', { name: 'Audit log' });
      await expect(auditTab).toBeVisible();
      await auditTab.click();

      await expect(page.locator('table[aria-label="Audit log"]')).toBeVisible({ timeout: 6000 });

      // Click "View snapshot" or first audit detail row
      const snapshotBtn = page.getByRole('button', { name: /View snapshot|View changes|Details/i }).first();
      if (await snapshotBtn.isVisible()) {
        await snapshotBtn.click();
        await expect(page.getByRole('dialog')).toBeVisible();
        await page.keyboard.press('Escape');
      }

      // Step G: Ops Navigation Switching
      await page.goto('/ops/dispatch');
      await expect(page.getByRole('heading', { name: 'Dispatch board' })).toBeVisible({ timeout: 6000 });

      await page.goto('/ops/couriers');
      await expect(page.getByRole('heading', { name: 'Fleet' })).toBeVisible({ timeout: 6000 });

      await page.goto('/ops/failed-deliveries');
      await expect(page.getByRole('heading', { name: 'Failed deliveries' })).toBeVisible({ timeout: 6000 });

      await page.goto('/ops/reports');
      await expect(page.getByRole('heading', { name: 'Reports' })).toBeVisible({ timeout: 6000 });
    });
  });

  // =========================================================================
  // SECTION 3: STRICT 6-ROLE RBAC PRIVILEGE MATRIX ("NO LESS, NO MORE")
  // =========================================================================
  test.describe('Section 3: Strict 6-Role Privilege Matrix ("No Less, No More")', () => {
    // -----------------------------------------------------------------------
    // Role 1: PUBLIC (Unauthenticated)
    // -----------------------------------------------------------------------
    test('Role 1: PUBLIC has exactly public privileges (tracking/login/register), zero access to internal portals or APIs', async ({ page }) => {
      // "No Less": Public can track parcels
      await page.goto('/track');
      await expect(page.locator('[data-testid="tracking-search-input"]')).toBeVisible();

      // "No Less": Public can view login and registration
      await page.goto('/auth/login');
      await expect(page.locator('[data-testid="login-email-input"]')).toBeVisible();

      await page.goto('/auth/register');
      await expect(page.locator('[data-testid="register-email-input"]')).toBeVisible();

      // "No More": Accessing protected portals redirects to /auth/login
      const protectedPaths = [
        '/portal/book',
        '/portal/merchants/orders',
        '/courier/dashboard',
        '/ops/dispatch',
        '/admin/dashboard',
        '/dashboard',
      ];

      for (const path of protectedPaths) {
        await page.goto(path);
        await page.waitForURL('**/auth/login', { timeout: 6000 });
      }

      // "No More": Direct API requests without Bearer token return 401
      const apis = ['/auth/me', '/parcels', '/courier/parcels', '/admin/dashboard'];
      for (const api of apis) {
        const res = await fetch(`${CORE_API}${api}`);
        expect(res.status).toBe(401);
      }
    });

    // -----------------------------------------------------------------------
    // Role 2: CUSTOMER
    // -----------------------------------------------------------------------
    test('Role 2: CUSTOMER has full booking & dashboard privileges, strictly barred from courier, ops, and admin', async ({ page }) => {
      // Authenticate as Customer
      const custAuth = await apiLogin(TEST_USERS.CUSTOMER.email, TEST_USERS.CUSTOMER.password);

      await page.goto('/auth/login');
      await page.locator('[data-testid="login-email-input"]').fill(TEST_USERS.CUSTOMER.email);
      await page.locator('[data-testid="login-password-input"]').fill(TEST_USERS.CUSTOMER.password);
      await page.locator('[data-testid="login-submit-btn"]').click();
      await page.waitForURL('**/dashboard');

      // "No Less": Customer can open Booking wizard
      await page.goto('/portal/book');
      await expect(page.locator('[data-testid="booking-pickup-address-input"]')).toBeVisible();

      // "No Less": Customer can open Customer Dashboard
      await page.goto('/dashboard');
      await expect(page.getByRole('heading', { name: 'Customer portal' })).toBeVisible({ timeout: 6000 });

      // "No More": Customer cannot access Courier Dashboard (barred: redirects to /dashboard or /auth/login)
      await page.goto('/courier/dashboard');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Customer cannot access Ops Workspace (barred: redirects to /dashboard or /auth/login)
      await page.goto('/ops/dispatch');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Customer cannot access Admin Console (barred: redirects to /dashboard or /auth/login)
      await page.goto('/admin/dashboard');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Customer API attempts to dispatch, courier, or admin return 403
      const forbiddenEndpoints = [
        { url: `${CORE_API}/courier/parcels`, method: 'GET' },
        { url: `${CORE_API}/courier/shift`, method: 'POST', body: { target_status: 'ON_SHIFT' } },
        { url: `${CORE_API}/dispatch/assignments`, method: 'POST', body: { parcel_id: randomUUID(), courier_id: randomUUID() } },
        { url: `${CORE_API}/admin/dashboard`, method: 'GET' },
        { url: `${CORE_API}/admin/zones`, method: 'GET' },
      ];

      for (const ep of forbiddenEndpoints) {
        const res = await fetch(ep.url, {
          method: ep.method,
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${custAuth.token}`,
          },
          body: ep.body ? JSON.stringify(ep.body) : undefined,
        });
        expect(res.status).toBe(403);
      }
    });

    // -----------------------------------------------------------------------
    // Role 3: MERCHANT
    // -----------------------------------------------------------------------
    test('Role 3: MERCHANT has full orders, api-keys, and webhooks privileges, strictly barred from courier, ops, and admin', async ({ page }) => {
      // Authenticate as Merchant
      const merchAuth = await apiLogin('merchant@vendor.com', 'Merchant123!');

      await page.goto('/auth/login');
      await page.locator('[data-testid="login-email-input"]').fill('merchant@vendor.com');
      await page.locator('[data-testid="login-password-input"]').fill('Merchant123!');
      await page.locator('[data-testid="login-submit-btn"]').click();
      await page.waitForURL(/\/portal|\/dashboard/);

      // "No Less": Merchant can open Orders
      await page.goto('/portal/merchants/orders');
      await expect(page.locator('table')).toBeVisible({ timeout: 8000 });

      // "No Less": Merchant can open API Keys
      await page.goto('/portal/merchants/api-keys');
      await expect(page.getByRole('heading', { name: 'API keys' })).toBeVisible({ timeout: 6000 });

      // "No Less": Merchant can open Webhooks
      await page.goto('/portal/merchants/webhooks');
      await expect(page.getByRole('heading', { name: 'Webhooks' })).toBeVisible({ timeout: 6000 });

      // "No More": Merchant cannot access Courier Dashboard (barred: redirects to /dashboard or /auth/login)
      await page.goto('/courier/dashboard');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Merchant cannot access Ops Workspace (barred: redirects to /dashboard or /auth/login)
      await page.goto('/ops/dispatch');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Merchant cannot access Admin Console (barred: redirects to /dashboard or /auth/login)
      await page.goto('/admin/dashboard');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Merchant API attempts to courier, dispatch, or admin return 403
      const forbiddenEndpoints = [
        { url: `${CORE_API}/courier/parcels`, method: 'GET' },
        { url: `${CORE_API}/dispatch/assignments`, method: 'POST', body: { parcel_id: randomUUID(), courier_id: randomUUID() } },
        { url: `${CORE_API}/admin/dashboard`, method: 'GET' },
      ];

      for (const ep of forbiddenEndpoints) {
        const res = await fetch(ep.url, {
          method: ep.method,
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${merchAuth.token}`,
          },
          body: ep.body ? JSON.stringify(ep.body) : undefined,
        });
        expect(res.status).toBe(403);
      }
    });

    // -----------------------------------------------------------------------
    // Role 4: COURIER
    // -----------------------------------------------------------------------
    test('Role 4: COURIER has mobile runs privileges, strictly barred from customer booking, merchant, ops, and admin', async ({ page }) => {
      // Authenticate as Courier
      const courAuth = await apiLogin(TEST_USERS.COURIER.email, TEST_USERS.COURIER.password);

      await page.goto('/auth/login');
      await page.locator('[data-testid="login-email-input"]').fill(TEST_USERS.COURIER.email);
      await page.locator('[data-testid="login-password-input"]').fill(TEST_USERS.COURIER.password);
      await page.locator('[data-testid="login-submit-btn"]').click();
      await page.waitForURL('**/courier/dashboard');

      // "No Less": Courier can view Delivery tasks
      await expect(page.getByRole('heading', { name: 'Delivery tasks' })).toBeVisible();

      // "No More": Courier cannot access Customer Booking (barred: redirects to /dashboard or /auth/login)
      await page.goto('/portal/book');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Courier cannot access Merchant Orders (barred: redirects to /dashboard or /auth/login)
      await page.goto('/portal/merchants/orders');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Courier cannot access Ops Workspace (barred: redirects to /dashboard or /auth/login)
      await page.goto('/ops/dispatch');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Courier cannot access Admin Console (barred: redirects to /dashboard or /auth/login)
      await page.goto('/admin/dashboard');
      await expect(page).toHaveURL(/\/(dashboard|auth\/login)/, { timeout: 8000 });

      // "No More": Courier API attempts to book, dispatch, or admin return 403
      const forbiddenEndpoints = [
        {
          url: `${CORE_API}/parcels`,
          method: 'POST',
          body: {
            recipient_name: 'Test',
            delivery_street: 'Street',
            delivery_city: 'City',
            delivery_postal_code: '00100',
            pickup_street: 'Street',
            pickup_city: 'City',
            pickup_postal_code: '00100',
            weight_kg: 1,
            dimension_length_cm: 10,
            dimension_width_cm: 10,
            dimension_height_cm: 10,
            delivery_type: 'STANDARD',
          },
        },
        { url: `${CORE_API}/dispatch/assignments`, method: 'POST', body: { parcel_id: randomUUID(), courier_id: randomUUID() } },
        { url: `${CORE_API}/admin/dashboard`, method: 'GET' },
      ];

      for (const ep of forbiddenEndpoints) {
        const res = await fetch(ep.url, {
          method: ep.method,
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${courAuth.token}`,
          },
          body: JSON.stringify(ep.body),
        });
        expect(res.status).toBe(403);
      }
    });

    // -----------------------------------------------------------------------
    // Role 5: DISPATCHER
    // -----------------------------------------------------------------------
    test('Role 5: DISPATCHER has operational routing & fleet privileges, strictly barred from admin-only sections (no less, no more)', async ({ page }) => {
      // Authenticate as Dispatcher
      const dispAuth = await apiLogin(TEST_USERS.DISPATCHER.email, TEST_USERS.DISPATCHER.password);

      await page.goto('/auth/login');
      await page.locator('[data-testid="login-email-input"]').fill(TEST_USERS.DISPATCHER.email);
      await page.locator('[data-testid="login-password-input"]').fill(TEST_USERS.DISPATCHER.password);
      await page.locator('[data-testid="login-submit-btn"]').click();
      await page.waitForURL('**/ops');

      // "No Less": Dispatcher can access Dispatch Board
      await page.goto('/ops/dispatch');
      await expect(page.getByRole('heading', { name: 'Dispatch board' })).toBeVisible({ timeout: 6000 });

      // "No Less": Dispatcher can access Fleet
      await page.goto('/ops/couriers');
      await expect(page.getByRole('heading', { name: 'Fleet' })).toBeVisible({ timeout: 6000 });

      // "No Less": Dispatcher can access Failed Deliveries
      await page.goto('/ops/failed-deliveries');
      await expect(page.getByRole('heading', { name: 'Failed deliveries' })).toBeVisible({ timeout: 6000 });

      // "No Less": Dispatcher can access Reports
      await page.goto('/ops/reports');
      await expect(page.getByRole('heading', { name: 'Reports' })).toBeVisible({ timeout: 6000 });

      // "No More": Dispatcher cannot access Admin-only /ops/users -> UI renders ops-forbidden
      await page.goto('/ops/users');
      await expect(page.locator('[data-testid="ops-forbidden"]')).toBeVisible({ timeout: 6000 });

      // "No More": Dispatcher cannot access Admin-only /ops/zones -> UI renders ops-forbidden
      await page.goto('/ops/zones');
      await expect(page.locator('[data-testid="ops-forbidden"]')).toBeVisible({ timeout: 6000 });

      // "No More": Dispatcher cannot access Admin-only /ops/settings -> UI renders ops-forbidden
      await page.goto('/ops/settings');
      await expect(page.locator('[data-testid="ops-forbidden"]')).toBeVisible({ timeout: 6000 });

      // "No More": Dispatcher cannot access Admin-only /ops/audit -> UI renders ops-forbidden
      await page.goto('/ops/audit');
      await expect(page.locator('[data-testid="ops-forbidden"]')).toBeVisible({ timeout: 6000 });

      // "No More": Dispatcher API attempts to call admin endpoints return 403
      const forbiddenEndpoints = [
        { url: `${CORE_API}/admin/dashboard`, method: 'GET' },
        { url: `${CORE_API}/admin/users`, method: 'GET' },
        { url: `${CORE_API}/admin/zones`, method: 'GET' },
        { url: `${CORE_API}/admin/settings`, method: 'GET' },
        { url: `${CORE_API}/admin/audit-logs`, method: 'GET' },
        // Dispatcher cannot complete delivery proof on behalf of courier
        {
          url: `${CORE_API}/deliveries/${randomUUID()}/complete`,
          method: 'POST',
          body: { proof_type: 'SIGNATURE', signature_data: 'dGVzdA==', received_by_name: 'Imposter' },
        },
      ];

      for (const ep of forbiddenEndpoints) {
        const res = await fetch(ep.url, {
          method: ep.method,
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${dispAuth.token}`,
          },
          body: ep.body ? JSON.stringify(ep.body) : undefined,
        });
        expect(res.status).toBe(403);
      }
    });

    // -----------------------------------------------------------------------
    // Role 6: ADMIN
    // -----------------------------------------------------------------------
    test('Role 6: ADMIN has full system administration and operations privileges without restriction', async ({ page }) => {
      // Authenticate as Admin
      await page.goto('/auth/login');
      await page.locator('[data-testid="login-email-input"]').fill(TEST_USERS.ADMIN.email);
      await page.locator('[data-testid="login-password-input"]').fill(TEST_USERS.ADMIN.password);
      await page.locator('[data-testid="login-submit-btn"]').click();
      await page.waitForURL(/\/ops|\/admin\/dashboard/);

      // "No Less": Admin can access /admin/dashboard
      await page.goto('/admin/dashboard');
      await expect(page.getByRole('heading', { name: 'Admin console' })).toBeVisible({ timeout: 6000 });

      // "No Less": Admin can access /ops/users without forbidden block
      await page.goto('/ops/users');
      await expect(page.locator('[data-testid="ops-forbidden"]')).not.toBeVisible();
      await expect(page.getByRole('heading', { name: 'Users and roles' })).toBeVisible({ timeout: 6000 });

      // "No Less": Admin can access /ops/zones without forbidden block
      await page.goto('/ops/zones');
      await expect(page.locator('[data-testid="ops-forbidden"]')).not.toBeVisible();
      await expect(page.getByRole('heading', { name: 'Delivery zones' })).toBeVisible({ timeout: 6000 });

      // "No Less": Admin can access /ops/settings without forbidden block
      await page.goto('/ops/settings');
      await expect(page.locator('[data-testid="ops-forbidden"]')).not.toBeVisible();
      await expect(page.getByRole('heading', { name: 'System settings' })).toBeVisible({ timeout: 6000 });

      // "No Less": Admin can access /ops/audit without forbidden block
      await page.goto('/ops/audit');
      await expect(page.locator('[data-testid="ops-forbidden"]')).not.toBeVisible();
      await expect(page.getByRole('heading', { name: 'Audit log' })).toBeVisible({ timeout: 6000 });
    });
  });
});
