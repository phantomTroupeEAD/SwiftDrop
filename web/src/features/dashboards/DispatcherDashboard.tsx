import { useEffect, useState } from 'react';
import { useAuth } from '@/app/providers/AuthContext';
import { portalRequest, errorMessage, statusLabel } from '@/features/customer-merchant/portal/api';
import { PortalHeader } from '@/features/customer-merchant/portal/PortalLayout';
import { Card, CardHeader, CardContent } from '@/shared/components/ui/card';
import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import { NativeSelect } from '@/shared/components/ui/native-select';

type Parcel = { id: string; tracking_number: string; recipient_city: string; weight_kg: number };
type Courier = { courier_id: string; name: string; employee_code: string; status: string; max_capacity: number; active_assignments: number };
type Board = { unassigned_parcels: Parcel[] | null; courier_loads: Courier[] | null };

export function DispatcherDashboard() {
  const { accessToken } = useAuth();
  const [board, setBoard] = useState<Board>();
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [revision, setRevision] = useState(0);
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [pending, setPending] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    void portalRequest<Board>('/dispatch/board', { token: accessToken, signal: controller.signal })
      .then(result => { if (!controller.signal.aborted) setBoard(result); })
      .catch(cause => { if (!controller.signal.aborted) setError(errorMessage(cause)); });
    return () => controller.abort();
  }, [accessToken, revision]);

  function refresh() {
    setError('');
    setBoard(undefined);
    setSelected({});
    setRevision(current => current + 1);
  }

  async function assign(parcel: Parcel) {
    setPending(true);
    setNotice('');
    try {
      await portalRequest('/dispatch/assignments', {
        token: accessToken, method: 'POST', body: { parcel_id: parcel.id, courier_id: selected[parcel.id] },
      });
      setNotice(parcel.tracking_number + ' assigned.');
      refresh();
    } catch (cause) {
      setNotice(errorMessage(cause));
    } finally {
      setPending(false);
    }
  }

  const parcels = board?.unassigned_parcels ?? [];
  const couriers = board?.courier_loads ?? [];
  const available = couriers.filter(courier => courier.status === 'ON_SHIFT' && courier.active_assignments < courier.max_capacity);

  return <div data-role="DISPATCHER" className="min-h-screen bg-canvas text-text">
    <PortalHeader profile="dispatcher" />
    <main className="m6-main" id="main-content">
      <section className="portal-hero" aria-labelledby="dispatch-title">
        <img src="/login-bg.jpg" alt="" fetchPriority="high" />
        <div><h1 id="dispatch-title" className="text-4xl font-semibold tracking-tight sm:text-5xl">Dispatch board</h1>
          <p className="mt-3 max-w-sm">Review parcels awaiting assignment and check courier capacity.</p></div>
      </section>
      {notice && <p className="mb-4" role="status">{notice}</p>}
      {error ? <Card><CardContent className="space-y-3 p-5">
        <p role="alert">Couldn’t load the dispatch board. {error}</p>
        <Button variant="outline" onClick={refresh} data-testid="dispatcher-retry-btn">Retry</Button>
      </CardContent></Card> : !board ? <p role="status">Loading dispatch board…</p> :
        <div className="grid items-start gap-4 lg:grid-cols-3" data-testid="dispatcher-board">
          <Card className="lg:col-span-2">
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
              <h2 className="text-lg font-semibold">Unassigned parcels</h2>
              <Badge variant="neutral" showDot={false}>{parcels.length} waiting</Badge>
            </CardHeader>
            <CardContent>
              {!parcels.length && <p className="text-sm text-text-muted">No parcels are waiting for assignment.</p>}
              <ul className="divide-y divide-border">
                {parcels.map(parcel => <li key={parcel.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                  <div><p className="font-mono font-semibold">{parcel.tracking_number}</p>
                    <p className="text-sm text-text-muted">{parcel.recipient_city} · {parcel.weight_kg} kg</p></div>
                  {available.length > 0 && <div className="flex flex-wrap gap-2">
                    <NativeSelect aria-label={'Courier for ' + parcel.tracking_number} data-testid="dispatcher-courier-select"
                      value={selected[parcel.id] ?? ''} disabled={pending}
                      onChange={event => setSelected(current => ({ ...current, [parcel.id]: event.target.value }))}>
                      <option value="">Choose courier</option>
                      {available.map(courier => <option key={courier.courier_id} value={courier.courier_id}>{courier.name || courier.employee_code}</option>)}
                    </NativeSelect>
                    <Button disabled={pending || !selected[parcel.id]} data-testid="dispatcher-assign-btn"
                      onClick={() => void assign(parcel)}>Assign courier</Button>
                  </div>}
                </li>)}
              </ul>
              {parcels.length > 0 && !available.length && <p className="text-sm text-text-muted">No on-shift couriers have spare capacity.</p>}
            </CardContent>
          </Card>
          <Card>
            <CardHeader><h2 className="text-lg font-semibold">Courier roster</h2></CardHeader>
            <CardContent>
              {!couriers.length && <p className="text-sm text-text-muted">No couriers are listed on this board.</p>}
              <ul className="divide-y divide-border">
                {couriers.map(courier => <li key={courier.courier_id} className="py-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="font-medium">{courier.name || courier.employee_code}</p>
                    <Badge variant={courier.status === 'ON_SHIFT' ? 'success' : 'neutral'} className="capitalize">{statusLabel(courier.status)}</Badge>
                  </div>
                  <p className="text-sm text-text-muted">{courier.active_assignments} of {courier.max_capacity} assignments</p>
                </li>)}
              </ul>
            </CardContent>
          </Card>
        </div>}
    </main>
  </div>;
}
