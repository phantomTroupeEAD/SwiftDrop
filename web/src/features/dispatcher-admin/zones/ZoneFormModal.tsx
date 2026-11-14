import React, { useRef, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { ApiError, createZone, updateZone } from './zonesApi';
import type { Zone, ZoneUpdateInput } from './types';
import {
  DESCRIPTION_MAX,
  NAME_MAX,
  REASON_MAX,
  parsePrefixes,
  samePrefixSet,
  validateZoneForm,
  type ZoneFormErrors,
  type ZoneFormField,
  type ZoneFormValues,
} from './zoneValidation';
import { AdminDialog, Banner, Busy, FieldError, FieldHint } from '../admin-console/ui';

interface ZoneFormModalProps {
  mode: 'create' | 'edit';
  zone?: Zone;
  token: string | null;
  onClose: () => void;
  onSaved: (zone: Zone, message: string) => void;
  /** The zone changed under us (409 CONCURRENT_MODIFICATION): the panel reloads the list. */
  onStale: () => void;
}

const API_FIELD_TO_FORM: Record<string, ZoneFormField> = {
  id: 'id',
  code: 'code',
  name: 'name',
  description: 'description',
  postal_prefixes: 'prefixes',
};

/** Turns a refused request into inline field errors and an optional banner. */
function explain(err: ApiError, mode: 'create' | 'edit'): { fields: ZoneFormErrors; banner: string | null; stale: boolean } {
  const fields: ZoneFormErrors = {};
  if (err.code === 'ZONE_POSTAL_CODE_CONFLICT') {
    const named = err.details.map((d) => d.issue).filter(Boolean).join(' ');
    fields.prefixes = named ? `${err.message} ${named}` : err.message;
    return { fields, banner: null, stale: false };
  }
  if (err.code === 'ZONE_CODE_ALREADY_EXISTS') {
    if (mode === 'create') fields.id = err.message;
    fields.code = err.message;
    return { fields, banner: null, stale: false };
  }
  if (err.code === 'CONCURRENT_MODIFICATION') {
    return { fields, banner: err.message, stale: true };
  }
  if (err.code === 'VALIDATION_FAILED' && err.details.length > 0) {
    let placed = false;
    for (const d of err.details) {
      const f = d.field ? API_FIELD_TO_FORM[d.field] : undefined;
      if (f) {
        fields[f] = d.issue;
        placed = true;
      }
    }
    return { fields, banner: placed ? null : err.message, stale: false };
  }
  return { fields, banner: err.message, stale: false };
}

export const ZoneFormModal: React.FC<ZoneFormModalProps> = ({ mode, zone, token, onClose, onSaved, onStale }) => {
  const editing = mode === 'edit' && zone !== undefined;

  const [values, setValues] = useState<ZoneFormValues>({
    id: zone?.id ?? '',
    code: zone?.code ?? '',
    name: zone?.name ?? '',
    description: zone?.description ?? '',
    prefixesText: zone ? zone.postal_prefixes.join(', ') : '',
  });
  const [reason, setReason] = useState('');
  const [errors, setErrors] = useState<ZoneFormErrors>({});
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [banner, setBanner] = useState<string | null>(null);
  const [correlationId, setCorrelationId] = useState('');
  const [stale, setStale] = useState(false);
  const [saving, setSaving] = useState(false);

  // One key per attempt: a double click or a retry after a lost connection cannot create twice.
  const idempotencyKey = useRef<string>(crypto.randomUUID());

  const set = (field: keyof ZoneFormValues) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setValues((v) => ({ ...v, [field]: e.target.value }));
    const key: ZoneFormField = field === 'prefixesText' ? 'prefixes' : field;
    if (errors[key]) setErrors((prev) => ({ ...prev, [key]: undefined }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBanner(null);
    setCorrelationId('');
    setStale(false);

    const found = validateZoneForm(values, mode);
    let reasonProblem: string | null = null;
    if (editing && [...reason.trim()].length > REASON_MAX) {
      reasonProblem = `Reason can have at most ${REASON_MAX} characters.`;
    }
    setErrors(found);
    setReasonError(reasonProblem);
    if (Object.keys(found).length > 0 || reasonProblem) return;

    setSaving(true);
    try {
      const prefixes = parsePrefixes(values.prefixesText);
      if (editing && zone) {
        const patch: ZoneUpdateInput = { version: zone.version };
        if (values.code.trim() !== zone.code) patch.code = values.code.trim();
        if (values.name.trim() !== zone.name) patch.name = values.name.trim();
        if (values.description !== zone.description) patch.description = values.description;
        if (!samePrefixSet(prefixes, zone.postal_prefixes)) patch.postal_prefixes = prefixes;
        if (Object.keys(patch).length === 1) {
          setBanner('Nothing was changed, so there is nothing to save.');
          return;
        }
        if (reason.trim()) patch.reason = reason.trim();
        const saved = await updateZone(token, zone.id, patch);
        onSaved(saved, `Zone ${saved.id} updated (version ${saved.version}).`);
      } else {
        const saved = await createZone(
          token,
          {
            id: values.id.trim(),
            code: values.code.trim(),
            name: values.name.trim(),
            description: values.description.trim() || undefined,
            postal_prefixes: prefixes,
          },
          idempotencyKey.current,
        );
        onSaved(saved, `Zone ${saved.id} created.`);
      }
    } catch (err) {
      if (err instanceof ApiError) {
        const out = explain(err, mode);
        setErrors(out.fields);
        setBanner(out.banner);
        setStale(out.stale);
        setCorrelationId(err.correlationId);
        // A definite answer from the server ends this attempt. A lost connection keeps the key.
        if (err.status !== 0) idempotencyKey.current = crypto.randomUUID();
      } else {
        setBanner("Couldn't save the zone. Try again.");
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog
      title={editing ? `Edit zone ${zone?.id}` : 'New delivery zone'}
      description={editing ? 'Change the name, code or postal prefixes. The id never changes.' : 'Zones hold no fees. Prices are set as pricing rules.'}
      onClose={onClose}
      busy={saving}
      testId="admin-zone-form"
    >
      <form onSubmit={handleSubmit} noValidate>
        <div className="grid max-h-96 gap-4 overflow-y-auto px-5 py-5">
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

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label htmlFor="zone-id">Zone id</Label>
              <Input
                id="zone-id"
                data-testid="admin-zone-id-input"
                className="font-mono"
                value={values.id}
                onChange={set('id')}
                disabled={editing || saving}
                placeholder="ZONE_GALLE_01"
                autoComplete="off"
                aria-invalid={Boolean(errors.id)}
                aria-describedby="zone-id-note"
              />
              {errors.id ? (
                <FieldError id="zone-id-note">{errors.id}</FieldError>
              ) : (
                <FieldHint id="zone-id-note">{editing ? 'Fixed for the life of the zone.' : 'Fixed forever once saved.'}</FieldHint>
              )}
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="zone-code">Short code</Label>
              <Input
                id="zone-code"
                data-testid="admin-zone-code-input"
                className="font-mono"
                value={values.code}
                onChange={set('code')}
                disabled={saving}
                placeholder="GLE-01"
                autoComplete="off"
                aria-invalid={Boolean(errors.code)}
                aria-describedby={errors.code ? 'zone-code-error' : undefined}
              />
              {errors.code && <FieldError id="zone-code-error">{errors.code}</FieldError>}
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="zone-name">Name</Label>
            <Input
              id="zone-name"
              data-testid="admin-zone-name-input"
              value={values.name}
              onChange={set('name')}
              disabled={saving}
              placeholder="Galle"
              maxLength={NAME_MAX + 20}
              aria-invalid={Boolean(errors.name)}
              aria-describedby={errors.name ? 'zone-name-error' : undefined}
            />
            {errors.name && <FieldError id="zone-name-error">{errors.name}</FieldError>}
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="zone-description">Description (optional)</Label>
            <Textarea
              id="zone-description"
              data-testid="admin-zone-description-input"
              value={values.description}
              onChange={set('description')}
              disabled={saving}
              rows={2}
              placeholder="South coast, Galle Fort and surroundings"
              aria-invalid={Boolean(errors.description)}
              aria-describedby={errors.description ? 'zone-description-error' : undefined}
            />
            <div className="flex justify-between gap-3">
              {errors.description ? <FieldError id="zone-description-error">{errors.description}</FieldError> : <span />}
              <span className="font-mono text-xs tabular-nums text-text-muted">
                {[...values.description].length}/{DESCRIPTION_MAX}
              </span>
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="zone-prefixes">Postal code prefixes</Label>
            <Textarea
              id="zone-prefixes"
              data-testid="admin-zone-prefixes-input"
              className="font-mono"
              value={values.prefixesText}
              onChange={set('prefixesText')}
              disabled={saving}
              rows={2}
              placeholder="80000, 80100"
              aria-invalid={Boolean(errors.prefixes)}
              aria-describedby="zone-prefixes-note"
            />
            {errors.prefixes ? (
              <FieldError id="zone-prefixes-note">{errors.prefixes}</FieldError>
            ) : (
              <FieldHint id="zone-prefixes-note">
                3 to 5 digits each, separated by commas or spaces. A prefix can belong to one zone only.
              </FieldHint>
            )}
          </div>

          {editing && (
            <div className="grid gap-1.5">
              <Label htmlFor="zone-reason">Reason for the change (optional, kept in the audit log)</Label>
              <Input
                id="zone-reason"
                data-testid="admin-zone-reason-input"
                value={reason}
                onChange={(e) => {
                  setReason(e.target.value);
                  setReasonError(null);
                }}
                disabled={saving}
                placeholder="Added Bambalapitiya to the central route"
                aria-invalid={Boolean(reasonError)}
                aria-describedby={reasonError ? 'zone-reason-error' : undefined}
              />
              {reasonError && <FieldError id="zone-reason-error">{reasonError}</FieldError>}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" disabled={saving} data-testid="admin-zone-save-btn">
            {saving ? <Busy label="Saving…" /> : editing ? 'Save changes' : 'Create zone'}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};
