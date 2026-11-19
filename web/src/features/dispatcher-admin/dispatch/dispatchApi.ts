// Dispatch board calls (core-api/internal/dispatch/API.md sections 3, 4 and 6). ADMIN and
// DISPATCHER only. Couriers use the same reader as the Fleet screen, which accepts both the
// documented field names and the names the running endpoint sends.

import { request } from '../api';
import { normalizeCourier, type CourierLoad } from '../fleet/fleetApi';

export interface WaitingParcel {
  id: string;
  tracking_number: string;
  weight_kg: number;
  recipient_city: string;
  /** CREATED (never assigned) or ATTEMPT_FAILED (needs a new courier). */
  status: string;
  created_at: string;
}

export interface DispatchBoard {
  zone_id: string;
  parcels: WaitingParcel[];
  couriers: CourierLoad[];
}

interface RawBoard {
  zone_id?: string;
  unassigned_parcels?: WaitingParcel[] | null;
  courier_loads?: Record<string, unknown>[] | null;
}

/** An empty zone_id means every zone. Lists that come back null become empty lists. */
export async function getBoard(token: string | null, zoneId?: string): Promise<DispatchBoard> {
  const q = zoneId ? `?zone_id=${encodeURIComponent(zoneId)}` : '';
  const res = await request<{ data: RawBoard | null }>(token, 'GET', `/api/v1/dispatch/board${q}`);
  const raw = res.data ?? {};
  return {
    zone_id: raw.zone_id ?? zoneId ?? '',
    parcels: raw.unassigned_parcels ?? [],
    couriers: (raw.courier_loads ?? []).map(normalizeCourier),
  };
}

export interface AssignmentResult {
  assignment_id: string;
  parcel_id: string;
  courier_id: string;
  status: string;
  assigned_at: string;
}

/** A parcel that was never assigned (CREATED). The key makes a retried submit safe. */
export async function assignParcel(
  token: string | null,
  input: { parcel_id: string; courier_id: string; notes: string },
  idempotencyKey: string,
): Promise<AssignmentResult> {
  const res = await request<{ data: AssignmentResult }>(token, 'POST', '/api/v1/dispatch/assignments', input, idempotencyKey);
  return res.data;
}

/** A parcel whose delivery attempt failed (ATTEMPT_FAILED) goes to a courier for a new attempt. */
export async function rescheduleParcel(
  token: string | null,
  parcelId: string,
  input: { courier_id: string; notes: string },
): Promise<AssignmentResult> {
  const res = await request<{ data: AssignmentResult }>(token, 'POST', `/api/v1/dispatch/parcels/${encodeURIComponent(parcelId)}/reschedule`, input);
  return res.data;
}

/** How long a parcel has waited, for example "2 h 10 min". Display only. */
export function waitedFor(createdAt: string, now: Date = new Date()): string {
  const t = new Date(createdAt).getTime();
  if (Number.isNaN(t)) return '';
  const minutes = Math.max(0, Math.floor((now.getTime() - t) / 60_000));
  if (minutes < 1) return 'Just now';
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `${hours} h ${minutes % 60} min`;
  return `${Math.floor(hours / 24)} days`;
}
