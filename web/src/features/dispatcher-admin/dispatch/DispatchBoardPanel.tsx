import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { PackageCheck, RefreshCw, Truck } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError } from '../api';
import { Banner, FieldHint } from '../admin-console/ui';
import { listZoneOptions, type ZoneOption } from '../fleet/fleetApi';
import { AssignParcelModal, SHIFT_LABEL, whyNot } from './AssignParcelModal';
import { getBoard, waitedFor, type DispatchBoard, type WaitingParcel } from './dispatchApi';

// Dispatch board: parcels waiting for a courier and the couriers who could take them, for one
// zone or all zones. It reloads every 30 seconds while the tab is visible and the assign window
// is closed. Each load replaces the lists as the server sent them.

export const POLL_MS = 30_000;
const ANY = 'ANY';

const SHIFT_VARIANT: Record<string, 'success' | 'neutral' | 'warning' | 'error'> = {
  ON_SHIFT: 'success',
  OFF_SHIFT: 'neutral',
  ON_BREAK: 'warning',
  SUSPENDED: 'error',
};

function clock(d: Date): string {
  return d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false, timeZone: 'Asia/Colombo' });
}

interface DispatchBoardPanelProps {
  token: string | null;
  onNotice: (message: string) => void;
}

export const DispatchBoardPanel: React.FC<DispatchBoardPanelProps> = ({ token, onNotice }) => {
  const [zones, setZones] = useState<ZoneOption[]>([]);
  const [zone, setZone] = useState(ANY);
  const [board, setBoard] = useState<DispatchBoard | null>(null);
  const [loadedAt, setLoadedAt] = useState<Date | null>(null);
  const [initialLoading, setInitialLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [assigning, setAssigning] = useState<WaitingParcel | null>(null);
  const [now, setNow] = useState(() => new Date());

  const latest = useRef(0);
  const zoneRef = useRef(zone);
  zoneRef.current = zone;
  const loadedRef = useRef<Date | null>(null);
  loadedRef.current = loadedAt;

  const load = useCallback(
    async (zoneId: string) => {
      const mine = ++latest.current;
      setRefreshing(true);
      try {
        const next = await getBoard(token, zoneId === ANY ? undefined : zoneId);
        if (mine !== latest.current) return; // a newer request is in flight; this answer is out of date
        setBoard(next);
        setLoadedAt(new Date());
        setError(null);
      } catch (err) {
        if (mine !== latest.current) return;
        setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load the dispatch board. Try again."));
      } finally {
        if (mine === latest.current) {
          setRefreshing(false);
          setInitialLoading(false);
        }
      }
    },
    [token],
  );

  // First load, and a fresh load whenever the zone changes.
  useEffect(() => {
    setInitialLoading(true);
    setBoard(null);
    void load(zone);
  }, [load, zone]);

  useEffect(() => {
    listZoneOptions()
      .then(setZones)
      .catch(() => setZones([]));
  }, []);

  // Poll every 30 seconds while the tab is visible and nobody is in the assign window.
  useEffect(() => {
    if (assigning) return;
    const tick = () => {
      setNow(new Date());
      if (!document.hidden) void load(zoneRef.current);
    };
    const timer = window.setInterval(tick, POLL_MS);
    // Coming back to the tab after a long time: catch up straight away.
    const onVisible = () => {
      if (document.hidden) return;
      const last = loadedRef.current;
      if (!last || Date.now() - last.getTime() >= POLL_MS) tick();
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, [assigning, load]);

  const zoneName = useMemo(() => new Map(zones.map((z) => [z.id, z.name])), [zones]);
  const parcels = board?.parcels ?? [];
  const couriers = board?.couriers ?? [];
  const onShift = couriers.filter((c) => c.status === 'ON_SHIFT').length;
  const stale = error !== null && board !== null;
  const blocked = error !== null && board === null;

  const closeAssign = () => {
    setAssigning(null);
    void load(zoneRef.current);
  };

  return (
    <>
      <Card role="region" aria-label="Board settings">
        <CardContent className="flex flex-wrap items-end justify-between gap-4 p-4">
          <div className="grid w-full content-start gap-1.5 sm:w-72">
            <Label htmlFor="dispatch-zone">Zone</Label>
            <Select value={zone} onValueChange={setZone}>
              <SelectTrigger id="dispatch-zone" data-testid="dispatch-zone">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ANY}>All zones</SelectItem>
                {zones.map((z) => (
                  <SelectItem key={z.id} value={z.id}>
                    {z.name} ({z.code})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-xs text-text-muted tabular-nums" data-testid="dispatch-updated">
              {loadedAt ? `Updated ${clock(loadedAt)} · refreshes every 30 seconds` : 'Not loaded yet'}
            </span>
            <Button type="button" variant="outline" size="icon" onClick={() => void load(zone)} disabled={refreshing} aria-label="Refresh the board now" title="Refresh the board now" data-testid="dispatch-refresh-btn">
              <RefreshCw className={refreshing ? 'animate-spin' : undefined} aria-hidden="true" />
            </Button>
          </div>
        </CardContent>
      </Card>

      <p className="sr-only" role="status">
        {initialLoading ? 'Loading the dispatch board…' : board ? `${parcels.length} parcels waiting, ${couriers.length} couriers` : ''}
      </p>

      {error && (
        <Banner
          tone={stale ? 'attn' : 'err'}
          testId="dispatch-error"
          reference={error.correlationId}
          action={
            <div>
              <Button type="button" variant="outline" size="sm" onClick={() => void load(zone)}>
                Retry
              </Button>
            </div>
          }
        >
          {stale && loadedAt
            ? `Couldn't refresh the board. You are looking at the data from ${clock(loadedAt)}.`
            : error.status === 403
              ? 'Your account is not allowed to view the dispatch board.'
              : error.message || "Couldn't load the dispatch board. Try again."}
        </Banner>
      )}

      {!blocked && (
        <div className="grid gap-5 xl:grid-cols-5">
          <Card className="min-w-0 xl:col-span-3" role="region" aria-labelledby="dispatch-parcels-title">
            <CardHeader className="space-y-0">
              <CardTitle id="dispatch-parcels-title">Waiting parcels</CardTitle>
              <CardDescription data-testid="dispatch-parcels-summary">
                {board ? `${parcels.length} ${parcels.length === 1 ? 'parcel' : 'parcels'} waiting for a courier, oldest first.` : 'Oldest first.'}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {board && parcels.length === 0 ? (
                <div data-testid="dispatch-parcels-empty" className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
                  <PackageCheck size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
                  <CardTitle>No parcels are waiting</CardTitle>
                  <CardDescription>Every parcel{zone === ANY ? '' : ' in this zone'} has a courier. New parcels appear here within 30 seconds.</CardDescription>
                </div>
              ) : (
                <Table aria-label="Waiting parcels" data-testid="dispatch-parcels-table" className="min-w-max">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Parcel</TableHead>
                      <TableHead>Deliver to</TableHead>
                      <TableHead>Waiting</TableHead>
                      <TableHead className="text-right">Action</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {initialLoading || !board
                      ? [0, 1, 2, 3].map((i) => (
                          <TableRow key={i} aria-hidden="true">
                            <TableCell>
                              <Skeleton className="h-4 w-32" />
                            </TableCell>
                            <TableCell>
                              <Skeleton className="h-4 w-24" />
                            </TableCell>
                            <TableCell>
                              <Skeleton className="h-4 w-16" />
                            </TableCell>
                            <TableCell>
                              <Skeleton className="ml-auto h-8 w-20" />
                            </TableCell>
                          </TableRow>
                        ))
                      : parcels.map((p) => {
                          const failed = p.status === 'ATTEMPT_FAILED';
                          return (
                            <TableRow key={p.id} data-testid="dispatch-parcel-row">
                              <TableCell>
                                <div className="font-mono tabular-nums">{p.tracking_number}</div>
                                <Badge variant={failed ? 'warning' : 'neutral'} className="mt-1">
                                  {failed ? 'Attempt failed' : 'Waiting'}
                                </Badge>
                              </TableCell>
                              <TableCell>
                                <div>{p.recipient_city || <span className="text-text-muted">Not recorded</span>}</div>
                                <div className="font-mono text-xs tabular-nums text-text-muted">{p.weight_kg} kg</div>
                              </TableCell>
                              <TableCell className="whitespace-nowrap font-mono tabular-nums">{waitedFor(p.created_at, now)}</TableCell>
                              <TableCell className="text-right">
                                <Button
                                  type="button"
                                  size="sm"
                                  variant={failed ? 'outline' : 'default'}
                                  className="touch:min-h-[44px]"
                                  onClick={() => setAssigning(p)}
                                  aria-label={`${failed ? 'Reassign' : 'Assign'} parcel ${p.tracking_number}`}
                                  data-testid="dispatch-assign-btn"
                                >
                                  {failed ? 'Reassign' : 'Assign'}
                                </Button>
                              </TableCell>
                            </TableRow>
                          );
                        })}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card className="min-w-0 xl:col-span-2" role="region" aria-labelledby="dispatch-couriers-title">
            <CardHeader className="space-y-0">
              <CardTitle id="dispatch-couriers-title">Couriers</CardTitle>
              <CardDescription data-testid="dispatch-couriers-summary">
                {board ? `${couriers.length} ${couriers.length === 1 ? 'courier' : 'couriers'}, ${onShift} on shift.` : 'Shift and workload.'}
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3">
              {board && couriers.length === 0 ? (
                <div data-testid="dispatch-couriers-empty" className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
                  <Truck size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
                  <CardTitle>No couriers{zone === ANY ? '' : ' in this zone'}</CardTitle>
                  <CardDescription>Add a courier in Fleet so parcels can be assigned.</CardDescription>
                  <Button asChild variant="outline" className="mt-2">
                    <Link to="/ops/couriers">Go to Fleet</Link>
                  </Button>
                </div>
              ) : initialLoading || !board ? (
                [0, 1, 2].map((i) => <Skeleton key={i} className="h-16" aria-hidden="true" />)
              ) : (
                <ul className="grid gap-3" aria-label="Couriers" data-testid="dispatch-couriers">
                  {couriers.map((c) => {
                    const known = c.active_count !== null;
                    const full = whyNot(c) === 'Full';
                    return (
                      <li key={c.user_id} data-testid="dispatch-courier" className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
                        <div className="min-w-0">
                          <div className="truncate font-medium">{c.courier_name || <span className="text-text-muted">Name not provided</span>}</div>
                          <div className="font-mono text-xs text-text-muted">
                            {c.employee_code}
                            {c.current_zone_id ? ` · ${zoneName.get(c.current_zone_id) ?? c.current_zone_id}` : ''}
                          </div>
                        </div>
                        <div className="flex shrink-0 flex-col items-end gap-1">
                          <Badge variant={SHIFT_VARIANT[c.status] ?? 'neutral'}>{SHIFT_LABEL[c.status] ?? c.status}</Badge>
                          <span className="font-mono text-xs tabular-nums" title={known ? undefined : 'The API does not send the current workload yet'}>
                            {known ? c.active_count : '—'} / {c.max_capacity}
                            {full ? ' · Full' : ''}
                          </span>
                        </div>
                      </li>
                    );
                  })}
                </ul>
              )}
              <FieldHint>Parcels that already have a courier are not listed on this board.</FieldHint>
            </CardContent>
          </Card>
        </div>
      )}

      {assigning && board && (
        <AssignParcelModal
          token={token}
          parcel={assigning}
          couriers={couriers}
          stillWaiting={parcels.some((p) => p.id === assigning.id)}
          onClose={closeAssign}
          onDone={(message) => {
            setAssigning(null);
            onNotice(message);
            void load(zoneRef.current);
          }}
          onStale={() => void load(zoneRef.current)}
        />
      )}
    </>
  );
};
