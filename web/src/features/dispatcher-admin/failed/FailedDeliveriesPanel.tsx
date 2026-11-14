import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PackageCheck, RefreshCw, SearchX } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError } from '../api';
import { Banner, FieldHint, formatWhen } from '../admin-console/ui';
import { AssignParcelModal } from '../dispatch/AssignParcelModal';
import { getBoard, waitedFor, type DispatchBoard, type WaitingParcel } from '../dispatch/dispatchApi';
import { AttemptHistoryModal } from './AttemptHistoryModal';
import { REASON_LABEL, failedAttempts, getAttemptHistory, lastFailure, reasonLabel, type AttemptHistory } from './failedApi';

// Failed deliveries: parcels whose last delivery attempt failed and that wait for a new courier.
// The list comes from the dispatch board, the reasons from each parcel's attempt history.
// "Schedule retry" sends the parcel to a courier for a new attempt. The server decides whether
// another attempt is allowed.

const ANY = 'ANY';
const HISTORY_LIMIT = 50; // histories loaded per refresh, to keep the request count reasonable
const HISTORY_PARALLEL = 5;

type HistoryState = { status: 'loading' } | { status: 'error' } | { status: 'ok'; history: AttemptHistory };

interface FailedDeliveriesPanelProps {
  token: string | null;
  onNotice: (message: string) => void;
}

export const FailedDeliveriesPanel: React.FC<FailedDeliveriesPanelProps> = ({ token, onNotice }) => {
  const [board, setBoard] = useState<DispatchBoard | null>(null);
  const [histories, setHistories] = useState<Record<string, HistoryState>>({});
  const [reasonFilter, setReasonFilter] = useState(ANY);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [retrying, setRetrying] = useState<WaitingParcel | null>(null);
  const [viewing, setViewing] = useState<WaitingParcel | null>(null);
  const run = useRef(0);
  const cache = useRef(new Map<string, AttemptHistory>());

  const loadHistories = useCallback(
    async (parcels: WaitingParcel[], mine: number) => {
      const queue = parcels.slice(0, HISTORY_LIMIT).filter((p) => !cache.current.has(p.id));
      // Parcels already known show straight away; the rest show "Loading".
      setHistories(Object.fromEntries(parcels.slice(0, HISTORY_LIMIT).map((p) => [p.id, cache.current.has(p.id) ? { status: 'ok', history: cache.current.get(p.id) as AttemptHistory } : { status: 'loading' }])));
      const next = async () => {
        for (let p = queue.shift(); p; p = queue.shift()) {
          let state: HistoryState;
          try {
            const history = await getAttemptHistory(token, p.id);
            cache.current.set(p.id, history);
            state = { status: 'ok', history };
          } catch {
            state = { status: 'error' };
          }
          if (mine !== run.current) return;
          setHistories((h) => ({ ...h, [p.id]: state }));
        }
      };
      await Promise.all(Array.from({ length: HISTORY_PARALLEL }, next));
    },
    [token],
  );

  const load = useCallback(
    async (fresh: boolean) => {
      const mine = ++run.current;
      if (fresh) cache.current.clear();
      setLoading(true);
      setError(null);
      try {
        const next = await getBoard(token);
        if (mine !== run.current) return;
        const failed = { ...next, parcels: next.parcels.filter((p) => p.status === 'ATTEMPT_FAILED') };
        setBoard(failed);
        setLoading(false);
        await loadHistories(failed.parcels, mine);
      } catch (err) {
        if (mine !== run.current) return;
        setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load failed deliveries. Try again."));
        setLoading(false);
      }
    },
    [token, loadHistories],
  );

  useEffect(() => {
    void load(true);
  }, [load]);

  const parcels = board?.parcels ?? [];
  const couriers = board?.couriers ?? [];
  const stateOf = (id: string): HistoryState | undefined => histories[id];
  const shown = useMemo(
    () =>
      reasonFilter === ANY
        ? parcels
        : parcels.filter((p) => {
            const st = histories[p.id];
            return st?.status === 'ok' && lastFailure(st.history)?.reason_code === reasonFilter;
          }),
    [parcels, histories, reasonFilter],
  );
  const filtered = reasonFilter !== ANY;
  const stale = error !== null && board !== null;
  const blocked = error !== null && board === null;
  const showEmpty = !loading && !blocked && board !== null && shown.length === 0;
  const retryState = retrying ? stateOf(retrying.id) : undefined;
  const retryFailure = retryState?.status === 'ok' ? lastFailure(retryState.history) : undefined;
  const viewState = viewing ? stateOf(viewing.id) : undefined;

  return (
    <>
      <Card role="region" aria-labelledby="failed-title">
        <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
          <div className="grid gap-1.5">
            <CardTitle id="failed-title">Parcels waiting for a retry</CardTitle>
            <CardDescription data-testid="failed-summary">
              {board ? `${parcels.length} ${parcels.length === 1 ? 'parcel' : 'parcels'} with a failed attempt, oldest first.` : 'Parcels whose last delivery attempt failed, oldest first.'}
            </CardDescription>
          </div>
          <Button type="button" variant="outline" size="icon" onClick={() => void load(true)} disabled={loading} aria-label="Refresh failed deliveries" title="Refresh failed deliveries" data-testid="failed-refresh-btn">
            <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
          </Button>
        </CardHeader>

        <CardContent className="grid gap-4">
          <div role="group" aria-label="Failed delivery filters" data-testid="failed-filters" className="grid gap-4 rounded-md border border-border bg-surface-subtle p-4 sm:grid-cols-2 sm:items-end">
            <div className="grid content-start gap-1.5">
              <Label htmlFor="failed-reason">Last failure reason</Label>
              <Select value={reasonFilter} onValueChange={setReasonFilter}>
                <SelectTrigger id="failed-reason" data-testid="failed-filter-reason">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY}>Any reason</SelectItem>
                  {Object.entries(REASON_LABEL).map(([code, label]) => (
                    <SelectItem key={code} value={code}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Button type="button" variant="outline" onClick={() => setReasonFilter(ANY)} disabled={!filtered} data-testid="failed-reset-btn">
                Reset filters
              </Button>
            </div>
          </div>

          <p className="sr-only" role="status">
            {loading ? 'Loading failed deliveries…' : board ? `${parcels.length} parcels with a failed attempt` : ''}
          </p>

          {error && (
            <Banner
              tone={stale ? 'attn' : 'err'}
              testId="failed-error"
              reference={error.correlationId}
              action={
                <div>
                  <Button type="button" variant="outline" size="sm" onClick={() => void load(false)}>
                    Retry
                  </Button>
                </div>
              }
            >
              {error.status === 403 ? 'Your account is not allowed to view failed deliveries.' : error.message || "Couldn't load failed deliveries. Try again."}
            </Banner>
          )}

          {showEmpty ? (
            <div data-testid="failed-empty" className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
              {filtered ? <SearchX size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" /> : <PackageCheck size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />}
              <CardTitle>{filtered ? 'No parcels match your filters' : 'No failed deliveries'}</CardTitle>
              <CardDescription>{filtered ? 'Try another reason. Parcels whose history is still loading are not matched yet.' : 'Every parcel is on its way or delivered. A parcel appears here after a failed attempt.'}</CardDescription>
              {filtered && (
                <Button type="button" variant="outline" className="mt-2" onClick={() => setReasonFilter(ANY)} data-testid="failed-empty-reset-btn">
                  Reset filters
                </Button>
              )}
            </div>
          ) : (
            !blocked && (
              <Table aria-label="Failed deliveries" data-testid="failed-table" className="min-w-max">
                <TableHeader>
                  <TableRow>
                    <TableHead>Parcel</TableHead>
                    <TableHead>Last failure</TableHead>
                    <TableHead className="text-right">Failed attempts</TableHead>
                    <TableHead>Waiting</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading && !board
                    ? [0, 1, 2, 3].map((i) => (
                        <TableRow key={i} aria-hidden="true">
                          <TableCell>
                            <Skeleton className="h-4 w-36" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-40" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="ml-auto h-4 w-8" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-16" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="ml-auto h-8 w-28" />
                          </TableCell>
                        </TableRow>
                      ))
                    : shown.map((p) => {
                        const st = stateOf(p.id);
                        const last = st?.status === 'ok' ? lastFailure(st.history) : undefined;
                        return (
                          <TableRow key={p.id} data-testid="failed-row">
                            <TableCell>
                              <div className="font-mono tabular-nums">{p.tracking_number}</div>
                              <div className="text-xs text-text-muted">{p.recipient_city || 'City not recorded'}</div>
                            </TableCell>
                            <TableCell>
                              {st?.status === 'ok' ? (
                                last ? (
                                  <>
                                    <Badge variant="warning">{reasonLabel(last.reason_code)}</Badge>
                                    <div className="mt-1 font-mono text-xs tabular-nums text-text-muted">{last.attempted_at ? formatWhen(last.attempted_at) : ''}</div>
                                  </>
                                ) : (
                                  <span className="text-text-muted">No failed attempt recorded</span>
                                )
                              ) : st?.status === 'error' ? (
                                <span className="text-text-muted">Details unavailable</span>
                              ) : (
                                <Skeleton className="h-4 w-32" aria-label="Loading reason" />
                              )}
                            </TableCell>
                            <TableCell className="text-right font-mono tabular-nums">{st?.status === 'ok' ? failedAttempts(st.history).length : '—'}</TableCell>
                            <TableCell className="whitespace-nowrap font-mono tabular-nums">{waitedFor(p.created_at)}</TableCell>
                            <TableCell className="text-right">
                              <div className="flex justify-end gap-2">
                                <Button type="button" size="sm" variant="outline" className="touch:min-h-[44px]" disabled={st?.status !== 'ok'} onClick={() => setViewing(p)} aria-label={`View attempts for ${p.tracking_number}`} data-testid="failed-history-btn">
                                  Attempts
                                </Button>
                                <Button type="button" size="sm" className="touch:min-h-[44px]" onClick={() => setRetrying(p)} aria-label={`Schedule retry for ${p.tracking_number}`} data-testid="failed-retry-btn">
                                  Schedule retry
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        );
                      })}
                </TableBody>
              </Table>
            )
          )}

          {board && parcels.length > HISTORY_LIMIT && <FieldHint>Reasons are loaded for the {HISTORY_LIMIT} oldest parcels. Retry those first, then refresh.</FieldHint>}
          {board && parcels.length > 0 && <FieldHint>The server decides whether a parcel may be tried again. A parcel that used all its attempts cannot be scheduled.</FieldHint>}
        </CardContent>
      </Card>

      {retrying && board && (
        <AssignParcelModal
          token={token}
          mode="retry"
          parcel={retrying}
          couriers={couriers}
          stillWaiting={parcels.some((p) => p.id === retrying.id)}
          context={
            retryFailure ? (
              <div className="rounded-md border border-border bg-surface-subtle p-3 text-sm" data-testid="failed-retry-context">
                <div className="font-medium">Last failure: {reasonLabel(retryFailure.reason_code)}</div>
                {retryFailure.notes && <div className="text-text-muted">{retryFailure.notes}</div>}
              </div>
            ) : null
          }
          onClose={() => {
            setRetrying(null);
            void load(false);
          }}
          onDone={(message) => {
            setRetrying(null);
            cache.current.delete(retrying.id);
            onNotice(message);
            void load(false);
          }}
          onStale={() => void load(false)}
        />
      )}

      {viewing && viewState?.status === 'ok' && <AttemptHistoryModal tracking={viewing.tracking_number} attempts={viewState.history.attempts} onClose={() => setViewing(null)} />}
    </>
  );
};
