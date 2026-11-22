import React, { createContext, useCallback, useContext, useState } from 'react';
import { toast } from 'sonner';
import { useAuth } from './AuthContext';
import { useNotificationStream, InAppNotification } from '../hooks/useNotificationStream';

export type { InAppNotification };

export interface NotificationContextType {
  notifications: InAppNotification[];
  unreadCount: number;
  isConnected: boolean;
  markAsRead: (notificationId: string) => void;
  markAllAsRead: () => void;
}

const NotificationContext = createContext<NotificationContextType | undefined>(undefined);

export const NotificationProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { accessToken, isAuthenticated } = useAuth();
  const [notifications, setNotifications] = useState<InAppNotification[]>([]);
  const [unreadCount, setUnreadCount] = useState<number>(0);

  const handleNotification = useCallback((incoming: InAppNotification) => {
    setNotifications((prev) => {
      // Prevent duplicates
      if (prev.some((n) => n.notification_id === incoming.notification_id)) {
        return prev;
      }
      return [incoming, ...prev];
    });

    if (!incoming.is_read) {
      setUnreadCount((c) => c + 1);
      // Trigger sonner toast for incoming live notification
      toast(incoming.title, {
        description: incoming.body,
      });
    }
  }, []);

  const handleUnreadCount = useCallback((count: number) => {
    setUnreadCount(count);
  }, []);

  const { isConnected } = useNotificationStream({
    token: accessToken,
    enabled: isAuthenticated,
    onNotification: handleNotification,
    onUnreadCount: handleUnreadCount,
  });

  const markAsRead = useCallback((notificationId: string) => {
    setNotifications((prev) =>
      prev.map((n) => {
        if (n.notification_id === notificationId && !n.is_read) {
          return { ...n, is_read: true };
        }
        return n;
      })
    );
    setUnreadCount((c) => Math.max(0, c - 1));
  }, []);

  const markAllAsRead = useCallback(() => {
    setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
    setUnreadCount(0);
  }, []);

  const value: NotificationContextType = {
    notifications,
    unreadCount,
    isConnected,
    markAsRead,
    markAllAsRead,
  };

  return (
    <NotificationContext.Provider value={value}>
      {children}
    </NotificationContext.Provider>
  );
};

export function useNotifications(): NotificationContextType {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error('useNotifications must be used within a NotificationProvider');
  }
  return context;
}
