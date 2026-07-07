import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import type { ApiClient } from '../api/client';
import { createApiClient, mockClient } from '../api';
import type { CurrentUser, Role } from '../api/types';

interface AppContextValue {
  client: ApiClient;
  user: CurrentUser | null;
  role: Role;
  isMock: boolean;
  loading: boolean;
  setRole: (role: Role) => Promise<void>;
}

const AppContext = createContext<AppContextValue | null>(null);

export function AppProvider({ children }: { children: ReactNode }) {
  const client = useMemo(() => createApiClient(), []);
  const isMock = client === mockClient;
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [loading, setLoading] = useState(true);

  const refreshUser = useCallback(async () => {
    const next = await client.getCurrentUser();
    setUser(next);
    return next;
  }, [client]);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const next = await client.getCurrentUser();
        if (alive) setUser(next);
      } catch {
        if (alive) setUser(null);
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, [client]);

  const setRole = useCallback(
    async (role: Role) => {
      if (isMock) {
        mockClient.setCurrentRole(role);
        await refreshUser();
      }
    },
    [isMock, refreshUser],
  );

  const value = useMemo<AppContextValue>(
    () => ({
      client,
      user,
      role: user?.role ?? 'Viewer',
      isMock,
      loading,
      setRole,
    }),
    [client, user, isMock, loading, setRole],
  );

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error('useApp must be used within AppProvider');
  return ctx;
}
