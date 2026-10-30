import React from 'react';
import { useAuth } from '@/app/providers/AuthContext';
import { useNavigate } from 'react-router-dom';
import {
  Package,
  LogOut,
  Shield,
  Users,
  Truck,
  Store,
  User,
  Bell,
  Search,
} from 'lucide-react';
import { Role } from '@/shared/types/auth';

interface DashboardLayoutProps {
  children: React.ReactNode;
  activeRole: Role;
  title: string;
  subtitle: string;
}

export const DashboardLayout: React.FC<DashboardLayoutProps> = ({
  children,
  activeRole,
  title,
  subtitle,
}) => {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/auth/login');
  };

  const roleConfigs: Record<
    Role,
    { path: string; label: string; badge: string; icon: React.ReactNode }
  > = {
    ADMIN: {
      path: '/admin/dashboard',
      label: 'Admin',
      badge: 'bg-purple-500/10 text-purple-400 border-purple-500/30',
      icon: <Shield className="w-3.5 h-3.5" />,
    },
    DISPATCHER: {
      path: '/dispatcher/dashboard',
      label: 'Dispatcher',
      badge: 'bg-sky-500/10 text-sky-400 border-sky-500/30',
      icon: <Users className="w-3.5 h-3.5" />,
    },
    COURIER: {
      path: '/courier/dashboard',
      label: 'Courier',
      badge: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30',
      icon: <Truck className="w-3.5 h-3.5" />,
    },
    MERCHANT: {
      path: '/merchant/dashboard',
      label: 'Merchant',
      badge: 'bg-amber-500/10 text-amber-400 border-amber-500/30',
      icon: <Store className="w-3.5 h-3.5" />,
    },
    CUSTOMER: {
      path: '/customer/dashboard',
      label: 'Customer',
      badge: 'bg-blue-500/10 text-blue-400 border-blue-500/30',
      icon: <User className="w-3.5 h-3.5" />,
    },
  };

  const currentCfg = roleConfigs[activeRole];

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col antialiased">
      {/* Top Application Header */}
      <header className="h-16 border-b border-slate-800 bg-slate-900/80 backdrop-blur-md px-4 sm:px-8 flex items-center justify-between sticky top-0 z-30">
        <div className="flex items-center gap-4 sm:gap-6">
          <div
            className="flex items-center gap-2.5 cursor-pointer"
            onClick={() => navigate(currentCfg.path)}
          >
            <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-blue-600 to-indigo-600 flex items-center justify-center shadow-lg shadow-blue-600/25">
              <Package className="w-5 h-5 text-white" />
            </div>
            <span className="text-xl font-bold tracking-tight text-white hidden sm:inline">
              Swift<span className="text-blue-400">Drop</span>
            </span>
          </div>

          <div className="h-5 w-px bg-slate-800 hidden sm:block" />

          {/* Active Role Console Tag */}
          <div className="flex items-center gap-2">
            <span
              className={`inline-flex items-center gap-1.5 text-xs font-semibold px-3 py-1 rounded-full border ${currentCfg.badge}`}
            >
              {currentCfg.icon}
              {currentCfg.label} Console
            </span>
          </div>
        </div>

        {/* Global Search & User Profile Controls */}
        <div className="flex items-center gap-3 sm:gap-5">
          <div className="hidden md:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-950/80 border border-slate-800 text-xs text-slate-400 w-56">
            <Search className="w-3.5 h-3.5 text-slate-500" />
            <span>Search operations...</span>
          </div>

          <button
            title="Notifications"
            className="p-2 text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 rounded-lg transition"
          >
            <Bell className="w-4 h-4" />
          </button>

          <div className="h-5 w-px bg-slate-800" />

          {/* Current User Info */}
          <div className="text-right hidden sm:block">
            <div className="text-xs font-semibold text-slate-200">
              {user?.display_name || user?.email || 'Authenticated User'}
            </div>
            <div className="text-[11px] text-slate-400 font-mono">
              {user?.email || 'user@swiftdrop.local'}
            </div>
          </div>

          <div className="w-9 h-9 rounded-full bg-gradient-to-tr from-blue-600 to-indigo-600 flex items-center justify-center font-bold text-xs text-white shadow-md">
            {(user?.display_name || user?.email || 'U').charAt(0).toUpperCase()}
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

      {/* Main Container */}
      <main className="flex-1 max-w-7xl w-full mx-auto p-4 sm:p-8 space-y-6">
        {/* Real App Title Banner */}
        <div className="p-5 rounded-2xl bg-gradient-to-r from-slate-900 via-slate-900/90 to-blue-950/20 border border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="space-y-1">
            <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">
              {title}
            </h1>
            <p className="text-xs text-slate-400">{subtitle}</p>
          </div>
        </div>

        {/* Page Content */}
        {children}
      </main>

      {/* Clean Professional Footer */}
      <footer className="border-t border-slate-900 py-4 px-6 text-center text-xs text-slate-500">
        © 2026 SwiftDrop Logistics Inc. All rights reserved.
      </footer>
    </div>
  );
};
