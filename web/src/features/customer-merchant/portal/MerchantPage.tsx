import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '@/app/providers/AuthContext';
import { Input } from '@/shared/components/ui/input';
import { Table } from '@/shared/components/ui/table';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/shared/components/ui/dialog';
import { NativeSelect } from '@/shared/components/ui/native-select';
import { Button } from '@/shared/components/ui/button';
import { errorMessage, PortalError, portalRequest, statusLabel } from './api';
import { PortalErrorMessage, PortalLayout, PortalStatus } from './PortalLayout';
import { MerchantOrderImport } from './MerchantOrderImport';
import { MerchantProfileBanner, type MerchantProfile } from './MerchantProfileBanner';

type Section = 'orders' | 'api-keys' | 'webhooks';
const ordersPerPage = 20;
type Order = { parcel_id: string; tracking_number: string; merchant_reference?: string; status: string; created_at: string };
type ApiKey = { key_id: string; label: string; key_prefix: string; status: string; scopes?: string[]; created_at?: string };
type Webhook = { webhook_id: string; url: string; enabled: boolean; events: string[] };
type EventType = { event_type: string; description: string };
type Profile = MerchantProfile;

const titles: Record<Section, string> = { orders: 'Merchant orders', 'api-keys': 'API keys', webhooks: 'Webhooks' };
const paths = { orders: '/merchant/orders', 'api-keys': '/merchants/me/api-keys', webhooks: '/merchants/me/webhooks' };

function OneTimeSecret({ secret, close }: { secret: string; close: () => void }) {
  const [acknowledged, setAcknowledged] = useState(false);
  const [copyMessage, setCopyMessage] = useState('');
  return <Dialog open onOpenChange={open => { if (!open && acknowledged) close(); }}>
    <DialogContent showClose={acknowledged} className="space-y-4 p-6"
      onEscapeKeyDown={event => { if (!acknowledged) event.preventDefault(); }}
      onInteractOutside={event => event.preventDefault()}>
    <DialogTitle>Save your credential now</DialogTitle>
    <DialogDescription>Copy this secret now. It will never be displayed again.</DialogDescription>
    <code className="select-all break-all">{secret}</code>
    <label className="flex items-center gap-2">
      <Input type="checkbox" className="!h-4 !w-4" checked={acknowledged}
        onChange={event => setAcknowledged(event.target.checked)} data-testid="dismiss-key-modal-checkbox" />
      I have safely copied and stored this secret in my secure vault.
    </label>
    <div className="m6-actions">
      <Button data-testid="one-time-key-copy-btn" onClick={async () => {
        try {
          await navigator.clipboard.writeText(secret);
          setCopyMessage('Copied.');
        } catch {
          setCopyMessage('Clipboard unavailable. Select and copy the credential manually.');
        }
      }}>Copy credential</Button>
      <Button variant="outline" disabled={!acknowledged} onClick={close} data-testid="merchant-secret-close-btn">Done</Button>
    </div>
    <p role="status">{copyMessage}</p>
  </DialogContent></Dialog>;
}

export function MerchantPage({ section }: { section: Section }) {
  const { accessToken } = useAuth();
  const [orders, setOrders] = useState<Order[]>([]);
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [hooks, setHooks] = useState<Webhook[]>([]);
  const [catalog, setCatalog] = useState<EventType[]>([]);
  const [secret, setSecret] = useState('');
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [page, setPage] = useState(1);
  const [notice, setNotice] = useState('');
  const [loadError, setLoadError] = useState('');
  const [actionError, setActionError] = useState('');
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState(false);
  const [revision, setRevision] = useState(0);
  const [remove, setRemove] = useState<{ id: string; label: string }>();
  const mutationPending = useRef(false);
  const path = paths[section];

  useEffect(() => {
    const controller = new AbortController();
    const options = { token: accessToken, signal: controller.signal };
    setLoading(true);
    setLoadError('');
    async function load() {
      try {
        if (section === 'orders') setOrders((await portalRequest<Order[] | null>(path, options)) ?? []);
        if (section === 'api-keys') setKeys((await portalRequest<ApiKey[] | null>(path, options)) ?? []);
        if (section === 'webhooks') {
          const [webhooks, events] = await Promise.all([
            portalRequest<Webhook[] | null>(path, options),
            portalRequest<EventType[] | null>('/merchants/webhook-event-types', options),
          ]);
          setHooks(webhooks ?? []);
          setCatalog(events ?? []);
        }
      } catch (cause) {
        if (!controller.signal.aborted) setLoadError(errorMessage(cause));
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [accessToken, path, revision, section]);

  async function mutate(action: () => Promise<void>) {
    if (mutationPending.current) return;
    mutationPending.current = true;
    setPending(true);
    setActionError('');
    setNotice('');
    try {
      await action();
      setRevision(current => current + 1);
    } catch (cause) {
      setActionError(errorMessage(cause));
    } finally {
      mutationPending.current = false;
      setPending(false);
    }
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    await mutate(async () => {
      if (section === 'api-keys') {
        const scopes = data.getAll('scope');
        if (!scopes.length) throw new PortalError('Select at least one permission.', 400);
        const result = await portalRequest<{ api_key?: string; plaintext_key?: string }>(path, {
          token: accessToken, method: 'POST', body: { label: data.get('label'), scopes },
        });
        const plaintext = result?.api_key || result?.plaintext_key;
        if (!plaintext) throw new PortalError('The server did not return the new key. Check the key list.', 200);
        setSecret(plaintext);
      } else {
        const events = data.getAll('event');
        if (!events.length) throw new PortalError('Select at least one event.', 400);
        const url = String(data.get('url'));
        if (!url.startsWith('https://')) throw new PortalError('Enter an HTTPS endpoint.', 400);
        const result = await portalRequest<{ signing_secret: string }>(path, {
          token: accessToken, method: 'POST', body: { url, events },
        });
        if (!result?.signing_secret) throw new PortalError('The server did not return a signing secret. Check the webhook list.', 200);
        setSecret(result.signing_secret);
      }
      form.reset();
    });
  }

  const statuses = [...new Set(orders.map(order => order.status))];
  const filtered = orders.filter(order => (!status || status === order.status) &&
    `${order.tracking_number} ${order.merchant_reference ?? ''}`.toLowerCase().includes(search.trim().toLowerCase()));
  const pageCount = Math.max(1, Math.ceil(filtered.length / ordersPerPage));
  const currentPage = Math.min(page, pageCount);
  const start = (currentPage - 1) * ordersPerPage;
  const visibleOrders = filtered.slice(start, start + ordersPerPage);

  return <PortalLayout title={titles[section]} profile="merchant">
    {section === 'orders' && accessToken && <MerchantProfileBanner key={accessToken} token={accessToken} />}
    <PortalErrorMessage message={loadError} />
    {loadError && <Button variant="outline" data-testid="merchant-retry-btn" onClick={() => setRevision(current => current + 1)}>Retry</Button>}
    {loading && <p role="status">Loading {titles[section].toLowerCase()}…</p>}
    {!loading && !loadError && section === 'orders' && <>
      <section className="m6-card">
        <h2>Orders</h2>
        <div className="m6-grid">
          <label className="m6-field">Search orders
            <Input type="search" value={search} onChange={event => { setSearch(event.target.value); setPage(1); }} data-testid="merchant-order-search" />
          </label>
          <label className="m6-field">Status
            <NativeSelect value={status} onChange={event => { setStatus(event.target.value); setPage(1); }} data-testid="merchant-order-filter">
              <option value="">All statuses</option>
              {statuses.map(item => <option key={item} value={item}>{statusLabel(item)}</option>)}
            </NativeSelect>
          </label>
        </div>
        {filtered.length ? <div className="m6-scroll" tabIndex={0} role="region" aria-label="Orders table">
          <Table className="m6-table" role="table" aria-label="Merchant orders" data-testid="merchant-order-table"><thead role="rowgroup"><tr role="row">
            <th role="columnheader" scope="col">Tracking number</th><th role="columnheader" scope="col">Status</th>
            <th role="columnheader" scope="col">Reference</th><th role="columnheader" scope="col">Created</th>
          </tr></thead><tbody role="rowgroup">
            {visibleOrders.map(order => <tr key={order.parcel_id} role="row">
              <td role="cell"><Link to={`/portal/tracking/${encodeURIComponent(order.tracking_number)}`}>{order.tracking_number}</Link></td>
              <td role="cell"><PortalStatus status={order.status} /></td>
              <td role="cell" className="m6-order-meta"><span aria-hidden="true">Reference </span>{order.merchant_reference || '—'}</td>
              <td role="cell" className="m6-order-meta"><span aria-hidden="true">Created </span>{new Date(order.created_at).toLocaleDateString('en-LK')}</td>
            </tr>)}
          </tbody></Table>
        </div> : <div className="m6-empty">
          <p role="status" data-testid="merchant-order-empty">{orders.length ? 'No orders match your filters.' : 'No orders yet.'}</p>
          {orders.length > 0 && <Button type="button" variant="outline" data-testid="merchant-order-reset-btn"
            onClick={() => { setSearch(''); setStatus(''); setPage(1); }}>Reset filters</Button>}
        </div>}
        {pageCount > 1 && <nav className="m6-actions" aria-label="Order pages">
          <Button type="button" variant="outline" disabled={currentPage === 1} onClick={() => setPage(currentPage - 1)}>Previous</Button>
          <span role="status">Page {currentPage} of {pageCount}</span>
          <Button type="button" variant="outline" disabled={currentPage === pageCount} onClick={() => setPage(currentPage + 1)}>Next</Button>
        </nav>}
      </section>
    </>}
    {section === 'orders' && <MerchantOrderImport token={accessToken} onComplete={() => setRevision(current => current + 1)} />}
    {!loading && !loadError && section !== 'orders' && <>
      <section className="m6-card">
        <h2>{section === 'api-keys' ? 'Create an API key' : 'Add a webhook'}</h2>
        <form onSubmit={create}>
          <fieldset disabled={pending || Boolean(secret)}>
            {section === 'api-keys' ? <>
              <label className="m6-field">Key label<Input name="label" required maxLength={100} data-testid="merchant-key-label-input" /></label>
              <label className="m6-choice"><Input className="!h-4 !w-4" type="checkbox" name="scope" value="parcels:read" data-testid="merchant-key-read-scope" /> Read parcels</label>
              <label className="m6-choice"><Input className="!h-4 !w-4" type="checkbox" name="scope" value="parcels:create" data-testid="merchant-key-create-scope" /> Create parcels</label>
            </> : <>
              <label className="m6-field">Endpoint URL
                <Input name="url" type="url" required placeholder="https://" data-testid="merchant-webhook-url-input" />
              </label>
              {catalog.map(event => <label key={event.event_type} className="m6-choice">
                <Input className="!h-4 !w-4" type="checkbox" name="event" value={event.event_type} data-testid={`merchant-webhook-event-${event.event_type}`} /> {event.description || event.event_type}
              </label>)}
            </>}
            <div className="m6-actions"><Button data-testid="merchant-credential-create-btn">
              {section === 'api-keys' ? (pending && !remove ? 'Creating key…' : 'Create key') : (pending && !remove ? 'Adding webhook…' : 'Add webhook')}
            </Button></div>
          </fieldset>
        </form>
        {!remove && <PortalErrorMessage message={actionError} />}
        {notice && <p role="status">{notice}</p>}
      </section>
      <section className="m6-card">
        <h2>{titles[section]}</h2>
        {section === 'api-keys' ? keys.length ? keys.map(key => <div key={key.key_id} className="m6-item">
          <div>
            <p>{key.label} · <code>{key.key_prefix}</code> <PortalStatus status={key.status} /></p>
            {!!key.scopes?.length && <p className="m6-muted">{key.scopes.join(', ')}</p>}
            {key.created_at && <p className="m6-muted">Created <time dateTime={key.created_at}>{new Date(key.created_at).toLocaleDateString('en-LK')}</time></p>}
          </div>
          {key.status === 'ACTIVE' && <Button variant="outline" disabled={pending} data-testid="merchant-key-revoke-btn"
            onClick={() => setRemove({ id: key.key_id, label: key.label })}>Revoke</Button>}
        </div>) : <p>No API keys yet.</p> : hooks.length ? hooks.map(hook => <div key={hook.webhook_id} className="m6-item">
          <div><p>{hook.url} · {hook.enabled ? 'Enabled' : 'Disabled'}</p><p className="m6-muted">{hook.events.join(', ')}</p></div>
          <Button variant="outline" disabled={pending} data-testid="merchant-webhook-delete-btn"
            onClick={() => setRemove({ id: hook.webhook_id, label: hook.url })}>Remove</Button>
        </div>) : <p>No webhooks yet.</p>}
        {remove && <section className="m6-confirmation" aria-label="Confirm credential removal">
          <h3>{section === 'api-keys' ? 'Revoke API key' : 'Remove webhook'}</h3>
          <p><strong>{remove.label}</strong> {section === 'api-keys' ? 'will no longer be able to authenticate requests.' : 'will stop receiving SwiftDrop events.'}</p>
          <PortalErrorMessage message={actionError} />
          <div className="m6-actions">
          <Button variant="destructive" disabled={pending} data-testid="merchant-credential-confirm-btn"
            onClick={() => void mutate(async () => {
              await portalRequest(`${path}/${encodeURIComponent(remove.id)}`, { token: accessToken, method: 'DELETE' });
              setRemove(undefined);
              setNotice(section === 'api-keys' ? 'API key revoked.' : 'Webhook removed.');
            })}>{section === 'api-keys' ? (pending ? 'Revoking key…' : 'Revoke key') : (pending ? 'Removing webhook…' : 'Remove webhook')}</Button>
          <Button variant="outline" disabled={pending} onClick={() => { setRemove(undefined); setActionError(''); }}>Cancel</Button>
          </div>
        </section>}
      </section>
    </>}
    {secret && <OneTimeSecret secret={secret} close={() => setSecret('')} />}
  </PortalLayout>;
}

const applicationFields = [
  ['business_name', 'Business name', 'text'], ['contact_email', 'Contact email', 'email'],
  ['contact_phone', 'Contact phone (include country code)', 'tel'], ['line1', 'Street address', 'text'],
  ['city', 'City', 'text'], ['country', 'Country code', 'text'],
] as const;

export function MerchantApplication() {
  const { accessToken, refreshSession } = useAuth();
  const [profile, setProfile] = useState<Profile>();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState(false);
  const [retry, setRetry] = useState(0);
  const [lookupFailed, setLookupFailed] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setLookupFailed(false);
    setError('');
    void portalRequest<Profile>('/merchants/me', { token: accessToken, signal: controller.signal })
      .then(setProfile)
      .catch(cause => {
        if (!controller.signal.aborted && !(cause instanceof PortalError && cause.status === 404)) {
          setError(errorMessage(cause));
          setLookupFailed(true);
        }
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [accessToken, retry]);
  useEffect(() => {
    if (profile?.status === 'APPROVED') void refreshSession();
  }, [profile?.status, refreshSession]);

  return <PortalLayout title="Merchant application" profile="customer">
    <section className="m6-card">
      <p className="m6-muted">Apply for a business account to manage shipments in bulk. Merchant tools become available after approval.</p>
      <PortalErrorMessage message={error} />
      {loading ? <p role="status">Checking application status…</p> : lookupFailed
        ? <Button onClick={() => setRetry(current => current + 1)}>Retry lookup</Button>
        : profile ? <div data-testid="merchant-application-status">
          <div className="m6-result-heading"><h2>{profile.business_name}</h2><PortalStatus status={profile.status} /></div>
          {profile.status === 'APPROVED' && <p><Link to="/portal/merchants/orders">Open merchant workspace</Link></p>}
        </div>
        : <form onSubmit={async event => {
          event.preventDefault();
          if (pending) return;
          const data = new FormData(event.currentTarget);
          setPending(true);
          setError('');
          try {
            setProfile(await portalRequest<Profile>('/merchants/applications', {
              token: accessToken, method: 'POST', body: {
                business_name: data.get('business_name'), contact_email: data.get('contact_email'), contact_phone: data.get('contact_phone'),
                address: { line1: data.get('line1'), city: data.get('city'), country: String(data.get('country')).toUpperCase() },
              },
            }));
          } catch (cause) { setError(errorMessage(cause)); }
          finally { setPending(false); }
        }}>
          <fieldset className="m6-grid" disabled={pending}>
            {applicationFields.map(([name, label, type]) => <label className="m6-field" key={name}>{label}
              <Input name={name} type={type} required defaultValue={name === 'country' ? 'LK' : undefined}
                maxLength={name === 'country' ? 2 : undefined} data-testid={`merchant-application-${name}-input`} />
            </label>)}
          </fieldset>
          <div className="m6-actions"><Button disabled={pending} data-testid="merchant-application-submit-btn">
            {pending ? 'Submitting application…' : 'Submit application'}
          </Button></div>
        </form>}
    </section>
  </PortalLayout>;
}
