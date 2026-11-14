import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, Navigate } from 'react-router-dom';
import { Hammer, ShieldAlert } from 'lucide-react';
import { useAuth } from '@/app/providers/AuthContext';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardTitle } from '@/shared/components/ui/card';
import { AuditLogPanel } from '../admin-console/AuditLogPanel';
import { SettingsPanel } from '../admin-console/SettingsPanel';
import { UsersPanel } from '../admin-console/UsersPanel';
import { Banner } from '../admin-console/ui';
import { ReportsPanel } from '../reports/ReportsPanel';
import { ZonesPanel } from '../zones/ZonesPanel';
import { OPS_NAV, homePathFor, isOpsRole } from './opsNav';

// Pages of the ops workspace (route /ops/*). The real ones wrap the admin panels; the rest are
// honest placeholders that say the screen is not built yet and show no data.

const noop = () => undefined;

function PageHeader({ title, description }: { title: string; description: string }) {
  return (
    <div className="grid gap-1.5">
      <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
      <p className="text-sm text-text-muted">{description}</p>
    </div>
  );
}

/** A success message that clears itself after five seconds. */
function useNotice() {
  const [notice, setNotice] = useState<string | null>(null);
  const timer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  const show = useCallback((message: string) => {
    setNotice(message);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setNotice(null), 5000);
  }, []);
  const banner = notice ? (
    <Banner tone="ok" testId="ops-notice">
      {notice}
    </Banner>
  ) : null;
  return { banner, show };
}

function navItem(id: string) {
  const item = OPS_NAV.find((n) => n.id === id);
  if (!item) throw new Error(`Unknown ops page ${id}`);
  return item;
}

export function ZonesPage() {
  const { accessToken } = useAuth();
  const { banner, show } = useNotice();
  const item = navItem('zones');
  return (
    <>
      <PageHeader title={item.title} description={item.description} />
      {banner}
      <ZonesPanel token={accessToken} onNotice={show} onChanged={noop} />
    </>
  );
}

export function UsersPage() {
  const { accessToken, user } = useAuth();
  const { banner, show } = useNotice();
  const item = navItem('users');
  return (
    <>
      <PageHeader title={item.title} description={item.description} />
      {banner}
      <UsersPanel token={accessToken} currentUserId={user?.id} onNotice={show} onChanged={noop} />
    </>
  );
}

export function SettingsPage() {
  const { accessToken } = useAuth();
  const { banner, show } = useNotice();
  const item = navItem('settings');
  return (
    <>
      <PageHeader title={item.title} description={item.description} />
      {banner}
      <SettingsPanel token={accessToken} onNotice={show} onChanged={noop} />
    </>
  );
}

export function AuditPage() {
  const { accessToken } = useAuth();
  const item = navItem('audit');
  return (
    <>
      <PageHeader title={item.title} description={item.description} />
      <AuditLogPanel token={accessToken} refreshKey={0} />
    </>
  );
}

export function ReportsPage() {
  const { accessToken } = useAuth();
  const item = navItem('reports');
  return (
    <>
      <PageHeader title={item.title} description={item.description} />
      <ReportsPanel token={accessToken} />
    </>
  );
}

/** A screen that is planned but not built yet. It shows no data. */
export function NotBuiltPage({ id }: { id: string }) {
  const { user } = useAuth();
  const item = navItem(id);
  const home = isOpsRole(user?.role) ? homePathFor(user.role) : '/dashboard';
  return (
    <>
      <PageHeader title={item.title} description={item.description} />
      <Card data-testid="ops-not-built">
        <CardContent className="flex flex-col items-center gap-2 px-4 py-12 text-center">
          <Hammer size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
          <CardTitle>{item.title} is not built yet</CardTitle>
          <CardDescription>This screen is planned for the ops workspace. It shows no data until it is built.</CardDescription>
          {home !== item.path && (
            <Button asChild variant="outline" className="mt-2">
              <Link to={home}>Go to your start page</Link>
            </Button>
          )}
        </CardContent>
      </Card>
    </>
  );
}

/** The signed-in role may not open this page. The menu already hides it. */
export function ForbiddenPage() {
  const { user } = useAuth();
  const home = isOpsRole(user?.role) ? homePathFor(user.role) : '/dashboard';
  return (
    <Card data-testid="ops-forbidden">
      <CardContent className="flex flex-col items-center gap-2 px-4 py-12 text-center">
        <ShieldAlert size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
        <CardTitle>You can&apos;t open this page</CardTitle>
        <CardDescription>
          Your role ({user?.role ?? 'unknown'}) has no access to it. The pages you can open are in the menu.
        </CardDescription>
        <Button asChild variant="outline" className="mt-2">
          <Link to={home}>Go to your start page</Link>
        </Button>
      </CardContent>
    </Card>
  );
}

/** /ops sends each role to its start page. */
export function OpsIndex() {
  const { user } = useAuth();
  return <Navigate to={isOpsRole(user?.role) ? homePathFor(user.role) : '/dashboard'} replace />;
}

/** An /ops address that does not exist. */
export function OpsNotFound() {
  const { user } = useAuth();
  const home = isOpsRole(user?.role) ? homePathFor(user.role) : '/dashboard';
  return (
    <Card data-testid="ops-not-found">
      <CardContent className="flex flex-col items-center gap-2 px-4 py-12 text-center">
        <CardTitle>Page not found</CardTitle>
        <CardDescription>There is no page at this address.</CardDescription>
        <Button asChild variant="outline" className="mt-2">
          <Link to={home}>Go to your start page</Link>
        </Button>
      </CardContent>
    </Card>
  );
}
