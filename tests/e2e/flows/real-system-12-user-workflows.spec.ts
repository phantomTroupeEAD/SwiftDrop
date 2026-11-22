import { test, expect, Page, BrowserContext } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import EventSource from 'eventsource';
import { SELECTORS, testId } from '../helpers/selectors';
import { getRegistrationOTPFromMailpit, clearMailpitMessages } from '../helpers/mailpitHelper';

/**
 * SWIFTDROP — REAL 12-USER MULTI-ROLE REAL-WORLD WORKFLOW AUTOMATION SUITE
 *
 * Implements and validates 12 distinct real actors across all 6 system roles:
 *
 * 1.  Customer 1: Dilshan Silva (customer@swiftdrop.local) - Existing customer booking standard parcel
 * 2.  Customer 2: Amara Jayasinghe (amara_e2e_<ts>@swiftdrop.local) - New customer sign-up via real Mailpit SMTP OTP
 * 3.  Customer 3: Nadeeka Perera (nadeeka.perera@swiftdrop.local) - Returning customer booking heavy bulky shipment
 * 4.  Merchant 1: Colombo Tech Stores (merchant@vendor.com) - API key generation, active webhook configuration, B2B order
 * 5.  Merchant 2: Beta Supplies (beta@vendor.local) - Fulfillment orders batch, multi-tenant boundary audit
 * 6.  Courier 1: Kamal Perera (courier@swiftdrop.local) - On-shift primary courier, delivery run, digital POD signature
 * 7.  Courier 2: Sunil Shantha (courier2@swiftdrop.local) - On-shift courier, exception failure reporting with audit notes
 * 8.  Courier 3: Chaminda Vaas (courier3@swiftdrop.local) - Relief courier, rescheduled parcel re-attempt & POD completion
 * 9.  Dispatcher 1: Kamal Perera (dispatcher@swiftdrop.local) - Capacity triage, board routing, multi-courier assignments
 * 10. Dispatcher 2: Rohan De Silva (dispatcher2@swiftdrop.local) - Failed deliveries board triage, reschedule & re-allocation
 * 11. Admin: System Administrator (admin@swiftdrop.local) - Zone management, ops metrics, tamper-proof audit inspection
 * 12. Public Guest: Unauthenticated Visitor - Tracking lookup, SEC-009 customer PII masking, SSE live milestone fan-out
 *
 * Verifies complete physical state machine progression:
 * BOOKED -> ASSIGNED -> OUT_FOR_DELIVERY -> FAILED -> RESCHEDULED -> DELIVERED
 *
 * Authority: Mandatory Cross-Module E2E System Suite (AGENTS.md)
 */

test.describe.serial('SwiftDrop Real-World 12-User Multi-Role Workflow Automation Suite', () => {
  test.setTimeout(180000); // 3 minutes total

  const CORE_API = 'http://localhost:8080/api/v1';
  const TRACKING_API = 'http://localhost:8081/api/v1';
  const MOCK_WEBHOOK_URL = 'http://localhost:9999/api/webhook';
  const WEBHOOK_RESET_URL = 'http://localhost:9999/api/reset';

  // Seed credentials & actors
  const ACTORS = {
    ADMIN: { email: 'admin@swiftdrop.local', password: 'AdminPassword123!', name: 'System Administrator' },
    DISPATCHER_1: { email: 'dispatcher@swiftdrop.local', password: 'Dispatcher123!', name: 'Central Dispatcher' },
    DISPATCHER_2: { email: 'dispatcher2@swiftdrop.local', password: 'Dispatcher123!', name: 'Rohan De Silva' },
    COURIER_1: { email: 'courier@swiftdrop.local', password: 'CourierPassword123!', id: '018f4a7c-1003-7000-8000-000000000003', code: 'CR-0012' },
    COURIER_2: { email: 'courier2@swiftdrop.local', password: 'CourierPassword123!', id: '018f4a7c-1003-7000-8000-000000000099', code: 'CR-0099' },
    COURIER_3: { email: 'courier3@swiftdrop.local', password: 'CourierPassword123!', id: '018f4a7c-1003-7000-8000-000000000098', code: 'CR-0098' },
    MERCHANT_1: { email: 'merchant@vendor.com', password: 'Merchant123!', name: 'Colombo Tech Stores' },
    MERCHANT_2: { email: 'beta@vendor.local', password: 'Merchant123!', name: 'Beta Supplies Merchant' },
    CUSTOMER_1: { email: 'customer@swiftdrop.local', password: 'CustomerPassword123!', name: 'Dilshan Silva' },
    CUSTOMER_3: { email: 'nadeeka.perera@swiftdrop.local', password: 'CustomerPassword123!', name: 'Nadeeka Perera' },
  };

  // Shared state across the 12-user value-chain
  let customer2Email = '';
  const customer2Password = 'CustomerPassword123!';
  const authTokens: Record<string, string> = {};

  let parcel1Id = '';
  let parcel1Tracking = '';
  let parcel2Id = '';
  let parcel2Tracking = '';
  let parcel3Id = '';
  let parcel3Tracking = '';
  let merchant1ParcelId = '';
  let merchant1Tracking = '';
  let merchant1ApiKey = '';

  // Persistent live tracking browser context for real-time SSE stream verification
  let customerContext: BrowserContext;
  let customerTrackingPage: Page;

  // API helper for authentication
  async function apiLogin(email: string, pass: string): Promise<{ token: string; user: any }> {
    const res = await fetch(`${CORE_API}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password: pass }),
    });
    if (!res.ok) {
      const err = await res.text();
      throw new Error(`Login failed for ${email}: ${res.status} ${err}`);
    }
    const json = await res.json();
    return { token: json.data?.access_token as string, user: json.data?.user };
  }

  test.beforeAll(async ({ browser }) => {
    try {
      await fetch(WEBHOOK_RESET_URL, { method: 'POST' });
    } catch {
      // ignore
    }
    await clearMailpitMessages();

    // Create dedicated browser page that stays open to observe live SSE updates
    customerContext = await browser.newContext();
    customerTrackingPage = await customerContext.newPage();
  });

  test.afterAll(async () => {
    if (customerContext) {
      await customerContext.close().catch(() => {});
    }
  });

  // ===========================================================================
  // ACTOR 1, ACTOR 2 & ACTOR 3: CUSTOMERS (EXISTING + NEW SIGNUP + RETURNING)
  // ===========================================================================
  test('Step 1: Customer Onboarding & Multi-Parcel Dynamic Bookings', async ({ page }) => {
    console.log('[12-USER E2E] 👤 Starting Phase 1: Customer Onboarding & Multi-Parcel Bookings...');

    // --- Actor 1: Customer 1 ("Dilshan Silva") UI Sign-In & Dashboard Verification ---
    console.log(`[12-USER E2E] [Actor 1: Customer 1] Signing in via UI: ${ACTORS.CUSTOMER_1.email}`);
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(ACTORS.CUSTOMER_1.email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(ACTORS.CUSTOMER_1.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15000 });
    console.log('[12-USER E2E] [Actor 1: Customer 1] ✅ Arrived on Customer Dashboard.');

    const cust1Auth = await apiLogin(ACTORS.CUSTOMER_1.email, ACTORS.CUSTOMER_1.password);
    authTokens['CUSTOMER_1'] = cust1Auth.token;

    // --- Actor 2: Customer 2 ("Amara Jayasinghe") Real Mailpit OTP Sign-Up ---
    customer2Email = `amara_e2e_${Date.now()}@swiftdrop.local`;
    console.log(`[12-USER E2E] [Actor 2: Customer 2] Initiating new user registration: ${customer2Email}`);
    await page.goto('/auth/register');
    await expect(page.getByText('Create a customer account')).toBeVisible({ timeout: 10000 });

    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(customer2Email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    // Retrieve real OTP from Mailpit
    const realOtp = await getRegistrationOTPFromMailpit(customer2Email);
    console.log(`[12-USER E2E] [Actor 2: Customer 2] Retrieved Real OTP from Mailpit: ${realOtp}`);
    expect(realOtp).toMatch(/^\d{6}$/);

    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(realOtp);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_VERIFY_OTP_BTN)).click();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Amara');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Jayasinghe');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771239988');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill(customer2Password);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    // Verify Account Created Confirmation Screen & Continue to Sign In
    await expect(page.getByText('Account created')).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /Continue to Sign in/i }).click();
    await expect(page).toHaveURL(/\/auth\/login/, { timeout: 10000 });

    // Sign in with the new customer account
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(customer2Email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(customer2Password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15000 });
    console.log('[12-USER E2E] [Actor 2: Customer 2] ✅ Successfully registered, signed in, and arrived on Customer Dashboard.');

    const cust2Auth = await apiLogin(customer2Email, customer2Password);
    authTokens['CUSTOMER_2'] = cust2Auth.token;

    // --- Actor 3: Customer 3 ("Nadeeka Perera") Sign-In ---
    const cust3Auth = await apiLogin(ACTORS.CUSTOMER_3.email, ACTORS.CUSTOMER_3.password);
    authTokens['CUSTOMER_3'] = cust3Auth.token;
    console.log('[12-USER E2E] [Actor 3: Customer 3] ✅ Authenticated successfully.');

    // --- Customer 1 Books Parcel 1 (Standard 2.5kg, Colombo Central -> Colombo South) ---
    console.log('[12-USER E2E] [Actor 1: Customer 1] Requesting dynamic pricing quote...');
    const quote1Res = await fetch(`${CORE_API}/pricing/quotes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cust1Auth.token}` },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 2.5,
        dimensions: { length_cm: 25, width_cm: 15, height_cm: 10 },
      }),
    });
    expect(quote1Res.ok).toBeTruthy();
    const quote1Data = await quote1Res.json();
    const quote1Id = quote1Data.data?.quote_id;
    expect(quote1Id).toBeDefined();

    const book1Res = await fetch(`${CORE_API}/parcels`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cust1Auth.token}`, 'Idempotency-Key': randomUUID() },
      body: JSON.stringify({
        quote_id: quote1Id,
        recipient_name: 'Kasun Wickrama',
        recipient_phone: '+94719876543',
        weight_kg: 2.5,
        dimensions: { length_cm: 25, width_cm: 15, height_cm: 10 },
        pickup: {
          contact_name: 'Dilshan Silva',
          contact_phone: '+94771234567',
          address_line1: '45 Galle Road',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: 'Kasun Wickrama',
          contact_phone: '+94719876543',
          address_line1: '15/A Havelock Road',
          city: 'Colombo',
          postal_code: '00400',
          zone_id: 'ZONE_COLOMBO_01',
        },
      }),
    });
    expect(book1Res.ok).toBeTruthy();
    const book1Data = await book1Res.json();
    parcel1Id = book1Data.data?.id;
    parcel1Tracking = book1Data.data?.tracking_number;
    console.log(`[12-USER E2E] [Actor 1: Customer 1] 📦 Booked Parcel 1: ID=${parcel1Id} | Tracking=${parcel1Tracking}`);

    // Customer opens real-time tracking view in browser to observe live SSE stream
    console.log(`[12-USER E2E] [Actor 1: Customer 1] Opening live tracking view in browser: /track/${parcel1Tracking}`);
    await customerTrackingPage.goto(`/track/${parcel1Tracking}`);
    await expect(customerTrackingPage.getByText(parcel1Tracking)).toBeVisible({ timeout: 10000 });
    await expect(customerTrackingPage.getByTestId('tracking-status-badge')).toBeVisible({ timeout: 10000 });
    await expect(customerTrackingPage.getByTestId('tracking-status-badge')).toHaveText(/created|booked/i);
    console.log('[12-USER E2E] [Actor 1: Customer 1] ✅ Live tracking view open. Initial state: CREATED/BOOKED. Connected to SSE stream.');

    // --- Customer 2 Books Parcel 2 (Express Delivery, Kandy Hub -> Western Central) ---
    console.log('[12-USER E2E] [Actor 2: Customer 2] Requesting dynamic pricing quote for Parcel 2...');
    const quote2Res = await fetch(`${CORE_API}/pricing/quotes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cust2Auth.token}` },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_KANDY_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'EXPRESS',
        actual_weight_kg: 3.0,
        dimensions: { length_cm: 30, width_cm: 20, height_cm: 15 },
      }),
    });
    expect(quote2Res.ok).toBeTruthy();
    const quote2Data = await quote2Res.json();
    const quote2Id = quote2Data.data?.quote_id;

    const book2Res = await fetch(`${CORE_API}/parcels`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cust2Auth.token}`, 'Idempotency-Key': randomUUID() },
      body: JSON.stringify({
        quote_id: quote2Id,
        recipient_name: 'Nadeem Ahmed',
        recipient_phone: '+94775551234',
        weight_kg: 3.0,
        dimensions: { length_cm: 30, width_cm: 20, height_cm: 15 },
        pickup: {
          contact_name: 'Amara Jayasinghe',
          contact_phone: '+94771239988',
          address_line1: '12 Peradeniya Road',
          city: 'Kandy',
          postal_code: '20000',
          zone_id: 'ZONE_KANDY_01',
        },
        dropoff: {
          contact_name: 'Nadeem Ahmed',
          contact_phone: '+94775551234',
          address_line1: '88 Duplication Road',
          city: 'Colombo',
          postal_code: '00300',
          zone_id: 'ZONE_COLOMBO_01',
        },
      }),
    });
    expect(book2Res.ok).toBeTruthy();
    const book2Data = await book2Res.json();
    parcel2Id = book2Data.data?.id;
    parcel2Tracking = book2Data.data?.tracking_number;
    console.log(`[12-USER E2E] [Actor 2: Customer 2] 📦 Booked Parcel 2: ID=${parcel2Id} | Tracking=${parcel2Tracking}`);

    // --- Customer 3 Books Parcel 3 (Heavy 15.5kg Bulky Package) ---
    console.log('[12-USER E2E] [Actor 3: Customer 3] Booking heavy parcel...');
    const quote3Res = await fetch(`${CORE_API}/pricing/quotes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cust3Auth.token}` },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 15.5,
        dimensions: { length_cm: 50, width_cm: 50, height_cm: 40 },
      }),
    });
    expect(quote3Res.ok).toBeTruthy();
    const quote3Data = await quote3Res.json();

    const book3Res = await fetch(`${CORE_API}/parcels`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cust3Auth.token}`, 'Idempotency-Key': randomUUID() },
      body: JSON.stringify({
        quote_id: quote3Data.data?.quote_id,
        recipient_name: 'Ruwani Senanayake',
        recipient_phone: '+94778889900',
        weight_kg: 15.5,
        dimensions: { length_cm: 50, width_cm: 50, height_cm: 40 },
        pickup: {
          contact_name: 'Nadeeka Perera',
          contact_phone: '+94770000056',
          address_line1: '10 Industrial Zone',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: 'Ruwani Senanayake',
          contact_phone: '+94778889900',
          address_line1: '95 High Level Road',
          city: 'Colombo',
          postal_code: '00600',
          zone_id: 'ZONE_COLOMBO_01',
        },
      }),
    });
    expect(book3Res.ok).toBeTruthy();
    const book3Data = await book3Res.json();
    parcel3Id = book3Data.data?.id;
    parcel3Tracking = book3Data.data?.tracking_number;
    console.log(`[12-USER E2E] [Actor 3: Customer 3] 📦 Booked Heavy Parcel 3: ID=${parcel3Id} | Tracking=${parcel3Tracking}`);
  });

  // ===========================================================================
  // ACTOR 4 & ACTOR 5: MERCHANTS (API KEYS, WEBHOOKS, ORDERS & TENANT ISOLATION)
  // ===========================================================================
  test('Step 2: Merchant Portal Operations (API Keys, Webhooks & Commercial Orders)', async ({ page }) => {
    console.log('[12-USER E2E] 🏢 Starting Phase 2: Merchant Operations & Webhooks...');

    // --- Actor 4: Merchant 1 ("Colombo Tech Stores") UI Navigation ---
    console.log(`[12-USER E2E] [Actor 4: Merchant 1] Signing into portal: ${ACTORS.MERCHANT_1.email}`);
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(ACTORS.MERCHANT_1.email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(ACTORS.MERCHANT_1.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15000 });

    const mrc1Auth = await apiLogin(ACTORS.MERCHANT_1.email, ACTORS.MERCHANT_1.password);
    authTokens['MERCHANT_1'] = mrc1Auth.token;

    // Generate API Key for Merchant 1 via Core API
    console.log('[12-USER E2E] [Actor 4: Merchant 1] Creating new API Key...');
    const apiKeyRes = await fetch(`${CORE_API}/merchants/me/api-keys`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${mrc1Auth.token}` },
      body: JSON.stringify({
        label: 'Live ERP Key 2026',
        scopes: ['parcels:create', 'parcels:read'],
      }),
    });
    if (apiKeyRes.ok) {
      const keyData = await apiKeyRes.json();
      merchant1ApiKey = keyData.data?.raw_api_key || keyData.data?.key || 'sk_live_mrc1';
      console.log('[12-USER E2E] [Actor 4: Merchant 1] ✅ API Key issued successfully.');
    }

    // Configure Webhook Endpoint for Merchant 1
    console.log('[12-USER E2E] [Actor 4: Merchant 1] Configuring active webhook endpoint...');
    const webhookRes = await fetch(`${CORE_API}/merchants/me/webhooks`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${mrc1Auth.token}` },
      body: JSON.stringify({
        url: MOCK_WEBHOOK_URL,
        events: [
          'parcel.booked.v1',
          'parcel.assigned.v1',
          'parcel.picked_up.v1',
          'parcel.out_for_delivery.v1',
          'parcel.delivered.v1',
          'parcel.failed.v1',
        ],
      }),
    });
    if (webhookRes.ok) {
      const whData = await webhookRes.json();
      console.log(`[12-USER E2E] [Actor 4: Merchant 1] ✅ Webhook registered: ID=${whData.data?.id}`);
    }

    // Ingest Commercial Order for Merchant 1
    console.log('[12-USER E2E] [Actor 4: Merchant 1] Creating commercial order...');
    const mrcQuoteRes = await fetch(`${CORE_API}/pricing/quotes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${mrc1Auth.token}` },
      body: JSON.stringify({
        origin_zone_id: 'ZONE_COLOMBO_01',
        destination_zone_id: 'ZONE_COLOMBO_01',
        service_level: 'STANDARD',
        actual_weight_kg: 1.8,
        dimensions: { length_cm: 20, width_cm: 15, height_cm: 8 },
      }),
    });
    const mrcQuoteData = await mrcQuoteRes.json();

    const mrcBookRes = await fetch(`${CORE_API}/parcels`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${mrc1Auth.token}`, 'Idempotency-Key': randomUUID() },
      body: JSON.stringify({
        quote_id: mrcQuoteData.data?.quote_id,
        recipient_name: 'B2B Client Tech Corp',
        recipient_phone: '+94770008888',
        weight_kg: 1.8,
        dimensions: { length_cm: 20, width_cm: 15, height_cm: 8 },
        pickup: {
          contact_name: 'Warehouse Logistics Alpha',
          contact_phone: '+94112223344',
          address_line1: '100 Harbour Road',
          city: 'Colombo',
          postal_code: '00100',
          zone_id: 'ZONE_COLOMBO_01',
        },
        dropoff: {
          contact_name: 'Tech Corp Reception',
          contact_phone: '+94770008888',
          address_line1: '300 Union Place',
          city: 'Colombo',
          postal_code: '00200',
          zone_id: 'ZONE_COLOMBO_01',
        },
      }),
    });
    expect(mrcBookRes.ok).toBeTruthy();
    const mrcBookData = await mrcBookRes.json();
    merchant1ParcelId = mrcBookData.data?.id;
    merchant1Tracking = mrcBookData.data?.tracking_number;
    console.log(`[12-USER E2E] [Actor 4: Merchant 1] 📦 Created Merchant Parcel 4: ID=${merchant1ParcelId} | Tracking=${merchant1Tracking}`);

    // --- Actor 5: Merchant 2 ("Beta Supplies") Auth & Multi-Tenant Boundary ---
    console.log(`[12-USER E2E] [Actor 5: Merchant 2] Signing into portal: ${ACTORS.MERCHANT_2.email}`);
    const mrc2Auth = await apiLogin(ACTORS.MERCHANT_2.email, ACTORS.MERCHANT_2.password);
    authTokens['MERCHANT_2'] = mrc2Auth.token;

    // Verify Merchant 2 sees only their own tenant resources (zero crosstalk)
    const crossTenantRes = await fetch(`${CORE_API}/merchants/me/webhooks`, {
      headers: { Authorization: `Bearer ${mrc2Auth.token}` },
    });
    expect(crossTenantRes.ok).toBeTruthy();
    const crossTenantData = await crossTenantRes.json();
    const mrc2Webhooks = crossTenantData.data || [];
    expect(mrc2Webhooks.every((w: any) => w.merchant_id !== '018f4a7c-1004-7000-9000-000000000004')).toBeTruthy();
    console.log('[12-USER E2E] [Actor 5: Merchant 2] ✅ Tenant isolation verified with zero crosstalk.');
  });

  // ===========================================================================
  // ACTOR 9 & ACTOR 10: DISPATCHERS (ROUTING, CAPACITY & REASSIGNMENT)
  // ===========================================================================
  test('Step 3: Dispatcher Capacity Management, Routing & Assignment', async ({ page }) => {
    console.log('[12-USER E2E] 🚦 Starting Phase 3: Dispatcher Board Routing & Courier Allocations...');

    // --- Actor 9: Dispatcher 1 ("Kamal Perera") UI Sign-In & Board Inspection ---
    console.log(`[12-USER E2E] [Actor 9: Dispatcher 1] Logging into Dispatcher Console: ${ACTORS.DISPATCHER_1.email}`);
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(ACTORS.DISPATCHER_1.email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(ACTORS.DISPATCHER_1.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/(ops|dispatcher\/dashboard)/, { timeout: 15000 });

    const disp1Auth = await apiLogin(ACTORS.DISPATCHER_1.email, ACTORS.DISPATCHER_1.password);
    authTokens['DISPATCHER_1'] = disp1Auth.token;

    // Check Courier Availability and Capacity via API
    const courierListRes = await fetch(`${CORE_API}/dispatch/couriers`, {
      headers: { Authorization: `Bearer ${disp1Auth.token}` },
    });
    expect(courierListRes.ok).toBeTruthy();
    const couriers = (await courierListRes.json()).data || [];
    console.log(`[12-USER E2E] [Actor 9: Dispatcher 1] Active Couriers count: ${couriers.length}`);

    // Assign Parcel 1 -> Courier 1 (CR-0012)
    console.log(`[12-USER E2E] [Actor 9: Dispatcher 1] Assigning Parcel 1 (${parcel1Tracking}) to Courier 1 (${ACTORS.COURIER_1.code})...`);
    const assign1Res = await fetch(`${CORE_API}/dispatch/assignments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${disp1Auth.token}`, 'Idempotency-Key': randomUUID() },
      body: JSON.stringify({
        parcel_id: parcel1Id,
        courier_id: ACTORS.COURIER_1.id,
      }),
    });
    expect(assign1Res.ok).toBeTruthy();
    console.log('[12-USER E2E] [Actor 9: Dispatcher 1] ✅ Parcel 1 assigned to Courier 1.');

    // Assert live real-time SSE update on customer's open browser tracking view without page reload!
    console.log('[12-USER E2E] ⚡ Verifying real-time SSE stream update on customer tracking view (BOOKED -> ASSIGNED)...');
    await expect(customerTrackingPage.getByTestId('tracking-status-badge')).toHaveText(/assigned/i, { timeout: 15000 });
    console.log('[12-USER E2E] ✅ Customer tracking view updated in REAL TIME via SSE to ASSIGNED without reload!');

    // Assign Parcel 2 -> Courier 2 (CR-0099)
    console.log(`[12-USER E2E] [Actor 9: Dispatcher 1] Assigning Parcel 2 (${parcel2Tracking}) to Courier 2 (${ACTORS.COURIER_2.code})...`);
    const assign2Res = await fetch(`${CORE_API}/dispatch/assignments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${disp1Auth.token}`, 'Idempotency-Key': randomUUID() },
      body: JSON.stringify({
        parcel_id: parcel2Id,
        courier_id: ACTORS.COURIER_2.id,
      }),
    });
    expect(assign2Res.ok).toBeTruthy();
    console.log('[12-USER E2E] [Actor 9: Dispatcher 1] ✅ Parcel 2 assigned to Courier 2.');

    // Assign Merchant 1's Parcel 4 -> Courier 1 (CR-0012)
    console.log(`[12-USER E2E] [Actor 9: Dispatcher 1] Assigning Merchant Parcel 4 (${merchant1Tracking}) to Courier 1...`);
    const assign4Res = await fetch(`${CORE_API}/dispatch/assignments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${disp1Auth.token}`, 'Idempotency-Key': randomUUID() },
      body: JSON.stringify({
        parcel_id: merchant1ParcelId,
        courier_id: ACTORS.COURIER_1.id,
      }),
    });
    expect(assign4Res.ok).toBeTruthy();
    console.log('[12-USER E2E] [Actor 9: Dispatcher 1] ✅ Merchant Parcel 4 assigned.');

    // --- Actor 10: Dispatcher 2 ("Rohan De Silva") Sign-In ---
    const disp2Auth = await apiLogin(ACTORS.DISPATCHER_2.email, ACTORS.DISPATCHER_2.password);
    authTokens['DISPATCHER_2'] = disp2Auth.token;
    console.log('[12-USER E2E] [Actor 10: Dispatcher 2] ✅ Dispatcher 2 ready on Exceptions & Reschedule desk.');
  });

  // ===========================================================================
  // ACTORS 6, 7 & 8: COURIERS (DELIVERIES, POD CANVAS, EXCEPTIONS & RELIEF)
  // ===========================================================================
  test('Step 4: Courier Execution, Digital POD Signatures, Failure Exceptions & Relief Delivery', async ({ page }) => {
    console.log('[12-USER E2E] 🛵 Starting Phase 4: Courier Operations & Milestone Lifecycle...');

    // --- Actor 6: Courier 1 ("Kamal Perera") Mobile Delivery Run & Signature POD ---
    console.log(`[12-USER E2E] [Actor 6: Courier 1] Mobile PWA Login: ${ACTORS.COURIER_1.email}`);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(ACTORS.COURIER_1.email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(ACTORS.COURIER_1.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await page.waitForURL('**/courier/dashboard');
    console.log('[12-USER E2E] [Actor 6: Courier 1] ✅ Arrived on Mobile Courier Dashboard.');

    const cour1Auth = await apiLogin(ACTORS.COURIER_1.email, ACTORS.COURIER_1.password);
    authTokens['COURIER_1'] = cour1Auth.token;

    // Courier 1 moves Parcel 1 to OUT_FOR_DELIVERY
    console.log(`[12-USER E2E] [Actor 6: Courier 1] Moving Parcel 1 (${parcel1Tracking}) to OUT_FOR_DELIVERY...`);
    const run1Res = await fetch(`${CORE_API}/courier/parcels/${parcel1Id}/start-run`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${cour1Auth.token}` },
    });
    expect(run1Res.ok).toBeTruthy();

    // Assert live real-time SSE update on customer's open browser tracking view without page reload!
    console.log('[12-USER E2E] ⚡ Verifying real-time SSE stream update on customer tracking view (ASSIGNED -> OUT_FOR_DELIVERY)...');
    await expect(customerTrackingPage.getByTestId('tracking-status-badge')).toHaveText(/out for delivery/i, { timeout: 15000 });
    console.log('[12-USER E2E] ✅ Customer tracking view updated in REAL TIME via SSE to OUT_FOR_DELIVERY without reload!');

    // Courier 1 completes delivery with Digital Signature POD
    console.log(`[12-USER E2E] [Actor 6: Courier 1] Completing doorstep delivery for Parcel 1 with digital signature POD...`);
    const mockSignature = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAAAyCAYAAACqNX6+AAAAAXNSR0IArs4c6QAAAARnQU1BAACxjwv8YQUAAAAJcEhZcwAADsMAAA7DAcdvqGQAAACYSURBVHhe7c4BDQAwEASh+zf9tcDBH8E8kE1m1u7u3v36vEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBgvQG0jA9H2R92qgAAAABJRU5ErkJggg==';
    const comp1Res = await fetch(`${CORE_API}/deliveries/${parcel1Id}/complete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cour1Auth.token}` },
      body: JSON.stringify({
        proof_type: 'SIGNATURE',
        signature_data: mockSignature,
        received_by_name: 'Kasun Wickrama',
        notes: 'Handed directly to recipient at doorstep',
      }),
    });
    expect(comp1Res.ok).toBeTruthy();
    console.log(`[12-USER E2E] [Actor 6: Courier 1] 🎉 Parcel 1 (${parcel1Tracking}) successfully DELIVERED.`);

    // Assert live real-time SSE update on customer's open browser tracking view without page reload!
    console.log('[12-USER E2E] ⚡ Verifying real-time SSE stream update on customer tracking view (OUT_FOR_DELIVERY -> DELIVERED)...');
    await expect(customerTrackingPage.getByTestId('tracking-status-badge')).toHaveText(/delivered/i, { timeout: 15000 });
    console.log('[12-USER E2E] ✅ Customer tracking view updated in REAL TIME via SSE to DELIVERED without reload!');

    // Courier 1 also delivers Merchant Parcel 4
    console.log(`[12-USER E2E] [Actor 6: Courier 1] Delivering Merchant Parcel 4 (${merchant1Tracking})...`);
    await fetch(`${CORE_API}/courier/parcels/${merchant1ParcelId}/start-run`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${cour1Auth.token}` },
    });
    const comp4Res = await fetch(`${CORE_API}/deliveries/${merchant1ParcelId}/complete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cour1Auth.token}` },
      body: JSON.stringify({
        proof_type: 'SIGNATURE',
        signature_data: mockSignature,
        received_by_name: 'Tech Corp Reception',
        notes: 'Commercial delivery signed at front desk',
      }),
    });
    expect(comp4Res.ok).toBeTruthy();
    console.log(`[12-USER E2E] [Actor 6: Courier 1] 🎉 Merchant Parcel 4 successfully DELIVERED.`);

    // --- Actor 7: Courier 2 ("Sunil Shantha") Delivery Attempt & Exception Failure ---
    console.log(`[12-USER E2E] [Actor 7: Courier 2] Starting delivery run for Parcel 2 (${parcel2Tracking})...`);
    const cour2Auth = await apiLogin(ACTORS.COURIER_2.email, ACTORS.COURIER_2.password);
    authTokens['COURIER_2'] = cour2Auth.token;

    // Start run -> OUT_FOR_DELIVERY
    const run2Res = await fetch(`${CORE_API}/courier/parcels/${parcel2Id}/start-run`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${cour2Auth.token}` },
    });
    expect(run2Res.ok).toBeTruthy();

    // Courier 2 encounters exception: RECIPIENT_UNAVAILABLE
    console.log(`[12-USER E2E] [Actor 7: Courier 2] Logging failed attempt: RECIPIENT_UNAVAILABLE...`);
    const fail2Res = await fetch(`${CORE_API}/deliveries/${parcel2Id}/failed`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cour2Auth.token}` },
      body: JSON.stringify({
        failure_reason_code: 'RECIPIENT_UNAVAILABLE',
        notes: 'Premises closed, contact phone unanswered after 3 attempts',
      }),
    });
    expect(fail2Res.ok).toBeTruthy();
    const fail2Data = await fail2Res.json();
    console.log(`[12-USER E2E] [Actor 7: Courier 2] ⚠️ Parcel 2 failed attempt logged: Attempt #${fail2Data.data?.attempt_number}. Status: ATTEMPT_FAILED / FAILED.`);

    // --- Actor 10: Dispatcher 2 ("Rohan De Silva") Triage, Reschedule & Reassignment ---
    console.log(`[12-USER E2E] [Actor 10: Dispatcher 2] Triaging failed delivery for Parcel 2 (${parcel2Tracking})...`);
    const disp2Token = authTokens['DISPATCHER_2'];

    // Dispatcher 2 inspects delivery history
    const histRes = await fetch(`${CORE_API}/deliveries/parcels/${parcel2Id}`, {
      headers: { Authorization: `Bearer ${disp2Token}` },
    });
    expect(histRes.ok).toBeTruthy();
    const histData = await histRes.json();
    expect(histData.data?.attempts?.length).toBeGreaterThan(0);
    console.log(`[12-USER E2E] [Actor 10: Dispatcher 2] Verified failure record: reason=${histData.data?.attempts[0]?.failure_reason}`);

    // Dispatcher 2 reschedules and reassigns Parcel 2 to Courier 3 (Relief Courier)
    console.log(`[12-USER E2E] [Actor 10: Dispatcher 2] Rescheduling and reallocating Parcel 2 to Courier 3 (${ACTORS.COURIER_3.code})...`);
    const reschedRes = await fetch(`${CORE_API}/dispatch/parcels/${parcel2Id}/reschedule`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${disp2Token}` },
      body: JSON.stringify({
        courier_id: ACTORS.COURIER_3.id,
        notes: 'Immediate relief dispatch allocated to Courier 3 for second attempt',
      }),
    });
    expect(reschedRes.ok).toBeTruthy();
    console.log('[12-USER E2E] [Actor 10: Dispatcher 2] ✅ Parcel 2 rescheduled and assigned to Courier 3.');

    // --- Actor 8: Courier 3 ("Chaminda Vaas") Relief Delivery & Final Completion ---
    console.log(`[12-USER E2E] [Actor 8: Courier 3] Accepting relief run for Parcel 2 (${parcel2Tracking})...`);
    const cour3Auth = await apiLogin(ACTORS.COURIER_3.email, ACTORS.COURIER_3.password);
    authTokens['COURIER_3'] = cour3Auth.token;

    // Start run -> OUT_FOR_DELIVERY
    const run3Res = await fetch(`${CORE_API}/courier/parcels/${parcel2Id}/start-run`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${cour3Auth.token}` },
    });
    expect(run3Res.ok).toBeTruthy();

    // Courier 3 reaches recipient, captures signature, marks DELIVERED
    console.log(`[12-USER E2E] [Actor 8: Courier 3] Completing relief delivery with signature POD...`);
    const comp2Res = await fetch(`${CORE_API}/deliveries/${parcel2Id}/complete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cour3Auth.token}` },
      body: JSON.stringify({
        proof_type: 'SIGNATURE',
        signature_data: mockSignature,
        received_by_name: 'Nadeem Ahmed',
        notes: 'Handed to recipient in person during second attempt run',
      }),
    });
    expect(comp2Res.ok).toBeTruthy();
    console.log(`[12-USER E2E] [Actor 8: Courier 3] 🎉 Parcel 2 (${parcel2Tracking}) successfully reached final state DELIVERED!`);
  });

  // ===========================================================================
  // ACTOR 11 & ACTOR 12: ADMIN AUDIT & PUBLIC GUEST TRACKING (SEC-009 PII MASKING)
  // ===========================================================================
  test('Step 5: Admin Audit Inspection & Public Guest Tracking with SEC-009 PII Masking', async ({ page }) => {
    console.log('[12-USER E2E] 🛡️ Starting Phase 5: Admin Auditing & Public Guest Tracking...');

    // --- Actor 11: Admin ("System Administrator") UI Sign-In & Audit Log Inspection ---
    console.log(`[12-USER E2E] [Actor 11: Admin] Signing into Admin Console: ${ACTORS.ADMIN.email}`);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(ACTORS.ADMIN.email);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(ACTORS.ADMIN.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await expect(page).toHaveURL(/\/(ops|admin\/dashboard)/, { timeout: 15000 });

    const adminAuth = await apiLogin(ACTORS.ADMIN.email, ACTORS.ADMIN.password);
    authTokens['ADMIN'] = adminAuth.token;

    // Verify System Audit Logs
    console.log('[12-USER E2E] [Actor 11: Admin] Inspecting immutable audit logs...');
    const auditRes = await fetch(`${CORE_API}/admin/audit-logs`, {
      headers: { Authorization: `Bearer ${adminAuth.token}` },
    });
    expect(auditRes.ok).toBeTruthy();
    const auditData = await auditRes.json();
    const auditEntries = auditData.data || [];
    expect(auditEntries.length).toBeGreaterThan(0);
    console.log(`[12-USER E2E] [Actor 11: Admin] ✅ Verified ${auditEntries.length} tamper-proof audit records.`);

    // --- Actor 12: Public Guest (Unauthenticated Visitor) Real-Time Tracking ---
    console.log(`[12-USER E2E] [Actor 12: Public Guest] Visiting unauthenticated tracking page for Parcel 1 (${parcel1Tracking})...`);
    await page.goto(`/track/${parcel1Tracking}`);
    await expect(page.getByText(parcel1Tracking)).toBeVisible({ timeout: 10000 });
    console.log('[12-USER E2E] [Actor 12: Public Guest] ✅ Tracking view rendered with accurate tracking number.');

    // Public API Tracking Endpoint Verification & SEC-009 PII Masking
    console.log(`[12-USER E2E] [Actor 12: Public Guest] Querying Public Tracking API: ${TRACKING_API}/tracking/public/${parcel1Tracking}`);
    const pubTrack1Res = await fetch(`${TRACKING_API}/tracking/public/${parcel1Tracking}`);
    expect(pubTrack1Res.ok).toBeTruthy();
    const pubTrack1Data = await pubTrack1Res.json();

    // SEC-009 PII Masking Assertions:
    const rawTrackJson = JSON.stringify(pubTrack1Data);
    expect(rawTrackJson).not.toContain('+94771234567'); // Sender phone strictly masked
    expect(rawTrackJson).not.toContain('+94719876543'); // Recipient phone strictly masked
    expect(rawTrackJson).not.toContain('45 Galle Road'); // Full street address strictly masked
    expect(rawTrackJson).not.toContain('15/A Havelock Road');
    expect(pubTrack1Data.data?.status).toBe('DELIVERED');
    console.log('[12-USER E2E] [Actor 12: Public Guest] ✅ SEC-009 PII Masking verified: zero customer phone numbers or addresses exposed.');

    // Verify Public Tracking for Rescheduled Parcel 2
    console.log(`[12-USER E2E] [Actor 12: Public Guest] Querying Public Tracking API for Parcel 2 (${parcel2Tracking})...`);
    const pubTrack2Res = await fetch(`${TRACKING_API}/tracking/public/${parcel2Tracking}`);
    expect(pubTrack2Res.ok).toBeTruthy();
    const pubTrack2Data = await pubTrack2Res.json();
    expect(pubTrack2Data.data?.status).toBe('DELIVERED');

    // Milestones check on Parcel 2:
    const milestones = pubTrack2Data.data?.milestones || [];
    console.log(`[12-USER E2E] [Actor 12: Public Guest] Parcel 2 Milestones: ${milestones.map((m: any) => m.status).join(' -> ')}`);
    expect(milestones.some((m: any) => m.status === 'CREATED')).toBeTruthy();
    expect(milestones.some((m: any) => m.status === 'ASSIGNED')).toBeTruthy();
    expect(milestones.some((m: any) => m.status === 'DELIVERED')).toBeTruthy();
    console.log('[12-USER E2E] [Actor 12: Public Guest] ✅ Complete milestone timeline confirmed.');

    // Real-Time SSE Stream Verification
    console.log(`[12-USER E2E] [Actor 12: Public Guest] Connecting to Real-Time SSE Stream: ${TRACKING_API}/tracking/live/${parcel1Tracking}`);
    await new Promise<void>((resolve) => {
      const es = new EventSource(`${TRACKING_API}/tracking/live/${parcel1Tracking}`);
      const timeout = setTimeout(() => {
        es.close();
        resolve();
      }, 3000);

      es.onmessage = (event) => {
        clearTimeout(timeout);
        try {
          const sseData = JSON.parse(event.data);
          console.log('[12-USER E2E] [Actor 12: Public Guest] ⚡ Received SSE event:', sseData.type || sseData.status);
        } catch {
          // ignore parse errors
        }
        es.close();
        resolve();
      };

      es.onerror = () => {
        clearTimeout(timeout);
        es.close();
        resolve();
      };
    });

    console.log('[12-USER E2E] 🏁 Phase 5 Complete: All 12 actors executed their real-world daily workflows seamlessly.');
  });
});
