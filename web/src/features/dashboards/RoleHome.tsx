import { Link } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { useAuth } from '@/app/providers/AuthContext';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardHeader } from '@/shared/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { errorMessage, PortalError, portalRequest } from '@/features/customer-merchant/portal/api';
import { PortalHeader, PortalStatus } from '@/features/customer-merchant/portal/PortalLayout';
import { MerchantProfileBanner } from '@/features/customer-merchant/portal/MerchantProfileBanner';
import { AssistantChat } from '@/features/customer-merchant/portal/AssistantChat';

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
        {result?.unavailable && <p className="text-text-muted">Recent parcels are currently unavailable.</p>}
        {result && !result.error && !result.unavailable && recent.length === 0 && (
          <p className="text-text-muted">{merchant ? 'No orders yet.' : 'No parcels yet.'}</p>
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
  const merchant = user.role === 'MERCHANT';

  return (
    <div data-role={user.role} className="min-h-screen bg-canvas text-text">
      <PortalHeader profile={user.role === 'MERCHANT' ? 'merchant' : 'customer'} />
      <main className="m6-main">
        <section className="portal-hero" aria-labelledby="portal-title">
          <img src={merchant ? '/merchant-fulfillment-hero.jpg' : '/customer-parcel-hero.jpg'} alt="" fetchPriority="high" />
          <div>
            <h1 id="portal-title" className="text-4xl font-semibold tracking-tight sm:text-5xl">
              {merchant ? 'Merchant portal' : 'Customer portal'}
            </h1>
            <p className="mt-3 max-w-sm text-base">
              {merchant
                ? 'Review recent orders and manage shipping for your business.'
                : 'Book deliveries and follow your parcels from pickup to delivery.'}
            </p>
          </div>
        </section>
        <div className="grid items-start gap-4 lg:grid-cols-3">
        <div className={merchant ? "min-w-0 space-y-4 lg:col-span-2" : "min-w-0 space-y-4 lg:col-span-3"}>
        {(user.role === 'CUSTOMER' || user.role === 'MERCHANT') && accessToken && (
          <ShipmentOverview key={`${user.id}:${user.role}:${user.tenant_id ?? ''}`} merchant={user.role === 'MERCHANT'} token={accessToken} />
        )}
        </div>
        {merchant && <aside className="min-w-0 space-y-4">
        {user.role === 'MERCHANT' && accessToken && <MerchantProfileBanner key={accessToken} token={accessToken} />}
        </aside>}
        </div>
      </main>
      {!merchant && accessToken && <AssistantChat key={user.id} token={accessToken} />}
    </div>
  );
}
