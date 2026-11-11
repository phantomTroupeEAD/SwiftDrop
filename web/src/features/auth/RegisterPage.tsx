import React, { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import {
  Package,
  Mail,
  Lock,
  Eye,
  EyeOff,
  ArrowRight,
  ShieldCheck,
  CheckCircle2,
  KeyRound,
  User,
  Phone,
  AlertCircle,
  RefreshCw,
} from 'lucide-react';

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

      const data = await res.json();

      if (!res.ok) {
        if (res.status === 409) {
          throw new Error('An account with this email already exists. Please log in.');
        } else if (res.status === 429) {
          throw new Error('Please wait before requesting another verification code.');
        } else {
          throw new Error(data?.error?.message || 'Failed to send verification code.');
        }
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

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data?.error?.message || 'Registration failed. Please check your inputs.');
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
    <div className="min-h-screen bg-slate-950 flex flex-col justify-center">
      <div className="flex min-h-screen">
        {/* Left Hero Branding Column (Desktop) */}
        <div className="hidden lg:flex lg:w-1/2 relative bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900 border-r border-slate-800 flex-col justify-between p-12 overflow-hidden">
          <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-blue-600/15 rounded-full blur-3xl pointer-events-none" />
          <div className="absolute bottom-1/4 right-1/4 w-80 h-80 bg-indigo-600/10 rounded-full blur-3xl pointer-events-none" />

          {/* Logo */}
          <div className="relative z-10 flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-blue-600 to-indigo-500 flex items-center justify-center shadow-lg shadow-blue-500/25">
              <Package className="w-6 h-6 text-white" />
            </div>
            <span className="text-xl font-bold tracking-tight text-white">
              Swift<span className="text-blue-400">Drop</span>
            </span>
          </div>

          {/* Main Message */}
          <div className="relative z-10 my-auto max-w-lg space-y-6">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-semibold">
              <ShieldCheck className="w-3.5 h-3.5" />
              Verified Identity & Protection
            </div>
            <h1 className="text-4xl font-extrabold text-white tracking-tight leading-tight">
              Join SwiftDrop and send parcels with complete confidence.
            </h1>
            <p className="text-slate-400 text-base leading-relaxed">
              Book on-demand deliveries, track couriers in real time, and receive instant proof-of-delivery receipts.
            </p>

            <div className="space-y-3 pt-2 text-sm text-slate-300">
              <div className="flex items-center gap-2.5">
                <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                <span>Instant OTP email ownership verification</span>
              </div>
              <div className="flex items-center gap-2.5">
                <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                <span>Zero address friction — book and ship anytime</span>
              </div>
              <div className="flex items-center gap-2.5">
                <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                <span>End-to-end secure parcel tracking with SMS alerts</span>
              </div>
            </div>
          </div>

          <div className="relative z-10 text-xs text-slate-500 border-t border-slate-800/80 pt-4 flex justify-between items-center">
            <span>© 2026 SwiftDrop Logistics Inc.</span>
            <span>Customer Portal v1.0.0</span>
          </div>
        </div>

        {/* Right Form Column */}
        <div className="w-full lg:w-1/2 flex flex-col justify-center items-center px-6 py-12 sm:px-12">
          <div className="w-full max-w-md space-y-6">
            {/* Header */}
            <div className="space-y-1 text-center lg:text-left">
              <h2 className="text-2xl font-bold tracking-tight text-white">
                {success ? 'Account Created!' : 'Create your Customer Account'}
              </h2>
              <p className="text-sm text-slate-400">
                {success
                  ? 'Your account has been verified and registered successfully.'
                  : otpSent
                  ? `Enter the 6-digit code sent to ${email} to complete setup.`
                  : 'Start by entering your email to receive a verification code.'}
              </p>
            </div>

            {/* Error Banner */}
            {error && (
              <div className="flex items-start gap-3 p-3.5 rounded-xl bg-red-950/50 border border-red-500/30 text-red-300 text-xs animate-shake">
                <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-red-400" />
                <div className="flex-1">
                  <div className="font-semibold text-red-200">Registration Error</div>
                  <div className="mt-0.5">{error}</div>
                </div>
              </div>
            )}

            {/* Success State Screen */}
            {success ? (
              <div className="p-6 rounded-2xl bg-emerald-950/30 border border-emerald-500/30 text-center space-y-4">
                <div className="w-12 h-12 mx-auto rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center">
                  <CheckCircle2 className="w-7 h-7" />
                </div>
                <div className="space-y-1">
                  <h3 className="text-lg font-bold text-white">Welcome, {firstName}!</h3>
                  <p className="text-xs text-slate-300">
                    Your account is active. You can now log in using your email and password.
                  </p>
                </div>
                <button
                  onClick={() => navigate('/auth/login')}
                  className="w-full py-2.5 px-4 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 text-white text-sm font-semibold rounded-xl shadow-lg shadow-blue-600/25 transition cursor-pointer flex items-center justify-center gap-2"
                >
                  Proceed to Login
                  <ArrowRight className="w-4 h-4" />
                </button>
              </div>
            ) : !otpSent ? (
              /* STEP 1: Email Form */
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
                      placeholder="alice@example.com"
                      className="w-full pl-10 pr-4 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                    />
                  </div>
                  <p className="text-[11px] text-slate-500 mt-1">
                    We will send a 6-digit verification code to confirm ownership.
                  </p>
                </div>

                <button
                  type="submit"
                  disabled={loading}
                  className="w-full py-2.5 px-4 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 text-white text-sm font-semibold rounded-xl shadow-lg shadow-blue-600/25 transition flex items-center justify-center gap-2 disabled:opacity-60 cursor-pointer"
                >
                  {loading ? (
                    <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                  ) : (
                    <>
                      Send Verification Code
                      <ArrowRight className="w-4 h-4" />
                    </>
                  )}
                </button>
              </form>
            ) : (
              /* STEP 2: Full Registration Form (with OTP) */
              <form onSubmit={handleRegister} className="space-y-3.5">
                {/* Email Chip with Resend */}
                <div className="flex items-center justify-between p-2.5 rounded-xl bg-slate-900 border border-slate-800 text-xs">
                  <div className="flex items-center gap-2 text-slate-300 truncate">
                    <Mail className="w-3.5 h-3.5 text-blue-400 shrink-0" />
                    <span className="truncate">{email}</span>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <button
                      type="button"
                      disabled={cooldown > 0 || loading}
                      onClick={() => handleSendOTP()}
                      className="text-blue-400 hover:text-blue-300 disabled:text-slate-600 transition flex items-center gap-1 cursor-pointer"
                    >
                      <RefreshCw className={`w-3 h-3 ${loading ? 'animate-spin' : ''}`} />
                      {cooldown > 0 ? `Resend (${cooldown}s)` : 'Resend'}
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setOtpSent(false);
                        setOtp('');
                        setError(null);
                      }}
                      className="text-slate-500 hover:text-slate-400 transition"
                    >
                      Change
                    </button>
                  </div>
                </div>

                {/* 6-Digit OTP Code */}
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    6-Digit Verification Code
                  </label>
                  <div className="relative">
                    <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                      <KeyRound className="w-4 h-4" />
                    </div>
                    <input
                      type="text"
                      required
                      maxLength={6}
                      value={otp}
                      onChange={(e) => setOtp(e.target.value.replace(/\D/g, ''))}
                      placeholder="849201"
                      className="w-full pl-10 pr-4 py-2 bg-slate-900 border border-slate-800 rounded-xl text-base tracking-widest font-mono text-emerald-400 placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-blue-500 transition"
                    />
                  </div>
                  <p className="text-[10px] text-slate-500 mt-0.5">
                    Check your email (or backend server console in development mode).
                  </p>
                </div>

                {/* Names (Grid) */}
                <div className="grid grid-cols-2 gap-2.5">
                  <div>
                    <label className="block text-xs font-medium text-slate-300 mb-1">
                      First Name
                    </label>
                    <div className="relative">
                      <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-500">
                        <User className="w-3.5 h-3.5" />
                      </div>
                      <input
                        type="text"
                        required
                        value={firstName}
                        onChange={(e) => setFirstName(e.target.value)}
                        placeholder="Alice"
                        className="w-full pl-9 pr-3 py-2 bg-slate-900 border border-slate-800 rounded-xl text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 transition"
                      />
                    </div>
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-slate-300 mb-1">
                      Last Name
                    </label>
                    <input
                      type="text"
                      required
                      value={lastName}
                      onChange={(e) => setLastName(e.target.value)}
                      placeholder="Johnson"
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-800 rounded-xl text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 transition"
                    />
                  </div>
                </div>

                {/* Phone Number */}
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Phone Number (for courier contact)
                  </label>
                  <div className="relative">
                    <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                      <Phone className="w-4 h-4" />
                    </div>
                    <input
                      type="tel"
                      required
                      value={phone}
                      onChange={(e) => setPhone(e.target.value)}
                      placeholder="+94771234567"
                      className="w-full pl-10 pr-4 py-2 bg-slate-900 border border-slate-800 rounded-xl text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 transition"
                    />
                  </div>
                </div>

                {/* Password */}
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Password
                  </label>
                  <div className="relative">
                    <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                      <Lock className="w-4 h-4" />
                    </div>
                    <input
                      type={showPassword ? 'text' : 'password'}
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="Min 8 chars, 1 uppercase, 1 digit, 1 symbol"
                      className="w-full pl-10 pr-10 py-2 bg-slate-900 border border-slate-800 rounded-xl text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 transition"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute inset-y-0 right-0 pr-3 flex items-center text-slate-500 hover:text-slate-300"
                    >
                      {showPassword ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>

                {/* Terms Checkbox */}
                <div className="pt-1">
                  <label className="flex items-start gap-2 cursor-pointer text-xs text-slate-400">
                    <input
                      type="checkbox"
                      required
                      checked={termsAccepted}
                      onChange={(e) => setTermsAccepted(e.target.checked)}
                      className="mt-0.5 w-4 h-4 rounded bg-slate-900 border-slate-700 text-blue-600 focus:ring-blue-500"
                    />
                    <span>
                      I accept the{' '}
                      <span className="text-blue-400 underline">Terms of Service</span> and{' '}
                      <span className="text-blue-400 underline">Privacy Policy</span>.
                    </span>
                  </label>
                </div>

                {/* Submit Button */}
                <button
                  type="submit"
                  disabled={loading}
                  className="w-full mt-2 py-2.5 px-4 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 text-white text-sm font-semibold rounded-xl shadow-lg shadow-blue-600/25 transition flex items-center justify-center gap-2 disabled:opacity-60 cursor-pointer"
                >
                  {loading ? (
                    <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                  ) : (
                    <>
                      Create Account
                      <ArrowRight className="w-4 h-4" />
                    </>
                  )}
                </button>
              </form>
            )}

            {/* Bottom Link to Login */}
            {!success && (
              <p className="text-center text-xs text-slate-400 pt-2">
                Already have an account?{' '}
                <Link to="/auth/login" className="font-semibold text-blue-400 hover:text-blue-300 transition">
                  Sign in
                </Link>
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
