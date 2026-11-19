import React, { useCallback, useEffect, useState } from 'react';
import { BarChart3, RefreshCw } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ApiError } from '../api';
import { Banner, Busy, FieldError, FieldHint } from '../admin-console/ui';
import { colomboDate, getDeliveryReport, type DeliveryReport } from './reportsApi';

// Delivery report for ADMIN and DISPATCHER. Read only. The numbers come from the server as they
// are; the screen only formats them. The server limits the period (92 days by default) and says
// so in its 422 message, which is shown beside the date fields.

const DEFAULT_DAYS = 7;
// The server's 422 only says "Some fields are not valid", so the screen explains the likely cause.
const REFUSED_PERIOD = 'The server refused this period. Choose real dates, and keep the period within 92 days unless an admin raised the limit.';
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

interface Range {
  from: string;
  to: string;
}

function defaultRange(): Range {
  return { from: colomboDate(DEFAULT_DAYS - 1), to: colomboDate(0) };
}

function checkRange(r: Range): string | null {
  if (!DATE_PATTERN.test(r.from) || !DATE_PATTERN.test(r.to)) return 'Choose both a start and an end date.';
  if (r.from > r.to) return 'The end date must not be before the start date.';
  return null;
}

function percent(rate: number | null): string {
  return rate === null ? 'No attempts' : `${(rate * 100).toFixed(1)}%`;
}

function prettyDay(iso: string): string {
  const d = new Date(`${iso}T00:00:00Z`);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' });
}

function Stat({ label, value, testId }: { label: string; value: string; testId: string }) {
  return (
    <div className="grid gap-1 rounded-md border border-border bg-surface-subtle p-4">
      <span className="text-xs text-text-muted">{label}</span>
      <span className="font-mono text-2xl font-bold tabular-nums" data-testid={testId}>
        {value}
      </span>
    </div>
  );
}

interface ReportsPanelProps {
  token: string | null;
}

export const ReportsPanel: React.FC<ReportsPanelProps> = ({ token }) => {
  const [draft, setDraft] = useState<Range>(defaultRange);
  const [applied, setApplied] = useState<Range>(draft);
  const [rangeError, setRangeError] = useState<string | null>(null);
  const [report, setReport] = useState<DeliveryReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setReport(await getDeliveryReport(token, applied.from, applied.to));
    } catch (err) {
      setReport(null);
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load the report. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, applied]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = (next: Range) => {
    const problem = checkRange(next);
    setRangeError(problem);
    if (problem) return;
    setDraft(next);
    // A new object each time, so Show report reloads even when the dates did not change.
    setApplied({ ...next });
  };

  const preset = (days: number) => run({ from: colomboDate(days - 1), to: colomboDate(0) });

  // A refused period (422) belongs beside the dates, not in a general error banner.
  const refused = error?.status === 422 ? error : null;
  const failed = error && !refused ? error : null;
  const isStandIn = report?.source.toLowerCase().startsWith('stand-in') ?? false;

  return (
    <Card role="region" aria-labelledby="reports-title">
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
        <div className="grid gap-1.5">
          <CardTitle id="reports-title">Deliveries per day</CardTitle>
          <CardDescription>Delivered, failed and returned parcels for a period. Days are Sri Lanka days and both ends are included.</CardDescription>
        </div>
        <Button
          type="button"
          variant="outline"
          size="icon"
          onClick={() => void load()}
          disabled={loading}
          aria-label="Refresh report"
          title="Refresh report"
          data-testid="reports-refresh-btn"
        >
          <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
        </Button>
      </CardHeader>

      <CardContent className="grid gap-4">
        <form
          role="search"
          aria-label="Report period"
          data-testid="reports-range-form"
          noValidate
          onSubmit={(e) => {
            e.preventDefault();
            run(draft);
          }}
          className="grid gap-4 rounded-md border border-border bg-surface-subtle p-4"
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid content-start gap-1.5">
              <Label htmlFor="reports-from">From (Sri Lanka date)</Label>
              <Input
                id="reports-from"
                data-testid="reports-from"
                type="date"
                className="font-mono tabular-nums"
                value={draft.from}
                onChange={(e) => {
                  setDraft((d) => ({ ...d, from: e.target.value }));
                  setRangeError(null);
                }}
              />
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="reports-to">To (Sri Lanka date)</Label>
              <Input
                id="reports-to"
                data-testid="reports-to"
                type="date"
                className="font-mono tabular-nums"
                value={draft.to}
                onChange={(e) => {
                  setDraft((d) => ({ ...d, to: e.target.value }));
                  setRangeError(null);
                }}
                aria-invalid={Boolean(rangeError || refused)}
                aria-describedby={rangeError || refused ? 'reports-range-error' : 'reports-range-hint'}
              />
              {rangeError || refused ? (
                <FieldError id="reports-range-error">{rangeError ?? REFUSED_PERIOD}</FieldError>
              ) : (
                <FieldHint id="reports-range-hint">A period is limited to 92 days unless an admin changes the setting.</FieldHint>
              )}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button type="submit" disabled={loading} data-testid="reports-apply-btn">
              {loading ? <Busy label="Loading..." /> : 'Show report'}
            </Button>
            <Button type="button" variant="outline" onClick={() => preset(7)} disabled={loading} data-testid="reports-last-7-btn">
              Last 7 days
            </Button>
            <Button type="button" variant="outline" onClick={() => preset(30)} disabled={loading} data-testid="reports-last-30-btn">
              Last 30 days
            </Button>
          </div>
        </form>

        <p className="sr-only" role="status">
          {loading ? 'Loading the report…' : report ? `Report for ${report.from} to ${report.to} loaded` : ''}
        </p>

        {failed && (
          <Banner
            tone="err"
            testId="reports-error"
            reference={failed.correlationId}
            action={
              <div>
                <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                  Retry
                </Button>
              </div>
            }
          >
            {failed.status === 403 ? 'Your account is not allowed to view reports.' : failed.message || "Couldn't load the report. Try again."}
          </Banner>
        )}

        {isStandIn && report && (
          <Banner tone="attn" testId="reports-source-note">
            These numbers are not live yet. The server says: {report.source}.
          </Banner>
        )}

        {loading && !report ? (
          <div className="grid gap-4" aria-hidden="true">
            <div className="grid gap-4 sm:grid-cols-3">
              <Skeleton className="h-20" />
              <Skeleton className="h-20" />
              <Skeleton className="h-20" />
            </div>
            <Skeleton className="h-40" />
          </div>
        ) : (
          report && (
            <>
              <div className="grid gap-4 sm:grid-cols-3" data-testid="reports-totals">
                <Stat label="Delivered" value={String(report.delivered)} testId="reports-total-delivered" />
                <Stat label="Failed" value={String(report.failed)} testId="reports-total-failed" />
                <Stat label="Success rate" value={percent(report.success_rate)} testId="reports-success-rate" />
              </div>

              {report.days.length === 0 ? (
                <div
                  data-testid="reports-empty"
                  className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center"
                >
                  <BarChart3 size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
                  <CardTitle>No deliveries in this period</CardTitle>
                  <CardDescription>Nothing was delivered, failed or returned between {report.from} and {report.to}. Try a wider period.</CardDescription>
                </div>
              ) : (
                <Table aria-label="Deliveries per day" data-testid="reports-table" className="min-w-max">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Day</TableHead>
                      <TableHead className="text-right">Delivered</TableHead>
                      <TableHead className="text-right">Failed</TableHead>
                      <TableHead className="text-right">Returned</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {report.days.map((d) => (
                      <TableRow key={d.date}>
                        <TableCell className="whitespace-nowrap font-mono tabular-nums">{prettyDay(d.date)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{d.delivered}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{d.failed}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{d.returned}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}

              <p className="text-xs text-text-muted" data-testid="reports-period">
                Period {report.from} to {report.to}. Source: {report.source}
              </p>
            </>
          )
        )}
      </CardContent>
    </Card>
  );
};
