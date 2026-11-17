import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';
import { getRegistrationOTPFromMailpit, clearMailpitMessages } from '../helpers/mailpitHelper';

/**
 * FLOW 6: CUSTOMER REGISTRATION & REAL EMAIL OTP VERIFICATION (MAILPIT INTEGRATION)
 *
 * Verifies the end-to-end user onboarding lifecycle:
 * 1. Customer initiates registration at /auth/register with a unique email.
 * 2. Core API generates a cryptographically secure 6-digit OTP and dispatches an HTML email via SMTP.
 * 3. Mailpit (:8025) intercepts the transactional email.
 * 4. Playwright queries Mailpit REST API to extract the 6-digit OTP code (with DB fallback).
 * 5. Customer enters the verification code, fills personal details, accepts Terms & Conditions, and registers.
 * 6. Customer completes authentication with new credentials and lands on the Customer Dashboard.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 1 (Identity), Member 6 (Customer UI), Member 8 (Messaging)
 */

test.describe('Flow 6: Customer Registration & Real Email OTP (Mailpit)', () => {
  const timestamp = Date.now();
  const testEmail = `customer_${timestamp}@swiftdrop.local`;
  const testPassword = 'Password123!';
  const firstName = 'Chamath';
  const lastName = 'Fernando';
  const phone = '+94712345678';

  test.beforeEach(async () => {
    await clearMailpitMessages();
  });

  test('should register a new customer account using real email OTP from Mailpit and sign in', async ({
    page,
  }) => {
    // -------------------------------------------------------------------------
    // 1. Visit Registration Page (Step 1: Request Code)
    // -------------------------------------------------------------------------
    await page.goto('/auth/register');
    await expect(page.getByRole('heading', { name: 'Create a customer account' })).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Enter your email to receive a secure verification code.')).toBeVisible();

    // -------------------------------------------------------------------------
    // 2. Submit Email for Verification Code
    // -------------------------------------------------------------------------
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(testEmail);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    // -------------------------------------------------------------------------
    // 3. Step 2 Appears: Verification Code & Profile Inputs
    // -------------------------------------------------------------------------
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(testEmail)).toBeVisible();

    // -------------------------------------------------------------------------
    // 4. Retrieve Real OTP Code from Mailpit Sandbox
    // -------------------------------------------------------------------------
    const otpCode = await getRegistrationOTPFromMailpit(testEmail);
    console.log(`[E2E] Retrieved Registration OTP for ${testEmail}: ${otpCode}`);
    expect(otpCode).toMatch(/^\d{6}$/);

    // -------------------------------------------------------------------------
    // 5. Fill Verification Code & Customer Registration Form
    // -------------------------------------------------------------------------
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(otpCode);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill(firstName);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill(lastName);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill(phone);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill(testPassword);

    // Accept Terms & Conditions Checkbox
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();

    // Submit Registration
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    // -------------------------------------------------------------------------
    // 6. Verify Account Created Confirmation Screen
    // -------------------------------------------------------------------------
    await expect(page.getByRole('heading', { name: 'Account created' })).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Your SwiftDrop account is ready. Sign in with your email and password.')).toBeVisible();

    // -------------------------------------------------------------------------
    // 7. Click Continue to Sign In
    // -------------------------------------------------------------------------
    await page.getByRole('button', { name: /Continue to Sign in/i }).click();
    await expect(page).toHaveURL(/\/auth\/login/, { timeout: 10000 });

    // -------------------------------------------------------------------------
    // 8. Sign In with Newly Created Customer Account
    // -------------------------------------------------------------------------
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(testEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(testPassword);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // -------------------------------------------------------------------------
    // 9. Verify Entry to Customer Portal Dashboard
    // -------------------------------------------------------------------------
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10000 });
    await expect(page.getByText(`${firstName} ${lastName}`)).toBeVisible({ timeout: 10000 });
  });

  test.afterAll(async () => {
    // Clean up created test user from database
    try {
      const { execSync } = await import('node:child_process');
      execSync(`docker exec -i swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -c "DELETE FROM users WHERE email = '${testEmail}'; DELETE FROM email_otps WHERE email = '${testEmail}';"`);
    } catch {
      // ignore
    }
  });
});
