import { createContext, ReactNode, useContext, useEffect, useMemo, useState } from 'react';
import { api, logoutRequest, setAuthFailureHandler, setToken } from '../api/client';
import type { User } from '../types/domain';

type AuthContextValue = {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  hasPermission: (permission: string) => boolean;
  hasAnyPermission: (permissions?: string[]) => boolean;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setAuthFailureHandler(() => {
      setUser(null);
    });
    api<User>('/auth/me')
      .then(setUser)
      .catch(() => {
        setToken(null);
        setUser(null);
      })
      .finally(() => setLoading(false));
    return () => setAuthFailureHandler(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      async login(email: string, password: string) {
        const data = await api<{ token: string; user: User }>('/auth/login', {
          method: 'POST',
          body: JSON.stringify({ email, password }),
        });
        setToken(data.token);
        setUser(data.user);
      },
      async logout() {
        await logoutRequest();
        setUser(null);
      },
      hasPermission(permission: string) {
        return Boolean(user?.permissions?.includes(permission));
      },
      hasAnyPermission(permissions?: string[]) {
        if (!permissions?.length) return true;
        return permissions.some((permission) => user?.permissions?.includes(permission));
      },
    }),
    [user, loading],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used inside AuthProvider');
  return context;
}
