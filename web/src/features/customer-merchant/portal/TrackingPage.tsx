import { useEffect, useState } from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '@/app/providers/AuthContext';
import { Input } from '@/shared/components/ui/input';
import { MilestoneRail } from '@/shared/components/ui/milestone-rail';
import { Button } from '@/shared/components/ui/button';
import { apiBase, errorMessage, portalRequest, statusLabel } from './api';
import { PortalErrorMessage, PortalLayout, PortalStatus } from './PortalLayout';

type Snapshot = {
  tracking_number: string;
  status: string;
  destination?: { city: string; postal_code?: string; zone_id?: string };
  milestones: Array<{ status: string; timestamp: string; actor_role?: string; reason_code?: string }>;
};

export function TrackingPage({ privateView = false }: { privateView?: boolean }) {
  const { user, accessToken } = useAuth();
  const portalRole = user?.role === 'CUSTOMER' ? 'customer' : user?.role === 'MERCHANT' ? 'merchant' : undefined;
  const token = privateView ? accessToken : undefined;
  const basePath = privateView ? '/portal/tracking' : '/track';
  const trackingNumber = (useParams().trackingNumber ?? '').trim().toUpperCase();
  const navigate = useNavigate();
  const [input, setInput] = useState(trackingNumber);
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [degraded, setDegraded] = useState(false);
  const [revision, setRevision] = useState(0);

  useEffect(() => setInput(trackingNumber), [trackingNumber]);

  useEffect(() => {
    setSnapshot(undefined);
    setDegraded(false);
    setError('');
    if (!trackingNumber || (privateView && (!token || !portalRole))) { setLoading(false); return; }
    const controller = new AbortController();
    const path = `/tracking/${privateView ? '' : 'public/'}${encodeURIComponent(trackingNumber)}`;
    let stream: EventSource | undefined;
    let poll: number | undefined;
    let inFlight = false;
    let refreshQueued = false;
    setLoading(true);

    async function refresh() {
      if (inFlight) { refreshQueued = true; return false; }
      inFlight = true;
      try {
        const next = await portalRequest<Snapshot>(path, { token, signal: controller.signal });
        if (controller.signal.aborted) return false;
        if (!next?.tracking_number || typeof next.status !== 'string' || !Array.isArray(next.milestones)) {
          throw new Error('Unexpected tracking response');
        }
        setSnapshot(next);
        setError('');
        return true;
      } catch (cause) {
        if (!controller.signal.aborted) setError(errorMessage(cause));
        return false;
      } finally {
        inFlight = false;
        if (!controller.signal.aborted) {
          setLoading(false);
          if (refreshQueued) { refreshQueued = false; void refresh(); }
        }
      }
    }

    void refresh().then(found => {
      if (!found || controller.signal.aborted) return;
      const startPolling = () => {
        if (poll === undefined) poll = window.setInterval(() => void refresh(), 30_000);
      };
      if (privateView || typeof EventSource === 'undefined') {
        startPolling();
        return;
      }
      stream = new EventSource(apiBase + path + '/stream');
      let failures = 0;
      const seen = new Set<string>();
      const refreshOnUpdate = (event: MessageEvent) => {
        if (event.lastEventId) {
          if (seen.has(event.lastEventId)) return;
          if (seen.size >= 256) seen.clear();
          seen.add(event.lastEventId);
        }
        void refresh();
      };
      stream.addEventListener('status_update', refreshOnUpdate);
      stream.addEventListener('tracking-update', refreshOnUpdate);
      stream.onopen = () => {
        failures = 0;
        setDegraded(false);
        window.clearInterval(poll);
        poll = undefined;
        void refresh();
      };
      stream.onerror = () => {
        if (++failures >= 3 || stream?.readyState === EventSource.CLOSED) {
          setDegraded(true);
          startPolling();
        }
      };
    });
    return () => {
      controller.abort();
      stream?.close();
      window.clearInterval(poll);
    };
  }, [trackingNumber, revision, privateView, token, portalRole]);

  if (privateView && !portalRole) return <Navigate to="/dashboard" replace />;

  return (
    <PortalLayout title={privateView ? 'My parcel tracking' : 'Track a parcel'} profile={privateView ? (portalRole ?? 'public') : 'public'}>
      <form className="m6-card" onSubmit={event => {
        event.preventDefault();
        const number = input.trim().toUpperCase();
        if (!number) return;
        if (number === trackingNumber) setRevision(current => current + 1);
        else navigate(`${basePath}/${encodeURIComponent(number)}`);
      }}>
        <label className="m6-field">Tracking number
          <Input value={input} onChange={event => setInput(event.target.value)} required maxLength={60}
            data-testid="tracking-search-input" placeholder="SD-YYYYMMDD-XXXXX" />
        </label>
        <div className="m6-actions"><Button type="submit" data-testid="tracking-search-btn"
          disabled={loading && input.trim().toUpperCase() === trackingNumber}>
          {loading && input.trim().toUpperCase() === trackingNumber ? 'Looking up parcel…' : 'Track parcel'}
        </Button></div>
      </form>
      {degraded && <p role="status">Live updates degraded — polling</p>}
      {loading && <p role="status">Looking up your parcel…</p>}
      <PortalErrorMessage message={error} />
      {snapshot && snapshot.tracking_number === trackingNumber && <section className="m6-card" aria-live="polite">
        <div className="m6-result-heading">
          <h2>{snapshot.tracking_number}</h2>
          <PortalStatus status={snapshot.status} testId="tracking-status-badge" />
        </div>
        {snapshot.destination?.city && <p>Destination {snapshot.destination.city}</p>}
        {privateView && snapshot.destination?.postal_code && <p>Postal code {snapshot.destination.postal_code}</p>}
        {privateView && snapshot.destination?.zone_id && <p>Delivery zone {snapshot.destination.zone_id}</p>}
        <MilestoneRail entries={Array.from(new Map(snapshot.milestones.map(event => [
          `${event.status}-${event.timestamp}`, event,
        ])).entries()).sort(([, first], [, second]) => Date.parse(first.timestamp) - Date.parse(second.timestamp))
          .map(([id, event]) => ({ id, time: event.timestamp, title: statusLabel(event.status), detail: privateView && <>
            {event.actor_role && <p>Updated by {statusLabel(event.actor_role)}</p>}
            {event.reason_code && <p>Reason {statusLabel(event.reason_code)}</p>}
          </> }))} />
      </section>}
    </PortalLayout>
  );
}
