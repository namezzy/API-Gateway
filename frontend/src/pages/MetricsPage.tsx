import { Paper, Typography, Stack, Box } from '@mui/material';
import { useEffect, useState } from 'react';
import axios from 'axios';
import { LineChart, Line, XAxis, YAxis, Tooltip as RTooltip, ResponsiveContainer, Legend } from 'recharts';

interface MetricPoint { ts: number; requests: number; errors: number; }

export default function MetricsPage() {
  const [data, setData] = useState<MetricPoint[]>([]);

  useEffect(() => {
    const id = setInterval(async () => {
      try {
        const res = await axios.get('/metrics');
        const text: string = res.data || '';
        const lines = text.split('\n');
        const reqLine = lines.find(l => l.startsWith('http_requests_total') && l.includes('status_code="200"'));
        const errLine = lines.find(l => l.startsWith('http_requests_total') && l.includes('status_code="500"'));
        const parseVal = (ln?: string) => ln ? parseFloat((ln.split(' ')).pop() || '0') : 0;
        const point: MetricPoint = { ts: Date.now(), requests: parseVal(reqLine), errors: parseVal(errLine) };
        setData(d => [...d.slice(-50), point]);
      } catch {}
    }, 4000);
    return () => clearInterval(id);
  }, []);

  return (
    <Stack spacing={3}>
      <Typography variant="h5">指标</Typography>
      <Paper sx={{ p:2, height:420 }}>
        <Typography variant="subtitle2" gutterBottom>HTTP 请求趋势 (示例解析)</Typography>
        <Box sx={{ height:360 }}>
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ top:10, right:20, left:0, bottom:0 }}>
              <XAxis dataKey="ts" tickFormatter={v=>new Date(v).toLocaleTimeString()} minTickGap={50} />
              <YAxis />
              <RTooltip labelFormatter={v=>new Date(v as number).toLocaleTimeString()} />
              <Legend />
              <Line type="monotone" dataKey="requests" stroke="#4caf50" dot={false} strokeWidth={2} />
              <Line type="monotone" dataKey="errors" stroke="#f44336" dot={false} strokeWidth={2} />
            </LineChart>
          </ResponsiveContainer>
        </Box>
      </Paper>
    </Stack>
  );
}
