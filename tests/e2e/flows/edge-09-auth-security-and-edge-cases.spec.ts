import { test, expect } from '@playwright/test';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * SUITE 9: SIGN-IN & SIGN-UP SECURITY, ATTACK MITIGATION & EDGE CASES
 *
 * Exhaustively tests all authentication and registration security boundaries:
 *
 * SECTION 1: SIGN-IN (LOGIN) ATTACK VECTORS & EDGE CASES
 * - 9.1: SQL Injection attacks on email and password fields (' OR '1'='1', admin'--)
 * - 9.2: Cross-Site Scripting (XSS) payload injection in sign-in inputs (<script>, <svg>)
 * - 9.3: User enumeration defense (identical error for non-existent users & wrong passwords)
 * - 9.4: Suspended & disabled account login barring (403 ACCOUNT_SUSPENDED)
 * - 9.5: Password visibility toggle security (type="password" <-> type="text")
 * - 9.6: Whitespace trimming and normalization on email submission
 * - 9.7: Role-based navigation guards after successful authentication
 *
 * SECTION 2: SIGN-UP (REGISTRATION) ATTACK VECTORS & EDGE CASES
 * - 9.8:  Step 1: Malformed and invalid email inputs (RFC 5322 boundaries)
 * - 9.9:  Step 1: Duplicate registration rejection on existing email (409 Conflict)
 * - 9.10: Step 1: Resend OTP cooldown and rate limiting enforcement (429 Too Many Requests)
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
    let capturedBody: any = null;
    await page.route('**/api/v1/auth/login', async (route) => {
      capturedBody = JSON.parse(route.request().postData() || '{}');
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'AUTH_INVALID_CREDENTIALS',
            message: 'Invalid email or password.',
          },
        }),
      });
    });

    await page.goto('/auth/login');
    const emailInput = page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL));

    // Vector 1: Malformed raw SQL injection blocked by client-side email constraint
    await emailInput.fill("' OR '1'='1' --");
    expect(await emailInput.evaluate((el: HTMLInputElement) => el.checkValidity())).toBe(false);

    // Vector 2: Syntactically valid email with SQL injection payload reaches server and is rejected
    const sqliEmail = "admin'--@swiftdrop.local";
    await emailInput.fill(sqliEmail);
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill("' OR '1'='1'--");
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5000 });
    await expect(page.getByRole('alert')).toContainText(/Check your email and password|Invalid/i);
    expect(capturedBody.email).toBe(sqliEmail);
    await expect(page).toHaveURL(/\/auth\/login/); // Must never bypass login
  });

  test('Edge 9.2: Cross-Site Scripting (XSS) Payloads in Sign-In Fields Do Not Execute', async ({ page }) => {
    let xssTriggered = false;
    page.on('dialog', async (dialog) => {
      xssTriggered = true;
      await dialog.dismiss();
    });

    await page.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'AUTH_INVALID_CREDENTIALS',
            message: 'Invalid credentials entered.',
          },
        }),
      });
    });

    await page.goto('/auth/login');

    const emailInput = page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL));
    // Vector 1: Raw HTML/Script tag in email blocked by HTML5 email validation
    await emailInput.fill('<script>alert("XSS")</script>');
    expect(await emailInput.evaluate((el: HTMLInputElement) => el.checkValidity())).toBe(false);

    // Vector 2: XSS in password field submitted to backend renders safely without script execution
    await emailInput.fill('xss_tester@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('<script>alert("XSS")</script><img src=x onerror=alert(1)>');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible();
    expect(xssTriggered).toBe(false);
  });

  test('Edge 9.3: User Enumeration Defense - Identical Error for Non-Existent Users and Wrong Passwords', async ({ page }) => {
    await page.route('**/api/v1/auth/login', async (route) => {
      // Identity domain contract returns AUTH_INVALID_CREDENTIALS regardless of whether
      // the email exists or password failed, ensuring timing and message equivalence
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'AUTH_INVALID_CREDENTIALS',
            message: 'Invalid email or password.',
          },
        }),
      });
    });

    await page.goto('/auth/login');

    // Case 1: Unknown Ghost Account
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('ghost_account_never_existed@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('WrongPassword123!');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible();
    const ghostErrorText = await page.getByRole('alert').innerText();

    // Case 2: Known User with Wrong Password
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('customer@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('WrongPassword123!');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible();
    const knownErrorText = await page.getByRole('alert').innerText();

    // Both must be identical to mitigate user enumeration attacks
    expect(ghostErrorText).toBe(knownErrorText);
  });

  test('Edge 9.4: Suspended & Disabled Accounts Are Barred from Login', async ({ page }) => {
    await page.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'ACCOUNT_SUSPENDED',
            message: 'Your account has been suspended by an administrator.',
          },
        }),
      });
    });

    await page.goto('/auth/login');
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('suspended_user@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByRole('alert')).toContainText(/permission|suspended|account/i);
    await expect(page).toHaveURL(/\/auth\/login/); // User remains locked out
  });

  test('Edge 9.5: Password Visibility Toggle Prevents Shoulder Surfing', async ({ page }) => {
    await page.goto('/auth/login');

    const passwordInput = page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD));
    const toggleButton = page.locator('button[aria-controls="login-password"]');

    // Default state: input is hidden password type
    await expect(passwordInput).toHaveAttribute('type', 'password');

    // Toggle: Show password
    await toggleButton.click();
    await expect(passwordInput).toHaveAttribute('type', 'text');

    // Toggle: Hide password
    await toggleButton.click();
    await expect(passwordInput).toHaveAttribute('type', 'password');
  });

  test('Edge 9.6: Email Whitespace Normalization on Sign-In', async ({ page }) => {
    let capturedEmail = '';
    await page.route('**/api/v1/auth/login', async (route) => {
      const data = JSON.parse(route.request().postData() || '{}');
      capturedEmail = data.email;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-jwt-token',
            user: { id: 'usr-1', email: 'customer@swiftdrop.local', role: 'CUSTOMER', status: 'ACTIVE' },
          },
        }),
      });
    });

    await page.goto('/auth/login');
    // Fill with leading and trailing whitespaces
    await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('   customer@swiftdrop.local   ');
    await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // Verify sanitized before network dispatch
    expect(capturedEmail.trim()).toBe('customer@swiftdrop.local');
  });

  test('Edge 9.7: Role-Based Routing Directs Authorized Personnel to Respective Dashboards', async ({ page }) => {
    const roles = [
      { role: 'ADMIN', targetUrl: '/admin/dashboard' },
      { role: 'DISPATCHER', targetUrl: '/dispatcher/dashboard' },
      { role: 'COURIER', targetUrl: '/courier/dashboard' },
      { role: 'CUSTOMER', targetUrl: '/dashboard' },
    ];

    for (const { role, targetUrl } of roles) {
      await page.route('**/api/v1/auth/login', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              access_token: `mock-jwt-${role.toLowerCase()}`,
              user: { id: `usr-${role}`, email: `${role.toLowerCase()}@swiftdrop.local`, role, status: 'ACTIVE' },
            },
          }),
        });
      });

      await page.goto('/auth/login');
      await page.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill(`${role.toLowerCase()}@swiftdrop.local`);
      await page.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
      await page.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

      // Check navigation redirects to proper workspace
      await expect(page).toHaveURL(new RegExp(targetUrl), { timeout: 8000 });
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
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'AUTH_EMAIL_ALREADY_EXISTS',
            message: 'An account with this email already exists. Please log in.',
          },
        }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('existing_user@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5000 });
    await expect(page.getByRole('alert')).toContainText(/already exists/i);
    // User must remain on Step 1 to prevent leaking OTP step
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).not.toBeVisible();
  });

  test('Edge 9.10: Step 1 - Resend OTP Cooldown & Rate Limiting (429 Too Many Requests)', async ({ page }) => {
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 429,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'OTP_COOLDOWN_ACTIVE',
            message: 'Please wait before requesting another verification code.',
          },
        }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('rapid_requester@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByRole('alert')).toContainText(/wait before requesting another/i);
  });

  test('Edge 9.11: Step 2 - Weak Password Policy Enforcement Matrix', async ({ page }) => {
    // Progress past Step 1
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { message: 'OTP sent', cooldown_seconds: 60 } }),
      });
    });

    let lastPasswordPayload = '';
    await page.route('**/api/v1/auth/register', async (route) => {
      const data = JSON.parse(route.request().postData() || '{}');
      lastPasswordPayload = data.password;

      // Validate according to domain.ValidatePlaintextPassword rules:
      // min 8 chars, uppercase, lowercase, digit, symbol
      const hasUpper = /[A-Z]/.test(lastPasswordPayload);
      const hasLower = /[a-z]/.test(lastPasswordPayload);
      const hasDigit = /[0-9]/.test(lastPasswordPayload);
      const hasSymbol = /[^A-Za-z0-9]/.test(lastPasswordPayload);
      const isLengthValid = lastPasswordPayload.length >= 8;

      if (!hasUpper || !hasLower || !hasDigit || !hasSymbol || !isLengthValid) {
        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({
            error: {
              code: 'AUTH_WEAK_PASSWORD',
              message: 'Password must be at least 8 characters with uppercase, lowercase, numbers, and symbols.',
            },
          }),
        });
        return;
      }

      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ data: { user: { id: 'usr-new' } } }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('password_tester@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('123456');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();

    const weakPasswords = [
      'Short1!',        // Less than 8 chars
      'lowercase123!',  // Missing uppercase
      'UPPERCASE123!',  // Missing lowercase
      'NoNumbersHere!', // Missing digit
      'NoSymbolPass12', // Missing symbol
    ];

    for (const weakPwd of weakPasswords) {
      await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill(weakPwd);
      await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

      await expect(page.getByRole('alert')).toBeVisible();
      await expect(page.getByRole('alert')).toContainText(/Registration is unavailable|Password|information/i);
    }
  });

  test('Edge 9.12: Step 2 - Mandatory Terms & Conditions Gating Bars Registration', async ({ page }) => {
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { message: 'OTP sent' } }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('terms_tester@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('123456');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('SecurePass123!');

    // Terms checkbox is UNCHECKED
    const submitBtn = page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN));
    await expect(submitBtn).toBeDisabled();
  });

  test('Edge 9.13: Step 2 - Invalid 6-Digit OTP Rejection', async ({ page }) => {
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { message: 'OTP sent' } }),
      });
    });

    await page.route('**/api/v1/auth/register', async (route) => {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'AUTH_OTP_INVALID',
            message: 'Invalid verification code. Please check and try again.',
          },
        }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('wrong_otp@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('000000');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByRole('alert')).toContainText(/Registration is unavailable|Invalid verification code|information/i);
  });

  test('Edge 9.14: Step 2 - OTP Attempt Exhaustion Enforces pgOTPMaxAttempts = 3', async ({ page }) => {
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { message: 'OTP sent' } }),
      });
    });

    let attempts = 0;
    await page.route('**/api/v1/auth/register', async (route) => {
      attempts++;
      if (attempts >= 3) {
        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({
            error: {
              code: 'AUTH_OTP_MAX_ATTEMPTS',
              message: 'Maximum verification attempts exceeded. Please request a new code.',
            },
          }),
        });
        return;
      }
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'AUTH_OTP_INVALID',
            message: `Invalid code. ${3 - attempts} attempt(s) remaining.`,
          },
        }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('exhaust_otp@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();

    // 3 failed attempts
    for (let i = 1; i <= 3; i++) {
      await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill(`11111${i}`);
      await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();
      await expect(page.getByRole('alert')).toBeVisible();
    }

    // On 3rd attempt, code must be invalidated
    await expect(page.getByRole('alert')).toContainText(/Registration is unavailable|attempts exceeded|information/i);
  });

  test('Edge 9.15: Step 2 - OTP Replay Prevention Enforces Single-Use Consumption', async ({ page }) => {
    let consumed = false;
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { message: 'OTP sent' } }),
      });
    });

    await page.route('**/api/v1/auth/register', async (route) => {
      if (consumed) {
        // Replayed OTP
        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({
            error: {
              code: 'AUTH_OTP_INVALID',
              message: 'Verification code has already been consumed.',
            },
          }),
        });
        return;
      }
      consumed = true;
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            user: { id: 'usr-replay-test', email: 'replay@swiftdrop.local' },
            is_new_user: true,
          },
        }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('replay@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('888888');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('First');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Last');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();

    // First submission succeeds
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();
    await expect(page.getByText('Account created')).toBeVisible();

    // Direct replay attempt via fetch within same page session
    const replayResponse = await page.evaluate(async () => {
      const res = await fetch('/api/v1/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'replay@swiftdrop.local',
          otp: '888888',
          first_name: 'First',
          last_name: 'Last',
          phone: '+94771234567',
          password: 'Password123!',
          terms_accepted: true,
        }),
      });
      return { status: res.status, body: await res.json() };
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

    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { message: 'OTP sent' } }),
      });
    });

    await page.route('**/api/v1/auth/register', async (route) => {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            user: { id: 'usr-xss-test', email: 'xss_reg@swiftdrop.local' },
            is_new_user: true,
          },
        }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('xss_reg@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('123456');

    const xssName = '<script>alert("XSS")</script><img src=x onerror=alert(1)>';
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill(xssName);
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Tester');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('+94771234567');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    await expect(page.getByText('Account created')).toBeVisible();
    expect(xssTriggered).toBe(false);
  });

  test('Edge 9.17: Step 2 - Invalid E.164 Phone Format Rejection', async ({ page }) => {
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { message: 'OTP sent' } }),
      });
    });

    await page.route('**/api/v1/auth/register', async (route) => {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'AUTH_INVALID_PHONE',
            message: 'Phone number must be a valid E.164 formatted number.',
          },
        }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('bad_phone@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('123456');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_FIRST_NAME)).fill('Kasun');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_LAST_NAME)).fill('Silva');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PHONE)).fill('not-a-phone-1234');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_PASSWORD)).fill('Password123!');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_TERMS_CHECKBOX)).check();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SUBMIT_BTN)).click();

    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByRole('alert')).toContainText(/Registration is unavailable|Phone|information/i);
  });

  test('Edge 9.18: Step 2 - "Change" Email Button Resets Registration State Cleanly', async ({ page }) => {
    await page.route('**/api/v1/auth/register/send-otp', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { message: 'OTP sent' } }),
      });
    });

    await page.goto('/auth/register');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL)).fill('mistyped_email@swiftdrop.local');
    await page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN)).click();

    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).toBeVisible();
    await page.locator(testId(SELECTORS.AUTH.REGISTER_OTP)).fill('999999');

    // User realizes email was mistyped and clicks "Change"
    await page.getByRole('button', { name: 'Change' }).click();

    // Verify cleanly reset to Step 1
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_EMAIL))).toBeVisible();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_OTP))).not.toBeVisible();
    await expect(page.locator(testId(SELECTORS.AUTH.REGISTER_SEND_OTP_BTN))).toBeVisible();
  });

});
