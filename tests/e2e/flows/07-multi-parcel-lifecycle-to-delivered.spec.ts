import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';
import { getRegistrationOTPFromMailpit, clearMailpitMessages } from '../helpers/mailpitHelper';
import { randomUUID } from 'node:crypto';

/**
 * FLOW 7: MULTI-PARCEL REAL LIFECYCLE (REGISTRATION -> BOOK 6 PARCELS -> DELIVERED)
 *
 * Real end-to-end multi-actor system test with ZERO MOCKS:
 * 1. Creates a brand new test customer account via /auth/register using real Mailpit email OTP.
 * 2. Customer books 6 distinct parcels (> 5) for delivery through Core API pricing quotes & booking engine.
 * 3. Dispatcher assigns all 6 parcels to the active on-shift courier.
 * 4. Courier accepts the run and moves all 6 parcels to OUT_FOR_DELIVERY.
 * 5. Courier completes delivery on all 6 parcels with cryptographic Electronic Proof of Delivery (POD).
 * 6. Customer waits and verifies that all 6 parcels reach status DELIVERED on the authoritative system.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 1 (Identity), Member 2 (Parcels & Pricing), Member 3 (Dispatch & Delivery)
 */

test.describe('Flow 7: Multi-Parcel Real Lifecycle (Zero Mocks)', () => {
  const timestamp = Date.now();
  const customerEmail = `multibook_${timestamp}@swiftdrop.local`;
  const customerPassword = 'Password123!';
  const firstName = 'Nisal';
  const lastName = 'Perera';
  const phone = '+94771234567';

  // System credentials
  const dispatcherEmail = 'dispatcher@swiftdrop.local';
  const dispatcherPassword = 'Dispatcher123!';
  const courierEmail = 'courier@swiftdrop.local';
  const courierPassword = 'CourierPassword123!';
  const courierUserId = '018f4a7c-1003-7000-8000-000000000003'; // CR-0012

  const bookedParcels: Array<{ id: string; trackingNumber: string; index: number }> = [];

  test('should register test user, book 6 parcels, and complete full delivery lifecycle to DELIVERED', async ({
    page,
    request,
  }) => {
    // -------------------------------------------------------------------------
    // STEP 1: Create Test User Account with Real Mailpit OTP
    // -------------------------------------------------------------------------
    console.log(`[Flow 7] 1. Registering new test user: ${customerEmail}`);
    await page.goto('/auth/register');
    await expect(page.getByText('Create a customer account')).toBeVisible({ timeout: 10000 });

    // Step 1.1: Request Email Verification Code
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(customerEmail);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    // Step 1.2: Retrieve Real OTP from Mailpit
    const realOtp = await getRegistrationOTPFromMailpit(customerEmail);
    console.log(`[Flow 7] Retrieved Real OTP from Mailpit: ${realOtp}`);
    expect(realOtp).toMatch(/^\d{6}$/);

    // Step 1.3: Fill Registration Form & Accept Terms
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(realOtp);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_VERIFY_OTP_BTN)).click();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill(firstName);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill(lastName);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill(phone);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill(customerPassword);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    await expect(page.getByText('Account created')).toBeVisible({ timeout: 10000 });
    console.log('[Flow 7] Customer account created successfully in PostgreSQL.');

    // Step 1.4: Log In with New Account to obtain authenticated session
    await page.getByRole('button', { name: /Continue to Sign in/i }).click();
    await expect(page).toHaveURL(/\/auth\/login/, { timeout: 10000 });

    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(customerEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(customerPassword);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10000 });
    await expect(page.getByRole('button', { name: /Sign out/i })).toBeVisible({ timeout: 10000 });
    console.log('[Flow 7] Customer authenticated and logged into portal.');

    // Obtain Customer JWT from Core API
    const custLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: customerEmail, password: customerPassword },
    });
    expect(custLoginRes.ok()).toBeTruthy();
    const custLoginData = await custLoginRes.json();
    const customerToken = custLoginData.data.access_token;
    expect(customerToken).toBeTruthy();

    // -------------------------------------------------------------------------
    // STEP 2: Book 6 Parcels (> 5) for Delivery
    // -------------------------------------------------------------------------
    console.log('[Flow 7] 2. Booking 6 parcels for delivery through Core API...');

    for (let i = 1; i <= 6; i++) {
      // 2.1: Request dynamic pricing quote
      const quoteRes = await request.post('http://localhost:8080/api/v1/pricing/quotes', {
        headers: {
          Authorization: `Bearer ${customerToken}`,
          'Content-Type': 'application/json',
        },
        data: {
          origin_zone_id: 'ZONE_COLOMBO_01',
          destination_zone_id: 'ZONE_COLOMBO_01',
          actual_weight_kg: 1.5 + i * 0.2,
          dimensions: {
            length_cm: 20.0 + i,
            width_cm: 15.0,
            height_cm: 10.0,
          },
          service_level: 'STANDARD',
        },
      });

      expect(quoteRes.status(), `Quote creation failed for parcel #${i}`).toBe(201);
      const quoteData = await quoteRes.json();
      const quoteId = quoteData.data.quote_id;

      // 2.2: Commit parcel booking with unique Idempotency-Key
      const bookingRes = await request.post('http://localhost:8080/api/v1/parcels', {
        headers: {
          Authorization: `Bearer ${customerToken}`,
          'Content-Type': 'application/json',
          'Idempotency-Key': randomUUID(),
        },
        data: {
          quote_id: quoteId,
          recipient_name: `Recipient ${i} Perera`,
          recipient_phone: `+947198765${i.toString().padStart(2, '0')}`,
          weight_kg: 1.5 + i * 0.2,
          dimensions: {
            length_cm: 20.0 + i,
            width_cm: 15.0,
            height_cm: 10.0,
          },
          pickup: {
            contact_name: `${firstName} ${lastName}`,
            contact_phone: phone,
            address_line1: '45 Galle Road, Suite 3B',
            city: 'Colombo',
            postal_code: '00100',
            zone_id: 'ZONE_COLOMBO_01',
            instructions: `Pickup package ${i} at reception`,
          },
          dropoff: {
            contact_name: `Recipient ${i} Perera`,
            contact_phone: `+947198765${i.toString().padStart(2, '0')}`,
            address_line1: `${i * 12} Duplication Road`,
            city: 'Colombo',
            postal_code: '00400',
            zone_id: 'ZONE_COLOMBO_01',
            instructions: `Hand to Recipient ${i}`,
          },
          item_description: `Urgent Delivery Package #${i}`,
          special_instructions: 'Handle with care',
        },
      });

      expect(bookingRes.status(), `Booking failed for parcel #${i}`).toBe(201);
      const bookingData = await bookingRes.json();
      const parcelId = bookingData.data.id;
      const trackingNumber = bookingData.data.tracking_number;

      bookedParcels.push({ id: parcelId, trackingNumber, index: i });
      console.log(`[Flow 7] -> Booked Parcel [${i}/6]: ID: ${parcelId} | Tracking No: ${trackingNumber}`);
    }

    expect(bookedParcels.length).toBe(6);

    // -------------------------------------------------------------------------
    // STEP 3: Dispatcher Assigns All 6 Parcels to On-Shift Courier
    // -------------------------------------------------------------------------
    console.log('[Flow 7] 3. Dispatcher authenticating and assigning parcels to Courier CR-0012...');
    const dispLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: dispatcherEmail, password: dispatcherPassword },
    });
    expect(dispLoginRes.ok()).toBeTruthy();
    const dispToken = (await dispLoginRes.json()).data.access_token;

    for (const p of bookedParcels) {
      const assignRes = await request.post('http://localhost:8080/api/v1/dispatch/assignments', {
        headers: {
          Authorization: `Bearer ${dispToken}`,
          'Content-Type': 'application/json',
        },
        data: {
          parcel_id: p.id,
          courier_id: courierUserId,
          notes: `Batch morning assignment for package #${p.index}`,
        },
      });

      expect(assignRes.status(), `Assignment failed for parcel #${p.index}`).toBe(201);
      console.log(`[Flow 7] -> Assigned Parcel [${p.index}/6] to Courier CR-0012`);
    }

    // -------------------------------------------------------------------------
    // STEP 4: Courier Transitions Parcels to OUT_FOR_DELIVERY
    // -------------------------------------------------------------------------
    console.log('[Flow 7] 4. Courier authenticating and starting delivery runs...');
    const courLoginRes = await request.post('http://localhost:8080/api/v1/auth/login', {
      data: { email: courierEmail, password: courierPassword },
    });
    expect(courLoginRes.ok()).toBeTruthy();
    const courToken = (await courLoginRes.json()).data.access_token;

    for (const p of bookedParcels) {
      const startRes = await request.post(`http://localhost:8080/api/v1/courier/parcels/${p.id}/start-run`, {
        headers: {
          Authorization: `Bearer ${courToken}`,
        },
      });

      expect(startRes.ok(), `Start-run failed for parcel #${p.index}`).toBeTruthy();
      console.log(`[Flow 7] -> Parcel [${p.index}/6] transitioned to OUT_FOR_DELIVERY`);
    }

    // -------------------------------------------------------------------------
    // STEP 5: Courier Completes Delivery with Proof of Delivery (POD)
    // -------------------------------------------------------------------------
    console.log('[Flow 7] 5. Courier completing deliveries with electronic signature proof...');
    const dummySignature =
      'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDAiIGhlaWdodD0iMTAwIj48cGF0aCBkPSJNMTAsMTAgTDIwLDIwIiBzdHJva2U9ImJsYWNrIi8+PC9zdmc+';

    for (const p of bookedParcels) {
      const completeRes = await request.post(`http://localhost:8080/api/v1/deliveries/${p.id}/complete`, {
        headers: {
          Authorization: `Bearer ${courToken}`,
          'Content-Type': 'application/json',
        },
        data: {
          proof_type: 'SIGNATURE',
          signature_data: dummySignature,
          received_by_name: `Recipient ${p.index} Perera`,
          notes: `Handed over securely to recipient ${p.index}`,
        },
      });

      expect(completeRes.ok(), `Completion failed for parcel #${p.index}`).toBeTruthy();
      console.log(`[Flow 7] -> Parcel [${p.index}/6] transitioned to DELIVERED with POD`);
    }

    // -------------------------------------------------------------------------
    // STEP 6: Customer Verifies All 6 Parcels Are DELIVERED on Real System
    // -------------------------------------------------------------------------
    console.log('[Flow 7] 6. Verifying all 6 parcels reached DELIVERED status...');

    for (const p of bookedParcels) {
      // Verify via Core API parcel detail endpoint
      const parcelStatusRes = await request.get(`http://localhost:8080/api/v1/parcels/${p.id}`, {
        headers: {
          Authorization: `Bearer ${customerToken}`,
        },
      });

      expect(parcelStatusRes.ok()).toBeTruthy();
      const statusData = await parcelStatusRes.json();
      expect(statusData.data.status).toBe('DELIVERED');
      console.log(`[Flow 7] ✅ Verified: Parcel [${p.index}/6] (${p.trackingNumber}) is DELIVERED`);
    }

    // Step 6.2: Visually verify on Customer Portal Dashboard
    console.log('[Flow 7] 6.2: Navigating to Customer Portal Dashboard to visually verify recent parcels...');
    await page.goto('/dashboard');
    await expect(page.getByText('Recent parcels')).toBeVisible({ timeout: 10000 });

    // Verify summary metric cards
    await expect(page.getByText('Parcels on latest page')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Delivered', { exact: true }).first()).toBeVisible({ timeout: 10000 });

    // Verify that every single booked parcel appears in the table with its tracking number and DELIVERED badge
    for (const p of bookedParcels) {
      const row = page.getByRole('row', { name: new RegExp(p.trackingNumber, 'i') });
      await expect(row, `Table row for ${p.trackingNumber} should be visible in UI table`).toBeVisible({ timeout: 10000 });
      await expect(row.getByText(/delivered/i), `Row for ${p.trackingNumber} should show delivered badge`).toBeVisible({ timeout: 10000 });
      console.log(`[Flow 7] 👁️ Visually Verified in UI Table: ${p.trackingNumber} is DELIVERED`);
    }

    console.log('[Flow 7] All 6 parcels confirmed DELIVERED on live system without mocks!');
  });

  test.afterAll(async () => {
    // Clean up temporary user and parcels only if explicitly requested
    if (process.env.CLEANUP === 'true') {
      try {
        const { execSync } = await import('node:child_process');
        execSync(
          `docker exec -i swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -c "
            DELETE FROM delivery_attempts WHERE parcel_id IN (SELECT id FROM parcels WHERE sender_id IN (SELECT id FROM users WHERE email = '${customerEmail}'));
            DELETE FROM courier_assignments WHERE parcel_id IN (SELECT id FROM parcels WHERE sender_id IN (SELECT id FROM users WHERE email = '${customerEmail}'));
            DELETE FROM parcel_status_history WHERE parcel_id IN (SELECT id FROM parcels WHERE sender_id IN (SELECT id FROM users WHERE email = '${customerEmail}'));
            DELETE FROM parcels WHERE sender_id IN (SELECT id FROM users WHERE email = '${customerEmail}');
            DELETE FROM pricing_quotes WHERE requester_id IN (SELECT id FROM users WHERE email = '${customerEmail}');
            DELETE FROM users WHERE email = '${customerEmail}';
          "`
        );
      } catch {
        // ignore
      }
    } else {
      console.log(`[Flow 7] ℹ️ Test customer preserved for browser inspection: ${customerEmail} / ${customerPassword}`);
    }
  });
});
