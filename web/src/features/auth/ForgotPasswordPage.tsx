import React, { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import {
  Package,
  Eye,
  EyeOff,
  ArrowRight,
  ArrowLeft,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  Loader2,
} from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
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

export const ForgotPasswordPage: React.FC = () => {
  const navigate = useNavigate();

  // Step 1: Email & OTP request
  const [email, setEmail] = useState('');
  const [otpSent, setOtpSent] = useState(false);
  const [cooldown, setCooldown] = useState(0);

  // Step 2: OTP & New Password
  const [otp, setOtp] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  // UI States
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [infoMessage, setInfoMessage] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  // Countdown timer for resend OTP cooldown
  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => {
      setCooldown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  // Handle Step 1: Send Reset OTP
  const handleSendOTP = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!email.trim()) {
      setError('Please enter your email address.');
      return;
    }

    setLoading(true);
    setError(null);
    setInfoMessage(null);

    try {
      const res = await fetch('/api/v1/auth/forgot-password/send-otp', {
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
        if (res.status === 429) {
          throw new Error('Please wait before requesting another reset code.');
        }
        if (data?.error?.message) {
          throw new Error(data.error.message);
        }
        if (res.status === 502 || res.status === 504 || res.status === 500) {
          throw new Error('Backend server is unreachable. Please ensure the Core API is running on port 8080.');
        }
        throw new Error(`Failed to send password reset code (${res.status}).`);
      }

      setOtpSent(true);
      setCooldown(data?.data?.cooldown_seconds || 60);
      setInfoMessage(
        data?.data?.message ||
          'If an account with that email exists, a password reset code has been sent.'
      );
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

  // Handle Step 2: Verify OTP & Reset Password
  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault();

    if (newPassword !== confirmPassword) {
      setError('Passwords do not match.');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await fetch('/api/v1/auth/forgot-password/reset', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: email.trim(),
          otp: otp.trim(),
          new_password: newPassword,
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
        }
        if (res.status === 502 || res.status === 504 || res.status === 500) {
          throw new Error('Backend server is unreachable. Please ensure the Core API is running on port 8080.');
        }
        throw new Error(`Failed to reset password (${res.status}).`);
      }

      setSuccess(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Network error while resetting password.');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <main data-role="PUBLIC" className="relative min-h-screen bg-canvas text-text flex flex-col justify-between antialiased selection:bg-primary-tint selection:text-primary overflow-hidden">
      {/* Background Image Layer with Ambient Blur & Contrast Scrim */}
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

      {/* Centralized Card Window */}
      <div className="relative z-10 mx-auto flex w-full max-w-md flex-col gap-4 px-4 my-auto py-6">
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
                {success
                  ? 'Password reset successful'
                  : otpSent
                  ? 'Verify code & set new password'
                  : 'Forgot your password?'}
              </CardTitle>
              <CardDescription className="text-xs text-text-muted max-w-sm mx-auto">
                {success
                  ? 'Your password has been updated. You can now sign in using your new credentials.'
                  : otpSent
                  ? `Enter the 6-digit code sent to ${email} and choose a strong new password.`
                  : 'Enter your account email address and we will send you a 6-digit reset code.'}
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

            {infoMessage && !error && (
              <div
                role="status"
                className="flex items-start gap-2.5 rounded-lg border border-status-success-border bg-status-success-bg p-3 text-xs text-status-success-ink font-medium leading-relaxed"
              >
                <CheckCircle2 className="size-4 shrink-0 mt-0.5 text-status-success-ink" aria-hidden="true" />
                <span>{infoMessage}</span>
              </div>
            )}

            {success ? (
              <div className="pt-2">
                <Button
                  type="button"
                  size="lg"
                  className="w-full"
                  onClick={() => navigate('/auth/login')}
                >
                  Back to sign in
                  <ArrowRight className="ml-2 size-4" aria-hidden="true" />
                </Button>
              </div>
            ) : !otpSent ? (
              /* Step 1: Request Reset OTP */
              <form onSubmit={handleSendOTP} className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="reset-email" className="text-xs font-medium text-text">
                    Email address
                  </Label>
                  <Input
                    id="reset-email"
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="name@swiftdrop.local"
                    autoComplete="email"
                  />
                </div>

                <Button
                  type="submit"
                  size="lg"
                  className="w-full mt-2"
                  disabled={loading}
                >
                  {loading ? (
                    <>
                      <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
                      Sending code…
                    </>
                  ) : (
                    <>
                      Send reset code
                      <ArrowRight className="ml-2 size-4" aria-hidden="true" />
                    </>
                  )}
                </Button>
              </form>
            ) : (
              /* Step 2: Enter OTP & New Password */
              <form onSubmit={handleResetPassword} className="space-y-4">
                <div className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="reset-otp" className="text-xs font-medium text-text">
                      6-digit reset code
                    </Label>
                    <Button
                      type="button"
                      variant="link"
                      size="sm"
                      disabled={cooldown > 0 || loading}
                      onClick={() => handleSendOTP()}
                      className="text-xs font-medium text-primary p-0 h-auto"
                    >
                      <RefreshCw className="mr-1 size-3" aria-hidden="true" />
                      {cooldown > 0 ? `Resend in ${cooldown}s` : 'Resend code'}
                    </Button>
                  </div>
                  <Input
                    id="reset-otp"
                    type="text"
                    inputMode="numeric"
                    maxLength={6}
                    required
                    value={otp}
                    onChange={(e) => setOtp(e.target.value.replace(/\D/g, ''))}
                    placeholder="123456"
                    className="tracking-widest font-mono"
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="reset-new-password" className="text-xs font-medium text-text">
                    New password
                  </Label>
                  <div className="relative">
                    <Input
                      id="reset-new-password"
                      type={showNewPassword ? 'text' : 'password'}
                      required
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      placeholder="••••••••"
                      className="pr-10"
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="absolute right-0 top-0 h-[38px] w-[38px] text-text-muted hover:text-text hover:bg-transparent"
                      aria-controls="reset-new-password"
                      aria-label={showNewPassword ? 'Hide password' : 'Show password'}
                      onClick={() => setShowNewPassword((curr) => !curr)}
                    >
                      {showNewPassword ? (
                        <EyeOff className="size-4" aria-hidden="true" />
                      ) : (
                        <Eye className="size-4" aria-hidden="true" />
                      )}
                    </Button>
                  </div>
                  <p className="text-xs text-text-subtle">
                    Minimum 8 characters with uppercase, lowercase, digit, and symbol.
                  </p>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="reset-confirm-password" className="text-xs font-medium text-text">
                    Confirm new password
                  </Label>
                  <div className="relative">
                    <Input
                      id="reset-confirm-password"
                      type={showConfirmPassword ? 'text' : 'password'}
                      required
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      placeholder="••••••••"
                      className="pr-10"
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="absolute right-0 top-0 h-[38px] w-[38px] text-text-muted hover:text-text hover:bg-transparent"
                      aria-controls="reset-confirm-password"
                      aria-label={showConfirmPassword ? 'Hide password' : 'Show password'}
                      onClick={() => setShowConfirmPassword((curr) => !curr)}
                    >
                      {showConfirmPassword ? (
                        <EyeOff className="size-4" aria-hidden="true" />
                      ) : (
                        <Eye className="size-4" aria-hidden="true" />
                      )}
                    </Button>
                  </div>
                </div>

                <div className="flex gap-2.5 pt-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="lg"
                    onClick={() => {
                      setOtpSent(false);
                      setOtp('');
                      setError(null);
                      setInfoMessage(null);
                    }}
                    className="gap-1.5"
                  >
                    <ArrowLeft className="size-3.5" aria-hidden="true" />
                    Change email
                  </Button>
                  <Button
                    type="submit"
                    size="lg"
                    className="flex-1"
                    disabled={loading}
                  >
                    {loading ? (
                      <>
                        <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
                        Resetting…
                      </>
                    ) : (
                      <>
                        Reset password
                        <ArrowRight className="ml-2 size-4" aria-hidden="true" />
                      </>
                    )}
                  </Button>
                </div>
              </form>
            )}
          </CardContent>

          <CardFooter className="flex flex-col gap-2.5 border-t border-border py-4 px-6 text-center bg-surface-subtle rounded-b-2xl">
            <p className="text-xs text-text-muted">
              Remember your password?{' '}
              <Button asChild variant="link" className="text-xs font-semibold text-primary">
                <Link to="/auth/login">Sign in</Link>
              </Button>
            </p>
          </CardFooter>
        </Card>
      </div>

      {/* Bottom Footer Note */}
      <footer className="relative z-10 px-4 py-4 text-center text-xs text-text-subtle">
        <p>© 2026 SwiftDrop Logistics Platform. Enterprise Edition v2.4</p>
      </footer>
    </main>
  );
};
