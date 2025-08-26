import { CssBaseline, ThemeProvider, createTheme } from '@mui/material';
import { useMemo, useState } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import DashboardLayout from './components/layout/DashboardLayout';
import LoginPage from './pages/LoginPage';
import OverviewPage from './pages/OverviewPage';
import BackendsPage from './pages/BackendsPage';
import MetricsPage from './pages/MetricsPage';
import NotFoundPage from './pages/NotFoundPage';
import { AuthProvider, useAuth } from './context/AuthContext';

function PrivateRoute({ children }: { children: JSX.Element }) {
  const { token } = useAuth();
  if (!token) return <Navigate to="/login" replace />;
  return children;
}

export default function App() {
  const [dark, setDark] = useState(true);
  const theme = useMemo(() => createTheme({
    palette: { mode: dark ? 'dark' : 'light', primary: { main: '#1976d2' }, secondary: { main: '#8e24aa' } },
    shape: { borderRadius: 10 },
    typography: { fontFamily: 'Inter, Roboto, Helvetica, Arial, sans-serif' }
  }), [dark]);

  return (
    <AuthProvider>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Routes>
          <Route path="/login" element={<LoginPage onToggleTheme={() => setDark(d => !d)} />} />
          <Route element={<DashboardLayout onToggleTheme={() => setDark(d => !d)} dark={dark} />}>
            <Route index element={<PrivateRoute><OverviewPage /></PrivateRoute>} />
            <Route path="backends" element={<PrivateRoute><BackendsPage /></PrivateRoute>} />
            <Route path="metrics" element={<PrivateRoute><MetricsPage /></PrivateRoute>} />
          </Route>
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </ThemeProvider>
    </AuthProvider>
  );
}
