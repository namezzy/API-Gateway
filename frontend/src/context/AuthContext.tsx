import { createContext, useContext, useState, useCallback, ReactNode, useEffect, useRef } from 'react';
import { api } from '../lib/http';
import { useNotify } from './NotifyContext';

interface AuthCtx {
  token: string | null;
  login: (u: string, p: string) => Promise<boolean>;
  logout: () => void;
  refreshing: boolean;
}

interface StoredTokens {
  access: string;
  refresh: string;
  expiresAt: number; // epoch ms
}

const LS_KEY = 'gateway.tokens';

function decodeExp(jwt: string): number | null {
  try {
    const payload = JSON.parse(atob(jwt.split('.')[1]));
    if (payload.exp) return payload.exp * 1000;
  } catch {
    return null;
  }
  return null;
}

const AuthContext = createContext<AuthCtx | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [refreshToken, setRefreshToken] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const notify = useNotify();
  const refreshTimeout = useRef<number | null>(null);

  // Load from localStorage on init
  useEffect(() => {
    const raw = localStorage.getItem(LS_KEY);
    if (raw) {
      try {
        const parsed: StoredTokens = JSON.parse(raw);
        setToken(parsed.access);
        setRefreshToken(parsed.refresh);
        setExpiresAt(parsed.expiresAt);
      } catch {}
    }
  }, []);

  const persist = (access: string, refresh: string) => {
    const exp = decodeExp(access) || Date.now() + 10 * 60 * 1000;
    setToken(access);
    setRefreshToken(refresh);
    setExpiresAt(exp);
    localStorage.setItem(LS_KEY, JSON.stringify({ access, refresh, expiresAt: exp } satisfies StoredTokens));
  };

  const clearPersist = () => {
    localStorage.removeItem(LS_KEY);
    setToken(null);
    setRefreshToken(null);
    setExpiresAt(null);
  };

  const scheduleRefresh = useCallback((exp?: number | null) => {
    if (refreshTimeout.current) window.clearTimeout(refreshTimeout.current);
    const target = exp ?? expiresAt;
    if (!target) return;
    const now = Date.now();
    const delta = target - now - 60_000; // refresh 1 min before expiry
    if (delta <= 0) {
      // immediate refresh
      void doRefresh();
    } else {
      refreshTimeout.current = window.setTimeout(() => { void doRefresh(); }, Math.min(delta, 24 * 60 * 60 * 1000));
    }
  }, [expiresAt]);

  useEffect(() => { if (token && expiresAt) scheduleRefresh(expiresAt); }, [token, expiresAt, scheduleRefresh]);

  const login = useCallback(async (username: string, password: string) => {
    try {
      const res = await api.post('/auth/login', { username, password });
      persist(res.data.access_token, res.data.refresh_token);
      notify.success('登录成功');
      return true;
    } catch (e) {
      notify.error('登录失败');
      return false;
    }
  }, [notify]);

  const doRefresh = useCallback(async () => {
    if (refreshing) return;
    if (!refreshToken) return;
    setRefreshing(true);
    try {
      const res = await api.post('/auth/refresh', { refresh_token: refreshToken });
      persist(res.data.access_token, refreshToken); // backend不返回新的refresh
      notify.info('令牌已续期');
    } catch (e) {
      notify.error('令牌刷新失败, 请重新登录');
      clearPersist();
    } finally {
      setRefreshing(false);
    }
  }, [refreshToken, refreshing, notify]);

  const logout = useCallback(() => {
    clearPersist();
    notify.info('已退出');
  }, [notify]);

  return <AuthContext.Provider value={{ token, login, logout, refreshing }}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('AuthContext missing');
  return ctx;
}
