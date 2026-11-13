import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useAuth } from '@/app/providers/AuthContext';
import { AlertCircle, Eye, EyeOff, Package } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/shared/components/ui/dialog';

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

  const handleGoogleSignIn = async (token: string) => {
    setGoogleLoading(true);
    setError(null);

    try {
      const { requiresPhone } = await loginWithGoogle(token);
      setGoogleModalOpen(false);

      if (requiresPhone) {
        // Approach A: prompt for phone number and terms acceptance
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
    <main data-role="PUBLIC" className="min-h-screen bg-canvas text-text px-4 py-12 sm:px-6">
      <div className="mx-auto flex w-full max-w-sm flex-col gap-6">
        <div className="flex items-center justify-between gap-3">
          <Link to="/" className="inline-flex items-center gap-2 font-semibold">
            <Package className="size-5 text-primary" aria-hidden="true" />
            SwiftDrop
          </Link>
          <Button asChild variant="link" size="sm"><Link to="/track">Track a parcel</Link></Button>
        </div>

        <section className="rounded-xl border border-border bg-surface p-5 sm:p-6" aria-labelledby="sign-in-title">
          <h1 id="sign-in-title" className="text-2xl font-semibold tracking-tight">Sign in</h1>
          <p className="mt-1 text-sm text-text-muted">Use your SwiftDrop email and password.</p>

          {error && <p role="alert" className="mt-4 flex items-start gap-2 text-sm text-status-error-ink">
            <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />{error}
          </p>}

          <form onSubmit={handleSubmit} className="mt-6 space-y-4">
            <div className="space-y-2">
              <Label htmlFor="login-email">Email address</Label>
              <Input id="login-email" type="email" autoComplete="username" required
                value={email} onChange={event => setEmail(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="login-password">Password</Label>
              <div className="flex items-center gap-2">
                <Input id="login-password" type={showPassword ? 'text' : 'password'} autoComplete="current-password" required
                  value={password} onChange={event => setPassword(event.target.value)} />
                <Button type="button" variant="ghost" size="icon" aria-controls="login-password"
                  aria-label={showPassword ? 'Hide password' : 'Show password'}
                  onClick={() => setShowPassword(current => !current)}>
                  {showPassword ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
                </Button>
              </div>
            </div>
            <Button type="submit" size="lg" className="w-full" disabled={loading || googleLoading}>
              {loading ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>

          {import.meta.env.DEV && (
            <Button type="button" variant="outline" className="mt-3 w-full" disabled={loading || googleLoading}
              onClick={() => setGoogleModalOpen(true)}>
              Google sign-in test
            </Button>
          )}
        </section>

        <p className="text-center text-sm text-text-muted">
          New to SwiftDrop? <Button asChild variant="link"><Link to="/auth/register">Create an account</Link></Button>
        </p>
        {import.meta.env.DEV && (
          <Button asChild variant="link" size="sm">
            <Link to="/design-preview" data-testid="login-theme-preview-link">Preview customer and merchant screens</Link>
          </Button>
        )}
      </div>

      <Dialog open={googleModalOpen} onOpenChange={setGoogleModalOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Google sign-in test</DialogTitle>
            <DialogDescription>Choose a test account or enter a Google ID token.</DialogDescription>
          </DialogHeader>
          {error && <p role="alert" className="text-sm text-status-error-ink">{error}</p>}
          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="outline" disabled={googleLoading}
              onClick={() => handleGoogleSignIn('mock_google_id_token_alice')}>New customer</Button>
            <Button type="button" variant="outline" disabled={googleLoading}
              onClick={() => handleGoogleSignIn('mock_customer@swiftdrop.local')}>Existing customer</Button>
          </div>
          <form className="space-y-3" onSubmit={event => {
            event.preventDefault();
            if (googleTokenInput.trim() && !googleLoading) void handleGoogleSignIn(googleTokenInput.trim());
          }}>
            <Label htmlFor="google-token">Google ID token</Label>
            <Input id="google-token" type="password" value={googleTokenInput} autoComplete="off"
              onChange={event => setGoogleTokenInput(event.target.value)} />
            <Button type="submit" disabled={!googleTokenInput.trim() || googleLoading}>
              {googleLoading ? 'Signing in…' : 'Continue'}
            </Button>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={onboardingOpen}>
        <DialogContent onEscapeKeyDown={event => event.preventDefault()}
          onInteractOutside={event => event.preventDefault()} className="[&>button]:hidden">
          <DialogHeader>
            <DialogTitle>Complete your account</DialogTitle>
            <DialogDescription>Add a phone number for delivery updates.</DialogDescription>
          </DialogHeader>
          {onboardingError && <p role="alert" className="text-sm text-status-error-ink">{onboardingError}</p>}
          <form onSubmit={handleOnboardingSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="onboarding-phone">Phone number</Label>
              <Input id="onboarding-phone" type="tel" autoComplete="tel" required
                value={onboardingPhone} onChange={event => setOnboardingPhone(event.target.value)} />
            </div>
            <label className="flex items-start gap-2 text-sm">
              <input type="checkbox" required checked={onboardingTerms}
                onChange={event => setOnboardingTerms(event.target.checked)} />
              I agree to the Terms of Service and Privacy Policy.
            </label>
            <Button type="submit" className="w-full" disabled={!onboardingTerms || onboardingLoading}>
              {onboardingLoading ? 'Saving…' : 'Complete setup'}
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </main>
  );
};
