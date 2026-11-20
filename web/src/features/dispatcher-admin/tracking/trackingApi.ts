// Authenticated Tracking calls (services/tracking/API.md section 3):
// Look up parcel tracking details and milestone history for operational oversight.
// Role: ADMIN and DISPATCHER.

import { request } from '../api';

export interface TrackingMilestone {
  status: string;
  description: string;
  actor_role?: string;
  reason_code?: string;
  timestamp: string;
}

export interface AuthenticatedDestination {
  city: string;
  postal_code: string;
  zone_id: string;
}

export interface AuthenticatedTracking {
  tracking_number: string;
  parcel_id: string;
  status: string;
  status_description: string;
  destination: AuthenticatedDestination;
  milestones: TrackingMilestone[];
}

export const TRACKING_NUMBER_PATTERN = /^SD-\d{8}-[A-Za-z0-9]{4,10}$/;

export function isValidTrackingNumber(tn: string): boolean {
  return TRACKING_NUMBER_PATTERN.test(tn.trim().toUpperCase());
}

export async function getParcelTracking(
  token: string | null,
  trackingNumber: string,
): Promise<AuthenticatedTracking> {
  const clean = trackingNumber.trim().toUpperCase();
  const res = await request<{ data: AuthenticatedTracking }>(
    token,
    'GET',
    `/api/v1/tracking/${encodeURIComponent(clean)}`,
  );
  return res.data;
}
