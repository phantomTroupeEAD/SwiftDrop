import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldAlert } from 'lucide-react';
import { useAuth } from '@/app/providers/AuthContext';
import { Card, CardContent, CardDescription, CardTitle } from '@/shared/components/ui/card';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/shared/components/ui/tabs';
import { AdminShell } from './AdminShell';
import { AuditLogPanel } from './AuditLogPanel';
import { SettingsPanel } from './SettingsPanel';
import { UsersPanel } from './UsersPanel';
import { auditTotal, listUsers } from './consoleApi';
import { Banner } from './ui';
import { ZonesPanel } from '../zones/ZonesPanel';
import { listZones } from '../zones/zonesApi';

// Admin console: the screen an ADMIN lands on after sign-in (route /admin/dashboard).
// Design: web/design.md (ADMIN preset), process in web/design.mdx.

type TabId = 'zones' | 'users' | 'settings' | 'audit';

const TABS: { id: TabId; label: string }[] = [
  { id: 'zones', label: 'Delivery zones' },
  { id: 'users', label: 'Users and access' },
  { id: 'settings', label: 'Settings' },
  { id: 'audit', label: 'Audit log' },
];

interface Overview {
  users: number | null;
  zonesTotal: number | null;
  zonesEnabled: number | null;
  audit: number | null;
}

const EMPTY_OVERVIEW: Overview = { users: null, zonesTotal: null, zonesEnabled: null, audit: null };
const ZONE_SUMMARY_LIMIT = 100;

export const AdminConsole: React.FC = () => {
  const { user, accessToken, logout } = useAuth();
  const navigate = useNavigate();
  const isAdmin = user?.role === 'ADMIN';

  const [tab, setTab] = useState<TabId>('zones');
  const [notice, setNotice] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [overview, setOverview] = useState<Overview>(EMPTY_OVERVIEW);
  const [overviewLoading, setOverviewLoading] = useState(true);
  const noticeTimer = useRef<number | undefined>(undefined);

  // Real numbers only: each card shows what the API returned, or "Unavailable".
  useEffect(() => {
    if (!isAdmin) return;
    let cancelled = false;
    setOverviewLoading(true);
    void Promise.allSettled([listUsers(accessToken, undefined, 1), listZones(accessToken, 1, ZONE_SUMMARY_LIMIT), auditTotal(accessToken)]).then(
      ([users, zones, audit]) => {
        if (cancelled) return;
        const next: Overview = { ...EMPTY_OVERVIEW };
        if (users.status === 'fulfilled') next.users = users.value.total;
        if (zones.status === 'fulfilled') {
          next.zonesTotal = zones.value.meta.total_items;
          // Counting enabled zones needs every zone, which one page covers up to the limit.
          if (zones.value.meta.total_items <= ZONE_SUMMARY_LIMIT) {
            next.zonesEnabled = zones.value.zones.filter((z) => z.enabled).length;
          }
        }
        if (audit.status === 'fulfilled') next.audit = audit.value;
        setOverview(next);
        setOverviewLoading(false);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [accessToken, isAdmin, refreshKey]);

  useEffect(() => () => window.clearTimeout(noticeTimer.current), []);

  const showNotice = useCallback((message: string) => {
    setNotice(message);
    window.clearTimeout(noticeTimer.current);
    noticeTimer.current = window.setTimeout(() => setNotice(null), 5000);
  }, []);
  const markChanged = useCallback(() => setRefreshKey((k) => k + 1), []);

  const handleSignOut = () => {
    logout();
    navigate('/auth/login');
  };

  return (
    <AdminShell
      user={user}
      onSignOut={handleSignOut}
      title="Admin console"
      subtitle="Manage delivery zones, user access and system settings, and review every recorded change."
    >
      {!isAdmin ? (
        <Card data-testid="admin-forbidden">
          <CardContent className="flex flex-col items-center gap-2 px-4 py-12 text-center">
            <ShieldAlert size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
            <CardTitle>Administrators only</CardTitle>
            <CardDescription>
              The admin console is available to accounts with the ADMIN role. Sign in with an administrator account to continue.
            </CardDescription>
          </CardContent>
        </Card>
      ) : (
        <>
          {notice && (
            <Banner tone="ok" testId="admin-notice">
              {notice}
            </Banner>
          )}

          <section aria-label="Summary" className="grid gap-3 sm:grid-cols-3">
            <StatCard label="Delivery zones" loading={overviewLoading} value={overview.zonesTotal} note={zoneNote(overview)} />
            <StatCard label="User accounts" loading={overviewLoading} value={overview.users} note="Across all roles" />
            <StatCard label="Audit log entries" loading={overviewLoading} value={overview.audit} note="Append-only record of changes" />
          </section>

          <Tabs value={tab} onValueChange={(v) => setTab(v as TabId)}>
            <div className="overflow-x-auto">
              <TabsList aria-label="Admin sections">
                {TABS.map((t) => (
                  <TabsTrigger key={t.id} value={t.id} className="min-h-9 px-3 touch:min-h-[44px]" data-testid={`admin-tab-${t.id}`}>
                    {t.label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </div>
            <TabsContent value="zones" className="mt-4">
              <ZonesPanel token={accessToken} onNotice={showNotice} onChanged={markChanged} />
            </TabsContent>
            <TabsContent value="users" className="mt-4">
              <UsersPanel token={accessToken} currentUserId={user?.id} onNotice={showNotice} onChanged={markChanged} />
            </TabsContent>
            <TabsContent value="settings" className="mt-4">
              <SettingsPanel token={accessToken} onNotice={showNotice} onChanged={markChanged} />
            </TabsContent>
            <TabsContent value="audit" className="mt-4">
              <AuditLogPanel token={accessToken} refreshKey={refreshKey} />
            </TabsContent>
          </Tabs>
        </>
      )}
    </AdminShell>
  );
};

function zoneNote(o: Overview): string {
  if (o.zonesTotal === null) return '';
  if (o.zonesEnabled === null) return 'Enabled count shown in the list';
  return `${o.zonesEnabled} enabled, ${o.zonesTotal - o.zonesEnabled} disabled`;
}

const StatCard: React.FC<{ label: string; value: number | null; note: string; loading: boolean }> = ({ label, value, note, loading }) => (
  <Card>
    <CardContent className="p-4">
      <p className="text-sm text-text-muted">{label}</p>
      {loading ? (
        <Skeleton className="mt-2 h-8 w-16" />
      ) : (
        <p className="mt-1 text-2xl font-semibold tabular-nums">{value === null ? 'Unavailable' : value}</p>
      )}
      {!loading && value !== null && note && <p className="mt-1 text-xs text-text-muted">{note}</p>}
    </CardContent>
  </Card>
);
