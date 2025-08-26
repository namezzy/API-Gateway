import { AppBar, Toolbar, Typography, IconButton, Drawer, List, ListItemButton, ListItemText, Box, CssBaseline, Divider, Tooltip, Avatar, Popover, Stack, TextField, Button } from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';
import DashboardIcon from '@mui/icons-material/Dashboard';
import StorageIcon from '@mui/icons-material/Storage';
import LanIcon from '@mui/icons-material/Lan';
import LogoutIcon from '@mui/icons-material/Logout';
import DarkModeIcon from '@mui/icons-material/DarkMode';
import LightModeIcon from '@mui/icons-material/LightMode';
import { useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../../context/AuthContext';

const drawerWidth = 220;

export default function DashboardLayout({ onToggleTheme, dark, setPrimary, setSecondary, primary, secondary }: { onToggleTheme: () => void; dark: boolean; setPrimary: (c: string) => void; setSecondary: (c: string) => void; primary: string; secondary: string; }) {
  const [open, setOpen] = useState(true);
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const nav = useNavigate();
  const loc = useLocation();
  const { logout, token } = useAuth();

  const go = (path: string) => () => nav(path);
  const active = (p: string) => loc.pathname === '/' + p || loc.pathname === '/' + p + '/';

  return (
    <Box sx={{ display: 'flex' }}>
      <CssBaseline />
      <AppBar position="fixed" sx={{ zIndex: (t) => t.zIndex.drawer + 1 }}>
        <Toolbar>
          <IconButton color="inherit" edge="start" onClick={() => setOpen(o => !o)} sx={{ mr:2 }}><MenuIcon /></IconButton>
          <Typography variant="h6" sx={{ flexGrow: 1 }}>API Gateway Dashboard</Typography>
          <Tooltip title="切换主题"><IconButton color="inherit" onClick={onToggleTheme}>{dark ? <LightModeIcon /> : <DarkModeIcon />}</IconButton></Tooltip>
          <Tooltip title="品牌颜色"><IconButton color="inherit" onClick={(e)=>setAnchorEl(e.currentTarget)}>🎨</IconButton></Tooltip>
          {token && <Tooltip title="退出"><IconButton color="inherit" onClick={logout}><LogoutIcon /></IconButton></Tooltip>}
        </Toolbar>
      </AppBar>
      <Drawer variant="permanent" open={open} sx={{ width: drawerWidth, flexShrink: 0, [`& .MuiDrawer-paper`]: { width: drawerWidth, boxSizing: 'border-box' } }}>
        <Toolbar />
        <Box sx={{ overflow: 'auto' }}>
          <List>
            <ListItemButton selected={active('')} onClick={go('')}><DashboardIcon sx={{ mr:1 }} /> <ListItemText primary="概览" /></ListItemButton>
            <ListItemButton selected={active('backends')} onClick={go('backends')}><LanIcon sx={{ mr:1 }} /> <ListItemText primary="后端服务" /></ListItemButton>
            <ListItemButton selected={active('metrics')} onClick={go('metrics')}><StorageIcon sx={{ mr:1 }} /> <ListItemText primary="指标" /></ListItemButton>
          </List>
          <Divider />
          <Box sx={{ p:2, display:'flex', alignItems:'center', gap:1, opacity:.8 }}>
            <Avatar sx={{ width:32, height:32 }}>U</Avatar>
            <Typography variant="body2">登录中</Typography>
          </Box>
        </Box>
      </Drawer>
      <Box component="main" sx={{ flexGrow:1, p:3 }}>
        <Toolbar />
        <Outlet />
      </Box>
      <Popover open={!!anchorEl} anchorEl={anchorEl} onClose={()=>setAnchorEl(null)} anchorOrigin={{ vertical:'bottom', horizontal:'right' }}>
        <Box sx={{ p:2, width:260 }}>
          <Typography variant="subtitle2" gutterBottom>品牌自定义</Typography>
          <Stack spacing={2}>
            <TextField size="small" label="Primary" value={primary} onChange={e=>setPrimary(e.target.value)} />
            <TextField size="small" label="Secondary" value={secondary} onChange={e=>setSecondary(e.target.value)} />
            <Button size="small" onClick={()=>{ setPrimary('#1976d2'); setSecondary('#8e24aa'); }}>重置</Button>
          </Stack>
        </Box>
      </Popover>
    </Box>
  );
}
