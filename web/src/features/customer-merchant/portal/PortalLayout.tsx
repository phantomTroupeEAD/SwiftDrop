import type { ReactNode } from 'react';
import { Link, NavLink, useNavigate } from 'react-router-dom';
import { Package } from 'lucide-react';
import { useAuth } from '@/app/providers/AuthContext';
import { statusLabel } from './api';
import './portal.css';

type PortalProfile = 'public' | 'customer' | 'merchant';

export function PortalLayout({ title, profile, children }: { title: string; profile: PortalProfile; children: ReactNode }) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  return (
    <div className="m6-portal" data-profile={profile} data-role={profile.toUpperCase()}>
      <header className="m6-header">
        <Link className="m6-brand" to={profile === 'public' ? '/track' : '/dashboard'}><Package size={20} aria-hidden="true" /> SwiftDrop</Link>
        <nav aria-label="Portal navigation">
          {profile !== 'public' && <NavLink to="/dashboard" end>Home</NavLink>}
          {profile === 'customer' && <NavLink to="/portal/book">Book a parcel</NavLink>}
          {profile === 'customer' && <NavLink to="/portal/merchants/apply">Apply for a merchant account</NavLink>}
          {profile === 'merchant' && <>
            <NavLink to="/portal/merchants/orders">Orders</NavLink>
            <NavLink to="/portal/merchants/api-keys">API keys</NavLink>
            <NavLink to="/portal/merchants/webhooks">Webhooks</NavLink>
          </>}
          {profile === 'public'
            ? <NavLink to="/track">Track a parcel</NavLink>
            : <NavLink to="/portal/tracking">My parcel tracking</NavLink>}
          {profile === 'public' && user && <Link to="/dashboard">My dashboard</Link>}
          {user
            ? <button type="button" className="m6-quiet" onClick={() => { logout(); navigate('/auth/login'); }}>Sign out</button>
            : <NavLink to="/auth/login">Sign in</NavLink>}
        </nav>
      </header>
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
