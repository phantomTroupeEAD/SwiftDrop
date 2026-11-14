import React, { useCallback, useEffect, useState } from 'react';
import { ChevronLeft, ChevronRight, Filter, History, RefreshCw, SearchX } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError } from '../api';
import { listAuditPage, type AuditFilter, type AuditRow, type PageMeta } from './consoleApi';
import { AdminDialog, Banner, FieldError, formatWhen } from './ui';

// Audit log (core-api/internal/admin/API.md section 4). Read only: entries cannot be edited or
// deleted, and the server redacts secret-looking values in the snapshots. Filters and paging run
// on the server, so the total always counts the matching rows.

const DEFAULT_PER_PAGE = 20;
const PER_PAGE_CHOICES = ['10', '20', '50', '100'];
const ANY = 'ANY';

// The actions and target types the platform writes today. The server accepts any valid name,
// so a new action can still be filtered once this list learns about it.
const ACTIONS = ['ZONE_CREATED', 'ZONE_UPDATED', 'ZONE_ENABLED', 'ZONE_DISABLED', 'SETTING_CHANGED', 'USER_ROLE_UPDATED', 'USER_STATUS_UPDATED'];
const TARGET_TYPES = ['ZONE', 'SETTING', 'USER'];
const ACTOR_ROLES = ['ADMIN', 'DISPATCHER', 'COURIER', 'MERCHANT', 'CUSTOMER', 'SYSTEM'];
const TARGET_ID_PATTERN = /^[A-Za-z0-9_.:-]{1,64}$/;

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

interface FilterDraft {
  action: string;
  targetType: string;
  targetId: string;
  actorRole: string;
  from: string;
  to: string;
}

const EMPTY_FILTER: FilterDraft = { action: ANY, targetType: ANY, targetId: '', actorRole: ANY, from: '', to: '' };

function isEmpty(f: FilterDraft): boolean {
  return f.action === ANY && f.targetType === ANY && f.targetId.trim() === '' && f.actorRole === ANY && f.from === '' && f.to === '';
}

function toApi(f: FilterDraft): AuditFilter {
  return {
    action: f.action === ANY ? undefined : f.action,
    target_type: f.targetType === ANY ? undefined : f.targetType,
    target_id: f.targetId.trim() || undefined,
    actor_role: f.actorRole === ANY ? undefined : f.actorRole,
    from: f.from || undefined,
    to: f.to || undefined,
  };
}

/** A readable list of what is filtered, for the chips under the form. */
function describe(f: FilterDraft): string[] {
  const out: string[] = [];
  if (f.action !== ANY) out.push(`Change: ${humanizeAction(f.action)}`);
  if (f.targetType !== ANY) out.push(`Target type: ${titleCase(f.targetType)}`);
  if (f.targetId.trim()) out.push(`Target id: ${f.targetId.trim()}`);
  if (f.actorRole !== ANY) out.push(`By: ${f.actorRole}`);
  if (f.from) out.push(`From: ${f.from}`);
  if (f.to) out.push(`To: ${f.to}`);
  return out;
}

interface DraftErrors {
  targetId?: string;
  to?: string;
}

function check(f: FilterDraft): DraftErrors {
  const errors: DraftErrors = {};
  const id = f.targetId.trim();
  if (id && !TARGET_ID_PATTERN.test(id)) errors.targetId = 'Use 1 to 64 letters, digits or _ . : - (no spaces).';
  if (f.from && f.to && f.from > f.to) errors.to = 'The end date must not be before the start date.';
  return errors;
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
  const [perPage, setPerPage] = useState(DEFAULT_PER_PAGE);
  const [draft, setDraft] = useState<FilterDraft>(EMPTY_FILTER);
  const [applied, setApplied] = useState<FilterDraft>(EMPTY_FILTER);
  const [draftErrors, setDraftErrors] = useState<DraftErrors>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [detail, setDetail] = useState<AuditRow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await listAuditPage(token, page, perPage, toApi(applied));
      setRows(res.rows);
      setMeta(res.meta);
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load the audit log. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, page, perPage, applied]);

  useEffect(() => {
    void load();
  }, [load, refreshKey]);

  const apply = (e: React.FormEvent) => {
    e.preventDefault();
    const errors = check(draft);
    setDraftErrors(errors);
    if (Object.keys(errors).length > 0) return;
    setApplied(draft);
    setPage(1);
  };

  const reset = () => {
    setDraft(EMPTY_FILTER);
    setApplied(EMPTY_FILTER);
    setDraftErrors({});
    setPage(1);
  };

  const totalPages = meta?.total_pages ?? 1;
  const filtered = !isEmpty(applied);
  const showEmpty = !loading && !error && rows.length === 0;
  const first = meta && rows.length > 0 ? (meta.page - 1) * meta.per_page + 1 : 0;
  const last = first > 0 ? first + rows.length - 1 : 0;

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
        <form
          role="search"
          aria-label="Audit log filters"
          data-testid="admin-audit-filters"
          onSubmit={apply}
          noValidate
          className="grid gap-4 rounded-md border border-border bg-surface-subtle p-4"
        >
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div className="grid gap-1.5">
              <Label htmlFor="audit-action">Change</Label>
              <Select value={draft.action} onValueChange={(v) => setDraft((d) => ({ ...d, action: v }))}>
                <SelectTrigger id="audit-action" data-testid="admin-audit-filter-action">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY}>Any change</SelectItem>
                  {ACTIONS.map((a) => (
                    <SelectItem key={a} value={a}>
                      {humanizeAction(a)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="audit-target-type">Target type</Label>
              <Select value={draft.targetType} onValueChange={(v) => setDraft((d) => ({ ...d, targetType: v }))}>
                <SelectTrigger id="audit-target-type" data-testid="admin-audit-filter-target-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY}>Any target</SelectItem>
                  {TARGET_TYPES.map((t) => (
                    <SelectItem key={t} value={t}>
                      {titleCase(t)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="audit-actor-role">Done by</Label>
              <Select value={draft.actorRole} onValueChange={(v) => setDraft((d) => ({ ...d, actorRole: v }))}>
                <SelectTrigger id="audit-actor-role" data-testid="admin-audit-filter-actor-role">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY}>Any role</SelectItem>
                  {ACTOR_ROLES.map((r) => (
                    <SelectItem key={r} value={r}>
                      {r}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="audit-target-id">Target id</Label>
              <Input
                id="audit-target-id"
                data-testid="admin-audit-filter-target-id"
                className="font-mono"
                value={draft.targetId}
                onChange={(e) => {
                  setDraft((d) => ({ ...d, targetId: e.target.value }));
                  setDraftErrors((x) => ({ ...x, targetId: undefined }));
                }}
                placeholder="ZONE_COLOMBO_01"
                autoComplete="off"
                aria-invalid={Boolean(draftErrors.targetId)}
                aria-describedby={draftErrors.targetId ? 'audit-target-id-error' : undefined}
              />
              {draftErrors.targetId && <FieldError id="audit-target-id-error">{draftErrors.targetId}</FieldError>}
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="audit-from">From (Sri Lanka date)</Label>
              <Input
                id="audit-from"
                data-testid="admin-audit-filter-from"
                type="date"
                className="font-mono tabular-nums"
                value={draft.from}
                onChange={(e) => {
                  setDraft((d) => ({ ...d, from: e.target.value }));
                  setDraftErrors((x) => ({ ...x, to: undefined }));
                }}
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="audit-to">To (Sri Lanka date)</Label>
              <Input
                id="audit-to"
                data-testid="admin-audit-filter-to"
                type="date"
                className="font-mono tabular-nums"
                value={draft.to}
                onChange={(e) => {
                  setDraft((d) => ({ ...d, to: e.target.value }));
                  setDraftErrors((x) => ({ ...x, to: undefined }));
                }}
                aria-invalid={Boolean(draftErrors.to)}
                aria-describedby={draftErrors.to ? 'audit-to-error' : undefined}
              />
              {draftErrors.to && <FieldError id="audit-to-error">{draftErrors.to}</FieldError>}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button type="submit" data-testid="admin-audit-apply-btn">
              <Filter aria-hidden="true" /> Apply filters
            </Button>
            <Button type="button" variant="outline" onClick={reset} disabled={isEmpty(draft) && isEmpty(applied)} data-testid="admin-audit-reset-btn">
              Reset filters
            </Button>
          </div>
        </form>

        {filtered && (
          <div className="flex flex-wrap items-center gap-2" data-testid="admin-audit-active-filters" aria-label="Active filters">
            <span className="text-xs text-text-muted">Showing only:</span>
            {describe(applied).map((label) => (
              <Badge key={label} variant="outline" showDot={false}>
                {label}
              </Badge>
            ))}
          </div>
        )}

        <p className="sr-only" role="status">
          {loading ? 'Loading the audit log…' : meta ? `${meta.total_items} entries` : ''}
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
          <div data-testid="admin-audit-empty" className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
            {filtered ? (
              <SearchX size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
            ) : (
              <History size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
            )}
            <CardTitle>{filtered ? 'No entries match your filters' : 'No changes recorded yet'}</CardTitle>
            <CardDescription>
              {filtered ? 'Try a wider date range or fewer filters.' : 'Zone, setting and account changes appear here as they happen.'}
            </CardDescription>
            {filtered && (
              <Button type="button" variant="outline" className="mt-2" onClick={reset} data-testid="admin-audit-empty-reset-btn">
                Reset filters
              </Button>
            )}
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
                        <div>{r.target_type ? titleCase(r.target_type) : <span className="text-text-muted">Not recorded</span>}</div>
                        {r.target_id && <div className="font-mono text-xs text-text-muted">{r.target_id}</div>}
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
                          aria-label={`View details of ${humanizeAction(r.action)} on ${r.target_id || 'an entry'}`}
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

        {meta && (
          <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-text-muted">
            <span className="tabular-nums" data-testid="admin-audit-summary">
              {meta.total_items === 0
                ? filtered
                  ? 'No matching entries'
                  : 'No entries'
                : `Showing ${first}–${last} of ${meta.total_items} ${filtered ? 'matching ' : ''}${meta.total_items === 1 ? 'entry' : 'entries'} · page ${meta.page} of ${totalPages}`}
            </span>
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-2">
                <Label htmlFor="audit-per-page">Rows per page</Label>
                <Select
                  value={String(perPage)}
                  onValueChange={(v) => {
                    setPerPage(Number(v));
                    setPage(1);
                  }}
                >
                  <SelectTrigger id="audit-per-page" data-testid="admin-audit-per-page" className="w-20">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PER_PAGE_CHOICES.map((n) => (
                      <SelectItem key={n} value={n}>
                        {n}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={loading || page <= 1}
                  aria-label="Previous page"
                  data-testid="admin-audit-prev-btn"
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
                  data-testid="admin-audit-next-btn"
                >
                  <ChevronRight aria-hidden="true" />
                </Button>
              </div>
            </div>
          </div>
        )}
      </CardContent>

      {detail && (
        <AdminDialog
          title={`${humanizeAction(detail.action)}${detail.target_id ? ` · ${detail.target_id}` : ''}`}
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
