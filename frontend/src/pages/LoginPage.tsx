import { Box, Paper, Typography, TextField, Button, Stack, Alert } from '@mui/material';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export default function LoginPage({ onToggleTheme }: { onToggleTheme: () => void }) {
  const { login } = useAuth();
  const nav = useNavigate();
  const [u, setU] = useState('admin');
  const [p, setP] = useState('password123');
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    const ok = await login(u, p);
    setLoading(false);
    if (ok) nav('/'); else setErr('登录失败');
  };

  return (
    <Box sx={{ minHeight:'100vh', display:'flex', alignItems:'center', justifyContent:'center', p:2 }}>
      <Paper elevation={4} sx={{ p:5, width:380 }}>
        <Stack spacing={3}>
          <Typography variant="h5" textAlign="center">API Gateway 登录</Typography>
          {err && <Alert severity="error">{err}</Alert>}
          <form onSubmit={submit}>
            <Stack spacing={2}>
              <TextField label="用户名" value={u} onChange={e=>setU(e.target.value)} fullWidth />
              <TextField label="密码" type="password" value={p} onChange={e=>setP(e.target.value)} fullWidth />
              <Button variant="contained" type="submit" disabled={loading}>登录</Button>
              <Button size="small" onClick={onToggleTheme}>切换主题</Button>
            </Stack>
          </form>
        </Stack>
      </Paper>
    </Box>
  );
}
