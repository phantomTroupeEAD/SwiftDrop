import React, { createContext, useContext, useState, ReactNode } from 'react';
import { User, LoginResponseData, GoogleLoginResponseData } from '@/shared/types/auth';

interface AuthContextType {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<User>;
  loginWithGoogle: (idToken: string) => Promise<{ user: User; isNewUser: boolean; requiresPhone: boolean }>;
  updateProfile: (phone: string, termsAccepted: boolean) => Promise<User>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  // CRITICAL INVARIANT: Access token held strictly in-memory
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);

  const login = async (email: string, password: string): Promise<User> => {
    try {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      });

      const text = await res.text();
      let data: any = null;
      try {
        data = text ? JSON.parse(text) : null;
      } catch {
        // Not a JSON response (e.g. 502/504 Bad Gateway from proxy)
      }

      if (!res.ok) {
        if (data?.error?.message) {
          throw new Error(data.error.message);
        }
        if (res.status === 502 || res.status === 504 || res.status === 404) {
          throw new Error('Cannot connect to backend server. Please verify Core API is running on port 8080.');
        }
        throw new Error(`Authentication failed (${res.status})`);
      }

      if (!data?.data?.access_token) {
        throw new Error('Invalid authentication response from server');
      }

      const loginData: LoginResponseData = data.data;
      setAccessToken(loginData.access_token);
      setUser(loginData.user);
      return loginData.user;
    } catch (err: any) {
      if (err.message && err.message.includes('Failed to fetch')) {
        throw new Error('Network error: Unable to reach Core API server at port 8080.');
      }
      throw err;
    }
  };

  const loginWithGoogle = async (idToken: string): Promise<{ user: User; isNewUser: boolean; requiresPhone: boolean }> => {
    try {
      const res = await fetch('/api/v1/auth/google', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ id_token: idToken }),
      });

      const text = await res.text();
      let data: any = null;
      try {
        data = text ? JSON.parse(text) : null;
      } catch {}

      if (!res.ok) {
        if (data?.error?.message) {
          throw new Error(data.error.message);
        }
        throw new Error(`Google Sign-In failed (${res.status})`);
      }

      if (!data?.data?.access_token) {
        throw new Error('Invalid Google authentication response from server');
      }

      const googleData: GoogleLoginResponseData = data.data;
      setAccessToken(googleData.access_token);
      setUser(googleData.user);
      return {
        user: googleData.user,
        isNewUser: googleData.is_new_user,
        requiresPhone: googleData.requires_phone,
      };
    } catch (err: any) {
      if (err.message && err.message.includes('Failed to fetch')) {
        throw new Error('Network error: Unable to reach Core API server at port 8080.');
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
    let data: any = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {}

    if (!res.ok) {
      if (data?.error?.message) {
        throw new Error(data.error.message);
      }
      throw new Error(`Profile update failed (${res.status})`);
    }

    const updatedUser: User = data.data;
    setUser(updatedUser);
    return updatedUser;
  };

  const logout = async () => {
    try {
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
        login,
        loginWithGoogle,
        updateProfile,
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
