import React, { useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { ApiError } from '../api';
import { AdminDialog, Banner, Busy, FieldError, FieldHint } from '../admin-console/ui';
import { REASON_MAX, REASON_MIN, actionsFor, reasonLength, validateReason, type ActionInfo } from './merchantActions';
import { MerchantStatusBadge, statusLabel } from './MerchantStatusBadge';
import { changeMerchantStatus, getMerchant, type Merchant } from './merchantsApi';

interface MerchantActionDialogProps {
  token: string | null;
  /** The merchant as the page last loaded it. Its version is sent as expected_version. */
  merchant: Merchant;
  info: ActionInfo;
  onClose: () => void;
  onDone: (updated: Merchant, message: string) => void;
  /** The server said the merchant changed meanwhile. This is the latest copy. */
  onStale: (latest: Merchant) => void;
}

/** Confirms one status change. A reason (5 to 500 characters) is asked for only where the server requires it. */
export const MerchantActionDialog: React.FC<MerchantActionDialogProps> = ({ token, merchant, info, onClose, onDone, onStale }) => {
  const [reason, setReason] = useState('');
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [banner, setBanner] = useState<ApiError | null>(null);
  const [conflict, setConflict] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // The merchant prop is refreshed after a conflict, so this follows the latest status.
  const stillAllowed = actionsFor(merchant.status).some((a) => a.action === info.action);
  const count = reasonLength(reason);

  const submit = async (ev: React.FormEvent) => {
    ev.preventDefault();
    setBanner(null);
    setConflict(null);
    if (info.reasonRequired) {
      const problem = validateReason(reason);
      setReasonError(problem);
      if (problem) return;
    }
    setSaving(true);
    try {
      const updated = await changeMerchantStatus(token, merchant.merchant_id, info.action, {
        expected_version: merchant.version,
        reason: info.reasonRequired ? reason.trim() : '',
      });
      onDone(updated, info.done(updated.business_name));
    } catch (err) {
      const api = err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't save the change. Try again.");
      if (api.code === 'MERCHANT_VERSION_CONFLICT' || api.code === 'MERCHANT_INVALID_TRANSITION') {
        // Someone else changed this merchant. Show them the latest state and keep what was typed.
        try {
          const latest = await getMerchant(token, merchant.merchant_id);
          onStale(latest);
          const can = actionsFor(latest.status).some((a) => a.action === info.action);
          setConflict(
            can
              ? `Someone else changed this merchant. It is now ${statusLabel(latest.status)} (version ${latest.version}). Check it, then try again.`
              : `Someone else changed this merchant. It is now ${statusLabel(latest.status)}, so "${info.label}" is no longer possible.`,
          );
        } catch {
          setBanner(api);
        }
      } else if (api.code === 'REASON_REQUIRED') {
        setReasonError(`Use ${REASON_MIN} to ${REASON_MAX} characters.`);
      } else {
        setBanner(api);
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog title={info.title} description={`${merchant.business_name}`} onClose={onClose} busy={saving} testId="merchants-action-dialog">
      <form onSubmit={submit} noValidate>
        <div className="grid max-h-96 gap-4 overflow-y-auto px-5 py-5">
          {conflict && (
            <Banner tone="attn" testId="merchants-action-conflict">
              {conflict}
            </Banner>
          )}
          {banner && (
            <Banner tone="err" reference={banner.correlationId} testId="merchants-action-error">
              {banner.status === 403 ? 'Your account is not allowed to change merchants.' : banner.status === 404 ? 'No merchant has this id.' : banner.message}
            </Banner>
          )}

          <div className="grid gap-1.5 rounded-md border border-border bg-surface-subtle p-3 text-sm">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-text-muted">Now</span>
              <MerchantStatusBadge status={merchant.status} />
              <span className="font-mono text-xs tabular-nums text-text-muted">version {merchant.version}</span>
            </div>
            <p data-testid="merchants-action-consequence">{info.consequence}</p>
          </div>

          {info.reasonRequired ? (
            <div className="grid content-start gap-1.5">
              <Label htmlFor="merchants-reason">Reason</Label>
              <Textarea
                id="merchants-reason"
                data-testid="merchants-action-reason"
                value={reason}
                rows={4}
                onChange={(e) => {
                  setReason(e.target.value);
                  setReasonError(null);
                }}
                aria-invalid={Boolean(reasonError)}
                aria-describedby={reasonError ? 'merchants-reason-error' : 'merchants-reason-hint'}
                placeholder="For example: documents could not be verified"
              />
              <div className="flex flex-wrap items-start justify-between gap-2">
                {reasonError ? (
                  <FieldError id="merchants-reason-error">{reasonError}</FieldError>
                ) : (
                  <FieldHint id="merchants-reason-hint">
                    {REASON_MIN} to {REASON_MAX} characters. It is saved in the audit log.
                  </FieldHint>
                )}
                <span className="font-mono text-xs tabular-nums text-text-muted" data-testid="merchants-reason-count">
                  {count} / {REASON_MAX}
                </span>
              </div>
            </div>
          ) : (
            <FieldHint>No reason is needed for this change. It is saved in the audit log.</FieldHint>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" variant={info.destructive ? 'destructive' : 'default'} disabled={saving || !stillAllowed} data-testid="merchants-action-confirm-btn">
            {saving ? <Busy label={info.busy} /> : info.label}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};
