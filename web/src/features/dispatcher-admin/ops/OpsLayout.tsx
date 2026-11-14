import React, { useCallback, useEffect, useRef } from 'react';
import { Link, Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { LogOut, Menu, Package } from 'lucide-react';
import { useAuth } from '@/app/providers/AuthContext';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/shared/components/ui/dropdown-menu';
import { cn } from '@/shared/lib/utils';
import { UNAUTHORIZED_EVENT } from '../api';
import { useBodyRole } from '../admin-console/useBodyRole';
import { ForbiddenPage } from './OpsPages';
import { homePathFor, isOpsRole, itemForPath, navFor, type OpsGroup, type OpsNavItem } from './opsNav';
import { useSessionKeepAlive } from './useSessionKeepAlive';

// Ops workspace shell for ADMIN and DISPATCHER (route /ops/*). `data-role` applies the role's
// preset from index.css (AGENTS.md Rule 6). The menu lists only the pages this role can open.
// Sessions: the access token stays in memory (AuthContext) and is renewed through the HttpOnly
// refresh cookie before it expires; if the renewal fails the user goes back to sign-in.

const GROUPS: OpsGroup[] = ['Operations', 'Administration'];

export const OpsLayout: React.FC = () => {
  const { user, accessToken, isAuthenticated, isRestoringSession, refreshSession, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const endSession = useCallback(() => {
    void logout();
    navigate('/auth/login', { replace: true });
  }, [logout, navigate]);
  const endSessionRef = useRef(endSession);
  endSessionRef.current = endSession;

  // Dialogs and menus render in a portal on <body>: give them the role palette too.
  useBodyRole(user && isOpsRole(user.role) ? user.role : 'PUBLIC');

  // Renew the in-memory token shortly before it expires.
  useSessionKeepAlive(accessToken, refreshSession, () => endSessionRef.current());

  // A 401 from any admin call means the access token expired early: renew once, or sign out.
  useEffect(() => {
    const onUnauthorized = () => {
      void refreshSession().then((u) => {
        if (!u) endSessionRef.current();
      });
    };
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
  }, [refreshSession]);

  if (isRestoringSession) {
    return (
      <div data-role="PUBLIC" className="grid min-h-screen place-items-center bg-canvas text-sm text-text-muted">
        <p role="status">Restoring your session…</p>
      </div>
    );
  }
  if (!isAuthenticated || !user) return <Navigate to="/auth/login" replace />;
  if (!isOpsRole(user.role)) return <Navigate to="/dashboard" replace />;

  const role = user.role;
  const items = navFor(role);
  const current = itemForPath(location.pathname);
  const blocked = current !== undefined && !current.roles.includes(role);
  const isActive = (item: OpsNavItem) => location.pathname === item.path || location.pathname.startsWith(`${item.path}/`);

  return (
    <div data-role={role} className="flex min-h-screen flex-col bg-canvas text-sm text-text">
      <header className="border-b border-border bg-surface">
        <div className="flex w-full flex-wrap items-center justify-between gap-3 px-4 py-3 sm:px-8 lg:px-10">
          <div className="flex items-center gap-3">
            <div className="lg:hidden">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" variant="outline" data-testid="ops-menu-btn" aria-label="Open menu">
                    <Menu aria-hidden="true" /> Menu
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-64">
                  {items.map((item) => (
                    <DropdownMenuItem key={item.id} asChild>
                      <Link to={item.path} aria-current={isActive(item) ? 'page' : undefined} data-testid={`ops-menu-${item.id}`}>
                        <item.icon size={15} strokeWidth={2} className="mr-2" aria-hidden="true" />
                        {item.label}
                      </Link>
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
            <Link to={homePathFor(role)} className="flex items-center gap-2 font-semibold tracking-tight">
              <Package size={22} strokeWidth={2} className="text-primary" aria-hidden="true" />
              SwiftDrop
            </Link>
            <Badge variant="secondary" data-testid="ops-role-badge">
              {role === 'ADMIN' ? 'Admin' : 'Dispatcher'}
            </Badge>
          </div>
          <div className="flex items-center gap-3">
            <div className="text-right leading-tight">
              <div className="font-medium">{user.display_name || user.email}</div>
              {user.display_name && <div className="text-xs text-text-muted">{user.email}</div>}
            </div>
            <Button type="button" variant="outline" onClick={endSession} data-testid="ops-signout-btn">
              <LogOut aria-hidden="true" /> Sign out
            </Button>
          </div>
        </div>
      </header>

      <div className="flex flex-1">
        <aside className="hidden w-60 shrink-0 border-r border-border bg-surface lg:block">
          <nav aria-label="Ops menu" data-testid="ops-sidebar" className="grid gap-4 p-3">
            {GROUPS.map((group) => {
              const rows = items.filter((item) => item.group === group);
              if (rows.length === 0) return null;
              return (
                <div key={group} className="grid gap-1">
                  <p className="px-3 text-xs font-medium text-text-muted">{group}</p>
                  {rows.map((item) => {
                    const active = isActive(item);
                    return (
                      <Link
                        key={item.id}
                        to={item.path}
                        aria-current={active ? 'page' : undefined}
                        data-testid={`ops-nav-${item.id}`}
                        className={cn(
                          'flex min-h-9 items-center gap-2 rounded-sm px-3 py-2 text-sm font-medium transition-colors touch:min-h-[44px]',
                          active ? 'bg-primary-tint text-primary' : 'text-text-muted hover:bg-surface-subtle hover:text-text',
                        )}
                      >
                        <item.icon size={15} strokeWidth={2} aria-hidden="true" />
                        <span className="flex-1">{item.label}</span>
                        {!item.built && (
                          <Badge variant="neutral" showDot={false}>
                            Soon
                          </Badge>
                        )}
                      </Link>
                    );
                  })}
                </div>
              );
            })}
          </nav>
        </aside>

        <main className="min-w-0 flex-1 px-4 py-5 sm:px-8 sm:py-8 lg:px-10">
          <div className="mx-auto grid w-full max-w-6xl gap-5">{blocked ? <ForbiddenPage /> : <Outlet />}</div>
        </main>
      </div>
    </div>
  );
};
