import React from 'react';
import { LogOut, Package } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import type { User } from '@/shared/types/auth';
import { useBodyRole } from './useBodyRole';

interface AdminShellProps {
  user: User | null;
  onSignOut: () => void;
  title: string;
  subtitle: string;
  children: React.ReactNode;
}

/**
 * App shell for the admin console. `data-role="ADMIN"` applies the Admin preset from
 * index.css (AGENTS.md Rule 6). Branding, current section and account actions stay in the
 * same place, in a logical keyboard order.
 */
export const AdminShell: React.FC<AdminShellProps> = ({ user, onSignOut, title, subtitle, children }) => {
  useBodyRole('ADMIN');
  return (
    <div data-role="ADMIN" className="min-h-screen bg-canvas text-sm text-text">
      <header className="border-b border-border bg-surface">
        <div className="mx-auto flex w-full max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-3 sm:px-8 lg:px-10">
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-2 font-semibold tracking-tight">
              <Package size={22} strokeWidth={2} className="text-primary" aria-hidden="true" />
              SwiftDrop
            </span>
            <Badge variant="secondary">Admin console</Badge>
          </div>
          <div className="flex items-center gap-3">
            <div className="text-right leading-tight">
              <div className="font-medium">{user?.display_name || user?.email || 'Signed-in user'}</div>
              {user?.email && user.display_name && <div className="text-xs text-text-muted">{user.email}</div>}
            </div>
            <Button type="button" variant="outline" onClick={onSignOut} data-testid="admin-signout-btn">
              <LogOut aria-hidden="true" /> Sign out
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto grid w-full max-w-7xl gap-5 px-4 py-5 sm:px-8 sm:py-8 lg:px-10">
        <div className="grid gap-1.5">
          <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
          <p className="text-sm text-text-muted">{subtitle}</p>
        </div>
        {children}
      </main>
    </div>
  );
};
