import React, { useState, useEffect, useCallback } from 'react';
import { DashboardLayout } from './DashboardLayout';
import { useAuth } from '@/app/providers/AuthContext';
import { User, Role } from '@/shared/types/auth';
import {
  Shield,
  Users,
  MapPin,
  Activity,
  UserX,
  CheckCircle2,
  Layers,
  Edit3,
  RefreshCw,
  Search,
  Filter,
  X,
  Building,
  AlertCircle,
} from 'lucide-react';

export const AdminDashboard: React.FC = () => {
  const { accessToken } = useAuth();

  const [users, setUsers] = useState<User[]>([]);
  const [totalCount, setTotalCount] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // Filters
  const [selectedRole, setSelectedRole] = useState<string>('ALL');
  const [searchQuery, setSearchQuery] = useState<string>('');

  // Role Edit Modal State
  const [editModalOpen, setEditModalOpen] = useState<boolean>(false);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [targetRole, setTargetRole] = useState<Role>('CUSTOMER');
  const [targetTenantID, setTargetTenantID] = useState<string>('');
  const [updatingRole, setUpdatingRole] = useState<boolean>(false);
  const [roleError, setRoleError] = useState<string | null>(null);

  // Fetch users from Core API
  const fetchUsers = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const url = new URL('/api/v1/admin/users', window.location.origin);
      url.searchParams.set('limit', '50');
      if (selectedRole !== 'ALL') {
        url.searchParams.set('role', selectedRole);
      }

      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      if (accessToken) {
        headers['Authorization'] = `Bearer ${accessToken}`;
      }

      const res = await fetch(url.toString(), { headers });
      if (!res.ok) {
        const errorData = await res.json().catch(() => null);
        throw new Error(errorData?.error?.message || `Core API responded with HTTP ${res.status}`);
      }

      const data = await res.json();
      const items: User[] = data?.data?.items ?? [];
      setUsers(items);
      setTotalCount(data?.data?.total ?? items.length);
    } catch (err: unknown) {
      setUsers([]);
      setTotalCount(0);
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to load users from Core API.');
      }
    } finally {
      setLoading(false);
    }
  }, [accessToken, selectedRole]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  // Handle Role Change Modal Open
  const openRoleModal = (user: User) => {
    setSelectedUser(user);
    setTargetRole(user.role);
    setTargetTenantID(user.tenant_id || '');
    setRoleError(null);
    setEditModalOpen(true);
  };

  // Submit Role Change (PUT /api/v1/admin/users/{id}/role)
  const handleRoleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedUser) return;

    if (targetRole === 'MERCHANT' && !targetTenantID.trim()) {
      setRoleError('Tenant ID is required when assigning the MERCHANT role.');
      return;
    }

    setUpdatingRole(true);
    setRoleError(null);

    try {
      const payload: { role: Role; tenant_id?: string | null } = {
        role: targetRole,
        tenant_id: targetRole === 'MERCHANT' ? targetTenantID.trim() : null,
      };

      const res = await fetch(`/api/v1/admin/users/${selectedUser.id}/role`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
        },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const errorData = await res.json().catch(() => null);
        throw new Error(errorData?.error?.message || `Failed to update role (${res.status})`);
      }

      const responseData = await res.json();
      const updatedUser: User = responseData.data;

      // Update local state
      setUsers((prev) =>
        prev.map((u) => (u.id === selectedUser.id ? { ...u, role: updatedUser.role, tenant_id: updatedUser.tenant_id } : u))
      );

      setEditModalOpen(false);
      setSuccessMessage(`Role for ${selectedUser.email} successfully updated to ${targetRole}.`);
      setTimeout(() => setSuccessMessage(null), 4000);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setRoleError(err.message);
      } else {
        setRoleError('Failed to update user role.');
      }
    } finally {
      setUpdatingRole(false);
    }
  };

  // Toggle User Status (PUT /api/v1/admin/users/{id}/status)
  const handleToggleStatus = async (user: User) => {
    const nextStatus = user.status === 'SUSPENDED' ? 'ACTIVE' : 'SUSPENDED';

    try {
      const res = await fetch(`/api/v1/admin/users/${user.id}/status`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
        },
        body: JSON.stringify({ status: nextStatus }),
      });

      if (!res.ok) {
        const errorData = await res.json().catch(() => null);
        throw new Error(errorData?.error?.message || `Failed to change status (${res.status})`);
      }

      setUsers((prev) =>
        prev.map((u) => (u.id === user.id ? { ...u, status: nextStatus } : u))
      );
      setSuccessMessage(`Account ${user.email} is now ${nextStatus}.`);
      setTimeout(() => setSuccessMessage(null), 3000);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to update account status.');
      }
      setTimeout(() => setError(null), 4000);
    }
  };

  // Filter users by search query
  const filteredUsers = users.filter((u) => {
    const q = searchQuery.toLowerCase();
    const nameMatch = (u.display_name || '').toLowerCase().includes(q);
    const emailMatch = u.email.toLowerCase().includes(q);
    const idMatch = u.id.toLowerCase().includes(q);
    return nameMatch || emailMatch || idMatch;
  });

  return (
    <DashboardLayout
      activeRole="ADMIN"
      title="Administrator Operations & Governance Console"
      subtitle="System configuration, delivery zones, user role governance, and security audit logs."
    >
      {/* Notifications */}
      {successMessage && (
        <div className="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs flex items-center gap-2.5 animate-fadeIn mb-4">
          <CheckCircle2 className="w-4 h-4 shrink-0" />
          <span>{successMessage}</span>
        </div>
      )}

      {error && (
        <div className="p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2.5 animate-fadeIn mb-4">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* 4 Stat Overview Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Registered Users</div>
            <div className="text-2xl font-bold text-white mt-1">{totalCount || users.length}</div>
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
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-white">
                User Directory & Access Control
              </h2>
              <p className="text-xs text-slate-400">
                Manage accounts, role assignments, and active user status
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => fetchUsers()}
                disabled={loading}
                title="Refresh user directory"
                className="p-1.5 rounded-lg bg-slate-800 text-slate-300 hover:text-white border border-slate-700 transition disabled:opacity-50"
              >
                <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin text-blue-400' : ''}`} />
              </button>
              <span className="text-xs bg-slate-800 text-slate-300 px-2.5 py-1 rounded-lg border border-slate-700">
                {filteredUsers.length} Users
              </span>
            </div>
          </div>

          {/* Search Bar & Role Filter Pills */}
          <div className="space-y-3 pt-1">
            <div className="relative">
              <Search className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search by name, email, or user ID..."
                className="w-full pl-9 pr-3 py-2 bg-slate-950 border border-slate-800 rounded-xl text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              />
            </div>

            <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-xs">
              <span className="text-slate-500 flex items-center gap-1 text-[11px] pr-1">
                <Filter className="w-3 h-3" /> Role:
              </span>
              {['ALL', 'CUSTOMER', 'COURIER', 'DISPATCHER', 'MERCHANT', 'ADMIN'].map((role) => (
                <button
                  key={role}
                  onClick={() => setSelectedRole(role)}
                  className={`px-2.5 py-1 rounded-lg text-[11px] font-medium transition cursor-pointer shrink-0 ${
                    selectedRole === role
                      ? 'bg-blue-600 text-white shadow-sm shadow-blue-500/25'
                      : 'bg-slate-950 text-slate-400 hover:text-white border border-slate-800'
                  }`}
                >
                  {role === 'ALL' ? 'All Roles' : role}
                </button>
              ))}
            </div>
          </div>

          {/* User Table */}
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 uppercase text-[10px] tracking-wider border-b border-slate-800">
                <tr>
                  <th className="py-2.5 px-3">User & Email</th>
                  <th className="py-2.5 px-3">Role</th>
                  <th className="py-2.5 px-3">Status</th>
                  <th className="py-2.5 px-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {filteredUsers.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="py-8 text-center text-slate-500 text-xs">
                      No users match the selected criteria.
                    </td>
                  </tr>
                ) : (
                  filteredUsers.map((u) => (
                    <tr key={u.id} className="hover:bg-slate-800/30 transition">
                      <td className="py-3 px-3">
                        <div className="font-medium text-slate-200">
                          {u.display_name || (u.first_name ? `${u.first_name} ${u.last_name || ''}` : 'User')}
                        </div>
                        <div className="text-[11px] text-slate-400 font-mono">{u.email}</div>
                        {u.tenant_id && (
                          <div className="text-[10px] text-amber-400/90 font-mono flex items-center gap-1 mt-0.5">
                            <Building className="w-3 h-3" /> {u.tenant_id}
                          </div>
                        )}
                      </td>
                      <td className="py-3 px-3">
                        <span
                          className={`px-2 py-0.5 rounded text-[10px] font-bold border ${
                            u.role === 'ADMIN'
                              ? 'bg-purple-950/40 text-purple-300 border-purple-800/50'
                              : u.role === 'DISPATCHER'
                              ? 'bg-blue-950/40 text-blue-300 border-blue-800/50'
                              : u.role === 'COURIER'
                              ? 'bg-emerald-950/40 text-emerald-300 border-emerald-800/50'
                              : u.role === 'MERCHANT'
                              ? 'bg-amber-950/40 text-amber-300 border-amber-800/50'
                              : 'bg-slate-800 text-slate-300 border-slate-700'
                          }`}
                        >
                          {u.role}
                        </span>
                      </td>
                      <td className="py-3 px-3">
                        {u.status !== 'SUSPENDED' ? (
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
                        <div className="flex items-center justify-end gap-1.5">
                          <button
                            onClick={() => openRoleModal(u)}
                            className="px-2 py-1 rounded-md text-[11px] font-medium text-slate-300 bg-slate-800 hover:bg-slate-700 border border-slate-700 transition flex items-center gap-1 cursor-pointer"
                            title="Promote or Change Role"
                          >
                            <Edit3 className="w-3 h-3 text-blue-400" />
                            Role
                          </button>
                          <button
                            onClick={() => handleToggleStatus(u)}
                            className={`px-2.5 py-1 rounded-md text-[11px] font-medium transition border cursor-pointer ${
                              u.status !== 'SUSPENDED'
                                ? 'text-rose-400 bg-rose-950/30 border-rose-800/50 hover:bg-rose-900/40'
                                : 'text-emerald-400 bg-emerald-950/30 border-emerald-800/50 hover:bg-emerald-900/40'
                            }`}
                          >
                            {u.status !== 'SUSPENDED' ? 'Suspend' : 'Reactivate'}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))
                )}
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

          <button className="w-full py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-xl text-xs font-semibold border border-slate-700 transition flex items-center justify-center gap-1.5 cursor-pointer">
            <Layers className="w-3.5 h-3.5" />
            Configure New Polygon Zone
          </button>
        </div>
      </div>

      {/* ── Role Reassignment & Promotion Modal ── */}
      {editModalOpen && selectedUser && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-fadeIn">
          <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-2xl space-y-4 relative">
            <button
              onClick={() => setEditModalOpen(false)}
              className="absolute top-4 right-4 text-slate-400 hover:text-white transition cursor-pointer"
            >
              <X className="w-5 h-5" />
            </button>

            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center">
                <Edit3 className="w-5 h-5 text-blue-400" />
              </div>
              <div>
                <h3 className="text-base font-bold text-white">Assign User Role</h3>
                <p className="text-slate-400 text-xs">Governance control: PUT /api/v1/admin/users/{'{id}'}/role</p>
              </div>
            </div>

            <div className="p-3 rounded-xl bg-slate-950 border border-slate-800 text-xs space-y-1">
              <div className="text-slate-400">Target User:</div>
              <div className="font-semibold text-white">{selectedUser.display_name || selectedUser.email}</div>
              <div className="text-slate-500 font-mono text-[11px]">{selectedUser.email}</div>
              <div className="text-slate-500 font-mono text-[10px]">ID: {selectedUser.id}</div>
            </div>

            {roleError && (
              <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{roleError}</span>
              </div>
            )}

            <form onSubmit={handleRoleSubmit} className="space-y-4 pt-1">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Select Target Role
                </label>
                <select
                  value={targetRole}
                  onChange={(e) => setTargetRole(e.target.value as Role)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-xl text-xs text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
                >
                  <option value="CUSTOMER">CUSTOMER — Consumer parcel booking</option>
                  <option value="COURIER">COURIER — Mobile field delivery execution</option>
                  <option value="DISPATCHER">DISPATCHER — Operations and routing</option>
                  <option value="MERCHANT">MERCHANT — Commercial shipper</option>
                  <option value="ADMIN">ADMIN — Full system governance</option>
                </select>
              </div>

              {/* Conditional Tenant ID for MERCHANT role */}
              {targetRole === 'MERCHANT' && (
                <div className="animate-fadeIn">
                  <label className="block text-xs font-medium text-amber-300 mb-1.5">
                    Merchant Tenant ID (Required)
                  </label>
                  <input
                    type="text"
                    required
                    value={targetTenantID}
                    onChange={(e) => setTargetTenantID(e.target.value)}
                    placeholder="e.g. mrc_01HMERCHANTTENANT00001"
                    className="w-full px-3 py-2 bg-slate-950 border border-amber-800/60 rounded-xl text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-1 focus:ring-amber-500 font-mono"
                  />
                  <p className="text-[11px] text-slate-500 mt-1">
                    Merchants require a tenant identifier to partition commercial parcel orders.
                  </p>
                </div>
              )}

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setEditModalOpen(false)}
                  className="flex-1 py-2 px-3 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-xl text-xs font-semibold transition cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={updatingRole}
                  className="flex-1 py-2 px-3 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-semibold transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
                >
                  {updatingRole ? (
                    <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                  ) : (
                    'Save Role Change'
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </DashboardLayout>
  );
};
