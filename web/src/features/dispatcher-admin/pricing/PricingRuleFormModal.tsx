import React, { useRef, useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ApiError } from '../api';
import { AdminDialog, Banner, Busy, FieldError, FieldHint } from '../admin-console/ui';
import type { ZoneOption } from '../fleet/fleetApi';
import { centsToInput, parseLkrToCents } from './pricingMoney';
import { createRule, updateRule, type PricingRule, type RuleChange } from './pricingApi';

// Create a rule (pick a route) or change one (the route is fixed; a change makes a new version).
// The screen stores the fee and the rate per kilogram exactly as typed. It never calculates a price.

const colomboDay = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Colombo', year: 'numeric', month: '2-digit', day: '2-digit' });

/** The Sri Lanka calendar day of an instant, as YYYY-MM-DD. */
export function dayOf(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '' : colomboDay.format(d);
}

/** A Sri Lanka day as the start of that day, for the API. */
function startOfDay(day: string): string {
  return `${day}T00:00:00+05:30`;
}

interface Errors {
  origin?: string;
  destination?: string;
  base?: string;
  perKg?: string;
  from?: string;
  until?: string;
}

const FIELD_FOR: Record<string, keyof Errors> = {
  origin_zone_id: 'origin',
  destination_zone_id: 'destination',
  base_fee_cents: 'base',
  per_kg_rate_cents: 'perKg',
  effective_from: 'from',
  effective_until: 'until',
};

interface PricingRuleFormModalProps {
  token: string | null;
  zones: ZoneOption[];
  /** The rule being changed, or undefined to create one. */
  rule?: PricingRule;
  onClose: () => void;
  onSaved: (rule: PricingRule, message: string) => void;
  /** The rule changed behind this admin's back: reload the list. */
  onStale: () => void;
}

export const PricingRuleFormModal: React.FC<PricingRuleFormModalProps> = ({ token, zones, rule, onClose, onSaved, onStale }) => {
  const editing = Boolean(rule);
  const [origin, setOrigin] = useState(rule?.origin_zone_id ?? '');
  const [destination, setDestination] = useState(rule?.destination_zone_id ?? '');
  const [base, setBase] = useState(rule ? centsToInput(rule.base_fee_cents) : '');
  const [perKg, setPerKg] = useState(rule ? centsToInput(rule.per_kg_rate_cents) : '');
  const [from, setFrom] = useState(rule ? dayOf(rule.effective_from) : '');
  const [until, setUntil] = useState(rule?.effective_until ? dayOf(rule.effective_until) : '');
  const [errors, setErrors] = useState<Errors>({});
  const [banner, setBanner] = useState<ApiError | null>(null);
  const [stale, setStale] = useState(false);
  const [saving, setSaving] = useState(false);
  const key = useRef(crypto.randomUUID());
  const zoneName = (id: string) => zones.find((z) => z.id === id)?.name ?? id;

  const clear = (k: keyof Errors) => setErrors((e) => ({ ...e, [k]: undefined }));

  const submit = async (ev: React.FormEvent) => {
    ev.preventDefault();
    setBanner(null);
    const found: Errors = {};
    if (!editing && !origin) found.origin = 'Choose where parcels start.';
    if (!editing && !destination) found.destination = 'Choose where parcels go.';
    const baseCents = parseLkrToCents(base);
    const perKgCents = parseLkrToCents(perKg);
    if (baseCents === null) found.base = 'Enter an amount like 450 or 450.50.';
    if (perKgCents === null) found.perKg = 'Enter an amount like 80 or 80.50.';
    if (!from) found.from = 'Choose the first day this rule applies.';
    if (until && from && until <= from) found.until = 'The last day must be after the first day.';
    setErrors(found);
    if (Object.keys(found).length > 0 || baseCents === null || perKgCents === null) return;

    const change: RuleChange = { base_fee_cents: baseCents, per_kg_rate_cents: perKgCents, effective_from: startOfDay(from), effective_until: until ? startOfDay(until) : null };
    if (rule && baseCents === rule.base_fee_cents && perKgCents === rule.per_kg_rate_cents && from === dayOf(rule.effective_from) && until === (rule.effective_until ? dayOf(rule.effective_until) : '')) {
      setBanner(new ApiError(0, 'NO_CHANGE', 'Nothing has changed. Change a fee or a date, or cancel.'));
      return;
    }

    setSaving(true);
    try {
      const saved = rule ? await updateRule(token, rule.id, rule.version, change) : await createRule(token, { origin_zone_id: origin, destination_zone_id: destination, ...change }, key.current);
      onSaved(saved, rule ? `Rule ${saved.id} changed. It is now version ${saved.version}.` : `Rule ${saved.id} created for ${zoneName(saved.origin_zone_id)} to ${zoneName(saved.destination_zone_id)}.`);
    } catch (err) {
      const api = err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't save the rule. Try again.");
      if (api.status !== 0 && api.status < 500) key.current = crypto.randomUUID();
      if (api.code === 'CONCURRENT_MODIFICATION') {
        setStale(true);
        setBanner(api);
      } else if (api.code === 'VALIDATION_FAILED' && api.details.length > 0) {
        const next: Errors = {};
        for (const d of api.details) {
          const k = d.field ? FIELD_FOR[d.field] : undefined;
          if (k) next[k] = d.issue;
        }
        setErrors(next);
        if (Object.keys(next).length === 0) setBanner(api);
      } else {
        setBanner(api);
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog
      title={editing ? `Change rule ${rule?.id}` : 'Add a pricing rule'}
      description={editing ? 'A change creates a new version of the rule. The route stays the same.' : 'Set what parcels cost on one route. The server works out each price from these two amounts.'}
      onClose={onClose}
      busy={saving}
      testId="pricing-form-dialog"
    >
      <form onSubmit={submit} noValidate>
        <div className="grid max-h-96 gap-4 overflow-y-auto px-5 py-5">
          {banner && (
            <Banner
              tone={stale || banner.code === 'NO_CHANGE' ? 'attn' : 'err'}
              reference={banner.correlationId}
              testId="pricing-form-error"
              action={
                stale ? (
                  <div>
                    <Button type="button" size="sm" onClick={onStale} data-testid="pricing-stale-reload-btn">
                      Close and reload the list
                    </Button>
                  </div>
                ) : undefined
              }
            >
              {banner.status === 403 ? 'Your account is not allowed to change pricing rules.' : banner.message}
            </Banner>
          )}

          {editing ? (
            <div className="rounded-md border border-border bg-surface-subtle p-3 text-sm" data-testid="pricing-form-route">
              <span className="text-text-muted">Route</span> <span className="font-medium">{zoneName(origin)}</span> to <span className="font-medium">{zoneName(destination)}</span>
              <span className="ml-2 font-mono text-xs tabular-nums text-text-muted">version {rule?.version}</span>
            </div>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid content-start gap-1.5">
                <Label htmlFor="pricing-origin">From zone</Label>
                <Select
                  value={origin}
                  onValueChange={(v) => {
                    setOrigin(v);
                    clear('origin');
                  }}
                >
                  <SelectTrigger id="pricing-origin" data-testid="pricing-form-origin" aria-invalid={Boolean(errors.origin)} aria-describedby={errors.origin ? 'pricing-origin-error' : undefined}>
                    <SelectValue placeholder="Choose a zone" />
                  </SelectTrigger>
                  <SelectContent>
                    {zones.map((z) => (
                      <SelectItem key={z.id} value={z.id}>
                        {z.name} ({z.code})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {errors.origin && <FieldError id="pricing-origin-error">{errors.origin}</FieldError>}
              </div>
              <div className="grid content-start gap-1.5">
                <Label htmlFor="pricing-destination">To zone</Label>
                <Select
                  value={destination}
                  onValueChange={(v) => {
                    setDestination(v);
                    clear('destination');
                  }}
                >
                  <SelectTrigger id="pricing-destination" data-testid="pricing-form-destination" aria-invalid={Boolean(errors.destination)} aria-describedby={errors.destination ? 'pricing-destination-error' : undefined}>
                    <SelectValue placeholder="Choose a zone" />
                  </SelectTrigger>
                  <SelectContent>
                    {zones.map((z) => (
                      <SelectItem key={z.id} value={z.id}>
                        {z.name} ({z.code})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {errors.destination && <FieldError id="pricing-destination-error">{errors.destination}</FieldError>}
              </div>
            </div>
          )}

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid content-start gap-1.5">
              <Label htmlFor="pricing-base">Base fee (LKR)</Label>
              <Input
                id="pricing-base"
                data-testid="pricing-form-base"
                className="font-mono tabular-nums"
                inputMode="decimal"
                value={base}
                onChange={(e) => {
                  setBase(e.target.value);
                  clear('base');
                }}
                placeholder="450.00"
                autoComplete="off"
                aria-invalid={Boolean(errors.base)}
                aria-describedby={errors.base ? 'pricing-base-error' : 'pricing-base-hint'}
              />
              {errors.base ? <FieldError id="pricing-base-error">{errors.base}</FieldError> : <FieldHint id="pricing-base-hint">Charged once for every parcel on this route.</FieldHint>}
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="pricing-perkg">Rate per kg (LKR)</Label>
              <Input
                id="pricing-perkg"
                data-testid="pricing-form-perkg"
                className="font-mono tabular-nums"
                inputMode="decimal"
                value={perKg}
                onChange={(e) => {
                  setPerKg(e.target.value);
                  clear('perKg');
                }}
                placeholder="80.00"
                autoComplete="off"
                aria-invalid={Boolean(errors.perKg)}
                aria-describedby={errors.perKg ? 'pricing-perkg-error' : 'pricing-perkg-hint'}
              />
              {errors.perKg ? <FieldError id="pricing-perkg-error">{errors.perKg}</FieldError> : <FieldHint id="pricing-perkg-hint">Charged for each kilogram. The server works out the final price.</FieldHint>}
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="pricing-from">First day (Sri Lanka)</Label>
              <Input
                id="pricing-from"
                data-testid="pricing-form-from"
                type="date"
                className="font-mono tabular-nums"
                value={from}
                onChange={(e) => {
                  setFrom(e.target.value);
                  clear('from');
                  clear('until');
                }}
                aria-invalid={Boolean(errors.from)}
                aria-describedby={errors.from ? 'pricing-from-error' : undefined}
              />
              {errors.from && <FieldError id="pricing-from-error">{errors.from}</FieldError>}
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="pricing-until">Last day (optional)</Label>
              <Input
                id="pricing-until"
                data-testid="pricing-form-until"
                type="date"
                className="font-mono tabular-nums"
                value={until}
                onChange={(e) => {
                  setUntil(e.target.value);
                  clear('until');
                }}
                aria-invalid={Boolean(errors.until)}
                aria-describedby={errors.until ? 'pricing-until-error' : 'pricing-until-hint'}
              />
              {errors.until ? <FieldError id="pricing-until-error">{errors.until}</FieldError> : <FieldHint id="pricing-until-hint">Leave empty if the rule has no end date.</FieldHint>}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" disabled={saving || stale} data-testid="pricing-form-save-btn">
            {saving ? <Busy label="Saving..." /> : editing ? 'Save changes' : 'Add rule'}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};
