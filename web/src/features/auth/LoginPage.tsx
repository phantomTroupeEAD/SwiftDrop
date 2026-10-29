import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/app/providers/AuthContext';
import { Role } from '@/shared/types/auth';
import {
  Package,
  Mail,
  Lock,
  Eye,
  EyeOff,
  ArrowRight,
  ShieldCheck,
  Truck,
  Sparkles,
  AlertCircle,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';

interface QuickAccount {
  role: Role;
  name: string;
  email: string;
  password: string;
  badge: string;
}

const QUICK_ACCOUNTS: QuickAccount[] = [
  { role: 'ADMIN', name: 'System Admin', email: 'admin@swiftdrop.local', password: 'AdminPassword123!', badge: 'bg-purple-500/10 text-purple-400 border-purple-500/20' },
  { role: 'DISPATCHER', name: 'Dispatcher', email: 'dispatcher@swiftdrop.local', password: 'Dispatcher123!', badge: 'bg-sky-500/10 text-sky-400 border-sky-500/20' },
  { role: 'COURIER', name: 'Courier', email: 'courier@swiftdrop.local', password: 'CourierPassword123!', badge: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' },
  { role: 'MERCHANT', name: 'Merchant', email: 'merchant@vendor.com', password: 'Merchant123!', badge: 'bg-amber-500/10 text-amber-400 border-amber-500/20' },
  { role: 'CUSTOMER', name: 'Customer', email: 'customer@swiftdrop.local', password: 'CustomerPassword123!', badge: 'bg-blue-500/10 text-blue-400 border-blue-500/20' },
];

export const LoginPage: React.FC = () => {
  const { login } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showDemoBar, setShowDemoBar] = useState(true);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const loggedUser = await login(email, password);
      // Redirect based on role
      if (loggedUser.role === 'ADMIN' || loggedUser.role === 'DISPATCHER') {
        navigate('/ops/dashboard');
      } else if (loggedUser.role === 'COURIER') {
        navigate('/courier/dashboard');
      } else {
        navigate('/portal/dashboard');
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Login failed. Please check your credentials.');
      }
    } finally {
      setLoading(false);
    }
  };

  const fillQuickAccount = (acc: QuickAccount) => {
    setEmail(acc.email);
    setPassword(acc.password);
    setError(null);
  };

  return (
    <div className="min-h-screen bg-slate-950 flex flex-col justify-center">
      <div className="flex min-h-screen">
        {/* Left Hero Branding Column (Visible on Desktop) */}
        <div className="hidden lg:flex lg:w-1/2 relative bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900 border-r border-slate-800 flex-col justify-between p-12 overflow-hidden">
          {/* Subtle Ambient Glow */}
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
              <Sparkles className="w-3.5 h-3.5" />
              Enterprise Delivery Network
            </div>
            <h1 className="text-4xl font-extrabold text-white tracking-tight leading-tight">
              Fast, reliable delivery & logistics operations at scale.
            </h1>
            <p className="text-slate-400 text-base leading-relaxed">
              Experience end-to-end transparency with real-time parcel dispatch, automated tariff calculation, and auditable proof of delivery.
            </p>

            {/* Feature Pills */}
            <div className="grid grid-cols-2 gap-3 pt-2">
              <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800/80 backdrop-blur-sm">
                <div className="flex items-center gap-2.5 text-slate-200 text-sm font-semibold mb-1">
                  <Truck className="w-4 h-4 text-blue-400" />
                  Live Dispatch
                </div>
                <p className="text-xs text-slate-400">Real-time courier fleet assignment & routing.</p>
              </div>

              <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800/80 backdrop-blur-sm">
                <div className="flex items-center gap-2.5 text-slate-200 text-sm font-semibold mb-1">
                  <ShieldCheck className="w-4 h-4 text-emerald-400" />
                  Secure Multi-Tenant
                </div>
                <p className="text-xs text-slate-400">Role-based isolation & signed JWT authorization.</p>
              </div>
            </div>
          </div>

          {/* Footer Quality Tag */}
          <div className="relative z-10 text-xs text-slate-500 border-t border-slate-800/80 pt-4 flex justify-between items-center">
            <span>© 2026 SwiftDrop Logistics Inc.</span>
            <span>Version 1.0.0</span>
          </div>
        </div>

        {/* Right Form Column */}
        <div className="w-full lg:w-1/2 flex flex-col justify-center items-center px-6 py-12 sm:px-12">
          <div className="w-full max-w-md space-y-6">
            {/* Mobile Logo Only */}
            <div className="flex lg:hidden items-center gap-2.5 justify-center mb-4">
              <div className="w-9 h-9 rounded-xl bg-blue-600 flex items-center justify-center">
                <Package className="w-5 h-5 text-white" />
              </div>
              <span className="text-xl font-bold text-white">Swift<span className="text-blue-400">Drop</span></span>
            </div>

            {/* Heading */}
            <div className="space-y-1 text-center lg:text-left">
              <h2 className="text-2xl font-bold tracking-tight text-white">Sign in to your account</h2>
              <p className="text-sm text-slate-400">
                Welcome back! Please enter your credentials to access your portal.
              </p>
            </div>

            {/* Error Notification */}
            {error && (
              <div className="flex items-start gap-3 p-3.5 rounded-xl bg-red-950/50 border border-red-500/30 text-red-300 text-xs animate-shake">
                <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-red-400" />
                <div className="flex-1">
                  <div className="font-semibold text-red-200">Authentication Failed</div>
                  <div className="mt-0.5">{error}</div>
                </div>
              </div>
            )}

            {/* Login Form */}
            <form onSubmit={handleSubmit} className="space-y-4">
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

              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <label className="block text-xs font-medium text-slate-300">
                    Password
                  </label>
                  <a href="#forgot" className="text-xs text-blue-400 hover:text-blue-300 transition">
                    Forgot password?
                  </a>
                </div>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                    <Lock className="w-4 h-4" />
                  </div>
                  <input
                    type={showPassword ? 'text' : 'password'}
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="••••••••••••"
                    className="w-full pl-10 pr-11 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute inset-y-0 right-0 pr-3.5 flex items-center text-slate-500 hover:text-slate-300 transition"
                  >
                    {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              <div className="flex items-center justify-between pt-0.5">
                <label className="flex items-center gap-2 cursor-pointer text-xs text-slate-400">
                  <input
                    type="checkbox"
                    checked={rememberMe}
                    onChange={(e) => setRememberMe(e.target.checked)}
                    className="w-4 h-4 rounded bg-slate-900 border-slate-700 text-blue-600 focus:ring-blue-500 focus:ring-offset-slate-950"
                  />
                  Remember me for 30 days
                </label>
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
                    Sign In
                    <ArrowRight className="w-4 h-4" />
                  </>
                )}
              </button>
            </form>

            {/* Social OIDC Divider */}
            <div className="relative my-4">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-slate-800" />
              </div>
              <div className="relative flex justify-center text-xs uppercase">
                <span className="bg-slate-950 px-3 text-slate-500 font-medium">Or continue with</span>
              </div>
            </div>

            {/* Google OAuth Button */}
            <button
              type="button"
              onClick={() => alert("Google OAuth flow initiated per SRS §8.1. Redirecting to Google OIDC...")}
              className="w-full py-2.5 px-4 bg-slate-900 hover:bg-slate-800/80 border border-slate-800 rounded-xl text-xs font-semibold text-slate-300 flex items-center justify-center gap-2.5 transition"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24">
                <path
                  fill="#4285F4"
                  d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                />
                <path
                  fill="#34A853"
                  d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                />
                <path
                  fill="#FBBC05"
                  d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
                />
                <path
                  fill="#EA4335"
                  d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
                />
              </svg>
              Sign in with Google
            </button>

            {/* Register Link */}
            <p className="text-center text-xs text-slate-400">
              Don't have an account?{' '}
              <a href="#register" className="font-semibold text-blue-400 hover:text-blue-300 transition">
                Create an account
              </a>
            </p>

            {/* Subtle, Collapsible Team Demo Switcher at Bottom */}
            <div className="pt-4 border-t border-slate-900">
              <button
                type="button"
                onClick={() => setShowDemoBar(!showDemoBar)}
                className="w-full flex items-center justify-between text-[11px] text-slate-500 hover:text-slate-400 py-1 transition"
              >
                <span>🧪 Quick Demo Roles (For Team Development)</span>
                {showDemoBar ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
              </button>

              {showDemoBar && (
                <div className="mt-2.5 p-2 rounded-xl bg-slate-900/40 border border-slate-800/60 flex flex-wrap gap-1.5 justify-center">
                  {QUICK_ACCOUNTS.map((acc) => (
                    <button
                      key={acc.role}
                      type="button"
                      onClick={() => fillQuickAccount(acc)}
                      className={`text-[10px] font-medium px-2 py-1 rounded-lg border transition hover:opacity-100 ${
                        email === acc.email ? 'ring-1 ring-blue-500 opacity-100' : 'opacity-70'
                      } ${acc.badge}`}
                      title={acc.email}
                    >
                      {acc.role}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
