import {
  BarChart3,
  ClipboardList,
  History,
  MapPin,
  PackageX,
  Receipt,
  SlidersHorizontal,
  Store,
  Truck,
  Users,
  type LucideIcon,
} from 'lucide-react';
import type { Role } from '@/shared/types/auth';

// The ops workspace menu (Member 7). Each entry lists the roles that may open it, and the menu
// shows only the entries for the signed-in role. This is a convenience for the user: the backend
// still enforces every permission on every request.

export type OpsRole = 'ADMIN' | 'DISPATCHER';
export type OpsGroup = 'Operations' | 'Administration';

export interface OpsNavItem {
  id: string;
  path: string;
  label: string;
  title: string;
  description: string;
  icon: LucideIcon;
  group: OpsGroup;
  roles: OpsRole[];
  /** False while the screen is a placeholder that says it is not built yet. */
  built: boolean;
}

const BOTH: OpsRole[] = ['ADMIN', 'DISPATCHER'];
const ADMIN_ONLY: OpsRole[] = ['ADMIN'];

export const OPS_NAV: OpsNavItem[] = [
  {
    id: 'dispatch',
    path: '/ops/dispatch',
    label: 'Dispatch board',
    title: 'Dispatch board',
    description: 'Pick a zone, see waiting parcels and couriers, assign or reassign.',
    icon: ClipboardList,
    group: 'Operations',
    roles: BOTH,
    built: true,
  },
  {
    id: 'couriers',
    path: '/ops/couriers',
    label: 'Fleet',
    title: 'Fleet',
    description: 'Couriers, shifts and capacity, and adding a courier.',
    icon: Truck,
    group: 'Operations',
    roles: BOTH,
    built: true,
  },
  {
    id: 'failed-deliveries',
    path: '/ops/failed-deliveries',
    label: 'Failed deliveries',
    title: 'Failed deliveries',
    description: 'Failed attempts, and scheduling a retry.',
    icon: PackageX,
    group: 'Operations',
    roles: BOTH,
    built: true,
  },
  {
    id: 'merchants',
    path: '/ops/merchants',
    label: 'Merchants',
    title: 'Merchants',
    description: 'Merchant applications: approve, reject, suspend, reinstate or disable.',
    icon: Store,
    group: 'Operations',
    roles: ADMIN_ONLY,
    built: false,
  },
  {
    id: 'reports',
    path: '/ops/reports',
    label: 'Reports',
    title: 'Reports',
    description: 'Deliveries per day and success rate for a date range.',
    icon: BarChart3,
    group: 'Operations',
    roles: BOTH,
    built: true,
  },
  {
    id: 'users',
    path: '/ops/users',
    label: 'Users and roles',
    title: 'Users and roles',
    description: 'Find an account, change its role, or suspend it.',
    icon: Users,
    group: 'Administration',
    roles: ADMIN_ONLY,
    built: true,
  },
  {
    id: 'zones',
    path: '/ops/zones',
    label: 'Delivery zones',
    title: 'Delivery zones',
    description: 'Create zones, set their postal code prefixes, and switch them on or off.',
    icon: MapPin,
    group: 'Administration',
    roles: ADMIN_ONLY,
    built: true,
  },
  {
    id: 'pricing',
    path: '/ops/pricing',
    label: 'Pricing rules',
    title: 'Pricing rules',
    description: 'Delivery charges between zones.',
    icon: Receipt,
    group: 'Administration',
    roles: ADMIN_ONLY,
    built: false,
  },
  {
    id: 'settings',
    path: '/ops/settings',
    label: 'System settings',
    title: 'System settings',
    description: 'Values the platform reads at run time.',
    icon: SlidersHorizontal,
    group: 'Administration',
    roles: ADMIN_ONLY,
    built: true,
  },
  {
    id: 'audit',
    path: '/ops/audit',
    label: 'Audit log',
    title: 'Audit log',
    description: 'Every recorded change. Read only.',
    icon: History,
    group: 'Administration',
    roles: ADMIN_ONLY,
    built: true,
  },
];

export function isOpsRole(role: Role | null | undefined): role is OpsRole {
  return role === 'ADMIN' || role === 'DISPATCHER';
}

/** The menu entries this role may open, in menu order. */
export function navFor(role: OpsRole): OpsNavItem[] {
  return OPS_NAV.filter((item) => item.roles.includes(role));
}

/** The entry a path belongs to, for example /ops/merchants/abc belongs to Merchants. */
export function itemForPath(pathname: string): OpsNavItem | undefined {
  return OPS_NAV.find((item) => pathname === item.path || pathname.startsWith(`${item.path}/`));
}

/** Where /ops sends each role first. */
export function homePathFor(role: OpsRole): string {
  return role === 'ADMIN' ? '/ops/zones' : '/ops/dispatch';
}
