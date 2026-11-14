import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  AlertCircle,
  Eye,
  EyeOff,
  Package,
  ArrowRight,
  CheckCircle2,
  Loader2,
} from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '@/shared/components/ui/card';
import { withRequestReference } from '@/shared/lib/api-error';

interface AuthApiResponse {
  data?: {
    message?: string;
    cooldown_seconds?: number;
    [key: string]: unknown;
  };
  error?: {
    message?: string;
    correlation_id?: string;
  };
}

export const RegisterPage: React.FC = () => {
  const navigate = useNavigate();

  // Step 1: Email & OTP request
  const [email, setEmail] = useState('');
  const [otpSent, setOtpSent] = useState(false);
  const [cooldown, setCooldown] = useState(0);

  // Step 2: Verification & Details
  const [otp, setOtp] = useState('');
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [phone, setPhone] = useState('+94');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [termsAccepted, setTermsAccepted] = useState(false);

  // UI States
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  // Countdown timer for resend OTP cooldown
  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => {
      setCooldown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  // Handle Step 1: Send OTP
  const handleSendOTP = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!email.trim()) {
      setError('Please enter a valid email address.');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await fetch('/api/v1/auth/register/send-otp', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim() }),
      });

      const text = await res.text();
      let data: AuthApiResponse | null = null;
      try {
        data = text ? (JSON.parse(text) as AuthApiResponse) : null;
      } catch {
        // Response was not JSON
      }

      if (!res.ok) {
        let message = data?.error?.message || 'Failed to send verification code.';
        if (res.status === 409) {
          message = 'An account with this email already exists. Please log in.';
        } else if (res.status === 429) {
          throw new Error('Please wait before requesting another verification code.');
        } else if (data?.error?.message) {
          throw new Error(data.error.message);
        } else if (res.status === 502 || res.status === 504 || res.status === 500) {
          throw new Error('Backend server is unreachable. Please ensure the Core API is running on port 8080.');
        } else {
          throw new Error(`Failed to send verification code (${res.status}).`);
        }
        throw new Error(withRequestReference(message, res, data?.error?.correlation_id));
      }

      if (!data?.data) {
        throw new Error('Email verification is unavailable right now. Please try again later.');
      }
      setOtpSent(true);
      setCooldown(data?.data?.cooldown_seconds || 60);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Network error. Is the backend server running?');
      }
    } finally {
      setLoading(false);
    }
  };

  // Handle Step 2: Complete Registration
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
          email: email.trim(),
          otp: otp.trim(),
          first_name: firstName.trim(),
          last_name: lastName.trim(),
          phone: phone.trim(),
          password: password,
          terms_accepted: true,
        }),
      });

      const text = await res.text();
      let data: AuthApiResponse | null = null;
      try {
        data = text ? (JSON.parse(text) as AuthApiResponse) : null;
      } catch {
        // Response was not JSON
      }

      if (!res.ok) {
        if (data?.error?.message) {
          throw new Error(data.error.message);
        } else if (res.status === 502 || res.status === 504 || res.status === 500) {
          throw new Error('Backend server is unreachable. Please ensure the Core API is running on port 8080.');
        }
        throw new Error(`Registration failed (${res.status}). Please check your inputs.`);
      }

      if (!data?.data) {
        throw new Error('The server could not confirm your registration. Try signing in or request a new verification code.');
      }
      setSuccess(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Registration failed. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <main data-role="PUBLIC" className="relative min-h-screen bg-canvas text-text flex flex-col justify-between antialiased selection:bg-primary-tint selection:text-primary overflow-hidden">
      {/* Background Image Layer with Ambient Blur & Contrast Scrim (Aligned with Login) */}
      <div className="absolute inset-0 pointer-events-none select-none overflow-hidden" aria-hidden="true">
        <img
          src="/login-bg.jpg"
          alt=""
          className="h-full w-full object-cover object-center filter blur-sm scale-105"
        />
        {/* Soft atmospheric glow orbs */}
        <div className="absolute -top-32 -left-32 size-96 rounded-full bg-primary-tint blur-3xl opacity-50" />
        <div className="absolute -bottom-32 -right-32 size-96 rounded-full bg-primary-tint blur-3xl opacity-40" />

        {/* Ambient overlay scrim & blur veil for WCAG contrast in light/dark themes */}
        <div className="absolute inset-0 bg-canvas/75 backdrop-blur-sm" />
      </div>

      {/* Top Header */}
      <header className="relative z-10 px-4 sm:px-8 py-5">
        <div className="mx-auto flex max-w-5xl items-center justify-between">
          <Link
            to="/"
            className="inline-flex items-center gap-2.5 font-semibold tracking-tight text-text hover:opacity-90 transition-opacity"
            aria-label="SwiftDrop Home"
          >
            <div className="flex size-8 items-center justify-center rounded-lg bg-primary-tint text-primary border border-border">
              <Package className="size-4" aria-hidden="true" />
            </div>
            <span className="text-lg font-bold">
              Swift<span className="text-primary">Drop</span>
            </span>
          </Link>

          <Button asChild variant="ghost" size="sm" className="text-xs text-text-muted hover:text-text">
            <Link to="/track">
              Track a parcel
              <ArrowRight className="ml-1.5 size-3.5" aria-hidden="true" />
            </Link>
          </Button>
        </div>
      </header>

      {/* Centralized Clean Register Window */}
      <div className="relative z-10 mx-auto flex w-full max-w-md sm:max-w-lg flex-col gap-4 px-4 my-auto py-6">
        <Card className="border border-border bg-surface rounded-2xl">
          <CardHeader className="text-center space-y-2 pb-5 pt-7">
            <div className="mx-auto flex size-11 items-center justify-center rounded-xl bg-primary-tint text-primary border border-border mb-1">
              {success ? (
                <CheckCircle2 className="size-5 text-status-success-ink" aria-hidden="true" />
              ) : (
                <Package className="size-5" aria-hidden="true" />
              )}
            </div>
            <div className="space-y-1">
              <CardTitle className="text-xl font-bold tracking-tight text-text">
                {success ? 'Account created' : 'Create a customer account'}
              </CardTitle>
              <CardDescription className="text-xs text-text-muted max-w-sm mx-auto">
                {success
                  ? 'Your SwiftDrop account is ready. Sign in with your email and password.'
                  : otpSent
                  ? `Enter the 6-digit verification code sent to ${email.trim()}.`
                  : 'Enter your email to receive a secure verification code.'}
              </CardDescription>
            </div>
          </CardHeader>

          <CardContent className="space-y-4 px-6 sm:px-8">
            {error && (
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-lg border border-status-error-border bg-status-error-bg p-3 text-xs text-status-error-ink font-medium leading-relaxed"
              >
                <AlertCircle className="size-4 shrink-0 mt-0.5" aria-hidden="true" />
                <span>{error}</span>
              </div>
            )}

            {success ? (
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
            ) : !otpSent ? (
              /* Step 1: Request Verification Code */
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
            ) : (
              /* Step 2: Verification Code & Profile Details */
              <form onSubmit={handleRegister} className="space-y-4">
                <div className="flex items-center justify-between border-b border-border pb-3">
                  <div className="text-xs text-text-muted">
                    Email: <span className="font-semibold text-text">{email.trim()}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-xs text-text-muted hover:text-text"
                      disabled={loading}
                      onClick={() => {
                        setOtpSent(false);
                        setOtp('');
                        setError(null);
                      }}
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
                    id="register-code"
                    data-testid="register-otp-input"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    pattern="[0-9]{6}"
                    maxLength={6}
                    required
                    placeholder="123456"
                    className="font-mono text-center tracking-widest text-sm"
                    value={otp}
                    onChange={(event) => setOtp(event.target.value.replace(/\D/g, ''))}
                  />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <Label htmlFor="register-first" className="text-xs font-medium text-text">
                      First name
                    </Label>
                    <Input
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

          <CardFooter className="flex flex-col gap-2.5 border-t border-border py-4 px-6 text-center bg-surface-subtle rounded-b-2xl">
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
