import { Grid, Paper, Typography, Box, Chip, Stack } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';
import dayjs from 'dayjs';

interface StatusResp {
  load_balancers: Record<string, { url: string; healthy: boolean; connections: number; weight: number; }[]>;
}

export default function OverviewPage() {
  const { data: health } = useQuery(['health'], async () => (await axios.get('/health')).data, { refetchInterval: 10000 });
  const { data: status } = useQuery<StatusResp>(['status'], async () => (await axios.get('/admin/status')).data, { refetchInterval: 15000 });

  return (
    <Stack spacing={3}>
      <Typography variant="h5">概览</Typography>
      <Grid container spacing={2}>
        <Grid item xs={12} md={6} lg={4}>
          <Paper sx={{ p:2 }}>
            <Typography variant="subtitle2" gutterBottom>网关健康</Typography>
            <Typography variant="h4" color={health?.status === 'healthy' ? 'success.main' : 'error.main'}>{health?.status || '...'}</Typography>
            <Typography variant="caption" display="block" sx={{ mt:1 }}>更新时间: {dayjs().format('HH:mm:ss')}</Typography>
          </Paper>
        </Grid>
        <Grid item xs={12} md={6} lg={8}>
          <Paper sx={{ p:2 }}>
            <Typography variant="subtitle2" gutterBottom>负载均衡路由</Typography>
            <Box sx={{ display:'flex', flexWrap:'wrap', gap:1 }}>
              {status && Object.entries(status.load_balancers).map(([path, arr]) => (
                <Chip key={path} label={`${path} (${arr.length})`} color="primary" variant="outlined" />
              ))}
            </Box>
          </Paper>
        </Grid>
      </Grid>
    </Stack>
  );
}
