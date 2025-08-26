import { DataGrid, GridColDef } from '@mui/x-data-grid';
import { Paper, Typography, Stack, Chip } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';

interface BackendRow { id: string; path: string; url: string; healthy: boolean; connections: number; weight: number; }

export default function BackendsPage() {
  const { data } = useQuery<Record<string, { url: string; healthy: boolean; connections: number; weight: number; last_check: string; }[]>>(['backends'], async () => (await axios.get('/admin/backends')).data, { refetchInterval: 15000 });

  const rows: BackendRow[] = data ? Object.entries(data).flatMap(([path, arr]) => arr.map((b, idx) => ({ id: path + ':' + idx + ':' + b.url, path, url: b.url, healthy: b.healthy, connections: b.connections, weight: b.weight }))) : [];

  const cols: GridColDef[] = [
    { field: 'path', headerName: '路由', width: 180 },
    { field: 'url', headerName: '后端URL', flex: 1, minWidth: 240 },
    { field: 'healthy', headerName: '健康', width: 100, renderCell: p => <Chip size="small" label={p.value ? '健康' : '异常'} color={p.value ? 'success' : 'error'} /> },
    { field: 'connections', headerName: '连接数', width: 110 },
    { field: 'weight', headerName: '权重', width: 90 },
  ];

  return (
    <Stack spacing={3}>
      <Typography variant="h5">后端服务</Typography>
      <Paper sx={{ height: 520, p:1 }}>
        <DataGrid rows={rows} columns={cols} disableRowSelectionOnClick />
      </Paper>
    </Stack>
  );
}
