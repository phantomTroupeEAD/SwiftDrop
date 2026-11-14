import { useEffect, useState, type ReactNode } from 'react';
import { Link, NavLink, useNavigate } from 'react-router-dom';
import { LogIn, LogOut, Package } from 'lucide-react';
import { useAuth } from '@/app/providers/AuthContext';
import { Button } from '@/shared/components/ui/button';
import { portalRequest, PortalError, statusLabel } from './api';

type PortalProfile = 'public' | 'customer' | 'merchant' | 'dispatcher' | 'courier';

export function PortalHeader({ profile, applicationStatus }: { profile: PortalProfile; applicationStatus?: string | null }) {
  const { user, logout, accessToken } = useAuth();
  const navigate = useNavigate();
  const [lookup, setLookup] = useState<{ token: string; status: string | null }>();
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
  return (
      <header className="m6-header">
        <Link className="m6-brand" to={profile === 'public' ? '/' : '/dashboard'}>
          <span className="m6-brand-mark"><Package size={19} aria-hidden="true" /></span>
          SwiftDrop
        </Link>
        {user
          ? <Button type="button" variant="outline" className="m6-session-action" onClick={() => { logout(); navigate('/auth/login'); }}><LogOut aria-hidden="true" /> Sign out</Button>
          : <Button asChild variant="outline" className="m6-session-action"><Link to="/auth/login"><LogIn aria-hidden="true" /> Sign in</Link></Button>}
        <nav aria-label="Portal navigation">
          {(profile === 'customer' || profile === 'merchant') && <NavLink to="/dashboard" end>Home</NavLink>}
          {profile === 'customer' && <NavLink to="/portal/book">Book a parcel</NavLink>}
          {profile === 'customer' && <NavLink to={status === 'APPROVED' ? '/merchant/dashboard' : '/portal/merchants/apply'}>
            {status === 'APPROVED' ? 'Open merchant dashboard' : status === null ? 'Apply for a merchant account' : 'View application'}
          </NavLink>}
          {profile === 'merchant' && <>
            <NavLink to="/portal/merchants/orders">Orders</NavLink>
            <NavLink to="/portal/merchants/api-keys">API keys</NavLink>
            <NavLink to="/portal/merchants/webhooks">Webhooks</NavLink>
          </>}
          {profile === 'public'
            ? <NavLink to="/track">Track a parcel</NavLink>
            : profile === 'courier' ? <NavLink to="/courier/dashboard" end>Delivery tasks</NavLink> : profile === 'dispatcher' ? <NavLink to="/dispatcher/dashboard" end>Dispatch board</NavLink> : <NavLink to="/portal/tracking">My parcel tracking</NavLink>}
          {profile === 'public' && user && <Link to="/dashboard">My dashboard</Link>}
        </nav>
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
