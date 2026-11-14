import React from 'react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { AdminDialog, formatWhen } from '../admin-console/ui';
import { reasonLabel, type AttemptRecord } from './failedApi';

interface AttemptHistoryModalProps {
  tracking: string;
  attempts: AttemptRecord[];
  onClose: () => void;
}

/** Every delivery attempt of one parcel, oldest first. Read only. */
export const AttemptHistoryModal: React.FC<AttemptHistoryModalProps> = ({ tracking, attempts, onClose }) => (
  <AdminDialog title={`Attempts for ${tracking}`} description="Each delivery attempt, oldest first, with the reason the courier gave." onClose={onClose} testId="failed-history-dialog">
    <div className="grid max-h-96 gap-3 overflow-y-auto px-5 py-5">
      {attempts.length === 0 ? (
        <p className="text-sm text-text-muted">No attempts are recorded for this parcel yet.</p>
      ) : (
        <ol className="grid gap-3" aria-label="Delivery attempts">
          {attempts.map((a) => (
            <li key={a.id || a.attempt_number} data-testid="failed-history-item" className="grid gap-1 rounded-md border border-border p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium">Attempt {a.attempt_number}</span>
                <Badge variant={a.outcome === 'FAILED' ? 'warning' : a.outcome === 'SUCCESS' ? 'success' : 'neutral'}>{a.outcome === 'FAILED' ? 'Failed' : a.outcome === 'SUCCESS' ? 'Delivered' : a.outcome || 'Unknown'}</Badge>
              </div>
              {a.outcome === 'FAILED' && <div>{reasonLabel(a.reason_code)}</div>}
              {a.notes && <div className="text-text-muted">{a.notes}</div>}
              <div className="font-mono text-xs tabular-nums text-text-muted">{a.attempted_at ? formatWhen(a.attempted_at) : ''}</div>
            </li>
          ))}
        </ol>
      )}
    </div>
    <DialogFooter>
      <Button type="button" variant="outline" onClick={onClose}>
        Close
      </Button>
    </DialogFooter>
  </AdminDialog>
);
