import React, { useCallback, useEffect, useState } from 'react';
import { ChevronLeft, ChevronRight, History, RefreshCw } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError } from '../api';
import { listAuditPage, type AuditRow, type PageMeta } from './consoleApi';
import { AdminDialog, Banner, formatWhen } from './ui';

// Full audit log (core-api/internal/admin/API.md section 4). Read only: entries cannot be
// edited or deleted, and the server redacts secret-looking values in the snapshots.

const PER_PAGE = 20;

/** "ZONE_CREATED" becomes "Zone created". */
export function humanizeAction(action: string): string {
  const text = action.replace(/_/g, ' ').toLowerCase();
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function titleCase(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1).toLowerCase();
}

function snapshot(value: unknown): string {
  if (value === null || value === undefined) return 'None';
  return JSON.stringify(value, null, 2);
}

interface AuditLogPanelProps {
  token: string | null;
  /** Changes whenever the console knows data was edited, so the list reloads. */
  refreshKey: number;
}

export const AuditLogPanel: React.FC<AuditLogPanelProps> = ({ token, refreshKey }) => {
  const [rows, setRows] = useState<AuditRow[]>([]);
  const [meta, setMeta] = useState<PageMeta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [detail, setDetail] = useState<AuditRow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await listAuditPage(token, page, PER_PAGE);
      setRows(res.rows);
      setMeta(res.meta);
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load the audit log. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, page]);

  useEffect(() => {
    void load();
  }, [load, refreshKey]);

  const totalPages = meta?.total_pages ?? 1;
  const showEmpty = !loading && !error && rows.length === 0;

  return (
    <Card role="region" aria-labelledby="audit-title">
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
        <div className="grid gap-1.5">
          <CardTitle id="audit-title">Audit log</CardTitle>
          <CardDescription>Every recorded change, newest first. The log is read only: entries cannot be edited or deleted.</CardDescription>
        </div>
        <Button
          type="button"
          variant="outline"
          size="icon"
          onClick={() => void load()}
          disabled={loading}
          aria-label="Refresh audit log"
          title="Refresh audit log"
        >
          <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
        </Button>
      </CardHeader>

      <CardContent className="grid gap-4">
        <p className="sr-only" role="status">
          {loading ? 'Loading the audit log…' : ''}
        </p>

        {error && (
          <Banner
            tone="err"
            reference={error.correlationId}
            action={
              <div>
                <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                  Retry
                </Button>
              </div>
            }
          >
            {error.status === 403 ? 'Your account is not allowed to view the audit log.' : error.message || "Couldn't load the audit log. Try again."}
          </Banner>
        )}

        {showEmpty ? (
          <div className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
            <History size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
            <CardTitle>No changes recorded yet</CardTitle>
            <CardDescription>Zone, setting and account changes appear here as they happen.</CardDescription>
          </div>
        ) : (
          <Table aria-label="Audit log" data-testid="admin-audit-table" className="min-w-max">
            <TableHeader>
              <TableRow>
                <TableHead>When</TableHead>
                <TableHead>Change</TableHead>
                <TableHead>Target</TableHead>
                <TableHead>By</TableHead>
                <TableHead>Reason</TableHead>
                <TableHead className="text-right">Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading && rows.length === 0
                ? [0, 1, 2, 3, 4].map((i) => (
                    <TableRow key={i} aria-hidden="true">
                      <TableCell>
                        <Skeleton className="h-4 w-32" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-28" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-36" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-5 w-16" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-40" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="ml-auto h-8 w-16" />
                      </TableCell>
                    </TableRow>
                  ))
                : rows.map((r) => (
                    <TableRow key={r.id}>
                      <TableCell className="whitespace-nowrap font-mono tabular-nums">{formatWhen(r.created_at)}</TableCell>
                      <TableCell className="font-medium">{humanizeAction(r.action)}</TableCell>
                      <TableCell>
                        <div>{titleCase(r.target_type)}</div>
                        <div className="font-mono text-xs text-text-muted">{r.target_id}</div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="neutral">{r.actor_role}</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="max-w-xs">{r.reason ?? <span className="text-text-muted">None given</span>}</div>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="touch:min-h-[44px]"
                          onClick={() => setDetail(r)}
                          aria-label={`View details of ${humanizeAction(r.action)} on ${r.target_id}`}
                          data-testid="admin-audit-details-btn"
                        >
                          View
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
            </TableBody>
          </Table>
        )}

        {meta && meta.total_items > 0 && (
          <div className="flex items-center justify-between gap-3 text-sm text-text-muted">
            <span className="tabular-nums">
              Page {meta.page} of {totalPages} · {meta.total_items} {meta.total_items === 1 ? 'entry' : 'entries'}
            </span>
            {meta.total_pages > 1 && (
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
            )}
          </div>
        )}
      </CardContent>

      {detail && (
        <AdminDialog
          title={`${humanizeAction(detail.action)} · ${detail.target_id}`}
          description="Secret-looking values are redacted by the server before they are stored."
          onClose={() => setDetail(null)}
          testId="admin-audit-dialog"
        >
          <div className="grid max-h-96 gap-4 overflow-y-auto px-5 py-5 text-sm">
            <dl className="grid grid-cols-3 gap-x-4 gap-y-2">
              <dt className="text-text-muted">When</dt>
              <dd className="col-span-2 font-mono tabular-nums">{formatWhen(detail.created_at)}</dd>
              <dt className="text-text-muted">By</dt>
              <dd className="col-span-2">
                {detail.actor_role}
                {detail.actor_id ? <span className="block break-all font-mono text-xs text-text-muted">{detail.actor_id}</span> : null}
              </dd>
              <dt className="text-text-muted">Reason</dt>
              <dd className="col-span-2">{detail.reason ?? 'None given'}</dd>
              <dt className="text-text-muted">Request</dt>
              <dd className="col-span-2 break-all font-mono text-xs">{detail.request_id ?? 'Not recorded'}</dd>
            </dl>
            <div className="grid gap-1.5">
              <p className="text-xs font-medium text-text-muted">Before</p>
              <pre className="overflow-x-auto rounded-sm border border-border bg-surface-subtle p-3 font-mono text-xs">{snapshot(detail.previous_state)}</pre>
            </div>
            <div className="grid gap-1.5">
              <p className="text-xs font-medium text-text-muted">After</p>
              <pre className="overflow-x-auto rounded-sm border border-border bg-surface-subtle p-3 font-mono text-xs">{snapshot(detail.new_state)}</pre>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" data-autofocus onClick={() => setDetail(null)}>
              Close
            </Button>
          </DialogFooter>
        </AdminDialog>
      )}
    </Card>
  );
};
