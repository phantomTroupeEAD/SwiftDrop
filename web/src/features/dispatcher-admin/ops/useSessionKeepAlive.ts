import { useEffect, useRef } from 'react';
import type { User } from '@/shared/types/auth';

// The access token lives in memory only (AuthContext) and lasts about 15 minutes. The refresh
// token is an HttpOnly cookie the browser sends by itself, so this code never sees it. Calling
// AuthContext's refreshSession() rotates the cookie and puts a new access token in memory.
// This hook does that shortly before the token expires, so a screen left open keeps working.

const REFRESH_BEFORE_MS = 60_000;
const MIN_WAIT_MS = 5_000;

/** Reads the exp claim (in ms) from a JWT, or null if it cannot be read. The token stays in memory. */
export function tokenExpiryMs(token: string): number | null {
  try {
    const part = token.split('.')[1];
    if (!part) return null;
    const base64 = part.replace(/-/g, '+').replace(/_/g, '/');
    const json = atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, '='));
    const exp = (JSON.parse(json) as { exp?: unknown }).exp;
    return typeof exp === 'number' ? exp * 1000 : null;
  } catch {
    return null;
  }
}

/**
 * Refreshes the session before the access token expires, and again when the tab becomes
 * visible after being hidden. If the refresh fails (cookie expired or revoked), onExpired runs.
 */
export function useSessionKeepAlive(accessToken: string | null, refresh: () => Promise<User | null>, onExpired: () => void): void {
  const refreshRef = useRef(refresh);
  const expiredRef = useRef(onExpired);
  refreshRef.current = refresh;
  expiredRef.current = onExpired;

  useEffect(() => {
    if (!accessToken) return;
    const expiresAt = tokenExpiryMs(accessToken);
    if (expiresAt === null) return;

    let cancelled = false;
    const renew = async () => {
      const user = await refreshRef.current();
      if (!cancelled && !user) expiredRef.current();
    };

    const timer = window.setTimeout(() => void renew(), Math.max(expiresAt - Date.now() - REFRESH_BEFORE_MS, MIN_WAIT_MS));
    const onVisible = () => {
      if (document.visibilityState === 'visible' && expiresAt - Date.now() < REFRESH_BEFORE_MS) void renew();
    };
    document.addEventListener('visibilitychange', onVisible);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, [accessToken]);
}
