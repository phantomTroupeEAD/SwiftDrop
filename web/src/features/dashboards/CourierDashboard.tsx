import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useAuth } from '@/app/providers/AuthContext';
import { PortalHeader } from '@/features/customer-merchant/portal/PortalLayout';
import { portalRequest, errorMessage, statusLabel } from '@/features/customer-merchant/portal/api';
import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import { Card, CardHeader, CardContent } from '@/shared/components/ui/card';
import { Input } from '@/shared/components/ui/input';
import { NativeSelect } from '@/shared/components/ui/native-select';

type Task = {
  parcel_id: string; tracking_number: string; current_status: string;
  recipient_name: string; recipient_phone: string; delivery_street: string; delivery_city: string;
  handling_instructions?: string;
};
const reasons = ['RECIPIENT_UNAVAILABLE', 'CUSTOMER_REQUESTED_RESCHEDULE', 'ACCESS_RESTRICTED',
  'INVALID_ADDRESS', 'REFUSED_BY_RECIPIENT', 'DAMAGED_IN_TRANSIT', 'OTHER'];

export function CourierDashboard() {
  const { accessToken } = useAuth();
  const [tasks, setTasks] = useState<Task[]>();
  const [revision, setRevision] = useState(0);
  const [loadError, setLoadError] = useState('');
  const [actionError, setActionError] = useState('');
  const [notice, setNotice] = useState('');
  const [shift, setShift] = useState('');
  const [pending, setPending] = useState(false);
  const lock = useRef(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [panel, setPanel] = useState<{ id: string; type: 'proof' | 'failure' }>();
  const proof = useRef<{ file: File; id: string; url: string; key: string }>();

  useEffect(() => {
    const controller = new AbortController();
    void portalRequest<Task[] | null>('/courier/parcels', { token: accessToken, signal: controller.signal })
      .then(result => { if (!controller.signal.aborted) { setTasks(result ?? []); setLoadError(''); } })
      .catch(cause => { if (!controller.signal.aborted) setLoadError(errorMessage(cause)); });
    return () => controller.abort();
  }, [accessToken, revision]);

  async function run(action: () => Promise<void>) {
    if (lock.current) return;
    lock.current = true;
    setPending(true);
    setActionError('');
    setNotice('');
    try { await action(); }
    catch (cause) { setActionError(errorMessage(cause)); }
    finally { lock.current = false; setPending(false); }
  }

  function updated(message: string) {
    setNotice(message);
    setPanel(undefined);
    setSelected([]);
    setRevision(value => value + 1);
  }

  function submitAttempt(event: FormEvent<HTMLFormElement>, task: Task, type: 'proof' | 'failure') {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    const path = '/deliveries/' + encodeURIComponent(task.parcel_id);
    void run(async () => {
      if (type === 'failure') {
        await portalRequest(path + '/failed', {
          method: 'POST', token: accessToken, body: { failure_reason_code: fields.get('reason'), notes: fields.get('notes') },
        });
        updated('Delivery attempt recorded.');
        return;
      }
      const file = fields.get('photo');
      const name = String(fields.get('name') ?? '').trim();
      if (!(file instanceof File) || !file.size || !name) {
        setActionError('Add a proof photo and the recipient name.');
        return;
      }
      if (proof.current?.file !== file || proof.current.id !== task.parcel_id) {
        const payload = new FormData();
        payload.set('file', file);
        const uploaded = await portalRequest<{ photo_url: string }>(path + '/proof-photo', {
          method: 'POST', token: accessToken, body: payload,
        });
        proof.current = { file, id: task.parcel_id, url: uploaded.photo_url, key: crypto.randomUUID() };
      }
      await portalRequest(path + '/complete', {
        method: 'POST', token: accessToken, idempotencyKey: proof.current.key,
        body: { proof_type: 'PHOTO', proof_url: proof.current.url, received_by_name: name },
      });
      proof.current = undefined;
      updated('Delivery completed.');
    });
  }

  const batchIds = (tasks ?? []).filter(task => task.current_status === 'ASSIGNED' && selected.includes(task.parcel_id)).map(task => task.parcel_id);

  return <div data-role="COURIER" className="min-h-screen bg-canvas text-text">
    <PortalHeader profile="courier" />
    <main id="main-content" className="mx-auto max-w-5xl space-y-4 px-4 py-5 sm:px-6">
      <section className="portal-hero" aria-labelledby="courier-title">
        <img src="/courier-delivery-hero.jpg" alt="" fetchPriority="high" />
        <div><h1 id="courier-title" className="text-4xl font-semibold tracking-tight">Delivery tasks</h1>
          <p className="mt-2">Manage your shift and assigned deliveries.</p></div>
      </section>
      <Card><CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
        <div><h2 className="font-semibold">Shift</h2>{shift && <p className="capitalize text-text-muted">{statusLabel(shift)}</p>}</div>
        <div className="flex flex-wrap gap-2">
          {['ON_SHIFT', 'OFF_SHIFT'].filter(status => status !== shift).map(status =>
            <Button key={status} variant="outline" className="min-h-11" disabled={pending} onClick={() => void run(async () => {
              const result = await portalRequest<{ new_status: string }>('/courier/shift', {
                method: 'POST', token: accessToken, body: { target_status: status },
              });
              setShift(result.new_status);
            })}>{status === 'ON_SHIFT' ? 'Start shift' : 'End shift'}</Button>)}
        </div>
      </CardContent></Card>
      {notice && <p role="status">{notice}</p>}
      {actionError && <p role="alert">{actionError}</p>}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-lg font-semibold">Assigned deliveries</h2>
        <Button variant="outline" className="min-h-11" disabled={pending}
          onClick={() => setRevision(value => value + 1)}>Refresh tasks</Button>
      </div>
      {loadError ? <p role="alert">{loadError}</p> : !tasks ? <p role="status">Loading deliveries…</p> :
        !tasks.length && <p>No deliveries are assigned to you.</p>}
      {batchIds.length > 0 && <Button className="min-h-11 w-full" disabled={pending} onClick={() => void run(async () => {
        await portalRequest('/courier/runs/start-batch', {
          method: 'POST', token: accessToken, body: { parcel_ids: batchIds },
        });
        updated('Selected deliveries started.');
      })}>Start selected deliveries</Button>}
      {tasks?.map(task => {
        const active = panel?.id === task.parcel_id ? panel.type : undefined;
        const path = encodeURIComponent(task.parcel_id);
        const address = [task.delivery_street, task.delivery_city].filter(Boolean).join(', ');
        return <Card key={task.parcel_id} data-testid="courier-task">
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
            <h3 className="break-all font-mono font-semibold">{task.tracking_number}</h3>
            <Badge variant="active" className="capitalize">{statusLabel(task.current_status)}</Badge>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="font-medium">{task.recipient_name}</p>
            <p className="text-text-muted">{address}</p>
            {task.handling_instructions && <p>{task.handling_instructions}</p>}
            <div className="grid grid-cols-2 gap-2">
              {task.recipient_phone && <Button asChild variant="outline" className="min-h-11"><a href={'tel:' + task.recipient_phone}>Call recipient</a></Button>}
              {address && <Button asChild variant="outline" className="min-h-11"><a href={'https://www.google.com/maps/search/?api=1&query=' + encodeURIComponent(address)} target="_blank" rel="noreferrer">Directions</a></Button>}
            </div>
            {task.current_status === 'ASSIGNED' && <div className="grid gap-2 sm:grid-cols-2">
              <label className="flex min-h-11 items-center gap-2"><Input type="checkbox" className="!h-5 !w-5"
                disabled={pending} checked={selected.includes(task.parcel_id)}
                onChange={event => setSelected(current => event.target.checked ? [...current, task.parcel_id] : current.filter(id => id !== task.parcel_id))} />
                Select for batch</label>
              <Button className="min-h-11" disabled={pending || !!loadError} data-testid="courier-start-run" onClick={() => void run(async () => {
                await portalRequest('/courier/parcels/' + path + '/start-run', { method: 'POST', token: accessToken });
                updated('Delivery started.');
              })}>Start delivery</Button>
            </div>}
            {task.current_status === 'OUT_FOR_DELIVERY' && <div className="grid gap-2 sm:grid-cols-2">
              <Button className="min-h-11" disabled={pending} onClick={() => setPanel({ id: task.parcel_id, type: 'proof' })}>Record delivery</Button>
              <Button className="min-h-11" variant="outline" disabled={pending} onClick={() => setPanel({ id: task.parcel_id, type: 'failure' })}>Report failed attempt</Button>
            </div>}
            {(active === 'proof' || active === 'failure') && <form key={active} className="space-y-3"
              onSubmit={event => submitAttempt(event, task, active)}>
              <fieldset disabled={pending} className="space-y-3">
                {active === 'proof' ? <>
                  <label className="m6-field">Recipient name<Input className="min-h-11" name="name" required /></label>
                  <label className="m6-field">Proof photo<Input className="min-h-11" name="photo" type="file" accept="image/jpeg,image/png,image/webp" capture="environment" required /></label>
                </> : <>
                  <label className="m6-field">Reason<NativeSelect name="reason">{reasons.map(reason =>
                    <option key={reason} value={reason}>{statusLabel(reason)}</option>)}</NativeSelect></label>
                  <label className="m6-field">Notes<Input className="min-h-11" name="notes" /></label>
                </>}
                <Button className="min-h-11 w-full" disabled={pending}>{pending ? 'Saving…' : active === 'proof' ? 'Confirm delivery' : 'Save attempt'}</Button>
              </fieldset>
            </form>}
            {active && <Button className="min-h-11" variant="ghost" disabled={pending} onClick={() => setPanel(undefined)}>Close details</Button>}
          </CardContent>
        </Card>;
      })}
    </main>
  </div>;
}
