// Delivery report (core-api/internal/admin/API.md section 5), ADMIN and DISPATCHER, read only.

import { request } from '../api';

export interface DeliveryDay {
  date: string;
  delivered: number;
  failed: number;
  returned: number;
}

export interface DeliveryReport {
  from: string;
  to: string;
  days: DeliveryDay[];
  delivered: number;
  failed: number;
  /** delivered / (delivered + failed) from 0 to 1, or null when there were no attempts. */
  success_rate: number | null;
  /** Where the numbers came from. Shows stand-in text until parcel data is connected. */
  source: string;
}

/** from and to are Sri Lanka days, YYYY-MM-DD, both included. */
export async function getDeliveryReport(token: string | null, from: string, to: string): Promise<DeliveryReport> {
  const q = new URLSearchParams({ from, to });
  const res = await request<{ data: DeliveryReport }>(token, 'GET', `/api/v1/admin/reports/deliveries?${q.toString()}`);
  return res.data;
}

const colomboDay = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Colombo', year: 'numeric', month: '2-digit', day: '2-digit' });

/** Today in Sri Lanka as YYYY-MM-DD, shifted back by `daysAgo` days. */
export function colomboDate(daysAgo = 0, now: Date = new Date()): string {
  return colomboDay.format(new Date(now.getTime() - daysAgo * 86_400_000));
}
