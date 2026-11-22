// Calls to the admin zone API (core-api/internal/admin/API.md section 2).

import { ApiError, request } from '../api';
import type { PageMeta, Zone, ZoneCreateInput, ZoneListResult, ZoneStatusInput, ZoneUpdateInput } from './types';

export { ApiError };

const BASE = '/api/v1/admin/zones';

export async function listZones(token: string | null, page: number, perPage: number): Promise<ZoneListResult> {
  const res = await request<{ data: Zone[]; meta: PageMeta }>(token, 'GET', `${BASE}?page=${page}&per_page=${perPage}`);
  return { zones: res.data, meta: res.meta };
}

/** One zone, for example to read the latest version after a 409 CONCURRENT_MODIFICATION. */
export async function getZone(token: string | null, id: string): Promise<Zone> {
  const res = await request<{ data: Zone }>(token, 'GET', `${BASE}/${encodeURIComponent(id)}`);
  return res.data;
}

/** Sends an Idempotency-Key so a repeated submit (double click, retry) cannot create twice. */
export async function createZone(token: string | null, input: ZoneCreateInput, idempotencyKey: string): Promise<Zone> {
  const res = await request<{ data: Zone }>(token, 'POST', BASE, input, idempotencyKey);
  return res.data;
}

export async function updateZone(token: string | null, id: string, input: ZoneUpdateInput): Promise<Zone> {
  const res = await request<{ data: Zone }>(token, 'PATCH', `${BASE}/${encodeURIComponent(id)}`, input);
  return res.data;
}

export async function setZoneStatus(token: string | null, id: string, input: ZoneStatusInput): Promise<Zone> {
  const res = await request<{ data: Zone }>(token, 'PATCH', `${BASE}/${encodeURIComponent(id)}/status`, input);
  return res.data;
}
