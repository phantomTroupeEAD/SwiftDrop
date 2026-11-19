// Fleet calls (core-api/internal/dispatch/API.md sections 1 and 2): list couriers with their
// workload, and add a courier profile. ADMIN and DISPATCHER only.

import { request } from '../api';

export type ShiftStatus = 'ON_SHIFT' | 'OFF_SHIFT' | 'ON_BREAK' | 'SUSPENDED';
export type VehicleType = 'MOTORCYCLE' | 'VAN' | 'TRUCK';

export interface CourierLoad {
  user_id: string;
  /** Empty when the API does not send a name. */
  courier_name: string;
  employee_code: string;
  current_zone_id: string;
  /** Parcels the courier carries now, or null when the API does not send it. */
  active_count: number | null;
  max_capacity: number;
  status: ShiftStatus | string;
  vehicle_type: VehicleType | string;
}

export interface CourierCreateInput {
  user_id: string;
  employee_code: string;
  vehicle_type: VehicleType;
  /** 0 means "use the vehicle's default capacity". */
  max_capacity: number;
  current_zone_id: string;
}

export interface CourierProfile {
  user_id: string;
  employee_code: string;
  vehicle_type: string;
  max_capacity: number;
  status: string;
  current_zone_id: string;
  is_active: boolean;
  created_at: string;
}

/**
 * What GET /dispatch/couriers sends. API.md documents snake_case names with courier_name and
 * active_count. The running endpoint currently returns the domain struct instead (UserID,
 * Vehicle, MaxCapacity, Status, CurrentZoneID, no name and no workload). The screen accepts both,
 * so it works today and keeps working when the endpoint matches its documentation.
 */
type RawCourier = Record<string, unknown>;

function str(raw: RawCourier, ...keys: string[]): string {
  for (const k of keys) if (typeof raw[k] === 'string' && raw[k] !== '') return raw[k] as string;
  return '';
}

function num(raw: RawCourier, ...keys: string[]): number | null {
  for (const k of keys) if (typeof raw[k] === 'number' && Number.isFinite(raw[k])) return raw[k] as number;
  return null;
}

export function normalizeCourier(raw: RawCourier): CourierLoad {
  return {
    user_id: str(raw, 'user_id', 'courier_id', 'UserID'),
    courier_name: str(raw, 'courier_name', 'name', 'CourierName'),
    employee_code: str(raw, 'employee_code', 'EmployeeCode'),
    current_zone_id: str(raw, 'current_zone_id', 'CurrentZoneID'),
    active_count: num(raw, 'active_count', 'active_assignments', 'ActiveCount'),
    max_capacity: num(raw, 'max_capacity', 'MaxCapacity') ?? 0,
    status: str(raw, 'status', 'Status') || 'OFF_SHIFT',
    vehicle_type: str(raw, 'vehicle_type', 'Vehicle', 'VehicleType'),
  };
}

/** The API answers `{"data": null}` for an empty fleet, so that becomes an empty list. */
export async function listCouriers(token: string | null, zoneId?: string): Promise<CourierLoad[]> {
  const q = zoneId ? `?zone_id=${encodeURIComponent(zoneId)}` : '';
  const res = await request<{ data: RawCourier[] | null }>(token, 'GET', `/api/v1/dispatch/couriers${q}`);
  return (res.data ?? []).map(normalizeCourier);
}

export async function registerCourier(token: string | null, input: CourierCreateInput): Promise<CourierProfile> {
  const res = await request<{ data: CourierProfile }>(token, 'POST', '/api/v1/dispatch/couriers', input);
  return res.data;
}

export interface ZoneOption {
  id: string;
  code: string;
  name: string;
}

/** Enabled zones, public endpoint (no sign-in), the same list booking forms use. */
export async function listZoneOptions(): Promise<ZoneOption[]> {
  const res = await request<{ data: ZoneOption[] | null }>(null, 'GET', '/api/v1/zones');
  return res.data ?? [];
}
