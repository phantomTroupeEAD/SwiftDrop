import React, { useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { ApiError, setZoneStatus } from './zonesApi';
import type { Zone } from './types';
import { REASON_MAX, REASON_MIN, validateReason } from './zoneValidation';
import { AdminDialog, Banner, Busy, FieldError, FieldHint } from '../admin-console/ui';

interface ZoneStatusModalProps {
  zone: Zone;
  token: string | null;
  onClose: () => void;
  onSaved: (zone: Zone, message: string) => void;
  onStale: () => void;
}

/** Enable or disable one zone. A reason is required and goes into the audit log. */
export const ZoneStatusModal: React.FC<ZoneStatusModalProps> = ({ zone, token, onClose, onSaved, onStale }) => {
  const enabling = !zone.enabled;
  const verb = enabling ? 'Enable' : 'Disable';
  const [reason, setReason] = useState('');
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [banner, setBanner] = useState<string | null>(null);
  const [correlationId, setCorrelationId] = useState('');
  const [stale, setStale] = useState(false);
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBanner(null);
    setCorrelationId('');
    setStale(false);
    const problem = validateReason(reason);
    setReasonError(problem);
    if (problem) return;

    setSaving(true);
    try {
      const saved = await setZoneStatus(token, zone.id, { version: zone.version, enabled: enabling, reason: reason.trim() });
      onSaved(saved, `Zone ${saved.id} is now ${saved.enabled ? 'enabled' : 'disabled'}.`);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === 'REASON_REQUIRED') setReasonError(err.message);
        else setBanner(err.message);
        setStale(err.code === 'CONCURRENT_MODIFICATION');
        setCorrelationId(err.correlationId);
      } else {
        setBanner(`Couldn't ${verb.toLowerCase()} the zone. Try again.`);
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog
      title={`${verb} zone ${zone.name}`}
      description={
        enabling
          ? 'The zone returns to the public zone list and to postal code lookups.'
          : 'The zone disappears from the public zone list and from postal code lookups. Existing parcels are not changed.'
      }
      onClose={onClose}
      busy={saving}
      testId="admin-zone-status-dialog"
    >
      <form onSubmit={handleSubmit} noValidate>
        <div className="grid gap-4 px-5 py-5">
          <p className="font-mono text-xs tabular-nums text-text-muted">
            {zone.id} · version {zone.version}
          </p>

          {banner && (
            <Banner
              tone="err"
              reference={correlationId}
              action={
                stale ? (
                  <div>
                    <Button type="button" variant="outline" size="sm" onClick={onStale}>
                      <RefreshCw aria-hidden="true" /> Reload zones
                    </Button>
                  </div>
                ) : undefined
              }
            >
              {banner}
            </Banner>
          )}

          <div className="grid gap-1.5">
            <Label htmlFor="zone-status-reason">
              Reason ({REASON_MIN} to {REASON_MAX} characters)
            </Label>
            <Textarea
              id="zone-status-reason"
              data-testid="admin-zone-status-reason-input"
              data-autofocus
              value={reason}
              onChange={(e) => {
                setReason(e.target.value);
                setReasonError(null);
              }}
              disabled={saving}
              rows={3}
              placeholder={enabling ? 'Depot reopened after repairs' : 'Depot closed by flooding until further notice'}
              aria-invalid={Boolean(reasonError)}
              aria-describedby="zone-status-reason-note"
            />
            <div className="flex justify-between gap-3">
              {reasonError ? (
                <FieldError id="zone-status-reason-note">{reasonError}</FieldError>
              ) : (
                <FieldHint id="zone-status-reason-note">Kept in the audit log with your name and the time.</FieldHint>
              )}
              <span className="font-mono text-xs tabular-nums text-text-muted">
                {[...reason.trim()].length}/{REASON_MAX}
              </span>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant={enabling ? 'default' : 'destructive'}
            disabled={saving}
            data-testid="admin-zone-status-confirm-btn"
          >
            {saving ? <Busy label={enabling ? 'Enabling…' : 'Disabling…'} /> : `${verb} zone`}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};
