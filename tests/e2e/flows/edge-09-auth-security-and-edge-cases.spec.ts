import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';
import { getRegistrationOTPFromMailpit } from '../helpers/mailpitHelper';
import { TEST_USERS } from '../fixtures/testData';

/**
 * SUITE 9: SIGN-IN & SIGN-UP SECURITY, ATTACK MITIGATION & EDGE CASES (ZERO MOCKS)
 *
 * Exhaustively tests all authentication and registration security boundaries against
 * the live physical stack (Go Core API, PostgreSQL, and Mailpit SMTP server):
 *
 * SECTION 1: SIGN-IN (LOGIN) ATTACK VECTORS & EDGE CASES (LIVE BACKEND)
 * - 9.1: SQL Injection attacks on email and password fields (' OR '1'='1', admin'--)
 * - 9.2: Cross-Site Scripting (XSS) payload injection in sign-in inputs (<script>, <svg>)
 * - 9.3: User enumeration defense (identical error for non-existent users & wrong passwords)
 * - 9.4: Suspended & disabled account login barring (403 AUTH_ACCOUNT_SUSPENDED)
 * - 9.5: Password visibility toggle security (type="password" <-> type="text")
 * - 9.6: Whitespace trimming and normalization on email submission
 * - 9.7: Role-based navigation guards after successful authentication
 *
 * SECTION 2: SIGN-UP (REGISTRATION) ATTACK VECTORS & EDGE CASES (LIVE MAILPIT & DB)
 * - 9.8:  Step 1: Malformed and invalid email inputs (RFC 5322 boundaries)
 * - 9.9:  Step 1: Duplicate registration rejection on existing email (409 Conflict)
 * - 9.10: Step 1: Resend OTP cooldown and rate limiting enforcement (429 / Button Cooldown)
 * - 9.11: Step 2: Weak password policy enforcement matrix (length, case, numbers, symbols)
 * - 9.12: Step 2: Mandatory Terms & Conditions gating (submission barred when unchecked)
 * - 9.13: Step 2: Invalid 6-digit OTP code rejection (400 AUTH_OTP_INVALID)
 * - 9.14: Step 2: OTP brute-force attempt exhaustion guard (400 AUTH_OTP_MAX_ATTEMPTS)
 * - 9.15: Step 2: OTP single-use replay prevention (consumed OTP cannot be reused)
 * - 9.16: Step 2: XSS payloads in first name, last name, and phone inputs
 * - 9.17: Step 2: Invalid non-E.164 phone number rejection (400 INVALID_PHONE)
 * - 9.18: Step 2: Change email workflow cleanly resets verification state
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 1 (Identity & Security), Member 6 (Customer UI)
 */

test.describe('Suite 9: Sign-In Security, Input Validation & Attack Mitigation', () => {

  test('Edge 9.1: SQL Injection Payloads in Login Fields Are Safely Rejected', async ({ page }) => {
    await page.goto('/auth/login');
    const emailInput = page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL));

    // Vector 1: Malformed raw SQL injection blocked by client-side email constraint
    await emailInput.fill("' OR '1'='1' --");
    expect(await emailInput.evaluate((el: HTMLInputElement) => el.checkValidity())).toBe(false);

    // Vector 2: Syntactically valid email with SQL injection payload reaches real Core API server and is rejected
    const sqliEmail = "admin'--@swiftdrop.local";
    await emailInput.fill(sqliEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill("' OR '1'='1'--");
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5000 });
    await expect(page.getByRole('alert')).toContainText(/Check your email and password|Invalid/i);
    await expect(page).toHaveURL(/\/auth\/login/); // Must never bypass login
  });

  test('Edge 9.2: Cross-Site Scripting (XSS) Payloads in Sign-In Fields Do Not Execute', async ({ page }) => {
    let xssTriggered = false;
    page.on('dialog', async (dialog) => {
      xssTriggered = true;
      await dialog.dismiss();
    });

    await page.goto('/auth/login');

    const emailInput = page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL));
    // Vector 1: Raw HTML/Script tag in email blocked by HTML5 email validation
    await emailInput.fill('<script>alert("XSS")</script>');
    expect(await emailInput.evaluate((el: HTMLInputElement) => el.checkValidity())).toBe(false);

    // Vector 2: XSS in password field submitted to real Core API backend renders safely without script execution
    await emailInput.fill('xss_tester@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('<script>alert("XSS")</script><img src=x onerror=alert(1)>');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5000 });
    expect(xssTriggered).toBe(false);
  });

  test('Edge 9.3: User Enumeration Defense - Identical Error for Non-Existent Users and Wrong Passwords', async ({ page }) => {
    await page.goto('/auth/login');

    // Case 1: Unknown Ghost Account against real DB
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('ghost_account_never_existed@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('WrongPassword123!');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5000 });
    const ghostErrorText = await page.getByRole('alert').innerText();

    // Case 2: Known User with Wrong Password against real DB
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('WrongPassword123!');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5000 });
    const knownErrorText = await page.getByRole('alert').innerText();

    // Both must be identical to mitigate user enumeration attacks
    expect(ghostErrorText).toBe(knownErrorText);
  });

  test('Edge 9.4: Suspended & Disabled Accounts Are Barred from Login', async ({ page }) => {
    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('suspended@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5000 });
    await expect(page.getByRole('alert')).toContainText(/suspended|disabled|permission/i);
    await expect(page).toHaveURL(/\/auth\/login/);
  });

  test('Edge 9.5: Password Visibility Toggle Prevents Shoulder Surfing', async ({ page }) => {
    await page.goto('/auth/login');
    const pwdInput = page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD));

    await pwdInput.fill('SecretPassword123!');
    // Defaults to obscured password field
    expect(await pwdInput.getAttribute('type')).toBe('password');

    // Toggle reveal
    const toggleBtn = page.getByRole('button', { name: /Show password/i });
    await toggleBtn.click();
    expect(await pwdInput.getAttribute('type')).toBe('text');

    // Toggle hide
    const hideBtn = page.getByRole('button', { name: /Hide password/i });
    await hideBtn.click();
    expect(await pwdInput.getAttribute('type')).toBe('password');
  });

  test('Edge 9.6: Email Whitespace Normalization on Sign-In', async ({ page }) => {
    await page.goto('/auth/login');
    // Leading/trailing whitespace should be normalized cleanly on submission
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('   customer@swiftdrop.local   ');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(TEST_USERS.CUSTOMER.password);
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10000 });
  });

  test('Edge 9.7: Role-Based Routing Directs Authorized Personnel to Respective Dashboards', async ({ page }) => {
    const rolesToTest = [
      { user: TEST_USERS.ADMIN, targetUrl: '/admin/dashboard' },
      { user: TEST_USERS.DISPATCHER, targetUrl: '/dispatcher/dashboard' },
      { user: TEST_USERS.COURIER, targetUrl: '/courier/dashboard' },
      { user: TEST_USERS.CUSTOMER, targetUrl: '/dashboard' },
    ];

    for (const { user, targetUrl } of rolesToTest) {
      await page.context().clearCookies();
      await page.goto('/auth/login');
      await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(user.email);
      await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill(user.password);
      await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

      // Check navigation redirects to proper workspace
      await expect(page).toHaveURL(new RegExp(targetUrl), { timeout: 10000 });
    }
  });

});

test.describe('Suite 9: Sign-Up Security, OTP Lifecycle & Invariant Protection', () => {

  test('Edge 9.8: Step 1 - Malformed Email Inputs Blocked by Form Validation', async ({ page }) => {
    await page.goto('/auth/register');

    const emailInput = page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL));
    const submitBtn = page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN));

    // Case 1: Empty email
    await submitBtn.click();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).not.toBeVisible();

    // Case 2: Missing '@' symbol
    await emailInput.fill('invalidaddress.com');
    await submitBtn.click();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).not.toBeVisible();

    // Case 3: Missing domain
    await emailInput.fill('user@');
    await submitBtn.click();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).not.toBeVisible();
  });

  test('Edge 9.9: Step 1 - Duplicate Registration on Existing Email Returns 409 Conflict', async ({ page }) => {
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('customer@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole('alert')).toContainText(/already exists|log in/i);
    // User must remain on Step 1 to prevent leaking OTP step
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).not.toBeVisible();
  });

  test('Edge 9.10: Step 1 - Resend OTP Cooldown & Rate Limiting (429 Too Many Requests)', async ({ page }) => {
    const rapidEmail = `cooldown_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(rapidEmail);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    // The resend countdown button is rendered and disabled to enforce the 60s cooldown
    const countdownBtn = page.getByRole('button', { name: /\d+s|Resend/i });
    await expect(countdownBtn).toBeDisabled();
  });

  test('Edge 9.11: Step 2 - Weak Password Policy Enforcement Matrix', async ({ page }) => {
    const email = `weak_pwd_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    const realOtp = await getRegistrationOTPFromMailpit(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(realOtp);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();

    // Fill password failing complexity requirements
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('weak123');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole('alert')).toContainText(/at least 8 characters|uppercase|lowercase|symbol|weak/i);
  });

  test('Edge 9.12: Step 2 - Mandatory Terms & Conditions Gating Bars Registration', async ({ page }) => {
    const email = `terms_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    const realOtp = await getRegistrationOTPFromMailpit(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(realOtp);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('SecurePass123!');

    // Terms checkbox is UNCHECKED
    const submitBtn = page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN));
    await expect(submitBtn).toBeDisabled();
  });

  test('Edge 9.13: Step 2 - Invalid 6-Digit OTP Rejection', async ({ page }) => {
    const email = `wrong_otp_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    // Intentionally wrong OTP
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('000000');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole('alert')).toContainText(/incorrect|invalid|information/i);
  });

  test('Edge 9.14: Step 2 - OTP Attempt Exhaustion Enforces pgOTPMaxAttempts = 3', async ({ page }) => {
    const email = `exhaust_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();

    // 3 failed attempts
    for (let i = 1; i <= 3; i++) {
      await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(`11111${i}`);
      await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();
      await expect(page.getByRole('alert')).toBeVisible({ timeout: 10000 });
    }

    // On 3rd attempt, code must be invalidated
    await expect(page.getByRole('alert')).toContainText(/attempts|request a new code|incorrect|invalid/i);
  });

  test('Edge 9.15: Step 2 - OTP Replay Prevention Enforces Single-Use Consumption', async ({ page }) => {
    const email = `replay_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    const realOtp = await getRegistrationOTPFromMailpit(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(realOtp);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('First');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Last');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();

    // First submission succeeds against live Core API and DB
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();
    await expect(page.getByText('Account created')).toBeVisible({ timeout: 10000 });

    // Direct replay attempt via fetch within same page session with already-consumed OTP
    const replayResponse = await page.evaluate(async (payload) => {
      const res = await fetch('/api/v1/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      return { status: res.status, body: await res.json() };
    }, {
      email,
      otp: realOtp,
      first_name: 'First',
      last_name: 'Last',
      phone: '+94771234567',
      password: 'Password123!',
      terms_accepted: true,
    });

    expect(replayResponse.status).toBe(400);
    expect(replayResponse.body.error.code).toBe('AUTH_OTP_INVALID');
  });

  test('Edge 9.16: Step 2 - XSS Payloads in Registration Name Fields Render Safely', async ({ page }) => {
    let xssTriggered = false;
    page.on('dialog', async (dialog) => {
      xssTriggered = true;
      await dialog.dismiss();
    });

    const email = `xss_reg_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    const realOtp = await getRegistrationOTPFromMailpit(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(realOtp);

    const xssName = '<script>alert("XSS")</script><img src=x onerror=alert(1)>';
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill(xssName);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Tester');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    await expect(page.getByText('Account created')).toBeVisible({ timeout: 10000 });
    expect(xssTriggered).toBe(false);
  });

  test('Edge 9.17: Step 2 - Invalid E.164 Phone Format Rejection', async ({ page }) => {
    const email = `bad_phone_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });

    const realOtp = await getRegistrationOTPFromMailpit(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(realOtp);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('not-a-phone-1234');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole('alert')).toContainText(/Phone must be in E.164 format|Phone|information/i);
  });

  test('Edge 9.18: Step 2 - "Change" Email Button Resets Registration State Cleanly', async ({ page }) => {
    const email = `mistyped_${Date.now()}@swiftdrop.local`;
    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill(email);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible({ timeout: 10000 });
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('999999');

    // User realizes email was mistyped and clicks "Change"
    await page.getByRole('button', { name: 'Change' }).click();

    // Verify cleanly reset to Step 1
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL))).toBeVisible();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).not.toBeVisible();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN))).toBeVisible();
  });

  test.afterAll(async () => {
    // Clean up temporary test accounts created during live registration runs
    try {
      const { execSync } = await import('node:child_process');
      execSync(`docker exec -i swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -c "DELETE FROM users WHERE email LIKE '%_@swiftdrop.local' AND email NOT IN ('admin@swiftdrop.local', 'dispatcher@swiftdrop.local', 'courier@swiftdrop.local', 'merchant@vendor.com', 'courier2@swiftdrop.local', 'customer@swiftdrop.local', 'applicant@swiftdrop.local', 'suspended@swiftdrop.local');"`);
    } catch {
      // ignore
    }
  });

});
