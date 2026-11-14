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
      badge: 'bg-primary-tint text-primary border-border',
      icon: <Shield className="w-3.5 h-3.5" />,
    },
    DISPATCHER: {
      path: '/dispatcher/dashboard',
      label: 'Dispatcher',
      badge: 'bg-primary-tint text-primary border-border',
      icon: <Users className="w-3.5 h-3.5" />,
    },
    COURIER: {
      path: '/courier/dashboard',
      label: 'Courier',
      badge: 'bg-status-success-bg text-status-success-ink border-status-success-border',
      icon: <Truck className="w-3.5 h-3.5" />,
    },
    MERCHANT: {
      path: '/merchant/dashboard',
      label: 'Merchant',
      badge: 'bg-status-warning-bg text-status-warning-ink border-status-warning-border',
      icon: <Store className="w-3.5 h-3.5" />,
    },
    CUSTOMER: {
      path: '/customer/dashboard',
      label: 'Customer',
      badge: 'bg-primary-tint text-primary border-border',
      icon: <User className="w-3.5 h-3.5" />,
    },
  };

  const currentCfg = roleConfigs[activeRole];

  return (
    <div data-role={activeRole} className="min-h-screen bg-canvas text-text flex flex-col antialiased">
      {/* Top Application Header */}
      <header className="h-16 border-b border-border bg-surface backdrop-blur-md px-4 sm:px-8 flex items-center justify-between sticky top-0 z-30">
        <div className="flex items-center gap-4 sm:gap-6">
          <div
            className="flex items-center gap-2.5 cursor-pointer"
            onClick={() => navigate(currentCfg.path)}
          >
            <div className="w-9 h-9 rounded-xl bg-surface-subtle   flex items-center justify-center  ">
              <Package className="w-5 h-5 text-primary" />
            </div>
            <span className="text-xl font-bold tracking-tight text-text hidden sm:inline">
              Swift<span className="text-primary">Drop</span>
            </span>
          </div>

          <div className="h-5 w-px bg-surface-subtle hidden sm:block" />

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
          <div className="hidden md:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-canvas border border-border text-xs text-text-muted w-56">
            <Search className="w-3.5 h-3.5 text-text-muted" />
            <span>Search operations...</span>
          </div>

          <button
            title="Notifications"
            className="p-2 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition"
          >
            <Bell className="w-4 h-4" />
          </button>

          <div className="h-5 w-px bg-surface-subtle" />

          {/* Current User Info */}
          <div className="text-right hidden sm:block">
            <div className="text-xs font-semibold text-text">
              {user?.display_name || user?.email || 'Authenticated User'}
            </div>
            <div className="text-[11px] text-text-muted font-mono">
              {user?.email || 'user@swiftdrop.local'}
            </div>
          </div>

          <div className="w-9 h-9 rounded-full    flex items-center justify-center font-bold text-xs text-text ">
            {(user?.display_name || user?.email || 'U').charAt(0).toUpperCase()}
          </div>

          <button
            onClick={handleLogout}
            title="Sign Out"
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-text-muted hover:text-status-error-ink hover:bg-status-error-bg border border-border hover:border-status-error-border transition"
          >
            <LogOut className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Sign Out</span>
          </button>
        </div>
      </header>

      {/* Main Container */}
      <main className="flex-1 max-w-7xl w-full mx-auto p-4 sm:p-8 space-y-6">
        {/* Real App Title Banner */}
        <div className="p-5 rounded-2xl     border border-border flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="space-y-1">
            <h1 className="text-xl sm:text-2xl font-bold text-text tracking-tight">
              {title}
            </h1>
            <p className="text-xs text-text-muted">{subtitle}</p>
          </div>
        </div>

        {/* Page Content */}
        {children}
      </main>

      {/* Clean Professional Footer */}
      <footer className="border-t border-border py-4 px-6 text-center text-xs text-text-muted">
        © 2026 SwiftDrop Logistics Inc. All rights reserved.
      </footer>
    </div>
  );
};
