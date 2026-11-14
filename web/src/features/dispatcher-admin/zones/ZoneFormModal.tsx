import React, { useRef, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { ApiError, createZone, getZone, updateZone } from './zonesApi';
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
  /** The latest zone could not be read after a conflict: the panel reloads the list. */
  onStale: () => void;
}

const API_FIELD_TO_FORM: Record<string, ZoneFormField> = {
  id: 'id',
  code: 'code',
  name: 'name',
  description: 'description',
  postal_prefixes: 'prefixes',
};

type EditableField = 'code' | 'name' | 'description' | 'prefixes';

const FIELD_LABEL: Record<EditableField, string> = {
  code: 'short code',
  name: 'name',
  description: 'description',
  prefixes: 'postal prefixes',
};

/** Fields whose value differs between two versions of the same zone. */
function changedFields(a: Zone, b: Zone): EditableField[] {
  const out: EditableField[] = [];
  if (a.code !== b.code) out.push('code');
  if (a.name !== b.name) out.push('name');
  if (a.description !== b.description) out.push('description');
  if (!samePrefixSet(a.postal_prefixes, b.postal_prefixes)) out.push('prefixes');
  return out;
}

/** Fields the user has edited in the form compared with the zone they started from. */
function editedFields(base: Zone, v: ZoneFormValues): EditableField[] {
  const out: EditableField[] = [];
  if (v.code.trim() !== base.code) out.push('code');
  if (v.name.trim() !== base.name) out.push('name');
  if (v.description !== base.description) out.push('description');
  if (!samePrefixSet(parsePrefixes(v.prefixesText), base.postal_prefixes)) out.push('prefixes');
  return out;
}

function listOf(fields: EditableField[]): string {
  return fields.map((f) => FIELD_LABEL[f]).join(', ');
}

/** Turns a refused request into inline field errors and an optional banner. */
function explain(err: ApiError, mode: 'create' | 'edit'): { fields: ZoneFormErrors; banner: string | null } {
  const fields: ZoneFormErrors = {};
  if (err.code === 'ZONE_POSTAL_CODE_CONFLICT') {
    const named = err.details.map((d) => d.issue).filter(Boolean).join(' ');
    fields.prefixes = named ? `${err.message} ${named}` : err.message;
    return { fields, banner: null };
  }
  if (err.code === 'ZONE_CODE_ALREADY_EXISTS') {
    if (mode === 'create') fields.id = err.message;
    fields.code = err.message;
    return { fields, banner: null };
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
    return { fields, banner: placed ? null : err.message };
  }
  return { fields, banner: err.message };
}

interface Conflict {
  latest: Zone;
  /** Fields someone else changed since this dialog opened. */
  theirs: EditableField[];
  /** Fields both people changed. */
  both: EditableField[];
}

export const ZoneFormModal: React.FC<ZoneFormModalProps> = ({ mode, zone, token, onClose, onSaved, onStale }) => {
  const editing = mode === 'edit' && zone !== undefined;

  // The version of the zone this form is based on. It moves forward when the admin
  // chooses to keep their edits on top of a newer version after a conflict.
  const [base, setBase] = useState<Zone | undefined>(zone);
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
  const [conflict, setConflict] = useState<Conflict | null>(null);
  const [reloadNeeded, setReloadNeeded] = useState(false);
  const [saving, setSaving] = useState(false);

  // One key per attempt: a double click or a retry after a lost connection cannot create twice.
  const idempotencyKey = useRef<string>(crypto.randomUUID());

  const set = (field: keyof ZoneFormValues) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setValues((v) => ({ ...v, [field]: e.target.value }));
    const key: ZoneFormField = field === 'prefixesText' ? 'prefixes' : field;
    if (errors[key]) setErrors((prev) => ({ ...prev, [key]: undefined }));
  };

  /** Version conflict: read the latest zone and let the admin decide, without losing their edits. */
  const handleConflict = async (current: Zone, original: ApiError) => {
    try {
      const latest = await getZone(token, current.id);
      const theirs = changedFields(current, latest);
      const both = theirs.filter((f) => editedFields(current, values).includes(f));
      setConflict({ latest, theirs, both });
    } catch {
      setBanner(original.message);
      setReloadNeeded(true);
    }
  };

  /** Move the form onto the latest version. Fields the admin did not edit take the new value. */
  const keepMine = () => {
    if (!conflict || !base) return;
    const mine = editedFields(base, values);
    const { latest } = conflict;
    setValues((v) => ({
      ...v,
      code: mine.includes('code') ? v.code : latest.code,
      name: mine.includes('name') ? v.name : latest.name,
      description: mine.includes('description') ? v.description : latest.description,
      prefixesText: mine.includes('prefixes') ? v.prefixesText : latest.postal_prefixes.join(', '),
    }));
    setBase(latest);
    setConflict(null);
    setBanner(null);
  };

  /** Throw away the admin's edits and show the latest zone. */
  const discardMine = () => {
    if (!conflict) return;
    const { latest } = conflict;
    setValues((v) => ({
      ...v,
      code: latest.code,
      name: latest.name,
      description: latest.description,
      prefixesText: latest.postal_prefixes.join(', '),
    }));
    setBase(latest);
    setConflict(null);
    setBanner(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBanner(null);
    setCorrelationId('');
    setReloadNeeded(false);

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
      if (editing && base) {
        const patch: ZoneUpdateInput = { version: base.version };
        if (values.code.trim() !== base.code) patch.code = values.code.trim();
        if (values.name.trim() !== base.name) patch.name = values.name.trim();
        if (values.description !== base.description) patch.description = values.description;
        if (!samePrefixSet(prefixes, base.postal_prefixes)) patch.postal_prefixes = prefixes;
        if (Object.keys(patch).length === 1) {
          setBanner('Nothing was changed, so there is nothing to save.');
          return;
        }
        if (reason.trim()) patch.reason = reason.trim();
        try {
          const saved = await updateZone(token, base.id, patch);
          onSaved(saved, `Zone ${saved.id} updated (version ${saved.version}).`);
        } catch (err) {
          if (err instanceof ApiError && err.code === 'CONCURRENT_MODIFICATION') {
            setCorrelationId(err.correlationId);
            await handleConflict(base, err);
            return;
          }
          throw err;
        }
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
          {conflict && (
            <Banner
              tone="attn"
              testId="admin-zone-conflict"
              reference={correlationId}
              action={
                <div className="flex flex-wrap gap-2">
                  <Button type="button" size="sm" onClick={keepMine} data-testid="admin-zone-conflict-keep-btn">
                    Keep my changes on the latest version
                  </Button>
                  <Button type="button" variant="outline" size="sm" onClick={discardMine} data-testid="admin-zone-conflict-discard-btn">
                    Discard my changes
                  </Button>
                </div>
              }
            >
              Someone else changed this zone while you were editing. It is now version {conflict.latest.version}.{' '}
              {conflict.theirs.length > 0 ? `They changed the ${listOf(conflict.theirs)}. ` : ''}
              {conflict.both.length > 0
                ? `You both changed the ${listOf(conflict.both)}: keeping your changes replaces their value. `
                : ''}
              Your edits are still in the form.
            </Banner>
          )}

          {banner && (
            <Banner
              tone="err"
              reference={correlationId}
              action={
                reloadNeeded ? (
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
          <Button type="submit" disabled={saving || conflict !== null} data-testid="admin-zone-save-btn">
            {saving ? <Busy label="Saving…" /> : editing ? 'Save changes' : 'Create zone'}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};
