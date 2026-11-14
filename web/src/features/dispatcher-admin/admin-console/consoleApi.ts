// Admin console calls other than zones: user directory (Member 1's endpoints), system
// settings and the audit log (admin module, core-api/internal/admin/API.md sections 3 and 4).

import { request } from '../api';
import type { Role, User, UserStatus } from '@/shared/types/auth';

export interface UserList {
  users: User[];
  total: number;
}

export async function listUsers(token: string | null, role: Role | undefined, limit = 50): Promise<UserList> {
  const q = new URLSearchParams({ limit: String(limit) });
  if (role) q.set('role', role);
  const res = await request<{ data: { items?: User[]; total?: number } }>(token, 'GET', `/api/v1/admin/users?${q.toString()}`);
  const users = res.data.items ?? [];
  return { users, total: res.data.total ?? users.length };
}

export async function changeUserRole(token: string | null, id: string, role: Role, tenantId: string | null): Promise<User> {
  const res = await request<{ data: User }>(token, 'PUT', `/api/v1/admin/users/${encodeURIComponent(id)}/role`, {
    role,
    tenant_id: role === 'MERCHANT' ? tenantId : null,
  });
  return res.data;
}

export async function setUserStatus(token: string | null, id: string, status: UserStatus): Promise<void> {
  await request<unknown>(token, 'PUT', `/api/v1/admin/users/${encodeURIComponent(id)}/status`, { status });
}

// ── System settings ───────────────────────────────────────────

export interface Setting {
  key: string;
  value: number | string | boolean | null;
  value_type: string;
  description: string;
  version: number;
}

export async function listSettings(token: string | null): Promise<Setting[]> {
  const res = await request<{ data: Setting[] }>(token, 'GET', '/api/v1/admin/settings');
  return res.data;
}

export interface SettingUpdate {
  version: number;
  value: number | string;
  reason: string;
}

export async function updateSetting(token: string | null, key: string, input: SettingUpdate): Promise<Setting> {
  const res = await request<{ data: Setting }>(token, 'PUT', `/api/v1/admin/settings/${encodeURIComponent(key)}`, input);
  return res.data;
}

// ── Audit log ─────────────────────────────────────────────────

export interface AuditRow {
  id: string;
  actor_id: string | null;
  actor_role: string;
  action: string;
  target_type: string;
  target_id: string;
  /** Snapshots are redacted by the server: secret-looking values arrive as "[REDACTED]". */
  previous_state: unknown;
  new_state: unknown;
  reason: string | null;
  request_id: string | null;
  created_at: string;
}

export interface PageMeta {
  page: number;
  per_page: number;
  total_items: number;
  total_pages: number;
}

export interface AuditPage {
  rows: AuditRow[];
  meta: PageMeta;
}

/** Audit filters. A field that is empty or missing does not filter. Dates are Sri Lanka days, YYYY-MM-DD. */
export interface AuditFilter {
  action?: string;
  target_type?: string;
  target_id?: string;
  actor_role?: string;
  from?: string;
  to?: string;
}

/** Newest first. The API pages with page and per_page (api-conventions 2.2) and filters in SQL. */
export async function listAuditPage(token: string | null, page: number, perPage: number, filter: AuditFilter = {}): Promise<AuditPage> {
  const q = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  for (const [key, value] of Object.entries(filter)) {
    if (value) q.set(key, value);
  }
  const res = await request<{ data: AuditRow[]; meta: PageMeta }>(token, 'GET', `/api/v1/admin/audit-logs?${q.toString()}`);
  return { rows: res.data, meta: res.meta };
}

/** Total number of audit entries, for the console's summary card. */
export async function auditTotal(token: string | null): Promise<number> {
  const res = await listAuditPage(token, 1, 1);
  return res.meta.total_items;
}
