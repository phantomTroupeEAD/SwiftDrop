import React from 'react';
import { useAuth } from '@/app/providers/AuthContext';
import { useNavigate } from 'react-router-dom';
import {
  Package,
  LogOut,
  Truck,
  TrendingUp,
  Clock,
  Shield,
  CheckCircle2,
  Users,
  Store,
  MapPin,
  Bell,
  Search,
} from 'lucide-react';

export const DashboardPage: React.FC = () => {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/auth/login');
  };

  const getRoleBadge = () => {
    switch (user?.role) {
      case 'ADMIN':
        return { label: 'Administrator', bg: 'bg-purple-500/10 text-purple-400 border-purple-500/30', icon: <Shield className="w-4 h-4 text-purple-400" /> };
      case 'DISPATCHER':
        return { label: 'Dispatcher', bg: 'bg-sky-500/10 text-sky-400 border-sky-500/30', icon: <Users className="w-4 h-4 text-sky-400" /> };
      case 'COURIER':
        return { label: 'Active Courier', bg: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30', icon: <Truck className="w-4 h-4 text-emerald-400" /> };
      case 'MERCHANT':
        return { label: 'Merchant Shipper', bg: 'bg-amber-500/10 text-amber-400 border-amber-500/30', icon: <Store className="w-4 h-4 text-amber-400" /> };
      default:
        return { label: 'Customer', bg: 'bg-blue-500/10 text-blue-400 border-blue-500/30', icon: <Package className="w-4 h-4 text-blue-400" /> };
    }
  };

  const badge = getRoleBadge();

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col">
      {/* Top Application Navigation Bar */}
      <header className="h-16 border-b border-slate-800 bg-slate-900/60 backdrop-blur-md px-6 flex items-center justify-between sticky top-0 z-30">
        <div className="flex items-center gap-6">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-blue-600 flex items-center justify-center shadow-md shadow-blue-600/30">
              <Package className="w-5 h-5 text-white" />
            </div>
            <span className="text-lg font-bold tracking-tight text-white">
              Swift<span className="text-blue-400">Drop</span>
            </span>
          </div>

          <div className="hidden md:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-950/80 border border-slate-800 text-xs text-slate-400 w-64">
            <Search className="w-3.5 h-3.5" />
            <span>Search tracking, parcels...</span>
          </div>
        </div>

        {/* User Identity & Logout Controls */}
        <div className="flex items-center gap-4">
          <button className="text-slate-400 hover:text-slate-200 transition p-1.5 rounded-lg hover:bg-slate-800">
            <Bell className="w-4 h-4" />
          </button>

          <div className="h-5 w-px bg-slate-800" />

          <div className="flex items-center gap-3">
            <div className="text-right hidden sm:block">
              <div className="text-xs font-semibold text-slate-200">{user?.display_name || user?.email}</div>
              <div className="text-[11px] text-slate-400 font-mono">{user?.email}</div>
            </div>

            <div className="w-9 h-9 rounded-full bg-gradient-to-tr from-blue-600 to-indigo-600 flex items-center justify-center font-bold text-sm text-white shadow-md">
              {(user?.display_name || user?.email || 'U').charAt(0).toUpperCase()}
            </div>
          </div>

          <button
            onClick={handleLogout}
            title="Sign Out"
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-slate-400 hover:text-red-400 hover:bg-red-950/30 border border-slate-800 hover:border-red-900/40 transition"
          >
            <LogOut className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Sign Out</span>
          </button>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto p-6 sm:p-8 space-y-6">
        {/* Welcome Banner */}
        <div className="p-6 rounded-2xl bg-gradient-to-r from-slate-900 via-blue-950/40 to-slate-900 border border-slate-800 flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-bold text-white tracking-tight">
                Welcome back, {user?.display_name || 'Operator'}!
              </h1>
              <span className={`inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-0.5 rounded-full border ${badge.bg}`}>
                {badge.icon}
                {badge.label}
              </span>
            </div>
            <p className="text-xs text-slate-400">
              SwiftDrop Logistics Management Console • Authenticated Session Active
            </p>
          </div>

          {user?.tenant_id && (
            <div className="text-xs bg-slate-950/80 border border-slate-800 rounded-xl px-4 py-2.5">
              <span className="text-slate-500 block text-[10px] uppercase font-semibold">Merchant Tenant ID</span>
              <span className="font-mono text-amber-400">{user.tenant_id}</span>
            </div>
          )}
        </div>

        {/* Operational Stat Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="p-5 rounded-xl bg-slate-900/70 border border-slate-800 flex items-center justify-between">
            <div>
              <div className="text-xs text-slate-400 font-medium">Active Shipments</div>
              <div className="text-2xl font-bold text-white mt-1">128</div>
              <div className="text-[11px] text-emerald-400 flex items-center gap-1 mt-1">
                <TrendingUp className="w-3 h-3" /> +14% vs yesterday
              </div>
            </div>
            <div className="p-3 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-400">
              <Package className="w-5 h-5" />
            </div>
          </div>

          <div className="p-5 rounded-xl bg-slate-900/70 border border-slate-800 flex items-center justify-between">
            <div>
              <div className="text-xs text-slate-400 font-medium">In Transit</div>
              <div className="text-2xl font-bold text-white mt-1">42</div>
              <div className="text-[11px] text-slate-400 flex items-center gap-1 mt-1">
                <Clock className="w-3 h-3" /> On scheduled routes
              </div>
            </div>
            <div className="p-3 rounded-xl bg-sky-500/10 border border-sky-500/20 text-sky-400">
              <Truck className="w-5 h-5" />
            </div>
          </div>

          <div className="p-5 rounded-xl bg-slate-900/70 border border-slate-800 flex items-center justify-between">
            <div>
              <div className="text-xs text-slate-400 font-medium">Delivered Today</div>
              <div className="text-2xl font-bold text-white mt-1">86</div>
              <div className="text-[11px] text-emerald-400 flex items-center gap-1 mt-1">
                <CheckCircle2 className="w-3 h-3" /> 98.4% success SLA
              </div>
            </div>
            <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
              <CheckCircle2 className="w-5 h-5" />
            </div>
          </div>

          <div className="p-5 rounded-xl bg-slate-900/70 border border-slate-800 flex items-center justify-between">
            <div>
              <div className="text-xs text-slate-400 font-medium">Service Zones</div>
              <div className="text-2xl font-bold text-white mt-1">14</div>
              <div className="text-[11px] text-slate-400 flex items-center gap-1 mt-1">
                <MapPin className="w-3 h-3" /> Colombo & Western Province
              </div>
            </div>
            <div className="p-3 rounded-xl bg-purple-500/10 border border-purple-500/20 text-purple-400">
              <MapPin className="w-5 h-5" />
            </div>
          </div>
        </div>

        {/* Informational Card for Teammates */}
        <div className="p-6 rounded-2xl bg-slate-900/50 border border-slate-800 space-y-3">
          <h2 className="text-sm font-semibold text-slate-200">
            Authentication Integration Status (Member 1)
          </h2>
          <p className="text-xs text-slate-400 leading-relaxed">
            You are successfully authenticated. Your JWT session claims are held in memory.
            Other module owners (Members 2, 3, 4, 6, 7) can mount their domain modules directly onto their respective subroutes:
          </p>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-3 pt-2 text-xs">
            <div className="p-3 rounded-lg bg-slate-950 border border-slate-800">
              <div className="font-semibold text-blue-400">Member 6 (Customer & Merchant)</div>
              <div className="text-slate-400 text-[11px] mt-1">Mounts at <code>/portal/*</code> (Parcel booking wizard, tracking timeline).</div>
            </div>
            <div className="p-3 rounded-lg bg-slate-950 border border-slate-800">
              <div className="font-semibold text-sky-400">Member 7 (Ops & Dispatch)</div>
              <div className="text-slate-400 text-[11px] mt-1">Mounts at <code>/ops/*</code> (Interactive dispatch board, courier fleet).</div>
            </div>
            <div className="p-3 rounded-lg bg-slate-950 border border-slate-800">
              <div className="font-semibold text-emerald-400">Member 3 & 7 (Courier Tasks)</div>
              <div className="text-slate-400 text-[11px] mt-1">Mounts at <code>/courier/*</code> (Mobile task list & proof of delivery).</div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
};
