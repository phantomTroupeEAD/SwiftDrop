// Admin pricing rules (Member 2 owns the backend, Member 7 the screen).
//
// What the server has today (core-api/internal/pricing, GET only):
//   GET    /api/v1/admin/pricing-rules?page&per_page&origin_zone_id&destination_zone_id&is_active   ADMIN
// What it does not have yet, so the screen cannot change prices on live data:
//   POST   /api/v1/admin/pricing-rules                       (Idempotency-Key) -> 201 rule
//   PUT    /api/v1/admin/pricing-rules/{id}                  { expected_version, base_fee_cents, per_kg_rate_cents, effective_from, effective_until } -> new version
//   DELETE /api/v1/admin/pricing-rules/{id}?expected_version -> the rule, now inactive, new version
//
// The list is read from the real endpoint. Writes are never faked on live data: before showing Add,
// Change and Deactivate the screen checks that the server has those routes (see writesAvailable), so
// the buttons appear on their own when Member 2 ships the endpoints in this shape. If the server has
// no pricing endpoint at all (404 or 405 on the list), the screen switches to an in-memory SAMPLE
// API and says so.
//
// Money is integer LKR cents. The screen never calculates a price; quotes come from
// POST /api/v1/pricing/quotes.

import { ApiError, request } from '../api';
import type { PageMeta } from '../admin-console/consoleApi';
import { mockPricing } from './mockPricingApi';

export interface PricingRule {
  id: string;
  version: number;
  origin_zone_id: string;
  destination_zone_id: string;
  base_fee_cents: number;
  per_kg_rate_cents: number;
  effective_from: string;
  effective_until: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  /** Sent by the live list; not used by this screen yet. */
  base_weight_kg?: number;
  max_weight_kg?: number;
  currency?: string;
}

export interface RuleInput {
  origin_zone_id: string;
  destination_zone_id: string;
  base_fee_cents: number;
  per_kg_rate_cents: number;
  effective_from: string;
  effective_until: string | null;
}

export type RuleChange = Omit<RuleInput, 'origin_zone_id' | 'destination_zone_id'>;

export interface PricingPage {
  rows: PricingRule[];
  meta: PageMeta;
}

export interface PricingFilter {
  origin?: string;
  destination?: string;
  /** 'true', 'false' or undefined for both. */
  active?: string;
}

export type Backend = 'real' | 'sample';

let backend: Backend | null = null;

/** Which API the screen is using right now. null until the first list call. */
export function currentBackend(): Backend | null {
  return backend;
}

/** The route does not exist on the server (an unknown path answers 404 or 405 without the error envelope). */
function routeMissing(err: unknown): boolean {
  return err instanceof ApiError && (err.status === 404 || err.status === 405) && err.code === 'HTTP_ERROR';
}

const BASE = '/api/v1/admin/pricing-rules';

export async function listRules(token: string | null, page: number, perPage: number, filter: PricingFilter = {}): Promise<PricingPage> {
  if (backend !== 'sample') {
    try {
      const q = new URLSearchParams({ page: String(page), per_page: String(perPage) });
      if (filter.origin) q.set('origin_zone_id', filter.origin);
      if (filter.destination) q.set('destination_zone_id', filter.destination);
      if (filter.active) q.set('is_active', filter.active);
      const res = await request<{ data: PricingRule[] | null; meta: PageMeta }>(token, 'GET', `${BASE}?${q.toString()}`);
      backend = 'real';
      return { rows: res.data ?? [], meta: res.meta };
    } catch (err) {
      if (!routeMissing(err)) throw err;
      backend = 'sample';
    }
  }
  return mockPricing.list({ page, perPage, ...filter });
}

let writesProbe: Promise<boolean> | null = null;

/** A write request without a token. The server refuses it before doing anything, so it cannot change data. */
async function routeExists(method: 'POST' | 'PUT' | 'DELETE', url: string): Promise<boolean> {
  try {
    const res = await fetch(url, { method, headers: { 'Content-Type': 'application/json' }, body: method === 'DELETE' ? undefined : '{}' });
    // The router answers 405 (the path exists for another method) or 404 when the route is missing.
    // A route that exists answers 401 (no token) or another auth or validation status.
    return res.status !== 404 && res.status !== 405;
  } catch {
    return false;
  }
}

/**
 * Whether rules can be added, changed or deactivated right now. The sample API always can. On live
 * data the server must have the POST, PUT and DELETE routes. This checks once with three requests that
 * carry no token, so the server rejects them before touching anything. Any doubt means "no", so the
 * screen never offers a button that cannot work. OPTIONS is not used: some dev servers answer it
 * themselves and would claim every method is allowed.
 */
export function writesAvailable(): Promise<boolean> {
  if (backend === 'sample') return Promise.resolve(true);
  writesProbe ??= Promise.all([routeExists('POST', BASE), routeExists('PUT', `${BASE}/probe`), routeExists('DELETE', `${BASE}/probe?expected_version=1`)]).then((all) => all.every(Boolean));
  return writesProbe;
}

/** Forget the probe, for example after the user presses Refresh. */
export function resetWritesProbe(): void {
  writesProbe = null;
}

function notAvailable(): ApiError {
  return new ApiError(405, 'NOT_AVAILABLE', "The server can't change pricing rules yet. Ask Member 2 to add the create, change and deactivate endpoints.");
}

export async function createRule(token: string | null, input: RuleInput, idempotencyKey: string): Promise<PricingRule> {
  if (backend === 'sample') return mockPricing.create(input);
  try {
    const res = await request<{ data: PricingRule }>(token, 'POST', BASE, input, idempotencyKey);
    return res.data;
  } catch (err) {
    if (routeMissing(err)) throw notAvailable();
    throw err;
  }
}

export async function updateRule(token: string | null, id: string, expectedVersion: number, change: RuleChange): Promise<PricingRule> {
  if (backend === 'sample') return mockPricing.update(id, expectedVersion, change);
  try {
    const res = await request<{ data: PricingRule }>(token, 'PUT', `${BASE}/${encodeURIComponent(id)}`, { expected_version: expectedVersion, ...change });
    return res.data;
  } catch (err) {
    if (routeMissing(err)) throw notAvailable();
    throw err;
  }
}

export async function deactivateRule(token: string | null, id: string, expectedVersion: number): Promise<PricingRule> {
  if (backend === 'sample') return mockPricing.deactivate(id, expectedVersion);
  try {
    const res = await request<{ data: PricingRule }>(token, 'DELETE', `${BASE}/${encodeURIComponent(id)}?expected_version=${expectedVersion}`);
    return res.data;
  } catch (err) {
    if (routeMissing(err)) throw notAvailable();
    throw err;
  }
}
