import React, { useCallback, useEffect, useState } from 'react';
import { ChevronLeft, ChevronRight, RefreshCw, SearchX, Store } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError } from '../api';
import type { PageMeta } from '../admin-console/consoleApi';
import { Banner, formatWhen } from '../admin-console/ui';
import { MerchantStatusBadge, statusLabel } from './MerchantStatusBadge';
import { MERCHANT_STATUSES, listMerchants, type Merchant } from './merchantsApi';

// Merchant list for admins: every merchant account with its status, newest first, filtered by
// status and paged on the server (page, per_page). Read only.

const ANY = 'ANY';
const PER_PAGE_CHOICES = ['10', '20', '50', '100'];
const DEFAULT_PER_PAGE = 20;

interface MerchantsPanelProps {
  token: string | null;
  /** Opens one merchant (the page /ops/merchants/:id). */
  onOpen: (id: string) => void;
}

export const MerchantsPanel: React.FC<MerchantsPanelProps> = ({ token, onOpen }) => {
  const [rows, setRows] = useState<Merchant[]>([]);
  const [meta, setMeta] = useState<PageMeta | null>(null);
  const [status, setStatus] = useState(ANY);
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(DEFAULT_PER_PAGE);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await listMerchants(token, status === ANY ? '' : status, page, perPage);
      setRows(res.rows);
      setMeta(res.meta);
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load merchants. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, status, page, perPage]);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = status !== ANY;
  const totalPages = Math.max(meta?.total_pages ?? 1, 1);
  const showEmpty = !loading && !error && rows.length === 0;
  const first = meta && rows.length > 0 ? (meta.page - 1) * meta.per_page + 1 : 0;
  const last = first > 0 ? first + rows.length - 1 : 0;

  return (
    <>
      <Card role="region" aria-labelledby="merchants-title">
        <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
          <div className="grid gap-1.5">
            <CardTitle id="merchants-title">Merchant accounts</CardTitle>
            <CardDescription>Every merchant that applied, newest first. Open one to review it.</CardDescription>
          </div>
          <Button type="button" variant="outline" size="icon" onClick={() => void load()} disabled={loading} aria-label="Refresh merchants" title="Refresh merchants" data-testid="merchants-refresh-btn">
            <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
          </Button>
        </CardHeader>

        <CardContent className="grid gap-4">
          <div role="group" aria-label="Merchant filters" data-testid="merchants-filters" className="grid gap-4 rounded-md border border-border bg-surface-subtle p-4 sm:grid-cols-2 sm:items-end">
            <div className="grid content-start gap-1.5">
              <Label htmlFor="merchants-status">Status</Label>
              <Select
                value={status}
                onValueChange={(v) => {
                  setStatus(v);
                  setPage(1);
                }}
              >
                <SelectTrigger id="merchants-status" data-testid="merchants-filter-status">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY}>Any status</SelectItem>
                  {MERCHANT_STATUSES.map((s) => (
                    <SelectItem key={s} value={s}>
                      {statusLabel(s)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setStatus(ANY);
                  setPage(1);
                }}
                disabled={!filtered}
                data-testid="merchants-reset-btn"
              >
                Reset filters
              </Button>
            </div>
          </div>

          <p className="sr-only" role="status">
            {loading ? 'Loading merchants…' : meta ? `${meta.total_items} merchants` : ''}
          </p>

          {error && (
            <Banner
              tone="err"
              testId="merchants-error"
              reference={error.correlationId}
              action={
                <div>
                  <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                    Retry
                  </Button>
                </div>
              }
            >
              {error.status === 403 ? 'Your account is not allowed to view merchants.' : error.message || "Couldn't load merchants. Try again."}
            </Banner>
          )}

          {showEmpty ? (
            <div data-testid="merchants-empty" className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
              {filtered ? <SearchX size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" /> : <Store size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />}
              <CardTitle>{filtered ? 'No merchants match your filters' : 'No merchants yet'}</CardTitle>
              <CardDescription>{filtered ? 'Try another status.' : 'Merchant applications appear here after a customer applies.'}</CardDescription>
              {filtered && (
                <Button
                  type="button"
                  variant="outline"
                  className="mt-2"
                  onClick={() => {
                    setStatus(ANY);
                    setPage(1);
                  }}
                  data-testid="merchants-empty-reset-btn"
                >
                  Reset filters
                </Button>
              )}
            </div>
          ) : (
            !error && (
              <Table aria-label="Merchants" data-testid="merchants-table" className="min-w-max">
                <TableHeader>
                  <TableRow>
                    <TableHead>Business</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Phone</TableHead>
                    <TableHead>City</TableHead>
                    <TableHead>Applied</TableHead>
                    <TableHead className="text-right">Review</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading && rows.length === 0
                    ? [0, 1, 2, 3, 4].map((i) => (
                        <TableRow key={i} aria-hidden="true">
                          <TableCell>
                            <Skeleton className="h-4 w-44" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-5 w-24" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-28" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-20" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-32" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="ml-auto h-8 w-16" />
                          </TableCell>
                        </TableRow>
                      ))
                    : rows.map((m) => (
                        <TableRow key={m.merchant_id} data-testid="merchants-row">
                          <TableCell>
                            <div className="font-medium">{m.business_name}</div>
                            <div className="text-xs text-text-muted">{m.contact_email}</div>
                          </TableCell>
                          <TableCell>
                            <MerchantStatusBadge status={m.status} />
                          </TableCell>
                          <TableCell className="whitespace-nowrap font-mono tabular-nums">{m.contact_phone}</TableCell>
                          <TableCell>{m.address?.city || <span className="text-text-muted">Not provided</span>}</TableCell>
                          <TableCell className="whitespace-nowrap font-mono tabular-nums">{formatWhen(m.created_at)}</TableCell>
                          <TableCell className="text-right">
                            <Button type="button" variant="outline" size="sm" className="touch:min-h-[44px]" onClick={() => onOpen(m.merchant_id)} aria-label={`Open ${m.business_name}`} data-testid="merchants-view-btn">
                              Open
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                </TableBody>
              </Table>
            )
          )}

          {meta && !error && (
            <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-text-muted">
              <span className="tabular-nums" data-testid="merchants-summary">
                {meta.total_items === 0
                  ? filtered
                    ? 'No matching merchants'
                    : 'No merchants'
                  : `Showing ${first}–${last} of ${meta.total_items} ${filtered ? 'matching ' : ''}${meta.total_items === 1 ? 'merchant' : 'merchants'} · page ${meta.page} of ${totalPages}`}
              </span>
              <div className="flex flex-wrap items-center gap-3">
                <div className="flex items-center gap-2">
                  <Label htmlFor="merchants-per-page" className="text-xs">
                    Rows per page
                  </Label>
                  <Select
                    value={String(perPage)}
                    onValueChange={(v) => {
                      setPerPage(Number(v));
                      setPage(1);
                    }}
                  >
                    <SelectTrigger id="merchants-per-page" className="w-24" data-testid="merchants-per-page">
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
                  <Button type="button" variant="outline" size="sm" className="touch:min-h-[44px]" onClick={() => setPage((p) => Math.max(p - 1, 1))} disabled={loading || page <= 1} data-testid="merchants-prev-btn">
                    <ChevronLeft aria-hidden="true" /> Previous
                  </Button>
                  <Button type="button" variant="outline" size="sm" className="touch:min-h-[44px]" onClick={() => setPage((p) => p + 1)} disabled={loading || page >= totalPages} data-testid="merchants-next-btn">
                    Next <ChevronRight aria-hidden="true" />
                  </Button>
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

    </>
  );
};
