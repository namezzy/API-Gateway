import { Paper, Typography, Stack, Box, TextField, Button, CircularProgress } from '@mui/material';
import { useEffect, useState } from 'react';
import axios from 'axios';
import { api } from '../lib/http';
import { LineChart, Line, XAxis, YAxis, Tooltip as RTooltip, ResponsiveContainer, Legend } from 'recharts';

interface MetricPoint { ts: number; requests: number; errors: number; }

interface PromResult {
  metric: Record<string, string>;
  value: [number, string];
}

export default function MetricsPage() {
  const [data, setData] = useState<MetricPoint[]>([]);
  const [query, setQuery] = useState('up');
  const [querying, setQuerying] = useState(false);
  const [results, setResults] = useState<PromResult[] | null>(null);
  const [qErr, setQErr] = useState<string>('');

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
      <Paper sx={{ p:2 }}>
        <Stack direction={{ xs:'column', sm:'row' }} spacing={2} alignItems="center" mb={2}>
          <TextField fullWidth label="PromQL 查询" value={query} onChange={e=>setQuery(e.target.value)} size="small" />
          <Button variant="contained" disabled={querying} onClick={async ()=>{
            setQuerying(true); setQErr('');
            try {
              const res = await api.get('http://localhost:9091/api/v1/query', { params: { query } });
              if (res.data.status === 'success') {
                setResults(res.data.data.result as PromResult[]);
              } else {
                setQErr(JSON.stringify(res.data));
              }
            } catch (e:any) {
              setQErr(e.message || '查询失败');
            } finally { setQuerying(false); }
          }}>{querying ? <CircularProgress size={20} /> : '执行'}</Button>
        </Stack>
        {qErr && <Typography color="error" variant="body2" mb={1}>{qErr}</Typography>}
        {results && results.length === 0 && <Typography variant="body2" sx={{ opacity:.7 }}>无结果</Typography>}
        <Stack spacing={1} maxHeight={240} sx={{ overflow:'auto' }}>
          {results && results.map((r,i)=>{
            const val = parseFloat(r.value[1]);
            return <Box key={i} sx={{ fontFamily:'monospace', fontSize:13, display:'flex', justifyContent:'space-between', border:'1px solid', borderColor:'divider', borderRadius:1, p:1 }}>
              <span>{Object.entries(r.metric).map(([k,v])=>`${k}="${v}"`).join(', ')}</span>
              <b>{val}</b>
            </Box>;
          })}
        </Stack>
      </Paper>
    </Stack>
  );
}
