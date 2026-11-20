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
  Loader2,
} from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { Separator } from '@/shared/components/ui/separator';
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

// Where each role lands after sign-in (shown on the right-hand panel).
const LOGIN_ROLES = [
  { name: 'CUSTOMER', does: 'Book pickups, track parcels, print receipts' },
  { name: 'MERCHANT', does: 'Orders, CSV uploads, API keys and webhooks' },
  { name: 'DISPATCHER', does: 'Assign waiting parcels to couriers' },
  { name: 'COURIER', does: 'Today’s run, proof of delivery, failed attempts' },
  { name: 'ADMIN', does: 'Zones, pricing, users and merchant approvals' },
];

export const LoginPage: React.FC = () => {
  const { login, loginWithGoogle, updateProfile } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Google Sign-In & Dev Selector State
  const [googleModalOpen, setGoogleModalOpen] = useState(false);
  const [googleTokenInput, setGoogleTokenInput] = useState('');
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
      const { requiresPhone } = await loginWithGoogle(token);
      setGoogleModalOpen(false);

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
    <main data-role="PUBLIC" className="grid min-h-screen bg-canvas text-text lg:grid-cols-2">
      <div className="flex flex-col px-4 py-6 sm:px-8">
        <header className="flex items-center justify-between gap-4">
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
          <Button asChild variant="ghost" size="sm" className="text-text-muted hover:text-text">
            <Link to="/track">
              Track a parcel
              <ArrowRight className="ml-1.5 size-3.5" aria-hidden="true" />
            </Link>
          </Button>
        </header>

        <div className="mx-auto my-auto flex w-full max-w-[400px] flex-col gap-6 py-12">
          <div className="space-y-2">
            <h1 className="font-display text-4xl font-bold tracking-tight">Sign in</h1>
            <p className="text-text-muted">Use the account you book with or work with.</p>
          </div>
          <div className="space-y-4">
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
                <div className="flex items-center justify-between">
                  <Label htmlFor="login-password" className="text-xs font-medium text-text">
                    Password
                  </Label>
                  <Button asChild variant="link" className="text-xs font-normal text-text-muted hover:text-primary p-0 h-auto">
                    <Link to="/auth/forgot-password">Forgot password?</Link>
                  </Button>
                </div>
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

            {/* Google OAuth Button */}
            <div className="space-y-3 pt-1">
              <div className="relative flex items-center justify-center">
                <Separator className="w-full" />
                <span className="absolute bg-canvas px-2 text-xs uppercase tracking-wider text-text-muted">
                  or continue with
                </span>
              </div>

              <div className="w-full flex flex-col items-center justify-center min-h-[44px]">
                <div
                  ref={googleButtonContainerRef}
                  className={`w-full flex justify-center ${googleReady ? '' : 'hidden'}`}
                />
                {!googleReady && (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleGoogleButtonClick}
                    disabled={googleLoading}
                    className="w-full"
                  >
                    {googleLoading ? (
                      <>
                        <Loader2 className="mr-2 size-4 animate-spin text-primary" aria-hidden="true" />
                        Connecting…
                      </>
                    ) : (
                      <>
                        <svg className="size-4 mr-2" viewBox="0 0 24 24" aria-hidden="true">
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
                        Sign in with Google
                      </>
                    )}
                  </Button>
                )}
              </div>
            </div>

            {import.meta.env.DEV && (
              <div className="space-y-3 pt-1">
                <div className="relative flex items-center justify-center">
                  <Separator className="w-full" />
                  <span className="absolute bg-canvas px-2 text-xs uppercase tracking-wider text-text-muted">
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
          </div>

          <p className="text-center text-sm text-text-muted">
            Don’t have an account?{' '}
            <Button asChild variant="link" className="text-sm font-semibold text-primary-hover">
              <Link to="/auth/register">Create one</Link>
            </Button>
          </p>
        </div>

        <footer className="text-center text-xs text-text-subtle">
          <p>© 2026 SwiftDrop</p>
        </footer>
      </div>

      <aside className="hidden flex-col justify-center gap-8 bg-chrome p-14 text-chrome-foreground lg:flex" aria-labelledby="login-roles-title">
        <h2 id="login-roles-title" className="max-w-md font-display text-4xl font-bold leading-tight tracking-tight">
          One sign-in. You land on the workspace for your role.
        </h2>
        <dl className="max-w-lg border-b border-chrome-line">
          {LOGIN_ROLES.map((role) => (
            <div key={role.name} className="grid grid-cols-[7.5rem_minmax(0,1fr)] gap-4 border-t border-chrome-line py-4">
              <dt className="font-mono text-sm font-medium text-chrome-accent">{role.name}</dt>
              <dd className="text-chrome-muted">{role.does}</dd>
            </div>
          ))}
        </dl>
      </aside>

      {/* Google Dev Testing Dialog */}
      <Dialog open={googleModalOpen} onOpenChange={setGoogleModalOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base font-semibold">Google sign-in test</DialogTitle>
            <DialogDescription className="text-xs text-text-muted">
              Choose a test account or enter a Google ID token.
            </DialogDescription>
          </DialogHeader>

          {error && (
            <p role="alert" className="text-xs text-status-error-ink">
              {error}
            </p>
          )}

          <div className="flex flex-wrap gap-2 pt-1">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={googleLoading}
              onClick={() => handleGoogleSignIn('mock_google_id_token_alice')}
            >
              New customer (Alice)
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={googleLoading}
              onClick={() => handleGoogleSignIn('mock_customer@swiftdrop.local')}
            >
              Existing customer
            </Button>
          </div>

          <form
            className="space-y-3 pt-2"
            onSubmit={(event) => {
              event.preventDefault();
              if (googleTokenInput.trim() && !googleLoading) {
                void handleGoogleSignIn(googleTokenInput.trim());
              }
            }}
          >
            <div className="space-y-1.5">
              <Label htmlFor="google-token" className="text-xs font-medium text-text">
                Google ID token
              </Label>
              <Input
                id="google-token"
                type="password"
                value={googleTokenInput}
                autoComplete="off"
                placeholder="mock_google_id_token_..."
                onChange={(event) => setGoogleTokenInput(event.target.value)}
              />
            </div>
            <Button
              type="submit"
              className="w-full text-xs"
              disabled={!googleTokenInput.trim() || googleLoading}
            >
              {googleLoading ? (
                <>
                  <Loader2 className="mr-2 size-3.5 animate-spin" aria-hidden="true" />
                  Signing in…
                </>
              ) : (
                'Continue with token'
              )}
            </Button>
          </form>
        </DialogContent>
      </Dialog>

      {/* Progressive Onboarding Modal (Approach A) */}
      <Dialog open={onboardingOpen}>
        <DialogContent
          onEscapeKeyDown={(event) => event.preventDefault()}
          onInteractOutside={(event) => event.preventDefault()}
          className="sm:max-w-md [&>button]:hidden"
        >
          <DialogHeader>
            <DialogTitle className="text-base font-semibold">Complete your account</DialogTitle>
            <DialogDescription className="text-xs text-text-muted">
              Add a phone number for live delivery updates and agree to terms.
            </DialogDescription>
          </DialogHeader>

          {onboardingError && (
            <p role="alert" className="text-xs text-status-error-ink">
              {onboardingError}
            </p>
          )}

          <form onSubmit={handleOnboardingSubmit} className="space-y-4 pt-2">
            <div className="space-y-1.5">
              <Label htmlFor="onboarding-phone" className="text-xs font-medium text-text">
                Mobile phone number
              </Label>
              <Input
                id="onboarding-phone"
                type="tel"
                autoComplete="tel"
                required
                value={onboardingPhone}
                placeholder="+94771234567"
                onChange={(event) => setOnboardingPhone(event.target.value)}
              />
              <p className="text-xs text-text-subtle">Example: +94771234567 or +14155552671</p>
            </div>

            <div className="flex items-start gap-2.5">
              <Checkbox
                id="onboarding-terms"
                checked={onboardingTerms}
                onCheckedChange={(checked) => setOnboardingTerms(checked === true)}
                className="mt-0.5"
              />
              <Label
                htmlFor="onboarding-terms"
                className="text-xs font-normal leading-relaxed text-text-muted cursor-pointer"
              >
                I agree to the Terms of Service and Privacy Policy.
              </Label>
            </div>

            <Button
              type="submit"
              className="w-full text-xs"
              disabled={!onboardingTerms || onboardingLoading}
            >
              {onboardingLoading ? (
                <>
                  <Loader2 className="mr-2 size-3.5 animate-spin" aria-hidden="true" />
                  Completing setup…
                </>
              ) : (
                'Complete setup'
              )}
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </main>
  );
};
