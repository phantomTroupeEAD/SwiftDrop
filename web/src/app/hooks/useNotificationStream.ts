import { useEffect, useRef, useState } from 'react';

export interface InAppNotification {
  notification_id: string;
  user_id: string;
  tenant_id?: string | null;
  event_type: string;
  aggregate_id: string;
  title: string;
  body: string;
  metadata?: Record<string, unknown>;
  is_read: boolean;
  created_at: string;
}

interface UseNotificationStreamOptions {
  token: string | null;
  enabled: boolean;
  onNotification: (notification: InAppNotification) => void;
  onUnreadCount: (count: number) => void;
}

export function useNotificationStream({
  token,
  enabled,
  onNotification,
  onUnreadCount,
}: UseNotificationStreamOptions): { isConnected: boolean } {
  const [isConnected, setIsConnected] = useState(false);
  const reconnectTimeoutRef = useRef<number | undefined>(undefined);
  const backoffRef = useRef(1000);
  const streamRef = useRef<EventSource | null>(null);

  // Keep callback refs stable to avoid reconnect loops
  const onNotificationRef = useRef(onNotification);
  onNotificationRef.current = onNotification;

  const onUnreadCountRef = useRef(onUnreadCount);
  onUnreadCountRef.current = onUnreadCount;

  useEffect(() => {
    if (!enabled || !token || typeof EventSource === 'undefined') {
      setIsConnected(false);
      return;
    }

    let isDisposed = false;

    function connect() {
      if (isDisposed) return;

      const url = `/api/v1/notifications/stream?token=${encodeURIComponent(token!)}`;
      const stream = new EventSource(url);
      streamRef.current = stream;

      stream.onopen = () => {
        if (!isDisposed) {
          setIsConnected(true);
          backoffRef.current = 1000; // Reset backoff on successful handshake
        }
      };

      stream.addEventListener('unread_count', (e: MessageEvent) => {
        try {
          const parsed = JSON.parse(e.data) as { count?: number };
          if (typeof parsed?.count === 'number') {
            onUnreadCountRef.current(parsed.count);
          }
        } catch {
          // Ignore malformed unread_count payload
        }
      });

      stream.addEventListener('notification', (e: MessageEvent) => {
        try {
          const parsed = JSON.parse(e.data) as InAppNotification;
          if (parsed?.notification_id) {
            onNotificationRef.current(parsed);
          }
        } catch {
          // Ignore malformed notification payload
        }
      });

      stream.onerror = () => {
        if (isDisposed) return;
        setIsConnected(false);
        stream.close();
        streamRef.current = null;

        // Exponential backoff reconnect with jitter (1s to 30s)
        const delay = Math.min(backoffRef.current, 30000) + Math.random() * 500;
        backoffRef.current = Math.min(backoffRef.current * 1.5, 30000);

        reconnectTimeoutRef.current = window.setTimeout(() => {
          connect();
        }, delay);
      };
    }

    connect();

    return () => {
      isDisposed = true;
      if (reconnectTimeoutRef.current !== undefined) {
        window.clearTimeout(reconnectTimeoutRef.current);
      }
      if (streamRef.current) {
        streamRef.current.close();
        streamRef.current = null;
      }
      setIsConnected(false);
    };
  }, [token, enabled]);

  return { isConnected };
}
