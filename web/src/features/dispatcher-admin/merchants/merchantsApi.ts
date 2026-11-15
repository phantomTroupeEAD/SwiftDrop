// Merchant accounts for staff (core-api/internal/merchants/API.md section 5). ADMIN only, read only
// here: the list and one merchant. Status changes (approve, reject, suspend…) are not on this screen.

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
