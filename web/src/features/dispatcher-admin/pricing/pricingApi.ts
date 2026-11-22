// Admin pricing rules (Member 2 owns the backend, Member 7 the screen).
//
// core-api/internal/pricing/API.md section 2 lists these endpoints as candidates that are not
// implemented and gives no request or response formats. This file defines the contract the screen
// expects (below, and in web/README.md for Member 2) and tries the real endpoints first. If the
// server answers 404 or 405 (the route does not exist), it switches to an in-memory SAMPLE API for
// the rest of the session and the screen says so. When Member 2 ships the endpoints in this
// shape, the screen uses them with no change.
//
//   GET    /api/v1/admin/pricing-rules?page&per_page&origin_zone_id&destination_zone_id&is_active
//   POST   /api/v1/admin/pricing-rules                       (Idempotency-Key) -> 201 rule
//   PUT    /api/v1/admin/pricing-rules/{id}                  { expected_version, base_fee_cents, per_kg_rate_cents, effective_from, effective_until } -> new version
//   DELETE /api/v1/admin/pricing-rules/{id}?expected_version -> the rule, now inactive, new version
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

export async function createRule(token: string | null, input: RuleInput, idempotencyKey: string): Promise<PricingRule> {
  if (backend === 'sample') return mockPricing.create(input);
  try {
    const res = await request<{ data: PricingRule }>(token, 'POST', BASE, input, idempotencyKey);
    return res.data;
  } catch (err) {
    if (routeMissing(err)) return mockPricing.create(input);
    throw err;
  }
}

export async function updateRule(token: string | null, id: string, expectedVersion: number, change: RuleChange): Promise<PricingRule> {
  if (backend === 'sample') return mockPricing.update(id, expectedVersion, change);
  try {
    const res = await request<{ data: PricingRule }>(token, 'PUT', `${BASE}/${encodeURIComponent(id)}`, { expected_version: expectedVersion, ...change });
    return res.data;
  } catch (err) {
    if (routeMissing(err)) return mockPricing.update(id, expectedVersion, change);
    throw err;
  }
}

export async function deactivateRule(token: string | null, id: string, expectedVersion: number): Promise<PricingRule> {
  if (backend === 'sample') return mockPricing.deactivate(id, expectedVersion);
  try {
    const res = await request<{ data: PricingRule }>(token, 'DELETE', `${BASE}/${encodeURIComponent(id)}?expected_version=${expectedVersion}`);
    return res.data;
  } catch (err) {
    if (routeMissing(err)) return mockPricing.deactivate(id, expectedVersion);
    throw err;
  }
}
