import React, { useCallback, useEffect, useState } from 'react';
import { ChevronLeft, ChevronRight, MapPin, Plus, RefreshCw } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError, listZones } from './zonesApi';
import type { PageMeta, Zone } from './types';
import { ZoneFormModal } from './ZoneFormModal';
import { ZoneStatusModal } from './ZoneStatusModal';
import { Banner } from '../admin-console/ui';

// F-10 Delivery zone management (Member 7, Doc 2). Shown as a section of the admin console.

const PER_PAGE = 20;
const MAX_PREFIX_CHIPS = 3;

type FormState = { mode: 'create' } | { mode: 'edit'; zone: Zone } | null;

interface ZonesPanelProps {
  token: string | null;
  /** Tells the console to show a success message. */
  onNotice: (message: string) => void;
  /** Tells the console that zone data changed, so its summary numbers can refresh. */
  onChanged: () => void;
}

export const ZonesPanel: React.FC<ZonesPanelProps> = ({ token, onNotice, onChanged }) => {
  const [zones, setZones] = useState<Zone[]>([]);
  const [meta, setMeta] = useState<PageMeta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<ApiError | null>(null);
  const [form, setForm] = useState<FormState>(null);
  const [statusZone, setStatusZone] = useState<Zone | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const res = await listZones(token, page, PER_PAGE);
      setZones(res.zones);
      setMeta(res.meta);
    } catch (err) {
      setLoadError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load zones. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, page]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleSaved = (_zone: Zone, message: string) => {
    setForm(null);
    setStatusZone(null);
    onNotice(message);
    onChanged();
    void load();
  };

  const handleStale = () => {
    setForm(null);
    setStatusZone(null);
    void load();
  };

  const totalPages = meta?.total_pages ?? 1;
  const showEmpty = !loading && !loadError && zones.length === 0;

  return (
    <Card aria-labelledby="zones-title" role="region">
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
        <div className="grid gap-1.5">
          <CardTitle id="zones-title">Delivery zones</CardTitle>
          <CardDescription>
            Create zones, set the postal code prefixes they cover, and switch them on or off. Zones hold no prices.
          </CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="icon"
            onClick={() => void load()}
            disabled={loading}
            aria-label="Refresh zones"
            title="Refresh zones"
          >
            <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
          </Button>
          <Button type="button" onClick={() => setForm({ mode: 'create' })} data-testid="admin-zone-new-btn">
            <Plus aria-hidden="true" /> New zone
          </Button>
        </div>
      </CardHeader>

      <CardContent className="grid gap-4">
        <p className="sr-only" role="status">
          {loading ? 'Loading zones…' : ''}
        </p>

        {loadError && (
          <Banner
            tone="err"
            reference={loadError.correlationId}
            action={
              <div>
                <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                  Retry
                </Button>
              </div>
            }
          >
            {loadError.status === 403 ? 'Your account is not allowed to manage zones.' : loadError.message || "Couldn't load zones. Try again."}
          </Banner>
        )}

        {showEmpty ? (
          <div className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
            <MapPin size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
            <CardTitle>No delivery zones yet</CardTitle>
            <CardDescription>Create the first zone and add the postal code prefixes it covers.</CardDescription>
            <Button type="button" className="mt-2" onClick={() => setForm({ mode: 'create' })}>
              <Plus aria-hidden="true" /> Create a zone
            </Button>
          </div>
        ) : (
          <Table aria-label="Delivery zones" data-testid="admin-zones-table" className="min-w-max">
            <TableHeader>
              <TableRow>
                <TableHead>Zone</TableHead>
                <TableHead>Code</TableHead>
                <TableHead>Postal prefixes</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading && zones.length === 0
                ? [0, 1, 2, 3, 4].map((i) => (
                    <TableRow key={i} aria-hidden="true">
                      <TableCell>
                        <Skeleton className="h-4 w-36" />
                        <Skeleton className="mt-2 h-3 w-24" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-14" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-5 w-32" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-5 w-20" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="ml-auto h-8 w-28" />
                      </TableCell>
                    </TableRow>
                  ))
                : zones.map((z) => (
                    <TableRow key={z.id}>
                      <TableCell>
                        <div className="font-medium">{z.name}</div>
                        <div className="font-mono text-xs text-text-muted">{z.id}</div>
                        {z.description && <div className="mt-0.5 max-w-xs text-xs text-text-muted">{z.description}</div>}
                      </TableCell>
                      <TableCell className="font-mono tabular-nums">{z.code}</TableCell>
                      <TableCell>
                        <div className="flex flex-wrap items-center gap-1">
                          {z.postal_prefixes.slice(0, MAX_PREFIX_CHIPS).map((p) => (
                            <Badge key={p} variant="outline" className="font-mono tabular-nums">
                              {p}
                            </Badge>
                          ))}
                          {z.postal_prefixes.length > MAX_PREFIX_CHIPS && (
                            <span className="text-xs text-text-muted" title={z.postal_prefixes.join(', ')}>
                              +{z.postal_prefixes.length - MAX_PREFIX_CHIPS} more
                            </span>
                          )}
                        </div>
                        <div className="mt-1 text-xs tabular-nums text-text-muted">
                          {z.postal_prefixes.length} {z.postal_prefixes.length === 1 ? 'prefix' : 'prefixes'}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={z.enabled ? 'success' : 'neutral'}>{z.enabled ? 'Enabled' : 'Disabled'}</Badge>
                        <div className="mt-1 font-mono text-xs tabular-nums text-text-muted">version {z.version}</div>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex flex-wrap items-center justify-end gap-2">
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className="touch:min-h-[44px]"
                            onClick={() => setForm({ mode: 'edit', zone: z })}
                            aria-label={`Edit zone ${z.name}`}
                            data-testid="admin-zone-edit-btn"
                          >
                            Edit
                          </Button>
                          <Button
                            type="button"
                            variant={z.enabled ? 'destructive' : 'secondary'}
                            size="sm"
                            className="touch:min-h-[44px]"
                            onClick={() => setStatusZone(z)}
                            aria-label={`${z.enabled ? 'Disable' : 'Enable'} zone ${z.name}`}
                            data-testid="admin-zone-status-btn"
                          >
                            {z.enabled ? 'Disable' : 'Enable'}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
            </TableBody>
          </Table>
        )}

        {meta && meta.total_pages > 1 && (
          <div className="flex items-center justify-between gap-3 text-sm text-text-muted">
            <span className="tabular-nums">
              Page {meta.page} of {totalPages} · {meta.total_items} zones
            </span>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={loading || page <= 1}
                aria-label="Previous page"
              >
                <ChevronLeft aria-hidden="true" />
              </Button>
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={loading || page >= totalPages}
                aria-label="Next page"
              >
                <ChevronRight aria-hidden="true" />
              </Button>
            </div>
          </div>
        )}
      </CardContent>

      {form && (
        <ZoneFormModal
          mode={form.mode}
          zone={form.mode === 'edit' ? form.zone : undefined}
          token={token}
          onClose={() => setForm(null)}
          onSaved={handleSaved}
          onStale={handleStale}
        />
      )}
      {statusZone && (
        <ZoneStatusModal zone={statusZone} token={token} onClose={() => setStatusZone(null)} onSaved={handleSaved} onStale={handleStale} />
      )}
    </Card>
  );
};
