import React, { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import {
  Package,
  Mail,
  Lock,
  Eye,
  EyeOff,
  ArrowRight,
  ArrowLeft,
  ShieldCheck,
  CheckCircle2,
  KeyRound,
  AlertCircle,
  RefreshCw,
} from 'lucide-react';

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

      const data = await res.json();

      if (!res.ok) {
        if (res.status === 429) {
          throw new Error('Please wait before requesting another reset code.');
        }
        throw new Error(data?.error?.message || 'Failed to send password reset code.');
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

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data?.error?.message || 'Failed to reset password.');
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
    <div className="min-h-screen bg-slate-950 flex flex-col justify-center relative">
      <div className="flex min-h-screen">
        {/* Left Hero Branding Column (Visible on Desktop) */}
        <div className="hidden lg:flex lg:w-1/2 relative bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900 border-r border-slate-800 flex-col justify-between p-12 overflow-hidden">
          <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-blue-600/15 rounded-full blur-3xl pointer-events-none" />
          <div className="absolute bottom-1/4 right-1/4 w-80 h-80 bg-indigo-600/10 rounded-full blur-3xl pointer-events-none" />

          {/* Top Brand Logo */}
          <div className="relative z-10 flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-blue-600 to-indigo-500 flex items-center justify-center shadow-lg shadow-blue-500/25">
              <Package className="w-6 h-6 text-white" />
            </div>
            <span className="text-xl font-bold tracking-tight text-white">
              Swift<span className="text-blue-400">Drop</span>
            </span>
          </div>

          {/* Middle Value Proposition */}
          <div className="relative z-10 my-auto max-w-lg space-y-6">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-500/10 border border-blue-500/20 text-blue-400 text-xs font-semibold">
              <ShieldCheck className="w-3.5 h-3.5" />
              Account Recovery
            </div>
            <h1 className="text-4xl font-extrabold text-white tracking-tight leading-tight">
              Securely recover access to your SwiftDrop account.
            </h1>
            <p className="text-slate-400 text-base leading-relaxed">
              We verify your identity with a single-use 6-digit code sent to your registered email address before updating your credentials.
            </p>
          </div>

          {/* Footer Note */}
          <div className="relative z-10 text-xs text-slate-500 flex items-center justify-between">
            <span>© 2026 SwiftDrop Logistics Platform</span>
            <span>Enterprise Edition v2.4</span>
          </div>
        </div>

        {/* Right Form Column */}
        <div className="w-full lg:w-1/2 flex items-center justify-center p-6 sm:p-12 relative">
          <div className="w-full max-w-md space-y-6">
            {/* Mobile Logo */}
            <div className="flex lg:hidden items-center gap-2 mb-6">
              <div className="w-8 h-8 rounded-lg bg-blue-600 flex items-center justify-center">
                <Package className="w-5 h-5 text-white" />
              </div>
              <span className="text-lg font-bold text-white">
                Swift<span className="text-blue-400">Drop</span>
              </span>
            </div>

            {success ? (
              <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 text-center space-y-4">
                <div className="w-12 h-12 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center mx-auto">
                  <CheckCircle2 className="w-6 h-6 text-emerald-400" />
                </div>
                <h2 className="text-xl font-bold text-white">Password Reset Successful</h2>
                <p className="text-xs text-slate-400 leading-relaxed">
                  Your password has been updated. You can now sign in to your SwiftDrop account using your new password.
                </p>
                <button
                  type="button"
                  onClick={() => navigate('/auth/login')}
                  className="w-full py-2.5 px-4 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white text-sm font-semibold rounded-xl shadow-lg shadow-blue-600/25 transition flex items-center justify-center gap-2 cursor-pointer"
                >
                  Back to Sign In
                  <ArrowRight className="w-4 h-4" />
                </button>
              </div>
            ) : (
              <>
                {/* Header */}
                <div>
                  <h2 className="text-2xl font-bold tracking-tight text-white">
                    {otpSent ? 'Verify code & set new password' : 'Forgot your password?'}
                  </h2>
                  <p className="text-slate-400 text-xs mt-1">
                    {otpSent
                      ? `Enter the 6-digit code sent to ${email} and choose a strong new password.`
                      : 'Enter your account email address and we will send you a 6-digit reset code.'}
                  </p>
                </div>

                {/* Error Banner */}
                {error && (
                  <div className="p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2.5">
                    <AlertCircle className="w-4 h-4 shrink-0" />
                    <span>{error}</span>
                  </div>
                )}

                {/* Info Banner */}
                {infoMessage && !error && (
                  <div className="p-3.5 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-300 text-xs flex items-center gap-2.5">
                    <CheckCircle2 className="w-4 h-4 shrink-0 text-blue-400" />
                    <span>{infoMessage}</span>
                  </div>
                )}

                {!otpSent ? (
                  /* Step 1: Request Reset OTP */
                  <form onSubmit={handleSendOTP} className="space-y-4">
                    <div>
                      <label className="block text-xs font-medium text-slate-300 mb-1.5">
                        Email Address
                      </label>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                          <Mail className="w-4 h-4" />
                        </div>
                        <input
                          type="email"
                          required
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          placeholder="you@company.com"
                          className="w-full pl-10 pr-4 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                        />
                      </div>
                    </div>

                    <button
                      type="submit"
                      disabled={loading}
                      className="w-full py-2.5 px-4 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white text-sm font-semibold rounded-xl shadow-lg shadow-blue-600/25 transition flex items-center justify-center gap-2 disabled:opacity-60 cursor-pointer"
                    >
                      {loading ? (
                        <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                      ) : (
                        <>
                          Send Reset Code
                          <ArrowRight className="w-4 h-4" />
                        </>
                      )}
                    </button>
                  </form>
                ) : (
                  /* Step 2: Enter OTP & New Password */
                  <form onSubmit={handleResetPassword} className="space-y-4">
                    <div>
                      <div className="flex items-center justify-between mb-1.5">
                        <label className="block text-xs font-medium text-slate-300">
                          6-Digit Reset Code
                        </label>
                        <button
                          type="button"
                          disabled={cooldown > 0 || loading}
                          onClick={() => handleSendOTP()}
                          className="inline-flex items-center gap-1 text-xs text-blue-400 hover:text-blue-300 disabled:text-slate-500 transition cursor-pointer disabled:cursor-not-allowed"
                        >
                          <RefreshCw className="w-3 h-3" />
                          {cooldown > 0 ? `Resend in ${cooldown}s` : 'Resend code'}
                        </button>
                      </div>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                          <KeyRound className="w-4 h-4" />
                        </div>
                        <input
                          type="text"
                          inputMode="numeric"
                          maxLength={6}
                          required
                          value={otp}
                          onChange={(e) => setOtp(e.target.value.replace(/\D/g, ''))}
                          placeholder="123456"
                          className="w-full pl-10 pr-4 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white tracking-widest font-mono placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                        />
                      </div>
                    </div>

                    <div>
                      <label className="block text-xs font-medium text-slate-300 mb-1.5">
                        New Password
                      </label>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                          <Lock className="w-4 h-4" />
                        </div>
                        <input
                          type={showNewPassword ? 'text' : 'password'}
                          required
                          value={newPassword}
                          onChange={(e) => setNewPassword(e.target.value)}
                          placeholder="••••••••••••"
                          className="w-full pl-10 pr-11 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                        />
                        <button
                          type="button"
                          onClick={() => setShowNewPassword(!showNewPassword)}
                          className="absolute inset-y-0 right-0 pr-3.5 flex items-center text-slate-500 hover:text-slate-300 transition"
                        >
                          {showNewPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                        </button>
                      </div>
                      <p className="text-[11px] text-slate-500 mt-1">
                        Minimum 8 characters with uppercase, lowercase, digit, and symbol.
                      </p>
                    </div>

                    <div>
                      <label className="block text-xs font-medium text-slate-300 mb-1.5">
                        Confirm New Password
                      </label>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                          <Lock className="w-4 h-4" />
                        </div>
                        <input
                          type={showConfirmPassword ? 'text' : 'password'}
                          required
                          value={confirmPassword}
                          onChange={(e) => setConfirmPassword(e.target.value)}
                          placeholder="••••••••••••"
                          className="w-full pl-10 pr-11 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                        />
                        <button
                          type="button"
                          onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                          className="absolute inset-y-0 right-0 pr-3.5 flex items-center text-slate-500 hover:text-slate-300 transition"
                        >
                          {showConfirmPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                        </button>
                      </div>
                    </div>

                    <div className="flex gap-2.5 pt-1">
                      <button
                        type="button"
                        onClick={() => {
                          setOtpSent(false);
                          setOtp('');
                          setError(null);
                          setInfoMessage(null);
                        }}
                        className="px-4 py-2.5 bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-300 text-xs font-semibold rounded-xl transition flex items-center gap-1.5 cursor-pointer"
                      >
                        <ArrowLeft className="w-3.5 h-3.5" />
                        Change Email
                      </button>
                      <button
                        type="submit"
                        disabled={loading}
                        className="flex-1 py-2.5 px-4 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white text-sm font-semibold rounded-xl shadow-lg shadow-blue-600/25 transition flex items-center justify-center gap-2 disabled:opacity-60 cursor-pointer"
                      >
                        {loading ? (
                          <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                        ) : (
                          <>
                            Reset Password
                            <ArrowRight className="w-4 h-4" />
                          </>
                        )}
                      </button>
                    </div>
                  </form>
                )}

                {/* Back to Login Link */}
                <p className="text-center text-xs text-slate-400">
                  Remember your password?{' '}
                  <Link to="/auth/login" className="font-semibold text-blue-400 hover:text-blue-300 transition">
                    Sign in
                  </Link>
                </p>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
