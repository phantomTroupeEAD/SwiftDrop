import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { RefreshCw, Search, Users } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import type { Role, User } from '@/shared/types/auth';
import { ApiError } from '../api';
import { changeUserRole, listUsers, setUserStatus } from './consoleApi';
import { AdminDialog, Banner, Busy, FieldError, FieldHint } from './ui';

const ROLES: Role[] = ['CUSTOMER', 'COURIER', 'DISPATCHER', 'MERCHANT', 'ADMIN'];

const ROLE_HELP: Record<Role, string> = {
  CUSTOMER: 'Books and tracks parcels',
  COURIER: 'Delivers parcels in the field',
  DISPATCHER: 'Assigns couriers to parcels',
  MERCHANT: 'Ships on behalf of a business',
  ADMIN: 'Full system administration',
};

interface UsersPanelProps {
  token: string | null;
  currentUserId: string | undefined;
  onNotice: (message: string) => void;
  onChanged: () => void;
}

function displayName(u: User): string {
  return u.display_name || [u.first_name, u.last_name].filter(Boolean).join(' ') || 'Unnamed user';
}

export const UsersPanel: React.FC<UsersPanelProps> = ({ token, currentUserId, onNotice, onChanged }) => {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<ApiError | null>(null);
  const [roleFilter, setRoleFilter] = useState<Role | 'ALL'>('ALL');
  const [query, setQuery] = useState('');
  const [roleUser, setRoleUser] = useState<User | null>(null);
  const [suspendUser, setSuspendUser] = useState<User | null>(null);
  const [rowBusy, setRowBusy] = useState<string | null>(null);
  const [rowError, setRowError] = useState<ApiError | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const res = await listUsers(token, roleFilter === 'ALL' ? undefined : roleFilter);
      setUsers(res.users);
    } catch (err) {
      setLoadError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load users. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token, roleFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return users;
    return users.filter((u) => displayName(u).toLowerCase().includes(q) || u.email.toLowerCase().includes(q) || u.id.toLowerCase().includes(q));
  }, [users, query]);

  const filtered = roleFilter !== 'ALL' || query.trim() !== '';
  const resetFilters = () => {
    setRoleFilter('ALL');
    setQuery('');
  };

  const reactivate = async (u: User) => {
    setRowBusy(u.id);
    setRowError(null);
    try {
      await setUserStatus(token, u.id, 'ACTIVE');
      setUsers((prev) => prev.map((x) => (x.id === u.id ? { ...x, status: 'ACTIVE' } : x)));
      onNotice(`Account ${u.email} is active again.`);
      onChanged();
    } catch (err) {
      setRowError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't reactivate the account. Try again."));
    } finally {
      setRowBusy(null);
    }
  };

  const showEmpty = !loading && !loadError && visible.length === 0;

  return (
    <Card role="region" aria-labelledby="users-title">
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
        <div className="grid gap-1.5">
          <CardTitle id="users-title">Users and access</CardTitle>
          <CardDescription>Find an account, change its role, or suspend it.</CardDescription>
        </div>
        <Button
          type="button"
          variant="outline"
          size="icon"
          onClick={() => void load()}
          disabled={loading}
          aria-label="Refresh users"
          title="Refresh users"
        >
          <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
        </Button>
      </CardHeader>

      <CardContent className="grid gap-4">
        <div className="grid gap-4 sm:grid-cols-3">
          <div className="grid gap-1.5 sm:col-span-2">
            <Label htmlFor="users-search">Search users</Label>
            <div className="relative">
              <Search size={15} strokeWidth={2} className="absolute left-3 top-1/2 -translate-y-1/2 text-text-muted" aria-hidden="true" />
              <Input
                id="users-search"
                data-testid="admin-users-search"
                type="search"
                className="pl-9"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Name, email or user ID"
              />
            </div>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="users-role">Role</Label>
            <Select value={roleFilter} onValueChange={(v) => setRoleFilter(v as Role | 'ALL')}>
              <SelectTrigger id="users-role" data-testid="admin-users-role-filter">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ALL">All roles</SelectItem>
                {ROLES.map((r) => (
                  <SelectItem key={r} value={r}>
                    {r}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <p className="sr-only" role="status">
          {loading ? 'Loading users…' : `${visible.length} users shown`}
        </p>

        {loadError && (
          <Banner
            tone="err"
            reference={loadError.correlationId}
            action={
              <div>
                <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                  Retry
                </Button>
              </div>
            }
          >
            {loadError.status === 403 ? 'Your account is not allowed to view users.' : loadError.message || "Couldn't load users. Try again."}
          </Banner>
        )}
        {rowError && (
          <Banner tone="err" reference={rowError.correlationId}>
            {rowError.message}
          </Banner>
        )}

        {showEmpty ? (
          <div className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
            <Users size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
            <CardTitle>{filtered ? 'No users match your filters' : 'No users yet'}</CardTitle>
            <CardDescription>{filtered ? 'Try a different search or role.' : 'Accounts appear here once people register.'}</CardDescription>
            {filtered && (
              <Button type="button" variant="outline" className="mt-2" onClick={resetFilters}>
                Reset filters
              </Button>
            )}
          </div>
        ) : (
          <Table aria-label="User accounts" data-testid="admin-users-table" className="min-w-max">
            <TableHeader>
              <TableRow>
                <TableHead>Account</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading && users.length === 0
                ? [0, 1, 2, 3].map((i) => (
                    <TableRow key={i} aria-hidden="true">
                      <TableCell>
                        <Skeleton className="h-4 w-40" />
                        <Skeleton className="mt-2 h-3 w-48" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-5 w-20" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-5 w-16" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="ml-auto h-8 w-36" />
                      </TableCell>
                    </TableRow>
                  ))
                : visible.map((u) => {
                    const suspended = u.status === 'SUSPENDED';
                    const isSelf = u.id === currentUserId;
                    return (
                      <TableRow key={u.id}>
                        <TableCell>
                          <div className="font-medium">{displayName(u)}</div>
                          <div className="text-xs text-text-muted">{u.email}</div>
                          {u.tenant_id && <div className="font-mono text-xs text-text-muted">Tenant {u.tenant_id}</div>}
                        </TableCell>
                        <TableCell>
                          <Badge variant="neutral">{u.role}</Badge>
                        </TableCell>
                        <TableCell>
                          <Badge variant={suspended ? 'error' : 'success'}>{suspended ? 'Suspended' : 'Active'}</Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex flex-wrap items-center justify-end gap-2">
                            {isSelf ? (
                              <Badge variant="neutral" showDot={false} data-testid="admin-user-self-badge">
                                This is you
                              </Badge>
                            ) : (
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                className="touch:min-h-[44px]"
                                onClick={() => {
                                  setRowError(null);
                                  setRoleUser(u);
                                }}
                                aria-label={`Change role for ${u.email}`}
                                data-testid="admin-user-role-btn"
                              >
                                Change role
                              </Button>
                            )}
                            {suspended ? (
                              <Button
                                type="button"
                                variant="secondary"
                                size="sm"
                                className="touch:min-h-[44px]"
                                onClick={() => void reactivate(u)}
                                disabled={rowBusy === u.id}
                                aria-label={`Reactivate account ${u.email}`}
                                data-testid="admin-user-reactivate-btn"
                              >
                                {rowBusy === u.id ? <Busy label="Reactivating…" /> : 'Reactivate'}
                              </Button>
                            ) : (
                              !isSelf && (
                                <Button
                                  type="button"
                                  variant="destructive"
                                  size="sm"
                                  className="touch:min-h-[44px]"
                                  onClick={() => {
                                    setRowError(null);
                                    setSuspendUser(u);
                                  }}
                                  aria-label={`Suspend account ${u.email}`}
                                  data-testid="admin-user-suspend-btn"
                                >
                                  Suspend
                                </Button>
                              )
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
            </TableBody>
          </Table>
        )}
      </CardContent>

      {roleUser && (
        <RoleDialog
          user={roleUser}
          token={token}
          onClose={() => setRoleUser(null)}
          onSaved={(updated) => {
            setUsers((prev) => prev.map((x) => (x.id === updated.id ? { ...x, role: updated.role, tenant_id: updated.tenant_id } : x)));
            setRoleUser(null);
            onNotice(`Role for ${updated.email} is now ${updated.role}.`);
            onChanged();
          }}
        />
      )}
      {suspendUser && (
        <SuspendDialog
          user={suspendUser}
          token={token}
          onClose={() => setSuspendUser(null)}
          onSaved={() => {
            const u = suspendUser;
            setUsers((prev) => prev.map((x) => (x.id === u.id ? { ...x, status: 'SUSPENDED' } : x)));
            setSuspendUser(null);
            onNotice(`Account ${u.email} is suspended.`);
            onChanged();
          }}
        />
      )}
    </Card>
  );
};

interface RoleDialogProps {
  user: User;
  token: string | null;
  onClose: () => void;
  onSaved: (updated: User) => void;
}

const RoleDialog: React.FC<RoleDialogProps> = ({ user, token, onClose, onSaved }) => {
  const [role, setRole] = useState<Role>(user.role);
  const [tenant, setTenant] = useState(user.tenant_id ?? '');
  const [tenantError, setTenantError] = useState<string | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [saving, setSaving] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (role === 'MERCHANT' && !tenant.trim()) {
      setTenantError('Enter the merchant tenant ID.');
      return;
    }
    if (role === user.role && (role !== 'MERCHANT' || tenant.trim() === (user.tenant_id ?? ''))) {
      setError(new ApiError(0, 'NO_CHANGE', 'Nothing was changed, so there is nothing to save.'));
      return;
    }
    setSaving(true);
    try {
      const updated = await changeUserRole(token, user.id, role, role === 'MERCHANT' ? tenant.trim() : null);
      onSaved(updated);
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't change the role. Try again."));
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog title="Change role" description={`${displayName(user)} · ${user.email}`} onClose={onClose} busy={saving} testId="admin-user-role-dialog">
      <form onSubmit={submit} noValidate>
        <div className="grid gap-4 px-5 py-5">
          {error && (
            <Banner tone="err" reference={error.correlationId}>
              {error.message}
            </Banner>
          )}
          <div className="grid gap-1.5">
            <Label htmlFor="role-select">New role</Label>
            <Select
              value={role}
              onValueChange={(v) => {
                setRole(v as Role);
                setTenantError(null);
              }}
              disabled={saving}
            >
              <SelectTrigger id="role-select" data-testid="admin-user-role-select">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLES.map((r) => (
                  <SelectItem key={r} value={r}>
                    {r} — {ROLE_HELP[r]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {role === 'MERCHANT' && (
            <div className="grid gap-1.5">
              <Label htmlFor="role-tenant">Merchant tenant ID</Label>
              <Input
                id="role-tenant"
                className="font-mono"
                value={tenant}
                onChange={(e) => {
                  setTenant(e.target.value);
                  setTenantError(null);
                }}
                disabled={saving}
                placeholder="mrc_01HMERCHANTTENANT00001"
                aria-invalid={Boolean(tenantError)}
                aria-describedby="role-tenant-note"
              />
              {tenantError ? (
                <FieldError id="role-tenant-note">{tenantError}</FieldError>
              ) : (
                <FieldHint id="role-tenant-note">Merchants need a tenant ID so their orders stay separate.</FieldHint>
              )}
            </div>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" disabled={saving} data-testid="admin-user-role-save-btn">
            {saving ? <Busy label="Saving…" /> : 'Save changes'}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};

const SuspendDialog: React.FC<{ user: User; token: string | null; onClose: () => void; onSaved: () => void }> = ({
  user,
  token,
  onClose,
  onSaved,
}) => {
  const [error, setError] = useState<ApiError | null>(null);
  const [saving, setSaving] = useState(false);

  const confirm = async () => {
    setError(null);
    setSaving(true);
    try {
      await setUserStatus(token, user.id, 'SUSPENDED');
      onSaved();
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't suspend the account. Try again."));
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog
      title={`Suspend account ${user.email}`}
      description="This marks the account as suspended. You can reactivate it later from this list."
      onClose={onClose}
      busy={saving}
      testId="admin-user-suspend-dialog"
    >
      <div className="grid gap-4 px-5 py-5">
        {error && (
          <Banner tone="err" reference={error.correlationId}>
            {error.message}
          </Banner>
        )}
        <p className="text-sm text-text-muted">
          {displayName(user)} · role {user.role}
        </p>
      </div>
      <DialogFooter>
        <Button type="button" variant="outline" data-autofocus onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button type="button" variant="destructive" onClick={() => void confirm()} disabled={saving} data-testid="admin-user-suspend-confirm-btn">
          {saving ? <Busy label="Suspending…" /> : 'Suspend account'}
        </Button>
      </DialogFooter>
    </AdminDialog>
  );
};
