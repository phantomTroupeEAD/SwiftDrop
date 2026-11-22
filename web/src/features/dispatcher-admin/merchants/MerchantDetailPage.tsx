import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, RefreshCw } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ApiError } from '../api';
import { Banner, FieldHint, formatWhen } from '../admin-console/ui';
import { MerchantActionDialog } from './MerchantActionDialog';
import { MerchantStatusBadge } from './MerchantStatusBadge';
import { actionsFor, noActionsText, type ActionInfo } from './merchantActions';
import { getMerchant, type Merchant } from './merchantsApi';

interface MerchantDetailPageProps {
  token: string | null;
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-0.5 sm:grid-cols-3 sm:gap-3">
      <dt className="text-xs text-text-muted sm:pt-0.5">{label}</dt>
      <dd className="sm:col-span-2">{children}</dd>
    </div>
  );
}

/** One merchant for an admin: its details, and the status changes the merchant's status allows. */
export const MerchantDetailPage: React.FC<MerchantDetailPageProps & { merchantId: string }> = ({ token, merchantId }) => {
  const [merchant, setMerchant] = useState<Merchant | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [acting, setActing] = useState<ActionInfo | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const statusHeading = useRef<HTMLHeadingElement>(null);
  const timer = useRef<number | undefined>(undefined);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setMerchant(await getMerchant(token, merchantId));
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load this merchant. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, merchantId]);

  useEffect(() => {
    setMerchant(null);
    setNotice(null);
    void load();
  }, [load]);

  useEffect(() => () => window.clearTimeout(timer.current), []);

  const a = merchant?.address;
  const addressLines = a ? [a.line1, a.line2, [a.postal_code, a.city].filter(Boolean).join(' '), a.country].filter(Boolean) : [];
  const actions = merchant ? actionsFor(merchant.status) : [];
  const notFound = error?.status === 404 || error?.code === 'MERCHANT_NOT_FOUND';

  const finished = (updated: Merchant, message: string) => {
    setActing(null);
    setMerchant(updated);
    setNotice(message);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setNotice(null), 8000);
    // The button that opened the dialog may be gone now, so move focus to the status heading.
    window.setTimeout(() => statusHeading.current?.focus(), 0);
  };

  return (
    <>
      <div className="grid gap-3">
        <div>
          <Button asChild variant="ghost" size="sm" className="-ml-3 touch:min-h-[44px]">
            <Link to="/ops/merchants" data-testid="merchants-back-link">
              <ArrowLeft aria-hidden="true" /> All merchants
            </Link>
          </Button>
        </div>
        <div className="grid gap-1.5">
          <h1 className="text-3xl font-bold tracking-tight" data-testid="merchants-detail-title">
            {merchant ? merchant.business_name : 'Merchant'}
          </h1>
          <p className="text-sm text-text-muted">Review the application and change the merchant&apos;s status.</p>
        </div>
      </div>

      {notice && (
        <Banner tone="ok" testId="merchants-notice">
          {notice}
        </Banner>
      )}

      <p className="sr-only" role="status">
        {loading ? 'Loading the merchant…' : merchant ? `${merchant.business_name}, ${merchant.status}` : ''}
      </p>

      {error && (
        <Banner
          tone="err"
          testId="merchants-detail-error"
          reference={error.correlationId}
          action={
            notFound ? (
              <div>
                <Button asChild variant="outline" size="sm">
                  <Link to="/ops/merchants">Back to the list</Link>
                </Button>
              </div>
            ) : (
              <div>
                <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                  Retry
                </Button>
              </div>
            )
          }
        >
          {notFound ? 'No merchant has this id.' : error.status === 403 ? 'Your account is not allowed to view merchants.' : error.message || "Couldn't load this merchant. Try again."}
        </Banner>
      )}

      {loading && !merchant && !error && (
        <div className="grid gap-5 lg:grid-cols-3" aria-hidden="true">
          <Skeleton className="h-72 lg:col-span-2" />
          <Skeleton className="h-48" />
        </div>
      )}

      {merchant && (
        <div className="grid gap-5 lg:grid-cols-3">
          <Card className="min-w-0 lg:col-span-2" role="region" aria-labelledby="merchants-details-title">
            <CardHeader className="flex-row items-start justify-between gap-3 space-y-0">
              <div className="grid gap-1.5">
                <CardTitle id="merchants-details-title">Details</CardTitle>
                <CardDescription>What the merchant gave when it applied.</CardDescription>
              </div>
              <Button type="button" variant="outline" size="icon" onClick={() => void load()} disabled={loading} aria-label="Refresh merchant" title="Refresh merchant" data-testid="merchants-detail-refresh-btn">
                <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
              </Button>
            </CardHeader>
            <CardContent>
              <dl className="grid gap-3" data-testid="merchants-detail">
                <Row label="Contact email">{merchant.contact_email || <span className="text-text-muted">Not provided</span>}</Row>
                <Row label="Contact phone">
                  <span className="font-mono tabular-nums">{merchant.contact_phone || 'Not provided'}</span>
                </Row>
                <Row label="Address">{addressLines.length > 0 ? addressLines.map((l) => <div key={l}>{l}</div>) : <span className="text-text-muted">Not provided</span>}</Row>
                <Row label="Applied">
                  <span className="font-mono tabular-nums">{formatWhen(merchant.created_at)}</span>
                </Row>
                <Row label="Approved">{merchant.approved_at ? <span className="font-mono tabular-nums">{formatWhen(merchant.approved_at)}</span> : <span className="text-text-muted">Not approved</span>}</Row>
                <Row label="Last changed">
                  <span className="font-mono tabular-nums">{formatWhen(merchant.updated_at)}</span>
                </Row>
                <Row label="Merchant id">
                  <span className="break-all font-mono text-xs">{merchant.merchant_id}</span>
                </Row>
              </dl>
            </CardContent>
          </Card>

          <Card className="min-w-0" role="region" aria-labelledby="merchants-status-title">
            <CardHeader className="space-y-0">
              <CardTitle id="merchants-status-title" ref={statusHeading} tabIndex={-1} className="outline-none">
                Status
              </CardTitle>
              <CardDescription>
                <span className="font-mono tabular-nums" data-testid="merchants-version">
                  Version {merchant.version}
                </span>
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-4">
              <div data-testid="merchants-status">
                <MerchantStatusBadge status={merchant.status} />
              </div>
              {actions.length > 0 ? (
                <div className="grid gap-2" role="group" aria-label="Status changes">
                  {actions.map((info) => (
                    <Button
                      key={info.action}
                      type="button"
                      variant={info.action === 'reject' || info.action === 'disable' ? 'destructive' : info.destructive ? 'outline' : 'default'}
                      className="w-full"
                      onClick={() => setActing(info)}
                      data-testid={`merchants-action-${info.action}-btn`}
                    >
                      {info.label}
                    </Button>
                  ))}
                  <FieldHint>You confirm each change in the next window. Changes are saved in the audit log.</FieldHint>
                </div>
              ) : (
                <p className="text-sm text-text-muted" data-testid="merchants-no-actions">
                  {noActionsText(merchant.status)}
                </p>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {acting && merchant && (
        <MerchantActionDialog
          token={token}
          merchant={merchant}
          info={acting}
          onClose={() => setActing(null)}
          onDone={finished}
          onStale={(latest) => setMerchant(latest)}
        />
      )}
    </>
  );
};
