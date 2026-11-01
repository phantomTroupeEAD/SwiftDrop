import React, { useState } from 'react';
import { DashboardLayout } from './DashboardLayout';
import {
  Shield,
  Users,
  MapPin,
  Activity,
  UserX,
  CheckCircle2,
  Layers,
} from 'lucide-react';

interface MockUser {
  id: string;
  name: string;
  email: string;
  role: string;
  status: 'ACTIVE' | 'SUSPENDED';
  created: string;
}

export const AdminDashboard: React.FC = () => {
  const [users, setUsers] = useState<MockUser[]>([
    {
      id: 'usr_01HADMIN000000000000000001',
      name: 'System Administrator',
      email: 'admin@swiftdrop.local',
      role: 'ADMIN',
      status: 'ACTIVE',
      created: '2026-10-21',
    },
    {
      id: 'usr_01HDISPATCH00000000000002',
      name: 'Central Dispatcher',
      email: 'dispatcher@swiftdrop.local',
      role: 'DISPATCHER',
      status: 'ACTIVE',
      created: '2026-10-22',
    },
    {
      id: 'usr_01HCOURIER00000000000003',
      name: 'Kamal Perera',
      email: 'courier@swiftdrop.local',
      role: 'COURIER',
      status: 'ACTIVE',
      created: '2026-10-24',
    },
    {
      id: 'usr_01HMERCHANT00000000000004',
      name: 'Colombo Tech Stores',
      email: 'merchant@vendor.com',
      role: 'MERCHANT',
      status: 'ACTIVE',
      created: '2026-10-25',
    },
    {
      id: 'usr_01HCUSTOMER00000000000005',
      name: 'Nimal Fernando',
      email: 'customer@swiftdrop.local',
      role: 'CUSTOMER',
      status: 'ACTIVE',
      created: '2026-10-26',
    },
  ]);

  const toggleStatus = (id: string) => {
    setUsers((prev) =>
      prev.map((u) =>
        u.id === id
          ? { ...u, status: u.status === 'ACTIVE' ? 'SUSPENDED' : 'ACTIVE' }
          : u
      )
    );
  };

  return (
    <DashboardLayout
      activeRole="ADMIN"
      title="Administrator Operations & Governance Console"
      subtitle="System configuration, delivery zones, user role governance, and security audit logs."
    >
      {/* 4 Stat Overview Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Registered Users</div>
            <div className="text-2xl font-bold text-white mt-1">1,428</div>
            <div className="text-[11px] text-purple-400 flex items-center gap-1 mt-1">
              <CheckCircle2 className="w-3 h-3" /> 5 Canonical Roles
            </div>
          </div>
          <div className="p-3 rounded-xl bg-purple-500/10 border border-purple-500/20 text-purple-400">
            <Users className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Active Delivery Zones</div>
            <div className="text-2xl font-bold text-white mt-1">14 Zones</div>
            <div className="text-[11px] text-slate-400 flex items-center gap-1 mt-1">
              <MapPin className="w-3 h-3" /> Western & Central Provinces
            </div>
          </div>
          <div className="p-3 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-400">
            <MapPin className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Security Audits</div>
            <div className="text-2xl font-bold text-white mt-1">3,892</div>
            <div className="text-[11px] text-emerald-400 flex items-center gap-1 mt-1">
              <Shield className="w-3 h-3" /> Zero Breaches Logged
            </div>
          </div>
          <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
            <Shield className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">System Health</div>
            <div className="text-2xl font-bold text-emerald-400 mt-1">99.98%</div>
            <div className="text-[11px] text-slate-400 flex items-center gap-1 mt-1">
              <Activity className="w-3 h-3" /> PostgreSQL + Core API
            </div>
          </div>
          <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
            <Activity className="w-5 h-5" />
          </div>
        </div>
      </div>

      {/* Main Grid: User Management & Configured Zones */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* User Governance Table (2 cols) */}
        <div className="lg:col-span-2 bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-base font-semibold text-white">
                User Directory & Access Control
              </h2>
              <p className="text-xs text-slate-400">
                Manage accounts, role assignments, and active user status
              </p>
            </div>
            <span className="text-xs bg-slate-800 text-slate-300 px-2.5 py-1 rounded-lg border border-slate-700">
              {users.length} Users
            </span>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 uppercase text-[10px] tracking-wider border-b border-slate-800">
                <tr>
                  <th className="py-2.5 px-3">User & Email</th>
                  <th className="py-2.5 px-3">Role</th>
                  <th className="py-2.5 px-3">Status</th>
                  <th className="py-2.5 px-3 text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {users.map((u) => (
                  <tr key={u.id} className="hover:bg-slate-800/30 transition">
                    <td className="py-3 px-3">
                      <div className="font-medium text-slate-200">{u.name}</div>
                      <div className="text-[11px] text-slate-400 font-mono">{u.email}</div>
                    </td>
                    <td className="py-3 px-3">
                      <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-slate-800 text-slate-300 border border-slate-700">
                        {u.role}
                      </span>
                    </td>
                    <td className="py-3 px-3">
                      {u.status === 'ACTIVE' ? (
                        <span className="inline-flex items-center gap-1 text-[11px] text-emerald-400">
                          <CheckCircle2 className="w-3 h-3" /> Active
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-[11px] text-rose-400">
                          <UserX className="w-3 h-3" /> Suspended
                        </span>
                      )}
                    </td>
                    <td className="py-3 px-3 text-right">
                      <button
                        onClick={() => toggleStatus(u.id)}
                        className={`px-2.5 py-1 rounded-md text-[11px] font-medium transition border ${
                          u.status === 'ACTIVE'
                            ? 'text-rose-400 bg-rose-950/30 border-rose-800/50 hover:bg-rose-900/40'
                            : 'text-emerald-400 bg-emerald-950/30 border-emerald-800/50 hover:bg-emerald-900/40'
                        }`}
                      >
                        {u.status === 'ACTIVE' ? 'Suspend' : 'Reactivate'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Configured Delivery Zones (1 col) */}
        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-white">Delivery Zones & Tariffs</h2>
            <span className="text-[10px] text-blue-400 bg-blue-950/60 px-2 py-0.5 rounded border border-blue-500/30">
              Active Zones
            </span>
          </div>
          <p className="text-xs text-slate-400">
            Zoned operational boundaries and baseline delivery rates
          </p>

          <div className="space-y-2.5">
            {[
              { code: 'ZONE_COLOMBO_01', name: 'Colombo Central (1-15)', base: 'LKR 350', status: 'Enabled' },
              { code: 'ZONE_COLOMBO_02', name: 'Dehiwala / Mt. Lavinia', base: 'LKR 420', status: 'Enabled' },
              { code: 'ZONE_KANDY_01', name: 'Kandy Urban Metro', base: 'LKR 550', status: 'Enabled' },
              { code: 'ZONE_NEGOMBO_01', name: 'Negombo Express', base: 'LKR 500', status: 'Enabled' },
            ].map((z) => (
              <div
                key={z.code}
                className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between text-xs"
              >
                <div>
                  <div className="font-semibold text-slate-200">{z.name}</div>
                  <div className="text-[10px] text-slate-500 font-mono">{z.code}</div>
                </div>
                <div className="text-right">
                  <div className="font-bold text-emerald-400">{z.base}</div>
                  <div className="text-[10px] text-slate-400">Base rate</div>
                </div>
              </div>
            ))}
          </div>

          <button className="w-full py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-xl text-xs font-semibold border border-slate-700 transition flex items-center justify-center gap-1.5">
            <Layers className="w-3.5 h-3.5" />
            Configure New Polygon Zone
          </button>
        </div>
      </div>
    </DashboardLayout>
  );
};
