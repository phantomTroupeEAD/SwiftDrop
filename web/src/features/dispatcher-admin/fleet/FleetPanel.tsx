import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Plus, RefreshCw, SearchX, Truck } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError } from '../api';
import { Banner, FieldHint } from '../admin-console/ui';
import { AddCourierModal } from './AddCourierModal';
import { listCouriers, listZoneOptions, type CourierLoad, type ZoneOption } from './fleetApi';

// Fleet: couriers with their shift and workload (dispatch API.md section 2), and adding a courier
// (section 1). Shift is shown, not edited: the shift endpoint changes the caller's own shift, so
// staff cannot switch another courier's shift from here.

const ANY = 'ANY';

const SHIFT_LABEL: Record<string, string> = {
  ON_SHIFT: 'On shift',
  OFF_SHIFT: 'Off shift',
  ON_BREAK: 'On break',
  SUSPENDED: 'Suspended',
};
const SHIFT_VARIANT: Record<string, 'success' | 'neutral' | 'warning' | 'error'> = {
  ON_SHIFT: 'success',
  OFF_SHIFT: 'neutral',
  ON_BREAK: 'warning',
  SUSPENDED: 'error',
};

function titleCase(value: string): string {
  if (!value) return 'Not provided';
  return value.charAt(0).toUpperCase() + value.slice(1).toLowerCase();
}

interface FleetPanelProps {
  token: string | null;
  /** ADMIN can pick the new courier from a list of accounts; DISPATCHER enters an account id. */
  canListUsers: boolean;
  onNotice: (message: string) => void;
}

export const FleetPanel: React.FC<FleetPanelProps> = ({ token, canListUsers, onNotice }) => {
  const [couriers, setCouriers] = useState<CourierLoad[]>([]);
  const [zones, setZones] = useState<ZoneOption[]>([]);
  const [zoneFilter, setZoneFilter] = useState(ANY);
  const [shiftFilter, setShiftFilter] = useState(ANY);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [adding, setAdding] = useState(false);
  // Every courier, ignoring the zone filter, so the add dialog hides accounts that already have a profile.
  const [everyId, setEveryId] = useState<Set<string>>(new Set());

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listCouriers(token, zoneFilter === ANY ? undefined : zoneFilter);
      setCouriers(list);
      if (zoneFilter === ANY) setEveryId(new Set(list.map((c) => c.user_id)));
      else setEveryId((prev) => new Set([...prev, ...list.map((c) => c.user_id)]));
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load the fleet. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, zoneFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    // Zone names are a convenience: if this fails the screen shows zone ids instead.
    listZoneOptions()
      .then(setZones)
      .catch(() => setZones([]));
  }, []);

  const zoneName = useMemo(() => new Map(zones.map((z) => [z.id, z.name])), [zones]);
  const shown = useMemo(() => (shiftFilter === ANY ? couriers : couriers.filter((c) => c.status === shiftFilter)), [couriers, shiftFilter]);
  const filtered = zoneFilter !== ANY || shiftFilter !== ANY;
  const showEmpty = !loading && !error && shown.length === 0;

  const reset = () => {
    setZoneFilter(ANY);
    setShiftFilter(ANY);
  };

  // A courier may be in a zone that is now disabled, which the public list leaves out.
  const zoneChoices = useMemo(() => {
    const known = new Set(zones.map((z) => z.id));
    const extra = couriers.map((c) => c.current_zone_id).filter((id) => id && !known.has(id));
    return [...zones.map((z) => ({ id: z.id, label: z.name })), ...[...new Set(extra)].map((id) => ({ id, label: id }))];
  }, [zones, couriers]);

  return (
    <>
      <Card role="region" aria-labelledby="fleet-title">
        <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
          <div className="grid gap-1.5">
            <CardTitle id="fleet-title">Couriers</CardTitle>
            <CardDescription>Each courier&apos;s shift, zone and how many parcels they carry against their capacity.</CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <Button type="button" variant="outline" size="icon" onClick={() => void load()} disabled={loading} aria-label="Refresh fleet" title="Refresh fleet" data-testid="fleet-refresh-btn">
              <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
            </Button>
            <Button type="button" onClick={() => setAdding(true)} data-testid="fleet-add-btn">
              <Plus aria-hidden="true" /> Add courier
            </Button>
          </div>
        </CardHeader>

        <CardContent className="grid gap-4">
          <div role="group" aria-label="Fleet filters" data-testid="fleet-filters" className="grid gap-4 rounded-md border border-border bg-surface-subtle p-4 sm:grid-cols-3 sm:items-end">
            <div className="grid content-start gap-1.5">
              <Label htmlFor="fleet-filter-zone">Zone</Label>
              <Select value={zoneFilter} onValueChange={setZoneFilter}>
                <SelectTrigger id="fleet-filter-zone" data-testid="fleet-filter-zone">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY}>All zones</SelectItem>
                  {zoneChoices.map((z) => (
                    <SelectItem key={z.id} value={z.id}>
                      {z.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="fleet-filter-shift">Shift</Label>
              <Select value={shiftFilter} onValueChange={setShiftFilter}>
                <SelectTrigger id="fleet-filter-shift" data-testid="fleet-filter-shift">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY}>Any shift</SelectItem>
                  {Object.entries(SHIFT_LABEL).map(([value, label]) => (
                    <SelectItem key={value} value={value}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Button type="button" variant="outline" onClick={reset} disabled={!filtered} data-testid="fleet-reset-btn">
                Reset filters
              </Button>
            </div>
          </div>

          <p className="sr-only" role="status">
            {loading ? 'Loading the fleet…' : `${shown.length} couriers`}
          </p>

          {error && (
            <Banner
              tone="err"
              testId="fleet-error"
              reference={error.correlationId}
              action={
                <div>
                  <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                    Retry
                  </Button>
                </div>
              }
            >
              {error.status === 403 ? 'Your account is not allowed to view the fleet.' : error.message || "Couldn't load the fleet. Try again."}
            </Banner>
          )}

          {showEmpty ? (
            <div data-testid="fleet-empty" className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
              {filtered ? (
                <SearchX size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
              ) : (
                <Truck size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
              )}
              <CardTitle>{filtered ? 'No couriers match your filters' : 'No couriers yet'}</CardTitle>
              <CardDescription>
                {filtered ? 'Try another zone or shift.' : 'Add a courier to start assigning parcels. A courier needs an account with the Courier role.'}
              </CardDescription>
              {filtered ? (
                <Button type="button" variant="outline" className="mt-2" onClick={reset} data-testid="fleet-empty-reset-btn">
                  Reset filters
                </Button>
              ) : (
                <Button type="button" className="mt-2" onClick={() => setAdding(true)} data-testid="fleet-empty-add-btn">
                  <Plus aria-hidden="true" /> Add courier
                </Button>
              )}
            </div>
          ) : (
            !error && (
              <Table aria-label="Couriers" data-testid="fleet-table" className="min-w-max">
                <TableHeader>
                  <TableRow>
                    <TableHead>Courier</TableHead>
                    <TableHead>Shift</TableHead>
                    <TableHead>Zone</TableHead>
                    <TableHead>Vehicle</TableHead>
                    <TableHead className="text-right">Parcels / capacity</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading && couriers.length === 0
                    ? [0, 1, 2, 3].map((i) => (
                        <TableRow key={i} aria-hidden="true">
                          <TableCell>
                            <Skeleton className="h-4 w-40" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-5 w-20" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-28" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-20" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="ml-auto h-4 w-16" />
                          </TableCell>
                        </TableRow>
                      ))
                    : shown.map((c) => {
                        const known = c.active_count !== null;
                        const full = known && c.max_capacity > 0 && (c.active_count ?? 0) >= c.max_capacity;
                        return (
                          <TableRow key={c.user_id} data-testid="fleet-row">
                            <TableCell>
                              <div className="font-medium">{c.courier_name || <span className="text-text-muted">Name not provided</span>}</div>
                              <div className="font-mono text-xs text-text-muted">{c.employee_code}</div>
                            </TableCell>
                            <TableCell>
                              <Badge variant={SHIFT_VARIANT[c.status] ?? 'neutral'}>{SHIFT_LABEL[c.status] ?? c.status}</Badge>
                            </TableCell>
                            <TableCell>
                              <div>{zoneName.get(c.current_zone_id) ?? c.current_zone_id}</div>
                              {zoneName.has(c.current_zone_id) && <div className="font-mono text-xs text-text-muted">{c.current_zone_id}</div>}
                            </TableCell>
                            <TableCell>{titleCase(c.vehicle_type)}</TableCell>
                            <TableCell className="text-right">
                              <span className="font-mono tabular-nums" title={known ? undefined : 'The API does not send the current workload yet'}>
                                {known ? c.active_count : '—'} / {c.max_capacity}
                              </span>
                              {full && (
                                <Badge variant="warning" className="ml-2">
                                  Full
                                </Badge>
                              )}
                            </TableCell>
                          </TableRow>
                        );
                      })}
                </TableBody>
              </Table>
            )
          )}

          {!error && !loading && couriers.length > 0 && (
            <div className="grid gap-1">
              <p className="text-sm text-text-muted tabular-nums" data-testid="fleet-summary">
                {filtered ? `${shown.length} of ${couriers.length} couriers` : `${couriers.length} ${couriers.length === 1 ? 'courier' : 'couriers'}`}
              </p>
              <FieldHint>Couriers switch their own shift in the courier app. Staff cannot change another courier&apos;s shift from here.</FieldHint>
            </div>
          )}
        </CardContent>
      </Card>

      {adding && (
        <AddCourierModal
          token={token}
          canListUsers={canListUsers}
          zones={zones}
          existingIds={everyId}
          onClose={() => setAdding(false)}
          onSaved={(message) => {
            setAdding(false);
            onNotice(message);
            void load();
          }}
        />
      )}
    </>
  );
};
