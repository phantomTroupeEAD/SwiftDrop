import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { ChevronLeft, ChevronRight, Plus, Receipt, RefreshCw, SearchX } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError } from '../api';
import { Banner } from '../admin-console/ui';
import { listZoneOptions, type ZoneOption } from '../fleet/fleetApi';
import { DeactivateRuleModal } from './DeactivateRuleModal';
import { PricingRuleFormModal, dayOf } from './PricingRuleFormModal';
import { formatLkr } from './pricingMoney';
import { currentBackend, listRules, resetWritesProbe, writesAvailable, type PricingRule } from './pricingApi';
import type { PageMeta } from '../admin-console/consoleApi';

// Pricing rules: what a parcel costs on each route is set here as a base fee and a rate per
// kilogram, in integer cents shown as LKR. The screen only stores and shows these amounts. It never
// works out a price: the server does, when a quote is requested.

const ANY = 'ANY';
const PER_PAGE_CHOICES = ['10', '20', '50', '100'];

function dayText(iso: string): string {
  const d = dayOf(iso);
  if (!d) return iso;
  return new Date(`${d}T00:00:00Z`).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' });
}

interface PricingPanelProps {
  token: string | null;
  onNotice: (message: string) => void;
}

export const PricingPanel: React.FC<PricingPanelProps> = ({ token, onNotice }) => {
  const [rows, setRows] = useState<PricingRule[]>([]);
  const [meta, setMeta] = useState<PageMeta | null>(null);
  const [zones, setZones] = useState<ZoneOption[]>([]);
  const [origin, setOrigin] = useState(ANY);
  const [destination, setDestination] = useState(ANY);
  const [active, setActive] = useState('true');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(20);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [sample, setSample] = useState(false);
  // Add, Change and Deactivate only appear when the server can do them (always true for the sample API).
  const [canWrite, setCanWrite] = useState(false);
  const [editing, setEditing] = useState<PricingRule | 'new' | null>(null);
  const [deactivating, setDeactivating] = useState<PricingRule | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await listRules(token, page, perPage, {
        origin: origin === ANY ? undefined : origin,
        destination: destination === ANY ? undefined : destination,
        active: active === ANY ? undefined : active,
      });
      setRows(res.rows);
      setMeta(res.meta);
      setSample(currentBackend() === 'sample');
      void writesAvailable().then(setCanWrite);
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load pricing rules. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, page, perPage, origin, destination, active]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    listZoneOptions()
      .then(setZones)
      .catch(() => setZones([]));
  }, []);

  const zoneName = useMemo(() => new Map(zones.map((z) => [z.id, z.name])), [zones]);
  const label = (id: string) => zoneName.get(id) ?? id;
  const routeLabel = (r: PricingRule) => `${label(r.origin_zone_id)} to ${label(r.destination_zone_id)}`;

  const filtered = origin !== ANY || destination !== ANY || active !== 'true';
  const reset = () => {
    setOrigin(ANY);
    setDestination(ANY);
    setActive('true');
    setPage(1);
  };
  const totalPages = Math.max(meta?.total_pages ?? 1, 1);
  const showEmpty = !loading && !error && rows.length === 0;
  const first = meta && rows.length > 0 ? (meta.page - 1) * meta.per_page + 1 : 0;
  const last = first > 0 ? first + rows.length - 1 : 0;

  const saved = (_rule: PricingRule, message: string) => {
    setEditing(null);
    setDeactivating(null);
    onNotice(message);
    void load();
  };
  const stale = () => {
    setEditing(null);
    setDeactivating(null);
    void load();
  };

  const zoneSelect = (id: string, testId: string, value: string, set: (v: string) => void, anyText: string) => (
    <Select
      value={value}
      onValueChange={(v) => {
        set(v);
        setPage(1);
      }}
    >
      <SelectTrigger id={id} data-testid={testId}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ANY}>{anyText}</SelectItem>
        {zones.map((z) => (
          <SelectItem key={z.id} value={z.id}>
            {z.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );

  return (
    <>
      {sample && (
        <Banner tone="attn" testId="pricing-sample-banner">
          Sample data. The pricing rules service is not available yet, so this screen is using made-up rules that live only in this browser tab and reset when you reload. Nothing you change here reaches the real prices.
        </Banner>
      )}

      {!sample && !canWrite && !error && meta && (
        <Banner tone="info" testId="pricing-readonly-banner">
          Read only. These are the live pricing rules, read from the server, and quotes already use them. The server can list rules but cannot yet add, change or deactivate them, so those buttons are hidden until it can.
        </Banner>
      )}

      <Card role="region" aria-labelledby="pricing-title">
        <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
          <div className="grid gap-1.5">
            <CardTitle id="pricing-title">Pricing rules</CardTitle>
            <CardDescription>A base fee and a rate per kilogram for each route. The server uses these to work out every quote.</CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <Button type="button" variant="outline" size="icon" onClick={() => {
                resetWritesProbe();
                void load();
              }}
              disabled={loading}
              aria-label="Refresh pricing rules" title="Refresh pricing rules" data-testid="pricing-refresh-btn">
              <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
            </Button>
            {canWrite && (
              <Button type="button" onClick={() => setEditing('new')} disabled={Boolean(error) && rows.length === 0 && !meta} data-testid="pricing-add-btn">
                <Plus aria-hidden="true" /> Add rule
              </Button>
            )}
          </div>
        </CardHeader>

        <CardContent className="grid gap-4">
          <div role="group" aria-label="Pricing filters" data-testid="pricing-filters" className="grid gap-4 rounded-md border border-border bg-surface-subtle p-4 sm:grid-cols-2 lg:grid-cols-4 lg:items-end">
            <div className="grid content-start gap-1.5">
              <Label htmlFor="pricing-filter-origin">From zone</Label>
              {zoneSelect('pricing-filter-origin', 'pricing-filter-origin', origin, setOrigin, 'Any zone')}
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="pricing-filter-destination">To zone</Label>
              {zoneSelect('pricing-filter-destination', 'pricing-filter-destination', destination, setDestination, 'Any zone')}
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="pricing-filter-status">Status</Label>
              <Select
                value={active}
                onValueChange={(v) => {
                  setActive(v);
                  setPage(1);
                }}
              >
                <SelectTrigger id="pricing-filter-status" data-testid="pricing-filter-status">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="true">Active</SelectItem>
                  <SelectItem value="false">Inactive</SelectItem>
                  <SelectItem value={ANY}>Active and inactive</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Button type="button" variant="outline" onClick={reset} disabled={!filtered} data-testid="pricing-reset-btn">
                Reset filters
              </Button>
            </div>
          </div>

          <p className="sr-only" role="status">
            {loading ? 'Loading pricing rules…' : meta ? `${meta.total_items} pricing rules` : ''}
          </p>

          {error && (
            <Banner
              tone="err"
              testId="pricing-error"
              reference={error.correlationId}
              action={
                <div>
                  <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                    Retry
                  </Button>
                </div>
              }
            >
              {error.status === 403 ? 'Your account is not allowed to view pricing rules.' : error.message || "Couldn't load pricing rules. Try again."}
            </Banner>
          )}

          {showEmpty ? (
            <div data-testid="pricing-empty" className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
              {filtered ? <SearchX size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" /> : <Receipt size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />}
              <CardTitle>{filtered ? 'No rules match your filters' : 'No pricing rules yet'}</CardTitle>
              <CardDescription>{filtered ? 'Try other zones or another status.' : canWrite ? 'Add a rule for a route so parcels can be priced.' : 'The server has no pricing rules yet.'}</CardDescription>
              {filtered ? (
                <Button type="button" variant="outline" className="mt-2" onClick={reset} data-testid="pricing-empty-reset-btn">
                  Reset filters
                </Button>
              ) : (
                canWrite && (
                  <Button type="button" className="mt-2" onClick={() => setEditing('new')} data-testid="pricing-empty-add-btn">
                    <Plus aria-hidden="true" /> Add rule
                  </Button>
                )
              )}
            </div>
          ) : (
            !error && (
              <Table aria-label="Pricing rules" data-testid="pricing-table" className="min-w-max">
                <TableHeader>
                  <TableRow>
                    <TableHead>Route</TableHead>
                    <TableHead className="text-right">Base fee</TableHead>
                    <TableHead className="text-right">Per kg</TableHead>
                    <TableHead>Applies</TableHead>
                    {canWrite && <TableHead className="text-right">Actions</TableHead>}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading && rows.length === 0
                    ? [0, 1, 2, 3].map((i) => (
                        <TableRow key={i} aria-hidden="true">
                          <TableCell>
                            <Skeleton className="h-4 w-44" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="ml-auto h-4 w-24" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="ml-auto h-4 w-20" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-40" />
                          </TableCell>
                          {canWrite && (
                            <TableCell>
                              <Skeleton className="ml-auto h-8 w-32" />
                            </TableCell>
                          )}
                        </TableRow>
                      ))
                    : rows.map((r) => (
                        <TableRow key={`${r.id}-${r.version}`} data-testid="pricing-row">
                          <TableCell>
                            <div className="font-medium">{routeLabel(r)}</div>
                            <div className="mt-1 flex flex-wrap items-center gap-2">
                              <Badge variant={r.is_active ? 'success' : 'neutral'}>{r.is_active ? 'Active' : 'Inactive'}</Badge>
                              <span className="font-mono text-xs tabular-nums text-text-muted">
                                {r.id} · version {r.version}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell className="whitespace-nowrap text-right font-mono tabular-nums">{formatLkr(r.base_fee_cents)}</TableCell>
                          <TableCell className="whitespace-nowrap text-right font-mono tabular-nums">{formatLkr(r.per_kg_rate_cents)}</TableCell>
                          <TableCell className="whitespace-nowrap font-mono text-xs tabular-nums">
                            {dayText(r.effective_from)} {r.effective_until ? `to ${dayText(r.effective_until)}` : 'onwards'}
                          </TableCell>
                          {canWrite && (
                            <TableCell className="text-right">
                              {r.is_active ? (
                                <div className="flex justify-end gap-2">
                                  <Button type="button" size="sm" variant="outline" className="touch:min-h-[44px]" onClick={() => setEditing(r)} aria-label={`Change rule ${r.id}, ${routeLabel(r)}`} data-testid="pricing-edit-btn">
                                    Change
                                  </Button>
                                  <Button type="button" size="sm" variant="outline" className="touch:min-h-[44px]" onClick={() => setDeactivating(r)} aria-label={`Deactivate rule ${r.id}, ${routeLabel(r)}`} data-testid="pricing-deactivate-btn">
                                    Deactivate
                                  </Button>
                                </div>
                              ) : (
                                <span className="text-xs text-text-muted">No changes</span>
                              )}
                            </TableCell>
                          )}
                        </TableRow>
                      ))}
                </TableBody>
              </Table>
            )
          )}

          {meta && !error && (
            <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-text-muted">
              <span className="tabular-nums" data-testid="pricing-summary">
                {meta.total_items === 0 ? (filtered ? 'No matching rules' : 'No rules') : `Showing ${first}–${last} of ${meta.total_items} ${filtered ? 'matching ' : ''}${meta.total_items === 1 ? 'rule' : 'rules'} · page ${meta.page} of ${totalPages}`}
              </span>
              <div className="flex flex-wrap items-center gap-3">
                <div className="flex items-center gap-2">
                  <Label htmlFor="pricing-per-page" className="text-xs">
                    Rows per page
                  </Label>
                  <Select
                    value={String(perPage)}
                    onValueChange={(v) => {
                      setPerPage(Number(v));
                      setPage(1);
                    }}
                  >
                    <SelectTrigger id="pricing-per-page" className="w-24" data-testid="pricing-per-page">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {PER_PAGE_CHOICES.map((c) => (
                        <SelectItem key={c} value={c}>
                          {c}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex items-center gap-2">
                  <Button type="button" variant="outline" size="sm" className="touch:min-h-[44px]" onClick={() => setPage((p) => Math.max(p - 1, 1))} disabled={loading || page <= 1} data-testid="pricing-prev-btn">
                    <ChevronLeft aria-hidden="true" /> Previous
                  </Button>
                  <Button type="button" variant="outline" size="sm" className="touch:min-h-[44px]" onClick={() => setPage((p) => p + 1)} disabled={loading || page >= totalPages} data-testid="pricing-next-btn">
                    Next <ChevronRight aria-hidden="true" />
                  </Button>
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {editing && <PricingRuleFormModal token={token} zones={zones} rule={editing === 'new' ? undefined : editing} onClose={() => setEditing(null)} onSaved={saved} onStale={stale} />}
      {deactivating && <DeactivateRuleModal token={token} rule={deactivating} routeLabel={routeLabel(deactivating)} onClose={() => setDeactivating(null)} onSaved={saved} onStale={stale} />}
    </>
  );
};
