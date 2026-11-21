import React from 'react';
import { Link } from 'react-router-dom';
import { Bell } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { useNotifications, InAppNotification } from '@/app/providers/NotificationContext';
import { cn } from '@/shared/lib/utils';

export function formatRelativeTime(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    const now = new Date();
    const diffSec = Math.floor((now.getTime() - date.getTime()) / 1000);
    if (isNaN(diffSec) || diffSec < 60) return 'Just now';
    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}h ago`;
    return `${Math.floor(diffHr / 24)}d ago`;
  } catch {
    return '';
  }
}

export const NotificationBell: React.FC = () => {
  const { notifications, unreadCount, markAsRead, markAllAsRead } = useNotifications();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="relative text-muted-foreground hover:text-foreground"
          aria-label={`Notifications (${unreadCount} unread)`}
        >
          <Bell className="w-4 h-4" />
          {unreadCount > 0 && (
            <Badge
              variant="error"
              showDot={false}
              className="absolute -top-1 -right-1 h-4 min-w-4 px-1 rounded-full text-xs leading-none flex items-center justify-center font-bold"
            >
              {unreadCount > 99 ? '99+' : unreadCount}
            </Badge>
          )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80 p-0 bg-surface border-border">
        <div className="flex items-center justify-between px-3 py-2 border-b border-border bg-surface-subtle">
          <span className="text-xs font-semibold text-foreground">Notifications</span>
          {unreadCount > 0 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => markAllAsRead()}
              className="h-6 px-2 text-xs text-muted-foreground hover:text-foreground"
            >
              Mark all read
            </Button>
          )}
        </div>
        <div className="max-h-80 overflow-y-auto divide-y divide-border">
          {notifications.length === 0 ? (
            <div className="py-8 text-center text-xs text-muted-foreground">
              No notifications yet
            </div>
          ) : (
            notifications.slice(0, 25).map((n: InAppNotification) => (
              <DropdownMenuItem
                key={n.notification_id}
                onClick={() => markAsRead(n.notification_id)}
                className={cn(
                  'flex flex-col items-start gap-1 p-3 cursor-pointer rounded-none focus:bg-surface-active',
                  !n.is_read && 'bg-surface-subtle'
                )}
              >
                <div className="flex items-center justify-between w-full gap-2">
                  <span className="text-xs font-semibold text-foreground truncate">
                    {n.title}
                  </span>
                  <span className="text-xs text-muted-foreground shrink-0 font-mono">
                    {formatRelativeTime(n.created_at)}
                  </span>
                </div>
                <p className="text-xs text-muted-foreground line-clamp-2 text-left">
                  {n.body}
                </p>
              </DropdownMenuItem>
            ))
          )}
        </div>
        <div className="p-2 border-t border-border bg-surface-subtle text-center">
          <Button asChild variant="ghost" size="sm" className="w-full h-8 text-xs font-medium text-text hover:text-text hover:bg-surface-active">
            <Link to="/notifications">View all notifications</Link>
          </Button>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
