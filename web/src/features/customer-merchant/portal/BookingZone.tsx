import { useEffect, useState } from 'react';
import { Badge } from '@/shared/components/ui/badge';
import { NativeSelect } from '@/shared/components/ui/native-select';
import { errorMessage, PortalError, portalRequest } from './api';

type Zone = { id: string; name: string };

export function BookingZone({ postal, selected, onSelect }: {
  postal: string;
  selected: string;
  onSelect: (id: string) => void;
}) {
  const [lookup, setLookup] = useState<{ postal: string; zone?: Zone; choices?: Zone[]; error?: string }>();
  useEffect(() => {
    if (!/^\d{5}$/.test(postal)) return;
    const controller = new AbortController();
    const { signal } = controller;
    async function resolve() {
      try {
        const zone = await portalRequest<Zone>(`/zones?postal_code=${postal}`, { signal });
        if (!signal.aborted) setLookup({ postal, zone });
      } catch (cause) {
        if (signal.aborted) return;
        try {
          if (!(cause instanceof PortalError && cause.status === 404)) throw cause;
          const choices = await portalRequest<Zone[]>('/zones', { signal });
          if (!signal.aborted) setLookup({ postal, choices });
        } catch (failure) {
          if (!signal.aborted) setLookup({ postal, error: errorMessage(failure) });
        }
      }
    }
    void resolve();
    return () => controller.abort();
  }, [postal]);

  if (!/^\d{5}$/.test(postal)) return null;
  if (lookup?.postal !== postal) return <p className="mt-3 text-sm text-text-muted" role="status">Checking delivery zone…</p>;
  if (lookup.zone) return <div className="mt-3"><Badge variant="success">Delivery zone · {lookup.zone.name}</Badge></div>;
  if (lookup.error) return <p role="status">{lookup.error}</p>;
  if (!lookup.choices?.length) return <p className="mt-3 text-sm text-text-muted" role="status">No delivery zones are available. Check the address or try again later.</p>;
  return <label className="mt-4 flex flex-col gap-2 rounded-lg border border-border bg-surface p-3 text-sm">
    <span className="font-medium">Delivery zone</span>
    <span className="text-text-muted">Choose a delivery zone for this address. The postal code did not match a zone.</span>
    <NativeSelect required value={selected} onChange={event => onSelect(event.target.value)}>
      <option value="">Select delivery zone</option>
      {lookup.choices?.map(zone => <option key={zone.id} value={zone.id}>{zone.name}</option>)}
    </NativeSelect>
  </label>;
}
