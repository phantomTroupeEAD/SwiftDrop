// Authenticated Shipments calls (core-api/internal/parcels/API.md):
// Browse, filter, and inspect all parcels for operational oversight.
// Role: ADMIN and DISPATCHER.

import { request } from '../api';

export interface ParcelMoney {
  amount: number;
  currency: string;
}

export interface ParcelWeight {
  actual_kg: number;
  volumetric_kg: number;
  chargeable_kg: number;
}

export interface ParcelDimensions {
  length_cm: number;
  width_cm: number;
  height_cm: number;
}

export interface ParcelAddress {
  contact_name: string;
  contact_phone: string;
  address_line1: string;
  address_line2?: string;
  city: string;
  state?: string;
  postal_code: string;
  zone_id: string;
}

export interface ParcelSummary {
  id: string;
  tracking_number: string;
  sender_id?: string;
  merchant_id?: string;
  recipient_name: string;
  recipient_phone: string;
  quote_id: string;
  pricing_rule_id: string;
  pricing_rule_version: number;
  delivery_fee_cents: number;
  currency: string;
  origin_zone_id: string;
  destination_zone_id: string;
  service_level: string;
  weight: ParcelWeight;
  dimensions: ParcelDimensions;
  pickup?: ParcelAddress;
  dropoff?: ParcelAddress;
  item_description?: string;
  special_instructions?: string;
  status: string;
  attempt_count: number;
  max_attempts?: number;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface ShipmentsData {
  items: ParcelSummary[];
  page: number;
  page_size: number;
  total: number;
}

export interface ShipmentsMeta {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

export interface ShipmentsListResponse {
  data: ShipmentsData;
  meta: ShipmentsMeta;
}

export interface ListShipmentsParams {
  page?: number;
  per_page?: number;
  status?: string;
  zone_id?: string;
}

export interface StatusHistoryItem {
  id: string;
  parcel_id: string;
  from_status: string | null;
  to_status: string;
  actor_id?: string | null;
  actor_type: string;
  actor_role?: string;
  reason?: string;
  occurred_at: string;
}

export interface ParcelHistoryData {
  parcel_id: string;
  tracking_number: string;
  status: string;
  history: StatusHistoryItem[];
}

export async function getParcelHistory(
  token: string | null,
  parcelId: string,
): Promise<ParcelHistoryData> {
  const res = await request<{ data: ParcelHistoryData }>(
    token,
    'GET',
    `/api/v1/parcels/${encodeURIComponent(parcelId)}/history`,
  );
  return res.data;
}

export async function listShipments(
  token: string | null,
  params: ListShipmentsParams = {},
): Promise<ShipmentsListResponse> {
  const query = new URLSearchParams();
  if (params.page && params.page > 0) {
    query.set('page', String(params.page));
  }
  if (params.per_page && params.per_page > 0) {
    query.set('per_page', String(params.per_page));
  }
  if (params.status && params.status !== 'ALL') {
    query.set('status', params.status.trim().toUpperCase());
  }
  if (params.zone_id && params.zone_id.trim() !== '') {
    query.set('zone_id', params.zone_id.trim());
  }

  const qs = query.toString();
  const path = qs ? `/api/v1/parcels?${qs}` : '/api/v1/parcels';

  return request<ShipmentsListResponse>(token, 'GET', path);
}
