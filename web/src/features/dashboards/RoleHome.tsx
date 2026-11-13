import { Link } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { useAuth } from '@/app/providers/AuthContext';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader } from '@/shared/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { errorMessage, PortalError, portalRequest } from '@/features/customer-merchant/portal/api';
import { PortalHeader, PortalStatus } from '@/features/customer-merchant/portal/PortalLayout';
import { MerchantProfileBanner } from '@/features/customer-merchant/portal/MerchantProfileBanner';
import type { Role } from '@/shared/types/auth';

const homeByRole: Record<Extract<Role, 'CUSTOMER' | 'MERCHANT'>, { title: string; description: string; actions: { label: string; path: string }[] }> = {
  CUSTOMER: {
    title: 'Customer portal',
    description: 'Send a parcel or check the progress of a delivery.',
    actions: [
      { label: 'Book a parcel', path: '/portal/book' },
      { label: 'My parcel tracking', path: '/portal/tracking' },
    ],
  },
  MERCHANT: {
    title: 'Merchant portal',
    description: 'Manage your orders and integrations.',
    actions: [
      { label: 'View orders', path: '/portal/merchants/orders' },
      { label: 'My parcel tracking', path: '/portal/tracking' },
    ],
  },
};

type Shipment = {
  tracking_number: string;
  status: string;
  created_at: string;
};

type ShipmentResult = { rows: Shipment[]; error?: string; unavailable?: boolean };

function ShipmentOverview({ merchant, token }: { merchant: boolean; token: string }) {
  const [result, setResult] = useState<ShipmentResult>();
  const [attempt, setAttempt] = useState(0);
  const title = merchant ? 'Recent orders' : 'Recent parcels';

  useEffect(() => {
    const controller = new AbortController();
    const options = { token, signal: controller.signal };
    const request = merchant
      ? portalRequest<Shipment[]>('/merchant/orders', options)
      : portalRequest<{ items: Shipment[] }>('/parcels?page=1&page_size=20', { ...options, allowUnwrapped: true }).then(data => data.items);
    void request.then(rows => {
      if (!Array.isArray(rows)) throw new Error('Unexpected shipment list');
      if (!controller.signal.aborted) setResult({ rows });
    }).catch(cause => {
      if (controller.signal.aborted) return;
      if (!merchant && cause instanceof PortalError && [404, 405, 501].includes(cause.status)) {
        setResult({ rows: [], unavailable: true });
        return;
      }
      setResult({ rows: [], error: errorMessage(cause) });
    });
    return () => controller.abort();
  }, [merchant, token, attempt]);

  const rows = result?.rows ?? [];
  const recent = [...rows].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at)).slice(0, 6);
  const delivered = rows.filter(row => row.status === 'DELIVERED').length;
  const needsAttention = rows.filter(row => ['ATTEMPT_FAILED', 'NEEDS_ATTENTION', 'DELIVERY_FAILED'].includes(row.status)).length;
  const metrics = [
    { label: merchant ? 'Orders in list' : 'Parcels on latest page', value: rows.length },
    { label: 'Delivered', value: delivered },
    { label: 'Needs attention', value: needsAttention },
  ];

  return <>
    {result && !result.error && !result.unavailable && <section aria-label="Shipment summary" className="grid gap-3 sm:grid-cols-3">
      {metrics.map(({ label, value }) => <Card key={label}>
        <CardContent className="p-4">
          <p className="text-sm text-text-muted">{label}</p>
          <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
        </CardContent>
      </Card>)}
    </section>}
    <Card>
      <CardHeader><h2 className="text-lg font-semibold">{title}</h2></CardHeader>
      <CardContent>
        {!result && <p role="status">Loading {merchant ? 'orders' : 'parcels'}…</p>}
        {result?.error && <div className="space-y-3">
          <p role="alert">Couldn’t load {merchant ? 'orders' : 'parcels'}. {result.error}</p>
          <Button type="button" variant="outline" onClick={() => { setResult(undefined); setAttempt(value => value + 1); }}>Retry</Button>
        </div>}
        {result?.unavailable && <div className="space-y-3">
          <p className="text-text-muted">Recent parcels are unavailable on this server. You can still look up a parcel by its tracking number.</p>
          <Button asChild variant="outline"><Link to="/portal/tracking">Look up a parcel</Link></Button>
        </div>}
        {result && !result.error && !result.unavailable && recent.length === 0 && (
          <div className="space-y-3">
            <p className="text-text-muted">{merchant ? 'No orders yet.' : 'No parcels yet.'}</p>
            <Button asChild variant="outline">
              <Link to={merchant ? '/portal/merchants/orders' : '/portal/book'}>{merchant ? 'Open merchant orders' : 'Book your first parcel'}</Link>
            </Button>
          </div>
        )}
        {recent.length > 0 && <Table aria-label={title}>
          <TableHeader><TableRow><TableHead>Tracking number</TableHead><TableHead>Status</TableHead><TableHead>Booked</TableHead></TableRow></TableHeader>
          <TableBody>
            {recent.map(row => <TableRow key={row.tracking_number}>
              <TableCell>
                <Button asChild variant="link"><Link to={`/portal/tracking/${encodeURIComponent(row.tracking_number)}`}>{row.tracking_number}</Link></Button>
              </TableCell>
              <TableCell><PortalStatus status={row.status} /></TableCell>
              <TableCell><time dateTime={row.created_at}>{new Date(row.created_at).toLocaleDateString('en-LK')}</time></TableCell>
            </TableRow>)}
          </TableBody>
        </Table>}
      </CardContent>
    </Card>
  </>;
}

export function RoleHome() {
  const { user, accessToken } = useAuth();
  if (!user || (user.role !== 'CUSTOMER' && user.role !== 'MERCHANT')) return null;
  const home = homeByRole[user.role];

  return (
    <div data-role={user.role} className="min-h-screen bg-canvas text-text">
      <PortalHeader profile={user.role === 'MERCHANT' ? 'merchant' : 'customer'} />
      <main className="mx-auto max-w-4xl space-y-5 px-4 py-10 sm:px-6">
        <Card>
          <CardHeader>
            <h1 className="text-2xl font-semibold tracking-tight">{home.title}</h1>
            <CardDescription>{home.description}</CardDescription>
          </CardHeader>
          <CardContent>
            <nav aria-label="Workspace actions" className="flex flex-wrap gap-3">
              {home.actions.map(({ label, path }, index) => (
                <Button key={path} asChild variant={index === 0 ? 'default' : 'outline'}>
                  <Link to={path}>{label}</Link>
                </Button>
              ))}
            </nav>
          </CardContent>
        </Card>
        {user.role === 'MERCHANT' && accessToken && <MerchantProfileBanner key={accessToken} token={accessToken} />}
        {(user.role === 'CUSTOMER' || user.role === 'MERCHANT') && accessToken && (
          <ShipmentOverview key={`${user.id}:${user.role}:${user.tenant_id ?? ''}`} merchant={user.role === 'MERCHANT'} token={accessToken} />
        )}
        {(user.role === 'CUSTOMER' || user.role === 'MERCHANT') && <Card>
          <CardHeader><h2 className="text-lg font-semibold">{user.role === 'MERCHANT' ? 'Integrations' : 'Account'}</h2></CardHeader>
          <CardContent className="flex flex-wrap gap-3">
            {user.role === 'MERCHANT' ? <>
              <Button asChild variant="outline"><Link to="/portal/merchants/api-keys">API keys</Link></Button>
              <Button asChild variant="outline"><Link to="/portal/merchants/webhooks">Webhooks</Link></Button>
            </> : <Button asChild variant="outline"><Link to="/portal/merchants/apply">Apply for a merchant account</Link></Button>}
          </CardContent>
        </Card>}
      </main>
    </div>
  );
}
