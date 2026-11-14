// Failed deliveries. No endpoint lists failed attempts across parcels, so the list is the
// ATTEMPT_FAILED parcels on the dispatch board (dispatch API.md section 3), and each parcel's
// attempts come from GET /api/v1/deliveries/parcels/{id} (delivery API.md section 7).
// A retry is a reschedule to a courier (dispatch API.md section 6).

import { request } from '../api';

export interface AttemptRecord {
  id: string;
  attempt_number: number;
  /** FAILED or SUCCESS. */
  outcome: string;
  /** One of the approved failure reason codes, empty for a successful attempt. */
  reason_code: string;
  notes: string;
  attempted_at: string;
}

export interface AttemptHistory {
  parcel_id: string;
  current_status: string;
  attempts: AttemptRecord[];
}

/** Approved failure reasons (ADR-004), in the words staff read. */
export const REASON_LABEL: Record<string, string> = {
  RECIPIENT_UNAVAILABLE: 'Recipient unavailable',
  CUSTOMER_REQUESTED_RESCHEDULE: 'Customer asked to reschedule',
  ACCESS_RESTRICTED: 'Access restricted',
  INVALID_ADDRESS: 'Invalid address',
  REFUSED_BY_RECIPIENT: 'Refused by recipient',
  DAMAGED_IN_TRANSIT: 'Damaged in transit',
  OTHER: 'Other reason',
};

export function reasonLabel(code: string): string {
  if (!code) return 'No reason recorded';
  return REASON_LABEL[code] ?? code.charAt(0) + code.slice(1).toLowerCase().replace(/_/g, ' ');
}

type Raw = Record<string, unknown>;

function s(raw: Raw, ...keys: string[]): string {
  for (const k of keys) if (typeof raw[k] === 'string' && raw[k] !== '') return raw[k] as string;
  return '';
}

/**
 * API.md names the fields id and failure_reason_code; the running endpoint sends attempt_id and
 * reason_code. Both are accepted.
 */
export function normalizeAttempt(raw: Raw): AttemptRecord {
  return {
    id: s(raw, 'id', 'attempt_id'),
    attempt_number: typeof raw.attempt_number === 'number' ? raw.attempt_number : 0,
    outcome: s(raw, 'outcome'),
    reason_code: s(raw, 'failure_reason_code', 'reason_code'),
    notes: s(raw, 'notes'),
    attempted_at: s(raw, 'attempted_at'),
  };
}

export async function getAttemptHistory(token: string | null, parcelId: string): Promise<AttemptHistory> {
  const res = await request<{ data: { parcel_id?: string; current_status?: string; attempts?: Raw[] | null } | null }>(
    token,
    'GET',
    `/api/v1/deliveries/parcels/${encodeURIComponent(parcelId)}`,
  );
  const d = res.data ?? {};
  return {
    parcel_id: d.parcel_id ?? parcelId,
    current_status: d.current_status ?? '',
    attempts: (d.attempts ?? []).map(normalizeAttempt).sort((a, b) => a.attempt_number - b.attempt_number),
  };
}

export function failedAttempts(h: AttemptHistory): AttemptRecord[] {
  return h.attempts.filter((a) => a.outcome === 'FAILED');
}

/** The most recent failed attempt, if any. */
export function lastFailure(h: AttemptHistory): AttemptRecord | undefined {
  const failed = failedAttempts(h);
  return failed[failed.length - 1];
}
