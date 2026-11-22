import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, CheckCircle2, Clock, MapPin, Package, Search, Tag } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { ApiError } from '../api';
import { Banner, Busy, FieldError, FieldHint, formatWhen } from '../admin-console/ui';
import {
  getParcelTracking,
  isValidTrackingNumber,
  type AuthenticatedTracking,
} from './trackingApi';

const STATUS_VARIANT: Record<string, 'success' | 'neutral' | 'warning' | 'error'> = {
  DELIVERED: 'success',
  OUT_FOR_DELIVERY: 'neutral',
  IN_TRANSIT: 'neutral',
  PICKED_UP: 'neutral',
  ASSIGNED: 'neutral',
  CREATED: 'neutral',
  ATTEMPT_FAILED: 'warning',
  RETURNED: 'error',
  CANCELLED: 'error',
};

function humanizeStatus(status: string): string {
  return status
    .split('_')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ');
}

interface ParcelLookupPanelProps {
  token: string | null;
}

export const ParcelLookupPanel: React.FC<ParcelLookupPanelProps> = ({ token }) => {
  const [query, setQuery] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [apiError, setApiError] = useState<ApiError | null>(null);
  const [result, setResult] = useState<AuthenticatedTracking | null>(null);
  const [searchedQuery, setSearchedQuery] = useState<string | null>(null);

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    const clean = query.trim().toUpperCase();
    setApiError(null);

    if (!clean) {
      setError('Enter a tracking number to search.');
      return;
    }

    if (!isValidTrackingNumber(clean)) {
      setError('Tracking number must match format SD-YYYYMMDD-XXXXXX (e.g. SD-20261025-A8F3K2).');
      return;
    }

    setError(null);
    setLoading(true);

    try {
      const data = await getParcelTracking(token, clean);
      setResult(data);
      setSearchedQuery(clean);
    } catch (err) {
      setResult(null);
      setSearchedQuery(clean);
      setApiError(
        err instanceof ApiError
          ? err
          : new ApiError(0, 'UNKNOWN', 'Unable to retrieve tracking details. Please try again.'),
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="grid gap-6" data-testid="parcel-tracking-lookup">
      <Card role="region" aria-labelledby="tracking-search-title">
        <CardHeader>
          <CardTitle id="tracking-search-title">Find shipment</CardTitle>
          <CardDescription>
            Look up any parcel by tracking number to inspect destination details, current lifecycle stage, and the operational milestone audit trail.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSearch} noValidate className="grid max-w-xl gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="tracking-input">Tracking number</Label>
              <div className="flex flex-wrap items-center gap-2">
                <div className="relative min-w-0 flex-1">
                  <Input
                    id="tracking-input"
                    data-testid="tracking-search-input"
                    className="font-mono uppercase tabular-nums"
                    placeholder="SD-20261025-A8F3K2"
                    value={query}
                    onChange={(e) => {
                      setQuery(e.target.value);
                      setError(null);
                    }}
                    disabled={loading}
                    autoComplete="off"
                    aria-invalid={Boolean(error)}
                    aria-describedby={error ? 'tracking-input-error' : 'tracking-input-hint'}
                  />
                </div>
                <Button
                  type="submit"
                  disabled={loading}
                  data-testid="tracking-search-btn"
                  className="touch:min-h-[44px]"
                >
                  {loading ? <Busy label="Searching…" /> : <><Search aria-hidden="true" /> Search</>}
                </Button>
              </div>
              {error ? (
                <FieldError id="tracking-input-error">{error}</FieldError>
              ) : (
                <FieldHint id="tracking-input-hint">
                  Format: SD-YYYYMMDD-XXXXXX (case-insensitive).
                </FieldHint>
              )}
            </div>
          </form>
        </CardContent>
      </Card>

      {apiError && (
        <Banner
          tone="err"
          reference={apiError.correlationId}
          testId="tracking-error-banner"
          action={
            <div className="mt-2">
              <Button type="button" variant="outline" size="sm" onClick={handleSearch}>
                Retry lookup
              </Button>
            </div>
          }
        >
          {apiError.status === 404
            ? `No parcel found with tracking number "${searchedQuery}". Check the digits and try again.`
            : apiError.status === 403
            ? 'Your account does not have operational oversight permissions to track this parcel.'
            : apiError.message || 'Could not fetch tracking records. Please try again.'}
        </Banner>
      )}

      {result && (
        <div className="grid gap-6">
          <Card role="region" aria-labelledby="tracking-result-title">
            <CardHeader className="flex-row flex-wrap items-start justify-between gap-4 space-y-0">
              <div className="grid gap-1.5">
                <div className="flex flex-wrap items-center gap-2.5">
                  <Package className="text-text-muted" size={20} aria-hidden="true" />
                  <CardTitle id="tracking-result-title" className="font-mono text-xl tabular-nums">
                    {result.tracking_number}
                  </CardTitle>
                  <Badge variant={STATUS_VARIANT[result.status] ?? 'neutral'}>
                    {humanizeStatus(result.status)}
                  </Badge>
                </div>
                <CardDescription className="text-text">
                  {result.status_description}
                </CardDescription>
                <div className="font-mono text-xs tabular-nums text-text-muted">
                  Parcel ID: {result.parcel_id}
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                {result.status === 'ATTEMPT_FAILED' && (
                  <Button asChild variant="outline" size="sm" className="touch:min-h-[44px]">
                    <Link to="/ops/failed-deliveries">
                      Go to Failed deliveries <ArrowRight aria-hidden="true" className="ml-1" />
                    </Link>
                  </Button>
                )}
                {result.status === 'CREATED' && (
                  <Button asChild variant="outline" size="sm" className="touch:min-h-[44px]">
                    <Link to="/ops/dispatch">
                      Go to Dispatch board <ArrowRight aria-hidden="true" className="ml-1" />
                    </Link>
                  </Button>
                )}
              </div>
            </CardHeader>

            <CardContent className="grid gap-4 sm:grid-cols-3">
              <div className="rounded-md border border-border bg-surface-subtle p-3.5">
                <div className="flex items-center gap-2 text-xs font-medium text-text-muted">
                  <MapPin size={14} aria-hidden="true" /> Destination City
                </div>
                <div className="mt-1 text-base font-semibold text-text">
                  {result.destination.city || 'Not specified'}
                </div>
              </div>

              <div className="rounded-md border border-border bg-surface-subtle p-3.5">
                <div className="flex items-center gap-2 text-xs font-medium text-text-muted">
                  <Tag size={14} aria-hidden="true" /> Postal Code
                </div>
                <div className="mt-1 font-mono text-base font-semibold tabular-nums text-text">
                  {result.destination.postal_code || '—'}
                </div>
              </div>

              <div className="rounded-md border border-border bg-surface-subtle p-3.5">
                <div className="flex items-center gap-2 text-xs font-medium text-text-muted">
                  <Tag size={14} aria-hidden="true" /> Routing Zone
                </div>
                <div className="mt-1 font-mono text-base font-semibold tabular-nums text-text">
                  {result.destination.zone_id || '—'}
                </div>
              </div>
            </CardContent>
          </Card>

          <Card role="region" aria-labelledby="milestones-title">
            <CardHeader>
              <CardTitle id="milestones-title">Milestone journey</CardTitle>
              <CardDescription>
                Chronological record of status transitions committed to authoritative persistence.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {result.milestones.length === 0 ? (
                <p className="text-sm text-text-muted">No milestone history available for this shipment.</p>
              ) : (
                <ol className="relative border-l border-border pl-6" aria-label="Tracking milestones">
                  {result.milestones.map((m, index) => {
                    const isLatest = index === result.milestones.length - 1;
                    return (
                      <li key={`${m.status}-${m.timestamp}-${index}`} className="mb-8 last:mb-2">
                        <span
                          className={`absolute -left-2.5 mt-1.5 flex h-5 w-5 items-center justify-center rounded-full border border-border ${
                            isLatest ? 'bg-primary text-primary-foreground' : 'bg-surface text-text-muted'
                          }`}
                          aria-hidden="true"
                        >
                          <CheckCircle2 size={12} strokeWidth={2.5} />
                        </span>

                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-semibold text-text">
                              {humanizeStatus(m.status)}
                            </span>
                            <Badge variant={STATUS_VARIANT[m.status] ?? 'neutral'}>
                              {m.status}
                            </Badge>
                            {m.actor_role && (
                              <span className="rounded bg-surface-subtle px-1.5 py-0.5 text-xs text-text-muted">
                                By {m.actor_role}
                              </span>
                            )}
                            {m.reason_code && (
                              <Badge variant="warning">
                                {m.reason_code}
                              </Badge>
                            )}
                          </div>
                          <div className="flex items-center gap-1 font-mono text-xs tabular-nums text-text-muted">
                            <Clock size={12} aria-hidden="true" />
                            {formatWhen(m.timestamp)}
                          </div>
                        </div>

                        <p className="mt-1.5 text-sm text-text-muted">
                          {m.description}
                        </p>
                      </li>
                    );
                  })}
                </ol>
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
};
