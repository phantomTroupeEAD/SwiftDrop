// Shapes of the admin zone API (core-api/internal/admin/API.md section 2).
// A zone carries no money: fees belong to Member 2's pricing rules (Doc 1 R-04).

export interface Zone {
  id: string;
  code: string;
  name: string;
  description: string;
  postal_prefixes: string[];
  enabled: boolean;
  version: number;
}

export interface PageMeta {
  page: number;
  per_page: number;
  total_items: number;
  total_pages: number;
}

export interface ZoneListResult {
  zones: Zone[];
  meta: PageMeta;
}

export interface ZoneCreateInput {
  id: string;
  code: string;
  name: string;
  description?: string;
  postal_prefixes: string[];
}

export interface ZoneUpdateInput {
  version: number;
  code?: string;
  name?: string;
  description?: string;
  postal_prefixes?: string[];
  reason?: string;
}

export interface ZoneStatusInput {
  version: number;
  enabled: boolean;
  reason: string;
}

export type { FieldIssue } from '../api';
