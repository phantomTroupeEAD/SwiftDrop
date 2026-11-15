// Merchant accounts for staff (core-api/internal/merchants/API.md section 5). ADMIN only: the list,
// one merchant, and the five status changes.

import { request } from '../api';
import type { PageMeta } from '../admin-console/consoleApi';

export const MERCHANT_STATUSES = ['PENDING', 'APPROVED', 'REJECTED', 'SUSPENDED', 'DISABLED'] as const;
export type MerchantStatus = (typeof MERCHANT_STATUSES)[number];

export interface MerchantAddress {
  line1?: string;
  line2?: string;
  city?: string;
  postal_code?: string;
  country?: string;
}

export interface Merchant {
  merchant_id: string;
  business_name: string;
  status: MerchantStatus | string;
  contact_email: string;
  contact_phone: string;
  address: MerchantAddress | null;
  approved_at: string | null;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface MerchantPage {
  rows: Merchant[];
  meta: PageMeta;
}

/** Newest first as the server sends them. An empty status means every status. */
export async function listMerchants(token: string | null, status: string, page: number, perPage: number): Promise<MerchantPage> {
  const q = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (status) q.set('status', status);
  const res = await request<{ data: Merchant[] | null; meta: PageMeta }>(token, 'GET', `/api/v1/admin/merchants?${q.toString()}`);
  return { rows: res.data ?? [], meta: res.meta };
}

export async function getMerchant(token: string | null, id: string): Promise<Merchant> {
  const res = await request<{ data: Merchant }>(token, 'GET', `/api/v1/admin/merchants/${encodeURIComponent(id)}`);
  return res.data;
}

export type MerchantAction = 'approve' | 'reject' | 'suspend' | 'reinstate' | 'disable';

/**
 * Moves a merchant to a new status. expected_version must be the version the admin was looking at:
 * if someone else changed the merchant meanwhile the server answers 409 MERCHANT_VERSION_CONFLICT.
 * Returns the updated merchant, with its new version.
 */
export async function changeMerchantStatus(
  token: string | null,
  id: string,
  action: MerchantAction,
  input: { expected_version: number; reason: string },
): Promise<Merchant> {
  const res = await request<{ data: Merchant }>(token, 'POST', `/api/v1/admin/merchants/${encodeURIComponent(id)}/${action}`, input);
  return res.data;
}
