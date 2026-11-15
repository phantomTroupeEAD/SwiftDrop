import React, { useEffect, useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ApiError } from '../api';
import { AdminDialog, Banner, FieldHint, formatWhen } from '../admin-console/ui';
import { getMerchant, type Merchant } from './merchantsApi';
import { MerchantStatusBadge } from './MerchantStatusBadge';

interface MerchantDetailModalProps {
  token: string | null;
  merchantId: string;
  /** The row from the list, shown straight away while the latest copy loads. */
  preview?: Merchant;
  onClose: () => void;
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-0.5 sm:grid-cols-3 sm:gap-3">
      <dt className="text-xs text-text-muted sm:pt-0.5">{label}</dt>
      <dd className="sm:col-span-2">{children}</dd>
    </div>
  );
}

/** One merchant, read only. It loads the latest copy so the screen never shows a stale list row. */
export const MerchantDetailModal: React.FC<MerchantDetailModalProps> = ({ token, merchantId, preview, onClose }) => {
  const [merchant, setMerchant] = useState<Merchant | null>(preview ?? null);
  const [error, setError] = useState<ApiError | null>(null);
  const [tries, setTries] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    getMerchant(token, merchantId)
      .then((m) => !cancelled && setMerchant(m))
      .catch((err) => !cancelled && setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load this merchant. Try again.")));
    return () => {
      cancelled = true;
    };
  }, [token, merchantId, tries]);

  const a = merchant?.address;
  const addressLines = a ? [a.line1, a.line2, [a.postal_code, a.city].filter(Boolean).join(' '), a.country].filter(Boolean) : [];

  return (
    <AdminDialog title={merchant ? merchant.business_name : 'Merchant'} description="Application details. This window is read only." onClose={onClose} testId="merchants-detail-dialog">
      <div className="grid max-h-96 gap-4 overflow-y-auto px-5 py-5">
        {error && (
          <Banner
            tone="err"
            testId="merchants-detail-error"
            reference={error.correlationId}
            action={
              error.code === 'MERCHANT_NOT_FOUND' || error.status === 404 ? undefined : (
                <div>
                  <Button type="button" variant="outline" size="sm" onClick={() => setTries((n) => n + 1)}>
                    Retry
                  </Button>
                </div>
              )
            }
          >
            {error.status === 404 || error.code === 'MERCHANT_NOT_FOUND' ? 'No merchant has this id.' : error.message || "Couldn't load this merchant. Try again."}
          </Banner>
        )}
        {merchant ? (
          <dl className="grid gap-3" data-testid="merchants-detail">
            <Row label="Status">
              <MerchantStatusBadge status={merchant.status} />
            </Row>
            <Row label="Contact email">{merchant.contact_email || <span className="text-text-muted">Not provided</span>}</Row>
            <Row label="Contact phone">
              <span className="font-mono tabular-nums">{merchant.contact_phone || 'Not provided'}</span>
            </Row>
            <Row label="Address">
              {addressLines.length > 0 ? addressLines.map((l) => <div key={l}>{l}</div>) : <span className="text-text-muted">Not provided</span>}
            </Row>
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
            <Row label="Version">
              <span className="font-mono tabular-nums">{merchant.version}</span>
            </Row>
          </dl>
        ) : (
          !error && (
            <div className="grid gap-3" aria-hidden="true">
              <Skeleton className="h-5 w-40" />
              <Skeleton className="h-5 w-full" />
              <Skeleton className="h-5 w-3/4" />
            </div>
          )
        )}
        <FieldHint>Approving, rejecting, suspending and reinstating are not available on this screen yet.</FieldHint>
      </div>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onClose}>
          Close
        </Button>
      </DialogFooter>
    </AdminDialog>
  );
};
