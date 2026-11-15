import React, { useCallback, createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { User, LoginResponseData, GoogleLoginResponseData } from '@/shared/types/auth';
import { friendlyApiError } from '@/shared/lib/api-error';

interface ApiResponseEnvelope<T = unknown> {
  error?: { code?: string };
  data?: T;
}

interface AuthContextType {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isRestoringSession: boolean;
  login: (email: string, password: string) => Promise<User>;
  loginWithGoogle: (idToken: string) => Promise<{ user: User; isNewUser: boolean; requiresPhone: boolean }>;
  updateProfile: (phone: string, termsAccepted: boolean) => Promise<User>;
  refreshSession: () => Promise<User | null>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

let pendingRestore: Promise<LoginResponseData | null> | undefined;

function restoreSession(): Promise<LoginResponseData | null> {
  async function refresh(): Promise<LoginResponseData | null> {
    const response = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      credentials: 'same-origin',
      signal: AbortSignal.timeout(8000),
    });
    if (!response.ok) return null;
    const result: ApiResponseEnvelope<LoginResponseData> = await response.json();
    return result.data?.access_token && result.data.user ? result.data : null;
  }

  // Refresh rotates the cookie. Share the request across effect replays
  // and serialize refreshes across tabs where Web Locks are available.
  async function refreshWithLock(): Promise<LoginResponseData | null> {
    return navigator.locks
      ? await navigator.locks.request('swiftdrop-session-refresh', refresh)
      : refresh();
  }

  pendingRestore ??= refreshWithLock()
    .catch(() => null)
    .finally(() => { pendingRestore = undefined; });
  return pendingRestore;
}

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  // CRITICAL INVARIANT: Access token held strictly in-memory
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [isRestoringSession, setIsRestoringSession] = useState(true);

  useEffect(() => {
    let active = true;
    void restoreSession().then(session => {
      if (!active) return;
      if (session) {
        setAccessToken(session.access_token);
        setUser(session.user);
      }
      setIsRestoringSession(false);
    });
    return () => { active = false; };
  }, []);

  const refreshSession = useCallback(async () => {
    const session = await restoreSession();
    if (!session) return null;
    setAccessToken(session.access_token);
    setUser(session.user);
    return session.user;
  }, []);

  const login = async (email: string, password: string): Promise<User> => {
    try {
      await pendingRestore;
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      });

      const text = await res.text();
      let data: ApiResponseEnvelope<LoginResponseData> | null = null;
      try {
        data = text ? JSON.parse(text) : null;
      } catch {
        // Not a JSON response (e.g. 502/504 Bad Gateway from proxy)
      }

      if (!res.ok) {
        const unavailable = res.status === 502 || res.status === 504 || res.status === 404;
        const fallback = unavailable
          ? 'Sign-in is temporarily unavailable. Please try again later.'
          : res.status === 401 ? 'Check your email and password and try again.' : 'Could not sign in. Please try again.';
        throw new Error(friendlyApiError(res, data?.error?.code, fallback));
      }

      if (!data?.data?.access_token) {
        throw new Error('Could not complete sign-in. Please try again.');
      }

      const loginData: LoginResponseData = data.data;
      setAccessToken(loginData.access_token);
      setUser(loginData.user);
      return loginData.user;
    } catch (err: unknown) {
      if (err instanceof Error && err instanceof TypeError) {
        throw new Error('Could not connect to SwiftDrop. Please try again.');
      }
      throw err;
    }
  };

  const loginWithGoogle = async (idToken: string): Promise<{ user: User; isNewUser: boolean; requiresPhone: boolean }> => {
    try {
      await pendingRestore;
      const res = await fetch('/api/v1/auth/google', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ id_token: idToken }),
      });

      const text = await res.text();
      let data: ApiResponseEnvelope<GoogleLoginResponseData> | null = null;
      try {
        data = text ? JSON.parse(text) : null;
      } catch {}

      if (!res.ok) {
        const message = 'Could not sign in with Google. Please try again.';
        throw new Error(friendlyApiError(res, data?.error?.code, message));
      }

      if (!data?.data?.access_token) {
        throw new Error('Could not complete Google sign-in. Please try again.');
      }

      const googleData: GoogleLoginResponseData = data.data;
      setAccessToken(googleData.access_token);
      setUser(googleData.user);
      return {
        user: googleData.user,
        isNewUser: googleData.is_new_user,
        requiresPhone: googleData.requires_phone,
      };
    } catch (err: unknown) {
      if (err instanceof Error && err instanceof TypeError) {
        throw new Error('Could not connect to SwiftDrop. Please try again.');
      }
      throw err;
    }
  };

  const updateProfile = async (phone: string, termsAccepted: boolean): Promise<User> => {
    if (!accessToken) {
      throw new Error('You must be authenticated to update profile');
    }
    const res = await fetch('/api/v1/auth/me', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${accessToken}`,
      },
      body: JSON.stringify({ phone, terms_accepted: termsAccepted }),
    });

    const text = await res.text();
    let data: ApiResponseEnvelope<User> | null = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {}

    if (!res.ok) {
      const message = 'Could not update your profile. Please try again.';
      throw new Error(friendlyApiError(res, data?.error?.code, message));
    }

    if (!data?.data) {
      throw new Error('Could not confirm your profile update. Please try again.');
    }

    const updatedUser: User = data.data;
    setUser(updatedUser);
    return updatedUser;
  };

  const logout = async () => {
    try {
      await pendingRestore;
      await fetch('/api/v1/auth/logout', { method: 'POST' });
    } catch {}
    setAccessToken(null);
    setUser(null);
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        accessToken,
        isAuthenticated: !!user && !!accessToken,
        isRestoringSession,
        login,
        loginWithGoogle,
        updateProfile,
        refreshSession,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};


export const useAuth = (): AuthContextType => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
