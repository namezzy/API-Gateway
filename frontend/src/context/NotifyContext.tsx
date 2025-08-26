import { createContext, useContext, useState, ReactNode, useCallback } from 'react';
import { Snackbar, Alert, AlertColor } from '@mui/material';

interface Notice { id: number; msg: string; severity: AlertColor; }
interface NotifyCtx {
  success: (m: string) => void;
  error: (m: string) => void;
  info: (m: string) => void;
  warning: (m: string) => void;
}

const Ctx = createContext<NotifyCtx | undefined>(undefined);

export function NotifyProvider({ children }: { children: ReactNode }) {
  const [queue, setQueue] = useState<Notice[]>([]);
  const push = useCallback((severity: AlertColor, msg: string) => {
    setQueue(q => [...q, { id: Date.now() + Math.random(), msg, severity }]);
  }, []);
  const api: NotifyCtx = {
    success: m => push('success', m),
    error: m => push('error', m),
    info: m => push('info', m),
    warning: m => push('warning', m)
  };
  const current = queue[0];
  const handleClose = () => setQueue(q => q.slice(1));
  return (
    <Ctx.Provider value={api}>
      {children}
      <Snackbar open={!!current} autoHideDuration={3500} onClose={handleClose} anchorOrigin={{ vertical:'bottom', horizontal:'right' }}>
        {current && <Alert severity={current.severity} onClose={handleClose} variant="filled">{current.msg}</Alert>}
      </Snackbar>
    </Ctx.Provider>
  );
}

export function useNotify() {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error('NotifyContext missing');
  return ctx;
}
