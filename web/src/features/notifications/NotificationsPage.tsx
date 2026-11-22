import React, { useMemo } from 'react';
import { Link } from 'react-router-dom';
import { Check, CheckCheck, ExternalLink, Inbox } from 'lucide-react';
import { useAuth } from '@/app/providers/AuthContext';
import { useNotifications, InAppNotification } from '@/app/providers/NotificationContext';
import { formatRelativeTime } from '@/shared/components/NotificationBell';
import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import { Card } from '@/shared/components/ui/card';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/shared/components/ui/tabs';
import { DashboardLayout } from '@/features/dashboards/DashboardLayout';
import { PortalLayout } from '@/features/customer-merchant/portal/PortalLayout';
import { cn } from '@/shared/lib/utils';

function getBadgeVariant(eventType: string): 'default' | 'secondary' | 'neutral' | 'active' | 'success' | 'warning' | 'error' {
  const lower = eventType.toLowerCase();
  if (lower.includes('delivered') || lower.includes('completed') || lower.includes('approved')) {
    return 'success';
  }
  if (lower.includes('failed') || lower.includes('cancelled') || lower.includes('rejected')) {
    return 'error';
  }
  if (lower.includes('warning') || lower.includes('delayed') || lower.includes('attention') || lower.includes('attempt_failed')) {
    return 'warning';
  }
  if (lower.includes('in_transit') || lower.includes('out_for_delivery') || lower.includes('assigned') || lower.includes('picked_up')) {
    return 'active';
  }
  return 'neutral';
}

function formatEventType(eventType: string): string {
  if (!eventType) return 'EVENT';
  return eventType.replace(/\.v\d+$/i, '').replace(/[._]/g, ' ').toUpperCase();
}

function EmptyState({ isFiltered }: { isFiltered: boolean }) {
  return (
    <Card className="p-8 text-center bg-surface border-border">
      <div className="flex flex-col items-center justify-center space-y-2">
        <div className="w-10 h-10 rounded-full bg-surface-subtle flex items-center justify-center text-text-muted">
          <Inbox className="w-5 h-5" />
        </div>
        <div className="text-sm font-semibold text-text">No notifications found</div>
        <p className="text-xs text-text-muted max-w-sm">
          {isFiltered
            ? 'You have read all notifications. Switch to "All" to view past activity.'
            : 'You do not have any notifications yet. Operational alerts and delivery updates will appear here.'}
        </p>
      </div>
    </Card>
  );
}

function NotificationItem({
  notification,
  onMarkAsRead,
}: {
  notification: InAppNotification;
  onMarkAsRead: (id: string) => void;
}) {
  const trackingNumber =
    typeof notification.metadata?.tracking_number === 'string'
      ? notification.metadata.tracking_number
      : undefined;

  return (
    <Card
      key={notification.notification_id}
      className={cn(
        'p-4 border-border transition-colors',
        !notification.is_read ? 'bg-surface-subtle' : 'bg-surface'
      )}
    >
      <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
        <div className="space-y-1.5 flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={getBadgeVariant(notification.event_type)}>
              {formatEventType(notification.event_type)}
            </Badge>
            {!notification.is_read && (
              <Badge variant="active" showDot={false}>
                New
              </Badge>
            )}
            <span className="text-xs text-text-muted font-mono">
              {formatRelativeTime(notification.created_at)}
            </span>
          </div>
          <h3 className="text-sm font-semibold text-text leading-tight">
            {notification.title}
          </h3>
          <p className="text-xs text-text-muted whitespace-pre-wrap leading-relaxed">
            {notification.body}
          </p>
        </div>
        <div className="flex items-center gap-2 shrink-0 pt-1 sm:pt-0">
          {trackingNumber && (
            <Button asChild variant="outline" size="sm" className="h-7 text-xs">
              <Link to={`/track/${encodeURIComponent(trackingNumber)}`}>
                <ExternalLink className="w-3 h-3 mr-1" />
                Track
              </Link>
            </Button>
          )}
          {!notification.is_read && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onMarkAsRead(notification.notification_id)}
              className="h-7 text-xs text-text-muted hover:text-text hover:bg-surface-active"
            >
              <Check className="w-3 h-3 mr-1" />
              Mark as read
            </Button>
          )}
        </div>
      </div>
    </Card>
  );
}

export const NotificationsPage: React.FC = () => {
  const { user } = useAuth();
  const { notifications, unreadCount, markAsRead, markAllAsRead } = useNotifications();

  const unreadNotifications = useMemo(
    () => notifications.filter((n) => !n.is_read),
    [notifications]
  );

  const content = (
    <div className="space-y-4">
      {/* Header bar with title, unread count badge, and 'Mark all as read' button */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 border-b border-border">
        <div className="flex items-center gap-2">
          <h2 className="text-base font-semibold text-text">Inbox</h2>
          {unreadCount > 0 ? (
            <Badge variant="error" showDot={false}>
              {unreadCount} unread
            </Badge>
          ) : (
            <Badge variant="neutral" showDot={false}>
              All caught up
            </Badge>
          )}
        </div>
        {unreadCount > 0 && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => markAllAsRead()}
            className="text-xs"
          >
            <CheckCheck className="w-3.5 h-3.5 mr-1.5" />
            Mark all as read
          </Button>
        )}
      </div>

      {/* Tabs for filtering: 'All' and 'Unread' */}
      <Tabs defaultValue="all" className="w-full space-y-4">
        <div className="flex items-center justify-between">
          <TabsList>
            <TabsTrigger value="all">
              All ({notifications.length})
            </TabsTrigger>
            <TabsTrigger value="unread">
              Unread ({unreadCount})
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="all" className="space-y-3">
          {notifications.length === 0 ? (
            <EmptyState isFiltered={false} />
          ) : (
            notifications.map((n) => (
              <NotificationItem
                key={n.notification_id}
                notification={n}
                onMarkAsRead={markAsRead}
              />
            ))
          )}
        </TabsContent>

        <TabsContent value="unread" className="space-y-3">
          {unreadNotifications.length === 0 ? (
            <EmptyState isFiltered={true} />
          ) : (
            unreadNotifications.map((n) => (
              <NotificationItem
                key={n.notification_id}
                notification={n}
                onMarkAsRead={markAsRead}
              />
            ))
          )}
        </TabsContent>
      </Tabs>
    </div>
  );

  if (
    user?.role === 'ADMIN' ||
    user?.role === 'DISPATCHER' ||
    user?.role === 'COURIER'
  ) {
    return (
      <DashboardLayout
        activeRole={user.role}
        title="Notifications"
        subtitle="Review notifications and operational alerts"
      >
        {content}
      </DashboardLayout>
    );
  }

  return (
    <PortalLayout
      title="Notifications"
      profile={user?.role === 'MERCHANT' ? 'merchant' : 'customer'}
    >
      {content}
    </PortalLayout>
  );
};

export default NotificationsPage;
