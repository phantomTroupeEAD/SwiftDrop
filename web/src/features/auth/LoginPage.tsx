/// <reference types="vite/client" />
import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useAuth } from '@/app/providers/AuthContext';
import {
  AlertCircle,
  Eye,
  EyeOff,
  Package,
  ArrowRight,
  ShieldCheck,
  Truck,
  Sparkles,
  AlertCircle,
  Phone,
  CheckCircle2,
} from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { Separator } from '@/shared/components/ui/separator';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '@/shared/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/ui/dialog';

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: {
            client_id: string;
            callback: (response: { credential?: string }) => void;
          }) => void;
          prompt: () => void;
          renderButton: (
            parent: HTMLElement,
            options: {
              type?: 'standard' | 'icon';
              theme?: 'outline' | 'filled_blue' | 'filled_black';
              size?: 'large' | 'medium' | 'small';
              text?: 'signin_with' | 'signup_with' | 'continue_with' | 'signin';
              shape?: 'rectangular' | 'pill' | 'circle' | 'square';
              logo_alignment?: 'left' | 'center';
              width?: number | string;
            }
          ) => void;
        };
      };
    };
  }
}

export const LoginPage: React.FC = () => {
  const { login, loginWithGoogle, updateProfile } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Google Sign-In State
  const [googleLoading, setGoogleLoading] = useState(false);
  const [googleReady, setGoogleReady] = useState(false);
  const googleClientId = import.meta.env.VITE_GOOGLE_CLIENT_ID as string | undefined;
  const googleButtonContainerRef = useRef<HTMLDivElement>(null);

  // Progressive Onboarding Modal (Approach A for Google OAuth)
  const [onboardingOpen, setOnboardingOpen] = useState(false);
  const [onboardingPhone, setOnboardingPhone] = useState('+9477');
  const [onboardingTerms, setOnboardingTerms] = useState(false);
  const [onboardingLoading, setOnboardingLoading] = useState(false);
  const [onboardingError, setOnboardingError] = useState<string | null>(null);

  const redirectToDashboard = () => navigate('/dashboard', { replace: true });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      await login(email, password);
      redirectToDashboard();
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

  const handleGoogleSignIn = useCallback(async (token: string) => {
    setGoogleLoading(true);
    setError(null);

    try {
      const { user: authedUser, requiresPhone } = await loginWithGoogle(token);

      if (requiresPhone) {
        setOnboardingOpen(true);
      } else {
        redirectToDashboard();
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Google Sign-In failed.');
      }
    } finally {
      setGoogleLoading(false);
    }
  }, [loginWithGoogle]);

  useEffect(() => {
    if (!googleClientId) return;

    const initGoogle = () => {
      if (!window.google?.accounts?.id) return;
      window.google.accounts.id.initialize({
        client_id: googleClientId,
        callback: (response) => {
          if (response.credential) {
            void handleGoogleSignIn(response.credential);
          } else {
            setError('Google Sign-In did not return a credential.');
          }
        },
      });
      setGoogleReady(true);

      if (googleButtonContainerRef.current) {
        googleButtonContainerRef.current.innerHTML = '';
        window.google.accounts.id.renderButton(googleButtonContainerRef.current, {
          theme: 'outline',
          size: 'large',
          type: 'standard',
          text: 'signin_with',
          shape: 'rectangular',
          logo_alignment: 'left',
          width: 360,
        });
      }
    };

    if (window.google?.accounts?.id) {
      initGoogle();
      return;
    }

    const script = document.createElement('script');
    script.src = 'https://accounts.google.com/gsi/client';
    script.async = true;
    script.defer = true;
    script.onload = initGoogle;
    document.head.appendChild(script);
  }, [googleClientId, handleGoogleSignIn]);

  const handleGoogleButtonClick = () => {
    setError(null);
    if (!googleClientId) {
      setError('Google Sign-In is not configured. Set VITE_GOOGLE_CLIENT_ID.');
      return;
    }
    if (!googleReady || !window.google?.accounts?.id) {
      setError('Google Sign-In is still loading. Please try again in a moment.');
      return;
    }
    window.google.accounts.id.prompt();
  };

  const handleOnboardingSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setOnboardingLoading(true);
    setOnboardingError(null);

    try {
      await updateProfile(onboardingPhone, onboardingTerms);
      setOnboardingOpen(false);
      redirectToDashboard();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setOnboardingError(err.message);
      } else {
        setOnboardingError('Failed to complete onboarding profile.');
      }
    } finally {
      setOnboardingLoading(false);
    }
  };

  return (
    <main data-role="PUBLIC" className="relative min-h-screen bg-canvas text-text flex flex-col justify-between antialiased selection:bg-primary-tint selection:text-primary overflow-hidden">
      {/* Strategic Background: Cinematic Logistics Artwork with Ambient Blur & Contrast Scrim */}
      <div className="absolute inset-0 pointer-events-none select-none overflow-hidden" aria-hidden="true">
        <img
          src="/login-bg.jpg"
          alt=""
          className="h-full w-full object-cover object-center filter blur-sm scale-105"
        />

        {/* Soft atmospheric glow orbs */}
        <div className="absolute -top-32 -left-32 size-96 rounded-full bg-primary-tint blur-3xl opacity-50" />
        <div className="absolute -bottom-32 -right-32 size-96 rounded-full bg-primary-tint blur-3xl opacity-40" />

        {/* Backdrop Scrim & Blur Veil for Contrast and Legibility across Light/Dark modes */}
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

      {/* Centralized Clean Solid Login Window */}
      <div className="relative z-10 mx-auto flex w-full max-w-[420px] flex-col gap-4 px-4 my-auto">
        <Card className="border border-border bg-surface rounded-2xl">
          <CardHeader className="text-center space-y-2 pb-5 pt-7">
            <div className="mx-auto flex size-11 items-center justify-center rounded-xl bg-primary-tint text-primary border border-border mb-1">
              <Package className="size-5" aria-hidden="true" />
            </div>
            <div className="space-y-1">
              <CardTitle className="text-xl font-bold tracking-tight text-text">
                Welcome back
              </CardTitle>
              <CardDescription className="text-xs text-text-muted">
                Sign in to your SwiftDrop account to continue
              </CardDescription>
            </div>
          </CardHeader>

          <CardContent className="space-y-4 px-6">
            {error && (
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-lg border border-status-error-border bg-status-error-bg p-3 text-xs text-status-error-ink font-medium leading-relaxed"
              >
                <AlertCircle className="size-4 shrink-0 mt-0.5" aria-hidden="true" />
                <span>{error}</span>
              </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="login-email" className="text-xs font-medium text-text">
                  Email address
                </Label>
                <Input
                  id="login-email"
                  data-testid="login-email-input"
                  type="email"
                  autoComplete="username"
                  required
                  placeholder="name@swiftdrop.local"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="login-password" className="text-xs font-medium text-text">
                  Password
                </Label>
                <div className="relative">
                  <Input
                    id="login-password"
                    data-testid="login-password-input"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="current-password"
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
                    aria-controls="login-password"
                    aria-label={showPassword ? 'Hide password' : 'Show password'}
                    onClick={() => setShowPassword((current) => !current)}
                  >
                    {showPassword ? (
                      <EyeOff className="size-4" aria-hidden="true" />
                    ) : (
                      <Eye className="size-4" aria-hidden="true" />
                    )}
                  </Button>
                </div>
              </div>

              <Button
                type="submit"
                data-testid="login-submit-btn"
                size="lg"
                className="w-full mt-2"
                disabled={loading || googleLoading}
              >
                {loading ? (
                  <>
                    <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
                    Signing in…
                  </>
                ) : (
                  'Sign in'
                )}
              </Button>
            </form>

            {import.meta.env.DEV && (
              <div className="space-y-3 pt-1">
                <div className="relative flex items-center justify-center">
                  <Separator className="w-full" />
                  <span className="absolute bg-surface px-2 text-xs uppercase tracking-wider text-text-muted">
                    developer testing
                  </span>
                </div>

                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="w-full text-xs text-text-muted hover:text-text"
                  disabled={loading || googleLoading}
                  onClick={() => setGoogleModalOpen(true)}
                >
                  <svg className="size-3.5 mr-2 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
                    <path
                      fill="currentColor"
                      d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                    />
                    <path
                      fill="currentColor"
                      d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                    />
                    <path
                      fill="currentColor"
                      d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
                    />
                    <path
                      fill="currentColor"
                      d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
                    />
                  </svg>
                  Google Sign-In test dialog
                </Button>
              </div>
            )}
          </CardContent>

          <CardFooter className="flex flex-col gap-2.5 border-t border-border py-4 px-6 text-center bg-surface-subtle rounded-b-2xl">
            <p className="text-xs text-text-muted">
              Don’t have an account?{' '}
              <Button asChild variant="link" className="text-xs font-semibold text-primary">
                <Link to="/auth/register">Create one</Link>
              </Button>
            </p>

          </CardFooter>
        </Card>


      </div>

            {/* Google OAuth Button */}
            <div className="w-full flex flex-col items-center justify-center min-h-[44px]">
              <div
                ref={googleButtonContainerRef}
                className={`w-full flex justify-center ${googleReady ? '' : 'hidden'}`}
              />
              {!googleReady && (
                <button
                  type="button"
                  onClick={handleGoogleButtonClick}
                  disabled={googleLoading}
                  className="w-full py-2.5 px-4 bg-slate-900 hover:bg-slate-800/80 border border-slate-800 rounded-xl text-xs font-semibold text-slate-300 flex items-center justify-center gap-2.5 transition cursor-pointer"
                >
                  {googleLoading ? (
                    <div className="w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
                  ) : (
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
                  )}
                  Sign in with Google
                </button>
              )}
            </div>

            {/* Register Link */}
            <p className="text-center text-xs text-slate-400">
              Don't have an account?{' '}
              <Link to="/auth/register" className="font-semibold text-blue-400 hover:text-blue-300 transition">
                Create an account
              </Link>
            </p>
          </div>
        </div>
      </div>

      {/* ── Progressive Onboarding Modal (Approach A: Collect Phone & Terms) ── */}
      {onboardingOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/90 backdrop-blur-md animate-fadeIn">
          <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-2xl space-y-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center">
                <Phone className="w-5 h-5 text-amber-400" />
              </div>
              <div>
                <h3 className="text-base font-bold text-white">Complete Your Account</h3>
                <p className="text-slate-400 text-xs">One final step to activate delivery notifications</p>
              </div>
            </div>

            <p className="text-xs text-slate-300 leading-relaxed">
              Google authenticated your email successfully! To ensure couriers can contact you and coordinate parcel drop-offs, please provide your mobile phone number.
            </p>

            {onboardingError && (
              <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{onboardingError}</span>
              </div>
            )}

            <form onSubmit={handleOnboardingSubmit} className="space-y-4 pt-1">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Mobile Phone Number (E.164 International Format)
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                    <Phone className="w-4 h-4" />
                  </div>
                  <input
                    type="tel"
                    required
                    value={onboardingPhone}
                    onChange={(e) => setOnboardingPhone(e.target.value)}
                    placeholder="+94771234567"
                    className="w-full pl-10 pr-4 py-2.5 bg-slate-950 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </div>
                <p className="text-[11px] text-slate-500 mt-1">Example: +94771234567 or +14155552671</p>
              </div>

              <div className="p-3 rounded-xl bg-slate-950/60 border border-slate-800/80">
                <label className="flex items-start gap-2.5 cursor-pointer text-xs text-slate-300 leading-tight">
                  <input
                    type="checkbox"
                    checked={onboardingTerms}
                    onChange={(e) => setOnboardingTerms(e.target.checked)}
                    className="mt-0.5 w-4 h-4 rounded bg-slate-900 border-slate-700 text-blue-600 focus:ring-blue-500"
                  />
                  <span>
                    I agree to the SwiftDrop{' '}
                    <span className="text-blue-400 underline">Terms of Service</span> and{' '}
                    <span className="text-blue-400 underline">Privacy Policy</span>.
                  </span>
                </label>
              </div>

              <button
                type="submit"
                disabled={onboardingLoading || !onboardingTerms}
                className="w-full py-2.5 px-4 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 disabled:opacity-50 text-white text-sm font-semibold rounded-xl shadow-lg shadow-blue-600/25 transition flex items-center justify-center gap-2 cursor-pointer"
              >
                {onboardingLoading ? (
                  <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                ) : (
                  <>
                    <CheckCircle2 className="w-4 h-4" />
                    Complete Setup & Go to Dashboard
                  </>
                )}
              </button>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
