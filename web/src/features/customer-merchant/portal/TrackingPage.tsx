import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { apiBase, errorMessage, portalRequest, statusLabel } from './api';
import { PortalErrorMessage, PortalLayout, PortalStatus } from './PortalLayout';

type Snapshot = {
  tracking_number: string;
  status: string;
  destination_city?: string;
  milestones: Array<{ status: string; timestamp: string }>;
};

export function TrackingPage() {
  const trackingNumber = (useParams().trackingNumber ?? '').trim().toUpperCase();
  const navigate = useNavigate();
  const [input, setInput] = useState(trackingNumber);
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [revision, setRevision] = useState(0);

  useEffect(() => setInput(trackingNumber), [trackingNumber]);

  useEffect(() => {
    setSnapshot(undefined);
    setError('');
    if (!trackingNumber) { setLoading(false); return; }
    const controller = new AbortController();
    const path = `/tracking/public/${encodeURIComponent(trackingNumber)}`;
    let stream: EventSource | undefined;
    let poll: number | undefined;
    let inFlight = false;
    let refreshQueued = false;
    setLoading(true);

    async function refresh() {
      if (inFlight) { refreshQueued = true; return false; }
      inFlight = true;
      try {
        const next = await portalRequest<Snapshot>(path, { signal: controller.signal });
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
      if (typeof EventSource === 'undefined') {
        startPolling();
        return;
      }
      stream = new EventSource(apiBase + path + '/stream');
      stream.addEventListener('status_update', () => void refresh());
      stream.onopen = () => {
        window.clearInterval(poll);
        poll = undefined;
        void refresh();
      };
      stream.onerror = startPolling;
    });
    return () => {
      controller.abort();
      stream?.close();
      window.clearInterval(poll);
    };
  }, [trackingNumber, revision]);

  return (
    <PortalLayout title="Track a parcel" profile="public">
      <form className="m6-card" onSubmit={event => {
        event.preventDefault();
        const number = input.trim().toUpperCase();
        if (!number) return;
        if (number === trackingNumber) setRevision(current => current + 1);
        else navigate(`/track/${encodeURIComponent(number)}`);
      }}>
        <label className="m6-field">Tracking number
          <input value={input} onChange={event => setInput(event.target.value)} required maxLength={60}
            data-testid="tracking-search-input" placeholder="SD-YYYYMMDD-XXXXX" />
        </label>
        <div className="m6-actions"><button className="m6-btn" data-testid="tracking-search-btn"
          disabled={loading && input.trim().toUpperCase() === trackingNumber}>
          {loading && input.trim().toUpperCase() === trackingNumber ? 'Looking up parcel…' : 'Track parcel'}
        </button></div>
      </form>
      {loading && <p role="status">Looking up your parcel…</p>}
      <PortalErrorMessage message={error} />
      {snapshot && snapshot.tracking_number === trackingNumber && <section className="m6-card" aria-live="polite">
        <div className="m6-result-heading">
          <h2>{snapshot.tracking_number}</h2>
          <PortalStatus status={snapshot.status} testId="tracking-status-badge" />
        </div>
        {snapshot.destination_city && <p>Destination {snapshot.destination_city}</p>}
        <ol className="m6-timeline" data-testid="tracking-timeline">
          {snapshot.milestones.map((event, index) => <li key={`${event.status}-${event.timestamp}-${index}`}>
            <strong className="m6-timeline-label">{statusLabel(event.status)}</strong>
            <time dateTime={event.timestamp}>{new Date(event.timestamp).toLocaleString('en-LK')}</time>
          </li>)}
        </ol>
      </section>}
    </PortalLayout>
  );
}
