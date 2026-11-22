import React, { useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  AlertCircle,
  Eye,
  EyeOff,
  Package,
  ArrowRight,
  CheckCircle2,
  Loader2,
  MailCheck,
} from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Stepper } from '@/shared/components/ui/stepper';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '@/shared/components/ui/card';
import { friendlyApiError } from '@/shared/lib/api-error';

interface AuthApiResponse {
  data?: {
    message?: string;
    cooldown_seconds?: number;
    verified?: boolean;
    [key: string]: unknown;
  };
  error?: {
    message?: string;
    code?: string;
  };
}

// Registration runs in three steps so a customer only enters their details after the
// server has confirmed the emailed code (POST /api/v1/auth/register/verify-otp). The
// confirmed code is kept and sent once more with the final POST /api/v1/auth/register.
type RegisterStep = 'email' | 'code' | 'details' | 'done';

const STEP_LABELS = ['Email', 'Verify', 'Details'];
const STEP_INDEX: Record<RegisterStep, number> = { email: 0, code: 1, details: 2, done: 3 };

// Server codes that mean the emailed code can no longer be used: go back to the code step.
const CODE_ERRORS = new Set(['AUTH_OTP_INVALID', 'AUTH_OTP_EXPIRED', 'AUTH_OTP_MAX_ATTEMPTS']);

async function readJson(res: Response): Promise<AuthApiResponse | null> {
  const text = await res.text();
  try {
    return text ? (JSON.parse(text) as AuthApiResponse) : null;
  } catch {
    return null; // Response was not JSON (e.g. a proxy error page)
  }
}

export const RegisterPage: React.FC = () => {
  const navigate = useNavigate();
  const [step, setStep] = useState<RegisterStep>('email');

  // Step 1: email
  const [email, setEmail] = useState('');
  const [cooldown, setCooldown] = useState(0);

  // Step 2: emailed code
  const [otp, setOtp] = useState('');

  // Step 3: account details
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [phone, setPhone] = useState('+94');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [termsAccepted, setTermsAccepted] = useState(false);

  // UI state
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const codeInputRef = useRef<HTMLInputElement>(null);
  const firstNameRef = useRef<HTMLInputElement>(null);

  // Countdown timer for the resend cooldown
  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => {
      setCooldown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  // Move keyboard focus to the first field of each step.
  useEffect(() => {
    if (step === 'code') codeInputRef.current?.focus();
    if (step === 'details') firstNameRef.current?.focus();
  }, [step]);

  const trimmedEmail = email.trim();

  // Step 1: send (or resend) the code
  const handleSendOTP = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!trimmedEmail) {
      setError('Please enter a valid email address.');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await fetch('/api/v1/auth/register/send-otp', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: trimmedEmail }),
      });
      const data = await readJson(res);

      if (!res.ok) {
        if (res.status === 409) {
          throw new Error('An account with this email already exists. Please log in.');
        }
        if (res.status === 429) {
          throw new Error('Please wait before requesting another verification code.');
        }
        if (data?.error?.message) {
          throw new Error(data.error.message);
        }
        throw new Error(friendlyApiError(res, data?.error?.code, 'Failed to send verification code.'));
      }

      if (!data?.data) {
        throw new Error('Email verification is unavailable right now. Please try again later.');
      }
      setOtp('');
      setStep('code');
      setCooldown(data.data.cooldown_seconds || 60);
    } catch (err: unknown) {
      if (err instanceof Error && !(err instanceof TypeError)) {
        setError(err.message);
      } else {
        setError('Could not connect to SwiftDrop. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  // Step 2: check the code with the server before showing the details form
  const handleVerifyOTP = async (e: React.FormEvent) => {
    e.preventDefault();
    const code = otp.trim();
    if (!/^\d{6}$/.test(code)) {
      setError('Enter the 6-digit code from the email.');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await fetch('/api/v1/auth/register/verify-otp', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: trimmedEmail, otp: code }),
      });
      const data = await readJson(res);

      if (!res.ok) {
        const errorCode = data?.error?.code;
        // A locked or expired code is deleted on the server: clear it so the customer resends.
        if (errorCode === 'AUTH_OTP_MAX_ATTEMPTS' || errorCode === 'AUTH_OTP_EXPIRED') {
          setOtp('');
        }
        if (res.status === 409) {
          throw new Error('An account with this email already exists. Please log in.');
        }
        if (data?.error?.message) {
          throw new Error(data.error.message);
        }
        throw new Error(friendlyApiError(res, errorCode, 'Could not check the code. Please try again.'));
      }

      if (!data?.data?.verified) {
        throw new Error('The server could not confirm your code. Please try again.');
      }
      setStep('details');
    } catch (err: unknown) {
      if (err instanceof Error && !(err instanceof TypeError)) {
        setError(err.message);
      } else {
        setError('Could not connect to SwiftDrop. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  // Step 3: create the account with the confirmed code
  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!termsAccepted) {
      setError('You must accept the Terms & Conditions to register.');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await fetch('/api/v1/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: trimmedEmail,
          otp: otp.trim(),
          first_name: firstName.trim(),
          last_name: lastName.trim(),
          phone: phone.trim(),
          password: password,
          terms_accepted: true,
        }),
      });
      const data = await readJson(res);

      if (!res.ok) {
        // The code can expire while the details are typed: go back to the code step,
        // keeping everything entered so far.
        if (data?.error?.code && CODE_ERRORS.has(data.error.code)) {
          setOtp('');
          setStep('code');
          throw new Error(`${data.error.message ?? 'Your verification code is no longer valid.'} Your details are kept.`);
        }
        if (data?.error?.message) {
          throw new Error(data.error.message);
        }
        throw new Error(friendlyApiError(res, data?.error?.code, 'Registration is unavailable right now. Please try again later.'));
      }

      if (!data?.data) {
        throw new Error('The server could not confirm your registration. Try signing in or request a new verification code.');
      }
      setStep('done');
    } catch (err: unknown) {
      if (err instanceof Error && !(err instanceof TypeError)) {
        setError(err.message);
      } else {
        setError('Registration failed. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  const startOverWithNewEmail = () => {
    setStep('email');
    setOtp('');
    setError(null);
  };

  const description = {
    email: 'Enter your email to receive a secure verification code.',
    code: `Enter the 6-digit verification code sent to ${trimmedEmail}.`,
    details: 'Your email is verified. Add your details to finish creating your account.',
    done: 'Your SwiftDrop account is ready. Sign in with your email and password.',
  }[step];

  return (
    <main data-role="PUBLIC" className="relative min-h-screen bg-canvas text-text flex flex-col justify-between antialiased selection:bg-primary-tint selection:text-primary">

      {/* Top Header */}
      <header className="relative z-10 px-4 sm:px-8 py-5">
        <div className="mx-auto flex max-w-5xl items-center justify-between">
          <Link
            to="/"
            className="inline-flex items-center gap-2.5 font-display text-xl font-bold tracking-tight text-text"
            aria-label="SwiftDrop Home"
          >
            <span className="grid size-8 place-items-center rounded-sm bg-chrome text-primary">
              <Package size={17} strokeWidth={2} aria-hidden="true" />
            </span>
            SwiftDrop
          </Link>

          <Button asChild variant="ghost" size="sm" className="text-xs text-text-muted hover:text-text">
            <Link to="/track">
              Track a parcel
              <ArrowRight className="ml-1.5 size-3.5" aria-hidden="true" />
            </Link>
          </Button>
        </div>
      </header>

      {/* Registration window */}
      <div className="relative z-10 mx-auto flex w-full max-w-md sm:max-w-lg flex-col gap-4 px-4 my-auto py-6">
        <Card className="border border-border bg-surface rounded-md">
          <CardHeader className="text-center space-y-2 pb-5 pt-7">
            <div className="mx-auto flex size-11 items-center justify-center rounded-sm bg-chrome text-primary mb-1">
              {step === 'done' ? (
                <CheckCircle2 className="size-5 text-status-success-ink" aria-hidden="true" />
              ) : step === 'code' ? (
                <MailCheck className="size-5" aria-hidden="true" />
              ) : (
                <Package className="size-5" aria-hidden="true" />
              )}
            </div>
            <div className="space-y-1">
              <CardTitle className="text-xl font-bold tracking-tight text-text">
                {step === 'done' ? 'Account created' : step === 'code' ? 'Check your email' : 'Create a customer account'}
              </CardTitle>
              <CardDescription className="text-xs text-text-muted max-w-sm mx-auto">
                {description}
              </CardDescription>
            </div>
          </CardHeader>

          <CardContent className="space-y-4 px-6 sm:px-8">
            {step !== 'done' && (
              <Stepper steps={STEP_LABELS} current={STEP_INDEX[step]} label="Registration progress" />
            )}

            {error && (
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-sm border border-status-error-border bg-status-error-bg p-3 text-xs text-status-error-ink font-medium leading-relaxed"
              >
                <AlertCircle className="size-4 shrink-0 mt-0.5" aria-hidden="true" />
                <span>{error}</span>
              </div>
            )}

            {step === 'done' && (
              <div className="space-y-4 pt-2">
                <Button
                  type="button"
                  size="lg"
                  className="w-full text-xs font-semibold"
                  onClick={() => navigate('/auth/login')}
                >
                  Continue to Sign in
                  <ArrowRight className="ml-1.5 size-4" />
                </Button>
              </div>
            )}

            {step === 'email' && (
              <form onSubmit={handleSendOTP} className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="register-email" className="text-xs font-medium text-text">
                    Email address
                  </Label>
                  <Input
                    id="register-email"
                    data-testid="register-email-input"
                    type="email"
                    autoComplete="email"
                    required
                    placeholder="name@domain.com"
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                  />
                </div>

                <Button
                  type="submit"
                  data-testid="register-send-otp-btn"
                  size="lg"
                  className="w-full mt-2 text-xs font-semibold"
                  disabled={loading}
                >
                  {loading ? (
                    <>
                      <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
                      Sending code…
                    </>
                  ) : (
                    'Send verification code'
                  )}
                </Button>
              </form>
            )}

            {step === 'code' && (
              <form onSubmit={handleVerifyOTP} className="space-y-4">
                <div className="flex items-center justify-between gap-3 border-b border-border pb-3">
                  <div className="min-w-0 text-xs text-text-muted">
                    Email: <span className="break-all font-semibold text-text">{trimmedEmail}</span>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-xs text-text-muted hover:text-text"
                      disabled={loading}
                      onClick={startOverWithNewEmail}
                    >
                      Change
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-7 px-2.5 text-xs text-text-muted hover:text-text"
                      disabled={loading || cooldown > 0}
                      onClick={() => void handleSendOTP()}
                    >
                      {cooldown > 0 ? `${cooldown}s` : 'Resend'}
                    </Button>
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="register-code" className="text-xs font-medium text-text">
                    6-digit verification code
                  </Label>
                  <Input
                    ref={codeInputRef}
                    id="register-code"
                    data-testid="register-otp-input"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    pattern="[0-9]{6}"
                    maxLength={6}
                    required
                    placeholder="123456"
                    aria-describedby="register-code-help"
                    className="font-mono text-center tracking-widest text-sm"
                    value={otp}
                    onChange={(event) => setOtp(event.target.value.replace(/\D/g, ''))}
                  />
                  <p id="register-code-help" className="text-xs text-text-muted">
                    The code expires after 10 minutes. You have 3 tries before you need a new code.
                  </p>
                </div>

                <Button
                  type="submit"
                  data-testid="register-verify-otp-btn"
                  size="lg"
                  className="w-full mt-2 text-xs font-semibold"
                  disabled={loading || otp.length !== 6}
                >
                  {loading ? (
                    <>
                      <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
                      Checking code…
                    </>
                  ) : (
                    'Verify code'
                  )}
                </Button>
              </form>
            )}

            {step === 'details' && (
              <form onSubmit={handleRegister} className="space-y-4">
                <div
                  role="status"
                  data-testid="register-email-verified"
                  className="flex items-center justify-between gap-3 rounded-sm border border-status-success-border bg-status-success-bg px-3 py-2"
                >
                  <span className="flex min-w-0 items-center gap-2 text-xs text-status-success-ink">
                    <CheckCircle2 className="size-4 shrink-0" aria-hidden="true" />
                    <span className="min-w-0">
                      <span className="font-semibold">Email verified</span>{' '}
                      <span className="break-all">{trimmedEmail}</span>
                    </span>
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-7 shrink-0 px-2 text-xs text-text-muted hover:text-text"
                    disabled={loading}
                    onClick={startOverWithNewEmail}
                  >
                    Change
                  </Button>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <Label htmlFor="register-first" className="text-xs font-medium text-text">
                      First name
                    </Label>
                    <Input
                      ref={firstNameRef}
                      id="register-first"
                      data-testid="register-first-name-input"
                      autoComplete="given-name"
                      required
                      placeholder="Devinda"
                      value={firstName}
                      onChange={(event) => setFirstName(event.target.value)}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="register-last" className="text-xs font-medium text-text">
                      Last name
                    </Label>
                    <Input
                      id="register-last"
                      data-testid="register-last-name-input"
                      autoComplete="family-name"
                      required
                      placeholder="Perera"
                      value={lastName}
                      onChange={(event) => setLastName(event.target.value)}
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="register-phone" className="text-xs font-medium text-text">
                    Mobile phone number
                  </Label>
                  <Input
                    id="register-phone"
                    data-testid="register-phone-input"
                    type="tel"
                    autoComplete="tel"
                    required
                    placeholder="+94 77 123 4567"
                    value={phone}
                    onChange={(event) => setPhone(event.target.value)}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="register-password" className="text-xs font-medium text-text">
                    Create password
                  </Label>
                  <div className="relative">
                    <Input
                      id="register-password"
                      data-testid="register-password-input"
                      type={showPassword ? 'text' : 'password'}
                      autoComplete="new-password"
                      aria-describedby="register-password-help"
                      required
                      placeholder="••••••••"
                      className="pr-10"
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="absolute right-0 top-0 h-[38px] w-[38px] text-text-muted hover:text-text hover:bg-transparent"
                      aria-controls="register-password"
                      aria-label={showPassword ? 'Hide password' : 'Show password'}
                      onClick={() => setShowPassword((value) => !value)}
                    >
                      {showPassword ? (
                        <EyeOff className="size-4" aria-hidden="true" />
                      ) : (
                        <Eye className="size-4" aria-hidden="true" />
                      )}
                    </Button>
                  </div>
                  <p id="register-password-help" className="text-xs text-text-muted">
                    Must be at least 8 characters with an uppercase letter, number, and symbol.
                  </p>
                </div>

                <div className="flex items-start gap-2.5 pt-1">
                  <Checkbox
                    id="register-terms"
                    data-testid="register-terms-checkbox"
                    required
                    checked={termsAccepted}
                    onCheckedChange={(value) => setTermsAccepted(value === true)}
                    className="mt-0.5"
                  />
                  <Label
                    htmlFor="register-terms"
                    className="text-xs font-normal leading-relaxed text-text-muted cursor-pointer"
                  >
                    I agree to the Terms of Service and Privacy Policy.
                  </Label>
                </div>

                <Button
                  type="submit"
                  data-testid="register-submit-btn"
                  size="lg"
                  className="w-full mt-2 text-xs font-semibold"
                  disabled={loading || !termsAccepted}
                >
                  {loading ? (
                    <>
                      <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
                      Creating account…
                    </>
                  ) : (
                    'Create account'
                  )}
                </Button>
              </form>
            )}
          </CardContent>

          <CardFooter className="flex flex-col gap-2.5 border-t border-border py-4 px-6 text-center bg-surface-subtle rounded-b-md">
            <p className="text-xs text-text-muted">
              Already have an account?{' '}
              <Button asChild variant="link" className="text-xs font-semibold text-primary">
                <Link to="/auth/login">Sign in</Link>
              </Button>
            </p>
          </CardFooter>
        </Card>
      </div>

      {/* Footer */}
      <footer className="relative z-10 px-4 py-4 text-center text-xs text-text-muted">
        <span>&copy; 2026 SwiftDrop Logistics Inc.</span>
      </footer>
    </main>
  );
};
