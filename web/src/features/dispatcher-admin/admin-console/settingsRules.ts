// Quick form hints for system settings (core-api/internal/admin/API.md section 3). The backend
// repeats every check and stays the real authority, so a value that passes here can still be
// refused. Only these keys can be changed; any other key is refused by the server.

import type { Setting } from './consoleApi';

export type SettingKind = 'integer' | 'decimal' | 'text';

export interface SettingRule {
  kind: SettingKind;
  label: string;
  /** Allowed range for numbers, or maximum length for text. */
  min?: number;
  max?: number;
  unit?: string;
  help: string;
}

export const SETTING_RULES: Record<string, SettingRule> = {
  'reports.max_range_days': {
    kind: 'integer',
    label: 'Longest report period',
    min: 1,
    max: 366,
    unit: 'days',
    help: 'A report may not cover more days than this. Whole number from 1 to 366.',
  },
  'ops.board_refresh_seconds': {
    kind: 'integer',
    label: 'Dispatch board refresh',
    min: 10,
    max: 300,
    unit: 'seconds',
    help: 'How often the dispatch board reloads. Whole number from 10 to 300.',
  },
  'ops.maintenance_banner': {
    kind: 'text',
    label: 'Maintenance banner',
    max: 200,
    help: 'Message shown on operations screens. At most 200 characters; leave empty for none.',
  },
  'parcels.max_weight_kg': {
    kind: 'decimal',
    label: 'Heaviest parcel',
    min: 0.5,
    max: 30,
    unit: 'kg',
    help: 'Heaviest parcel accepted. A number from 0.5 to 30.',
  },
  'parcels.max_cod_cents': {
    kind: 'integer',
    label: 'Highest cash on delivery',
    min: 0,
    max: 10_000_000,
    unit: 'LKR cents',
    help: 'Highest cash-on-delivery amount in LKR cents (100 cents = LKR 1.00). Whole number from 0 to 10,000,000.',
  },
};

export function ruleFor(key: string): SettingRule | undefined {
  return SETTING_RULES[key];
}

/** The allowed values in words, for the table and the dialog, for example "Whole number from 10 to 300 seconds". */
export function allowedText(rule: SettingRule): string {
  if (rule.kind === 'text') return `Up to ${rule.max} characters`;
  const lo = (rule.min ?? 0).toLocaleString('en-LK');
  const hi = (rule.max ?? 0).toLocaleString('en-LK');
  const unit = rule.unit ? ` ${rule.unit}` : '';
  return `${rule.kind === 'integer' ? 'Whole number' : 'Number'} from ${lo} to ${hi}${unit}`;
}

/** A live rupee preview while the admin types cents, or null when there is nothing to show. */
export function previewFor(key: string, text: string): string | null {
  if (key !== 'parcels.max_cod_cents') return null;
  const t = text.trim();
  if (!/^\d{1,9}$/.test(t)) return null;
  return `= ${formatCents(Number(t))}`;
}

/** Formats whole LKR cents as "LKR 35,000.00" with integer arithmetic only (no floats). */
export function formatCents(cents: number): string {
  const negative = cents < 0;
  const abs = Math.abs(Math.trunc(cents));
  const whole = Math.floor(abs / 100);
  const frac = String(abs % 100).padStart(2, '0');
  return `${negative ? '-' : ''}LKR ${whole.toLocaleString('en-LK')}.${frac}`;
}

/** The current value as text for a form field. */
export function valueToText(value: Setting['value']): string {
  return value === null || value === undefined ? '' : String(value);
}

/** The current value for display in the table, with its unit. */
export function displayValue(setting: Setting): string {
  const rule = ruleFor(setting.key);
  if (setting.value === '' || setting.value === null) return 'Not set';
  if (setting.key === 'parcels.max_cod_cents' && typeof setting.value === 'number') {
    return `${setting.value.toLocaleString('en-LK')} cents (${formatCents(setting.value)})`;
  }
  const text = String(setting.value);
  return rule?.unit && rule.kind !== 'text' ? `${text} ${rule.unit}` : text;
}

export type ParsedSetting = { ok: true; value: number | string } | { ok: false; message: string };

/** Turns what the admin typed into the JSON value the API expects, or explains what is wrong. */
export function parseSettingInput(key: string, text: string): ParsedSetting {
  const rule = ruleFor(key);
  if (!rule) return { ok: false, message: 'This setting cannot be changed.' };

  if (rule.kind === 'text') {
    if ([...text].length > (rule.max ?? 200)) return { ok: false, message: `At most ${rule.max} characters.` };
    return { ok: true, value: text };
  }

  const trimmed = text.trim();
  if (trimmed === '') return { ok: false, message: 'Enter a value.' };
  // Plain digits only: no exponent ("1e2"), hex ("0x1F"), thousands separators or spaces.
  const shape = rule.kind === 'integer' ? /^-?\d+$/ : /^-?\d+(\.\d+)?$/;
  if (!shape.test(trimmed)) {
    return { ok: false, message: rule.kind === 'integer' ? 'Enter a whole number using digits only, for example 60.' : 'Enter a number using digits only, for example 25 or 12.5.' };
  }
  const n = Number(trimmed);
  if (!Number.isFinite(n)) return { ok: false, message: 'Enter a number.' };
  const min = rule.min ?? Number.NEGATIVE_INFINITY;
  const max = rule.max ?? Number.POSITIVE_INFINITY;
  if (n < min || n > max) return { ok: false, message: `Enter a value from ${min.toLocaleString('en-LK')} to ${max.toLocaleString('en-LK')}.` };
  return { ok: true, value: n };
}
