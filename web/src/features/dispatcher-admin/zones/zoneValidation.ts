// Quick form hints only (Doc 2 F-10 step 3). The backend repeats every check and
// stays the real authority, so a value that passes here can still be refused.

export const ZONE_ID_RE = /^ZONE_[A-Z0-9_]{2,27}$/;
export const ZONE_CODE_RE = /^[A-Z0-9_-]{2,32}$/;
export const PREFIX_RE = /^[0-9]{3,5}$/;

export const NAME_MAX = 120;
export const DESCRIPTION_MAX = 500;
export const REASON_MIN = 10;
export const REASON_MAX = 500;

export interface ZoneFormValues {
  id: string;
  code: string;
  name: string;
  description: string;
  prefixesText: string;
}

export type ZoneFormField = 'id' | 'code' | 'name' | 'description' | 'prefixes';
export type ZoneFormErrors = Partial<Record<ZoneFormField, string>>;

/** Splits "00100, 00200\n00300" into ["00100", "00200", "00300"]. */
export function parsePrefixes(text: string): string[] {
  return text
    .split(/[\s,;]+/)
    .map((p) => p.trim())
    .filter((p) => p.length > 0);
}

export function validateZoneForm(v: ZoneFormValues, mode: 'create' | 'edit'): ZoneFormErrors {
  const errors: ZoneFormErrors = {};

  if (mode === 'create') {
    if (!v.id.trim()) errors.id = 'Zone id is required.';
    else if (!ZONE_ID_RE.test(v.id.trim())) {
      errors.id = 'Use capital letters, digits and underscores, starting with ZONE_ (for example ZONE_GALLE_01).';
    }
  }

  if (!v.code.trim()) errors.code = 'Short code is required.';
  else if (!ZONE_CODE_RE.test(v.code.trim())) {
    errors.code = 'Use 2 to 32 capital letters, digits, "-" or "_" (for example GLE-01).';
  }

  const name = v.name.trim();
  if (!name) errors.name = 'Name is required.';
  else if ([...name].length > NAME_MAX) errors.name = `Name can have at most ${NAME_MAX} characters.`;

  if ([...v.description].length > DESCRIPTION_MAX) {
    errors.description = `Description can have at most ${DESCRIPTION_MAX} characters.`;
  }

  const prefixes = parsePrefixes(v.prefixesText);
  if (prefixes.length === 0) {
    errors.prefixes = 'Add at least one postal code prefix.';
  } else {
    const bad = prefixes.filter((p) => !PREFIX_RE.test(p));
    if (bad.length > 0) {
      errors.prefixes = `Prefixes must be 3 to 5 digits. Not valid: ${bad.join(', ')}.`;
    } else if (new Set(prefixes).size !== prefixes.length) {
      errors.prefixes = 'A prefix is listed twice.';
    }
  }

  return errors;
}

/** Reason for enabling or disabling a zone: 10 to 500 characters (API.md section 2). */
export function validateReason(reason: string): string | null {
  const n = [...reason.trim()].length;
  if (n < REASON_MIN || n > REASON_MAX) {
    return `Give a reason of ${REASON_MIN} to ${REASON_MAX} characters.`;
  }
  return null;
}

/** True when two prefix lists hold the same values, ignoring order. */
export function samePrefixSet(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const have = new Set(a);
  return b.every((p) => have.has(p));
}
