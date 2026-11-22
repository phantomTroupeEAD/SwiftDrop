import React, { useRef, useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Textarea } from '@/shared/components/ui/textarea';
import { ApiError } from '../api';
import { AdminDialog, Banner, Busy, FieldError, FieldHint } from '../admin-console/ui';
import type { CourierLoad } from '../fleet/fleetApi';
import { assignParcel, rescheduleParcel, type WaitingParcel } from './dispatchApi';

// Assign a waiting parcel (CREATED) or reassign one whose attempt failed (ATTEMPT_FAILED).
// The server decides who is allowed: the list below only explains why a courier is greyed out.

const NOTES_MAX = 500;

export const SHIFT_LABEL: Record<string, string> = {
  ON_SHIFT: 'On shift',
  OFF_SHIFT: 'Off shift',
  ON_BREAK: 'On break',
  SUSPENDED: 'Suspended',
};

/** Why a courier cannot take a parcel right now, or null when they can. Display aid only. */
export function whyNot(c: CourierLoad): string | null {
  if (c.status !== 'ON_SHIFT') return SHIFT_LABEL[c.status] ?? c.status;
  if (c.active_count !== null && c.max_capacity > 0 && c.active_count >= c.max_capacity) return 'Full';
  return null;
}

const ERROR_TEXT: Record<string, string> = {
  COURIER_CAPACITY_EXCEEDED: 'That courier is already at capacity. Choose another courier.',
  COURIER_NOT_ON_SHIFT: 'That courier is not on shift. Choose a courier who is on shift.',
  COURIER_SUSPENDED: 'That courier is suspended and cannot take parcels.',
  COURIER_NOT_FOUND: 'That courier no longer has a fleet profile.',
  PARCEL_ALREADY_ASSIGNED: 'Someone else already assigned this parcel.',
  INVALID_PARCEL_STATE: 'This parcel can no longer be assigned because its status changed.',
  MAX_ATTEMPTS_EXCEEDED: 'This parcel has used all its delivery attempts and cannot be scheduled again.',
  PARCEL_NOT_ASSIGNED: 'This parcel has no assignment to change.',
};
// After these, the board is out of date: refresh it so the parcel and courier lists are current.
const STALE_CODES = new Set(['PARCEL_ALREADY_ASSIGNED', 'INVALID_PARCEL_STATE', 'COURIER_CAPACITY_EXCEEDED', 'COURIER_NOT_ON_SHIFT', 'COURIER_SUSPENDED', 'COURIER_NOT_FOUND', 'MAX_ATTEMPTS_EXCEEDED']);

function courierLabel(c: CourierLoad): string {
  const name = c.courier_name || c.employee_code;
  const load = c.active_count === null ? `capacity ${c.max_capacity}` : `${c.active_count} / ${c.max_capacity} parcels`;
  const reason = whyNot(c);
  return `${name} · ${load}${reason ? ` · ${reason}` : ''}`;
}

interface AssignParcelModalProps {
  token: string | null;
  parcel: WaitingParcel;
  /** The latest couriers on the board. */
  couriers: CourierLoad[];
  /** False once a refresh shows the parcel is no longer waiting. */
  stillWaiting: boolean;
  onClose: () => void;
  onDone: (message: string) => void;
  /** The server said the board is out of date. */
  onStale: () => void;
  /** "retry" is the Failed deliveries wording for rescheduling a failed parcel. */
  mode?: 'assign' | 'retry';
  /** Extra facts shown above the courier field, for example why the last attempt failed. */
  context?: React.ReactNode;
}

export const AssignParcelModal: React.FC<AssignParcelModalProps> = ({ token, parcel, couriers, stillWaiting, onClose, onDone, onStale, mode = 'assign', context }) => {
  const retry = mode === 'retry';
  const reassign = parcel.status === 'ATTEMPT_FAILED';
  const verb = retry ? 'Schedule retry' : reassign ? 'Reassign' : 'Assign';
  const [courierId, setCourierId] = useState('');
  const [notes, setNotes] = useState('');
  const [courierError, setCourierError] = useState<string | null>(null);
  const [notesError, setNotesError] = useState<string | null>(null);
  const [banner, setBanner] = useState<ApiError | null>(null);
  const [saving, setSaving] = useState(false);
  const key = useRef(crypto.randomUUID());

  const chosen = couriers.find((c) => c.user_id === courierId);
  const eligibleCount = couriers.filter((c) => whyNot(c) === null).length;
  const chosenProblem = chosen ? whyNot(chosen) : null;

  const submit = async (ev: React.FormEvent) => {
    ev.preventDefault();
    setBanner(null);
    let bad: string | null = null;
    if (!courierId) bad = 'Choose a courier.';
    else if (!chosen) bad = 'That courier is no longer on the board. Choose another.';
    else if (chosenProblem) bad = `${chosen.courier_name || chosen.employee_code} is ${chosenProblem.toLowerCase()}. Choose another courier.`;
    setCourierError(bad);
    const tooLong = notes.length > NOTES_MAX ? `Use at most ${NOTES_MAX} characters.` : null;
    setNotesError(tooLong);
    if (bad || tooLong) return;

    setSaving(true);
    try {
      const body = { courier_id: courierId, notes: notes.trim() };
      if (reassign) await rescheduleParcel(token, parcel.id, body);
      else await assignParcel(token, { parcel_id: parcel.id, ...body }, key.current);
      const who = chosen?.courier_name || chosen?.employee_code;
      onDone(retry ? `Retry scheduled for parcel ${parcel.tracking_number} with ${who}.` : `Parcel ${parcel.tracking_number} ${reassign ? 'reassigned' : 'assigned'} to ${who}.`);
    } catch (err) {
      const api = err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', `Couldn't ${retry ? 'schedule the retry' : `${verb.toLowerCase()} the parcel`}. Try again.`);
      // A refused request is a new decision; only a network failure or a 5xx may reuse the key.
      if (api.status !== 0 && api.status < 500) key.current = crypto.randomUUID();
      if (STALE_CODES.has(api.code)) onStale();
      const friendly = ERROR_TEXT[api.code];
      if (friendly && /COURIER/.test(api.code)) setCourierError(friendly);
      else setBanner(friendly ? new ApiError(api.status, api.code, friendly, api.correlationId) : api);
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog
      title={retry ? `Schedule retry for ${parcel.tracking_number}` : `${verb} parcel ${parcel.tracking_number}`}
      description={
        reassign
          ? 'A delivery attempt failed. Choose the courier for the new attempt.'
          : `Waiting${parcel.recipient_city ? ` for delivery to ${parcel.recipient_city}` : ''}. Choose the courier who will deliver it.`
      }
      onClose={onClose}
      busy={saving}
      testId="dispatch-assign-dialog"
    >
      <form onSubmit={submit} noValidate>
        <div className="grid max-h-96 gap-4 overflow-y-auto px-5 py-5">
          {!stillWaiting && (
            <Banner tone="attn" testId="dispatch-assign-gone">
              This parcel is no longer waiting. Close this window to see the updated board.
            </Banner>
          )}
          {banner && (
            <Banner tone="err" reference={banner.correlationId} testId="dispatch-assign-error">
              {banner.status === 403 ? 'Your account is not allowed to assign parcels.' : banner.message}
            </Banner>
          )}
          {eligibleCount === 0 && (
            <Banner tone="info" testId="dispatch-assign-no-couriers">
              No courier can take a parcel right now. A courier must be on shift and below capacity. Couriers start their own shift in the courier app.
            </Banner>
          )}

          {context}

          <div className="grid content-start gap-1.5">
            <Label htmlFor="dispatch-courier">Courier</Label>
            <Select
              value={courierId}
              onValueChange={(v) => {
                setCourierId(v);
                setCourierError(null);
              }}
            >
              <SelectTrigger id="dispatch-courier" data-testid="dispatch-assign-courier" aria-invalid={Boolean(courierError)} aria-describedby={courierError ? 'dispatch-courier-error' : undefined}>
                <SelectValue placeholder={couriers.length === 0 ? 'No couriers on the board' : 'Choose a courier'} />
              </SelectTrigger>
              <SelectContent>
                {couriers.map((c) => (
                  <SelectItem key={c.user_id} value={c.user_id} disabled={whyNot(c) !== null}>
                    {courierLabel(c)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {courierError && <FieldError id="dispatch-courier-error">{courierError}</FieldError>}
          </div>

          <div className="grid content-start gap-1.5">
            <Label htmlFor="dispatch-notes">Notes (optional)</Label>
            <Textarea
              id="dispatch-notes"
              data-testid="dispatch-assign-notes"
              value={notes}
              onChange={(e) => {
                setNotes(e.target.value);
                setNotesError(null);
              }}
              rows={3}
              placeholder="For example: fragile, call before arriving"
              aria-invalid={Boolean(notesError)}
              aria-describedby={notesError ? 'dispatch-notes-error' : 'dispatch-notes-hint'}
            />
            {notesError ? <FieldError id="dispatch-notes-error">{notesError}</FieldError> : <FieldHint id="dispatch-notes-hint">The courier sees this note. Up to {NOTES_MAX} characters.</FieldHint>}
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" disabled={saving || !stillWaiting || eligibleCount === 0} data-testid="dispatch-assign-save-btn">
            {saving ? <Busy label={retry ? 'Scheduling...' : reassign ? 'Reassigning...' : 'Assigning...'} /> : retry ? 'Schedule retry' : `${verb} parcel`}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};
