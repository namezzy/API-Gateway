import { createContext, useContext, useState, useCallback, ReactNode } from 'react';
import axios from 'axios';

interface AuthCtx {
  token: string | null;
  login: (u: string, p: string) => Promise<boolean>;
  logout: () => void;
}

const AuthContext = createContext<AuthCtx | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null);

  const login = useCallback(async (username: string, password: string) => {
    try {
      const res = await axios.post('/auth/login', { username, password });
      setToken(res.data.access_token);
      axios.defaults.headers.common['Authorization'] = 'Bearer ' + res.data.access_token;
      return true;
    } catch (e) {
      return false;
    }
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    delete axios.defaults.headers.common['Authorization'];
  }, []);

  return <AuthContext.Provider value={{ token, login, logout }}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('AuthContext missing');
  return ctx;
}
