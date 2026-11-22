// SAMPLE DATA. An in-memory stand-in for the admin pricing rules API, used only while Member 2's
// endpoints do not exist (core-api/internal/pricing/API.md section 2 lists them as candidates).
// It follows the same rules the pricing domain has (zone ids, non-negative integer cents, an end
// date after the start, a new version for every change) so the screen behaves the same later.
// Nothing here is saved: a page reload resets it. It never calculates a price.

import { ApiError } from '../api';
import type { PricingPage, PricingRule, RuleInput } from './pricingApi';

const ZONE = /^ZONE_[A-Z0-9_]{2,27}$/;
const T = (iso: string) => iso;

let nextId = 7;
let rules: PricingRule[] = [
  seed('PR-0001', 'ZONE_COLOMBO_01', 'ZONE_COLOMBO_02', 35000, 8000, '2026-01-01T00:00:00+05:30', null, true, 2),
  seed('PR-0002', 'ZONE_COLOMBO_02', 'ZONE_COLOMBO_01', 35000, 8000, '2026-01-01T00:00:00+05:30', null, true, 1),
  seed('PR-0003', 'ZONE_COLOMBO_01', 'ZONE_KANDY_01', 60000, 12000, '2026-02-01T00:00:00+05:30', null, true, 1),
  seed('PR-0004', 'ZONE_COLOMBO_01', 'ZONE_GALLE_01', 55000, 11000, '2026-02-01T00:00:00+05:30', '2026-12-31T00:00:00+05:30', true, 3),
  seed('PR-0005', 'ZONE_COLOMBO_03', 'ZONE_NEGOMBO_01', 40000, 9000, '2026-03-01T00:00:00+05:30', null, true, 1),
  seed('PR-0006', 'ZONE_KANDY_01', 'ZONE_GALLE_01', 75000, 15000, '2025-06-01T00:00:00+05:30', '2025-12-31T00:00:00+05:30', false, 4),
];

function seed(id: string, o: string, d: string, base: number, perKg: number, from: string, until: string | null, active: boolean, version: number): PricingRule {
  return {
    id,
    version,
    origin_zone_id: o,
    destination_zone_id: d,
    base_fee_cents: base,
    per_kg_rate_cents: perKg,
    effective_from: T(from),
    effective_until: until,
    is_active: active,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  };
}

const wait = (ms = 220) => new Promise((r) => setTimeout(r, ms));
const fail = (status: number, code: string, message: string) => new ApiError(status, code, message, `sample-${Math.random().toString(16).slice(2, 10)}`);

function validate(input: RuleInput) {
  const details: { field: string; issue: string }[] = [];
  if (!ZONE.test(input.origin_zone_id)) details.push({ field: 'origin_zone_id', issue: 'Choose a delivery zone.' });
  if (!ZONE.test(input.destination_zone_id)) details.push({ field: 'destination_zone_id', issue: 'Choose a delivery zone.' });
  if (!Number.isInteger(input.base_fee_cents) || input.base_fee_cents < 0) details.push({ field: 'base_fee_cents', issue: 'Must be zero or more.' });
  if (!Number.isInteger(input.per_kg_rate_cents) || input.per_kg_rate_cents < 0) details.push({ field: 'per_kg_rate_cents', issue: 'Must be zero or more.' });
  if (Number.isNaN(Date.parse(input.effective_from))) details.push({ field: 'effective_from', issue: 'Choose a start date.' });
  if (input.effective_until && Date.parse(input.effective_until) <= Date.parse(input.effective_from)) details.push({ field: 'effective_until', issue: 'Must be after the start date.' });
  if (details.length > 0) throw new ApiError(422, 'VALIDATION_FAILED', 'Some fields are not valid', 'sample-validation', details);
}

function overlaps(a: { from: number; until: number }, b: { from: number; until: number }): boolean {
  return a.from < b.until && b.from < a.until;
}

function windowOf(r: { effective_from: string; effective_until: string | null }) {
  return { from: Date.parse(r.effective_from), until: r.effective_until ? Date.parse(r.effective_until) : Number.POSITIVE_INFINITY };
}

function assertNoOverlap(input: RuleInput, ignoreId?: string) {
  const w = windowOf(input);
  const clash = rules.find((r) => r.is_active && r.id !== ignoreId && r.origin_zone_id === input.origin_zone_id && r.destination_zone_id === input.destination_zone_id && overlaps(w, windowOf(r)));
  if (clash) throw fail(409, 'PRICING_RULE_OVERLAP', `Another active rule (${clash.id}) already covers this route in that period.`);
}

export interface ListParams {
  page: number;
  perPage: number;
  origin?: string;
  destination?: string;
  /** 'true', 'false' or undefined for both. */
  active?: string;
}

export const mockPricing = {
  async list(p: ListParams): Promise<PricingPage> {
    await wait();
    const rows = rules
      .filter((r) => (!p.origin || r.origin_zone_id === p.origin) && (!p.destination || r.destination_zone_id === p.destination) && (!p.active || String(r.is_active) === p.active))
      .sort((a, b) => a.origin_zone_id.localeCompare(b.origin_zone_id) || a.destination_zone_id.localeCompare(b.destination_zone_id) || a.id.localeCompare(b.id));
    const start = (p.page - 1) * p.perPage;
    return {
      rows: rows.slice(start, start + p.perPage),
      meta: { page: p.page, per_page: p.perPage, total_items: rows.length, total_pages: Math.ceil(rows.length / p.perPage) },
    };
  },

  async create(input: RuleInput): Promise<PricingRule> {
    await wait();
    validate(input);
    assertNoOverlap(input);
    const now = new Date().toISOString();
    const rule: PricingRule = { id: `PR-${String(nextId++).padStart(4, '0')}`, version: 1, ...input, is_active: true, created_at: now, updated_at: now };
    rules = [...rules, rule];
    return rule;
  },

  async update(id: string, expectedVersion: number, input: Omit<RuleInput, 'origin_zone_id' | 'destination_zone_id'>): Promise<PricingRule> {
    await wait();
    const cur = rules.find((r) => r.id === id);
    if (!cur) throw fail(404, 'PRICING_RULE_NOT_FOUND', 'No pricing rule has this id.');
    if (cur.version !== expectedVersion) throw fail(409, 'CONCURRENT_MODIFICATION', 'Someone else changed this rule. Reload and try again.');
    if (!cur.is_active) throw fail(409, 'PRICING_RULE_INACTIVE', 'This rule is inactive and cannot be changed.');
    const next: RuleInput = { origin_zone_id: cur.origin_zone_id, destination_zone_id: cur.destination_zone_id, ...input };
    validate(next);
    assertNoOverlap(next, id);
    const updated: PricingRule = { ...cur, ...input, version: cur.version + 1, updated_at: new Date().toISOString() };
    rules = rules.map((r) => (r.id === id ? updated : r));
    return updated;
  },

  async deactivate(id: string, expectedVersion: number): Promise<PricingRule> {
    await wait();
    const cur = rules.find((r) => r.id === id);
    if (!cur) throw fail(404, 'PRICING_RULE_NOT_FOUND', 'No pricing rule has this id.');
    if (cur.version !== expectedVersion) throw fail(409, 'CONCURRENT_MODIFICATION', 'Someone else changed this rule. Reload and try again.');
    if (!cur.is_active) throw fail(409, 'PRICING_RULE_INACTIVE', 'This rule is already inactive.');
    const updated: PricingRule = { ...cur, is_active: false, version: cur.version + 1, updated_at: new Date().toISOString() };
    rules = rules.map((r) => (r.id === id ? updated : r));
    return updated;
  },

  /** Another admin changes a rule behind this one's back. Used by tests through window.__pricingSample. */
  _touch(id: string) {
    rules = rules.map((r) => (r.id === id ? { ...r, version: r.version + 1 } : r));
  },
};

if (typeof window !== 'undefined') {
  (window as unknown as { __pricingSample?: { touch: (id: string) => void } }).__pricingSample = { touch: mockPricing._touch };
}
