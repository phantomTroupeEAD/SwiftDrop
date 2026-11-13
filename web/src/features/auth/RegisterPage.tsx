import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Eye, EyeOff, Package } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { withRequestReference } from '@/shared/lib/api-error';

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

      const data = await res.json().catch(() => null);

      if (!res.ok) {
        let message = data?.error?.message || 'Failed to send verification code.';
        if (res.status === 409) {
          message = 'An account with this email already exists. Please log in.';
        } else if (res.status === 429) {
          message = 'Please wait before requesting another verification code.';
        } else if (!data) {
          message = 'Email verification is unavailable right now. Please try again later.';
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

      const data = await res.json().catch(() => null);

      if (!res.ok) {
        const message = data?.error?.message || 'Registration is unavailable right now. Please try again later.';
        throw new Error(withRequestReference(message, res, data?.error?.correlation_id));
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
    <main data-role="PUBLIC" className="min-h-screen bg-canvas text-text px-4 py-12 sm:px-6">
      <div className="mx-auto flex w-full max-w-sm flex-col gap-6">
        <div className="flex items-center justify-between gap-3">
          <Link to="/" className="inline-flex items-center gap-2 font-semibold">
            <Package className="size-5 text-primary" aria-hidden="true" />
            SwiftDrop
          </Link>
          <Button asChild variant="link" size="sm"><Link to="/track">Track a parcel</Link></Button>
        </div>
        <section className="rounded-xl border border-border bg-surface p-5 sm:p-6" aria-labelledby="register-title">
          <h1 id="register-title" className="text-2xl font-semibold tracking-tight">
            {success ? 'Account created' : 'Create a customer account'}
          </h1>
          <p className="mt-1 break-words text-sm text-text-muted">
            {success ? 'Sign in with your email and password.' : otpSent
              ? `Enter the verification code sent to ${email.trim()}.`
              : 'Enter your email to receive a verification code.'}
          </p>
          {error && <p role="alert" className="mt-4 text-sm text-status-error-ink">{error}</p>}
          {success ? (
            <Button type="button" className="mt-6 w-full" size="lg" onClick={() => navigate('/auth/login')}>Sign in</Button>
          ) : !otpSent ? (
            <form className="mt-6 space-y-4" onSubmit={handleSendOTP}>
              <div className="space-y-2">
                <Label htmlFor="register-email">Email address</Label>
                <Input id="register-email" type="email" autoComplete="email" required
                  value={email} onChange={event => setEmail(event.target.value)} />
              </div>
              <Button type="submit" className="w-full" size="lg" disabled={loading}>
                {loading ? 'Sending code…' : 'Send verification code'}
              </Button>
            </form>
          ) : (
            <form className="mt-6 space-y-4" onSubmit={handleRegister}>
              <div className="flex flex-wrap items-center gap-3">
                <Button type="button" variant="outline" size="sm" disabled={loading} onClick={() => {
                  setOtpSent(false);
                  setOtp('');
                  setError(null);
                }}>Change email</Button>
                <Button type="button" variant="outline" size="sm" disabled={loading || cooldown > 0}
                  onClick={() => void handleSendOTP()}>
                  {cooldown > 0 ? `Resend in ${cooldown}s` : 'Resend code'}
                </Button>
              </div>
              <div className="space-y-2">
                <Label htmlFor="register-code">Verification code</Label>
                <Input id="register-code" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6}
                  required value={otp} onChange={event => setOtp(event.target.value.replace(/\D/g, ''))} />
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="register-first">First name</Label>
                  <Input id="register-first" autoComplete="given-name" required value={firstName}
                    onChange={event => setFirstName(event.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="register-last">Last name</Label>
                  <Input id="register-last" autoComplete="family-name" required value={lastName}
                    onChange={event => setLastName(event.target.value)} />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="register-phone">Phone number</Label>
                <Input id="register-phone" type="tel" autoComplete="tel" required value={phone}
                  onChange={event => setPhone(event.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="register-password">Password</Label>
                <div className="flex items-center gap-2">
                  <Input id="register-password" type={showPassword ? 'text' : 'password'} autoComplete="new-password"
                    aria-describedby="register-password-help" required value={password}
                    onChange={event => setPassword(event.target.value)} />
                  <Button type="button" variant="ghost" size="icon" aria-controls="register-password"
                    aria-label={showPassword ? 'Hide password' : 'Show password'}
                    onClick={() => setShowPassword(value => !value)}>
                    {showPassword ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
                  </Button>
                </div>
                <p id="register-password-help" className="text-xs text-text-muted">
                  Use at least 8 characters, including an uppercase letter, a number, and a symbol.
                </p>
              </div>
              <div className="flex items-start gap-2">
                <Checkbox id="register-terms" required checked={termsAccepted}
                  onCheckedChange={value => setTermsAccepted(value === true)} />
                <Label htmlFor="register-terms">I accept the Terms of Service and Privacy Policy.</Label>
              </div>
              <Button type="submit" className="w-full" size="lg" disabled={loading || !termsAccepted}>
                {loading ? 'Creating account…' : 'Create account'}
              </Button>
            </form>
          )}
        </section>
        {!success && <p className="text-center text-sm text-text-muted">
          Already have an account? <Button asChild variant="link"><Link to="/auth/login">Sign in</Link></Button>
        </p>}
      </div>
    </main>
  );
};
