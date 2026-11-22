import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom';
import { LogIn, LogOut, Menu, Package, X } from 'lucide-react';
import { useAuth } from '@/app/providers/AuthContext';
import { Button } from '@/shared/components/ui/button';
import { Separator } from '@/shared/components/ui/separator';
import { NotificationBell } from '@/shared/components/NotificationBell';
import { cn } from '@/shared/lib/utils';
import { portalRequest, PortalError, statusLabel } from './api';

type PortalProfile = 'public' | 'customer' | 'merchant' | 'dispatcher' | 'courier';

interface NavItem {
  to: string;
  label: string;
  end?: boolean;
}

export function PortalHeader({ profile, applicationStatus }: { profile: PortalProfile; applicationStatus?: string | null }) {
  const { user, logout, accessToken } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const headerRef = useRef<HTMLElement>(null);
  const [lookup, setLookup] = useState<{ token: string; status: string | null }>();

  // Close mobile dropdown on route change
  useEffect(() => {
    setMobileMenuOpen(false);
  }, [location.pathname]);

  // Close mobile dropdown when clicking outside header
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (headerRef.current && !headerRef.current.contains(event.target as Node)) {
        setMobileMenuOpen(false);
      }
    }
    if (mobileMenuOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [mobileMenuOpen]);

  useEffect(() => {
    if (profile !== 'customer' || !accessToken || applicationStatus !== undefined) return;
    const token = accessToken;
    const controller = new AbortController();
    async function loadStatus() {
      try {
        const result = await portalRequest<{ status: string }>('/merchants/me', { token, signal: controller.signal });
        if (!controller.signal.aborted) setLookup({ token, status: result.status });
      } catch (error) {
        if (!controller.signal.aborted && error instanceof PortalError && error.status === 404) {
          setLookup({ token, status: null });
        }
      }
    }
    void loadStatus();
    window.addEventListener('focus', loadStatus);
    return () => {
      controller.abort();
      window.removeEventListener('focus', loadStatus);
    };
  }, [profile, accessToken, applicationStatus]);
  const status = applicationStatus === undefined && lookup?.token === accessToken ? lookup?.status : applicationStatus;

  const navLinks: NavItem[] = [];
  if (profile === 'customer' || profile === 'merchant') {
    navLinks.push({ to: '/dashboard', label: 'Home', end: true });
  }
  if (profile === 'customer') {
    navLinks.push({ to: '/portal/book', label: 'Book a parcel' });
    navLinks.push({
      to: status === 'APPROVED' ? '/merchant/dashboard' : '/portal/merchants/apply',
      label: status === 'APPROVED' ? 'Open merchant dashboard' : status === null ? 'Apply for a merchant account' : 'View application',
    });
    navLinks.push({ to: '/portal/tracking', label: 'My parcel tracking' });
  }
  if (profile === 'merchant') {
    navLinks.push({ to: '/portal/merchants/orders', label: 'Orders' });
    navLinks.push({ to: '/portal/merchants/api-keys', label: 'API keys' });
    navLinks.push({ to: '/portal/merchants/webhooks', label: 'Webhooks' });
    navLinks.push({ to: '/portal/merchants/docs', label: 'API docs' });
  }
  if (profile === 'public') {
    navLinks.push({ to: '/track', label: 'Track a parcel' });
    if (user) {
      navLinks.push({ to: '/dashboard', label: 'My dashboard' });
    }
  }
  if (profile === 'courier') {
    navLinks.push({ to: '/courier/dashboard', label: 'Delivery tasks', end: true });
  }
  if (profile === 'dispatcher') {
    navLinks.push({ to: '/dispatcher/dashboard', label: 'Dispatch board', end: true });
  }

  return (
      <header ref={headerRef} className="m6-header">
        <Link className="m6-brand" to={profile === 'public' ? '/' : '/dashboard'}>
          <span className="m6-brand-mark"><Package size={19} aria-hidden="true" /></span>
          SwiftDrop
        </Link>

        {/* Desktop Navigation Links */}
        <nav aria-label="Portal navigation" className="max-md:!hidden md:flex">
          {navLinks.map((link) => (
            <NavLink key={link.to} to={link.to} end={link.end}>
              {link.label}
            </NavLink>
          ))}
        </nav>

        {/* Right-aligned Header Actions */}
        <div className="flex items-center gap-2 ml-auto order-1 md:order-2">
          {user && <NotificationBell />}
          {user ? (
            <Button
              type="button"
              variant="outline"
              className="m6-session-action hidden md:inline-flex"
              onClick={() => {
                logout();
                navigate('/auth/login');
              }}
              data-testid="portal-signout-btn"
            >
              <LogOut aria-hidden="true" className="w-4 h-4 mr-1" />
              <span>Sign out</span>
            </Button>
          ) : (
            <Button asChild variant="outline" className="m6-session-action hidden md:inline-flex">
              <Link to="/auth/login">
                <LogIn aria-hidden="true" className="w-4 h-4 mr-1" />
                <span>Sign in</span>
              </Link>
            </Button>
          )}

          {/* Mobile Menu Toggle Button */}
          {navLinks.length > 0 && (
            <div className="md:hidden">
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={() => setMobileMenuOpen((open) => !open)}
                aria-label={mobileMenuOpen ? 'Close navigation menu' : 'Open navigation menu'}
                aria-expanded={mobileMenuOpen}
                data-testid="portal-mobile-menu-btn"
              >
                {mobileMenuOpen ? (
                  <X className="w-4 h-4" aria-hidden="true" />
                ) : (
                  <Menu className="w-4 h-4" aria-hidden="true" />
                )}
              </Button>
            </div>
          )}
        </div>

        {/* Mobile Dropdown Menu (Inline dropdown beneath header, not a popup) */}
        {mobileMenuOpen && (
          <div
            role="navigation"
            aria-label="Mobile navigation"
            className="w-full basis-full order-3 md:hidden flex flex-col gap-1 pt-3 pb-2 border-t border-border mt-2 animate-in fade-in-0 slide-in-from-top-2 duration-150"
            data-testid="portal-mobile-dropdown"
          >
            {navLinks.map((link) => (
              <NavLink
                key={link.to}
                to={link.to}
                end={link.end}
                onClick={() => setMobileMenuOpen(false)}
                className={({ isActive }) =>
                  cn(
                    "flex items-center px-3 py-2.5 rounded-sm text-sm font-medium transition-colors min-h-[44px]",
                    isActive
                      ? "bg-surface-subtle text-text font-semibold border-l-2 border-primary pl-2.5"
                      : "text-text-muted hover:text-text hover:bg-surface-subtle"
                  )
                }
              >
                {link.label}
              </NavLink>
            ))}

            <Separator className="my-2" />

            {user ? (
              <Button
                type="button"
                variant="ghost"
                className="w-full justify-start px-3 py-2.5 h-auto min-h-[44px] text-sm font-medium text-status-error-ink hover:text-status-error-ink hover:bg-surface-subtle rounded-sm"
                onClick={() => {
                  setMobileMenuOpen(false);
                  logout();
                  navigate('/auth/login');
                }}
                data-testid="portal-mobile-signout-btn"
              >
                <LogOut className="w-4 h-4 mr-2" aria-hidden="true" />
                Sign out
              </Button>
            ) : (
              <Button
                asChild
                variant="ghost"
                className="w-full justify-start px-3 py-2.5 h-auto min-h-[44px] text-sm font-medium text-text hover:bg-surface-subtle rounded-sm"
              >
                <Link to="/auth/login" onClick={() => setMobileMenuOpen(false)}>
                  <LogIn className="w-4 h-4 mr-2" aria-hidden="true" />
                  Sign in
                </Link>
              </Button>
            )}
          </div>
        )}
      </header>
  );
}

export function PortalLayout({ title, profile, children, applicationStatus }: { title: string; profile: PortalProfile; children: ReactNode; applicationStatus?: string | null }) {
  return (
    <div className="m6-portal" data-profile={profile} data-role={profile.toUpperCase()}>
      <PortalHeader profile={profile} applicationStatus={applicationStatus} />
      <main className="m6-main" id="main-content">
        <h1>{title}</h1>
        {children}
      </main>
    </div>
  );
}

export function PortalErrorMessage({ message }: { message: string }) {
  return message ? <p role="alert" className="m6-error">{message}</p> : null;
}

const statusTones: Record<string, string> = {
  ASSIGNED: 'info', PICKED_UP: 'info', IN_TRANSIT: 'info', OUT_FOR_DELIVERY: 'info',
  DELIVERED: 'success', APPROVED: 'success', ACTIVE: 'success',
  ATTEMPT_FAILED: 'warning', NEEDS_ATTENTION: 'warning', SUSPENDED: 'warning', RETURNED: 'warning',
  DELIVERY_FAILED: 'error', REJECTED: 'error', FAILED: 'error',
};

export function PortalStatus({ status, testId }: { status: string; testId?: string }) {
  const tone = statusTones[status.toUpperCase()] ?? 'neutral';
  return <span className="m6-pill" data-tone={tone} data-testid={testId}>{statusLabel(status)}</span>;
}
