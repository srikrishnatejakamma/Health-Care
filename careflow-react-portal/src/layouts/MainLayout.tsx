import {
  Avatar,
  Badge,
  Box,
  Button,
  Chip,
  Dialog,
  DialogContent,
  Drawer,
  IconButton,
  InputAdornment,
  ListItemIcon,
  Menu,
  MenuItem,
  Snackbar,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  Activity,
  ArrowRight,
  Bell,
  Building2,
  CalendarDays,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleHelp,
  ClipboardList,
  Database,
  Eye,
  EyeOff,
  FileCheck2,
  FileText,
  GitBranch,
  HeartPulse,
  LayoutDashboard,
  ListChecks,
  LogOut,
  Menu as MenuIcon,
  Moon,
  Pill,
  Search,
  Settings2,
  ShieldCheck,
  Sun,
  Timer,
  Users,
  Wallet,
  Workflow,
  X,
  Zap,
  Cpu,
  MemoryStick,
  Radio,
  Webhook,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useSession } from '../stores/session';
import { useCollection } from '../shared/hooks';
import { useRealtime } from '../services/realtime';
import { changeConnection, queryClient } from '../services/apiClient';
import { demoMe } from '../services/demoData';
import { hasRole, type Role } from '../shared/types';
import { displayName, label } from '../shared/format';
import { Person } from '../shared/components';

interface NavItem {
  path: string;
  title: string;
  icon: LucideIcon;
  roles?: Role[];
  tag?: string;
}
const clinical: Role[] = ['tenant_admin', 'clinical'];
const workflowRoles: Role[] = ['tenant_admin', 'clinical', 'billing', 'integration'];
const operations: Role[] = ['tenant_admin', 'auditor'];
export const navigation: { title: string; items: NavItem[] }[] = [
  { title: 'WORKSPACE', items: [{ path: '/dashboard', title: 'Overview', icon: LayoutDashboard }] },
  {
    title: 'CARE OPERATIONS',
    items: [
      { path: '/patients', title: 'Patients', icon: Users, roles: clinical },
      { path: '/therapy', title: 'Therapy orders', icon: ClipboardList, roles: workflowRoles },
      { path: '/prescriptions', title: 'Prescriptions', icon: FileText, roles: clinical },
      {
        path: '/insurance',
        title: 'Authorizations',
        icon: ShieldCheck,
        roles: ['tenant_admin', 'clinical', 'integration'],
      },
      {
        path: '/pharmacy',
        title: 'Pharmacy',
        icon: Pill,
        roles: ['tenant_admin', 'clinical', 'integration'],
      },
      { path: '/appointments', title: 'Appointments', icon: CalendarDays, roles: clinical },
      {
        path: '/billing',
        title: 'Billing & claims',
        icon: Wallet,
        roles: ['tenant_admin', 'billing', 'integration'],
      },
    ],
  },
  {
    title: 'ORCHESTRATION',
    items: [
      { path: '/workflows', title: 'Workflows', icon: Workflow, roles: workflowRoles },
      {
        path: '/designer',
        title: 'Workflow designer',
        icon: GitBranch,
        roles: ['tenant_admin', 'integration'],
      },
      { path: '/scheduler', title: 'Job scheduler', icon: Timer, roles: operations },
    ],
  },
  {
    title: 'INFRASTRUCTURE',
    items: [
      { path: '/databases', title: 'Databases', icon: Database, roles: operations },
      { path: '/concurrency', title: 'Worker pools', icon: Cpu, roles: operations },
      { path: '/memory', title: 'Memory', icon: MemoryStick, roles: operations },
      { path: '/events', title: 'Kafka events', icon: Radio, roles: operations },
      { path: '/callbacks', title: 'Callbacks', icon: Webhook, roles: operations },
      { path: '/health', title: 'System health', icon: Activity, roles: operations },
    ],
  },
  {
    title: 'ADMINISTRATION',
    items: [
      { path: '/configuration', title: 'Configuration', icon: Settings2, roles: ['tenant_admin'] },
      { path: '/users', title: 'Users & access', icon: Users, roles: ['tenant_admin'] },
      { path: '/audit', title: 'Audit logs', icon: ListChecks, roles: operations },
      { path: '/alerts', title: 'Alerts', icon: Bell },
    ],
  },
];
export function MainLayout() {
  const s = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const [mobile, setMobile] = useState(false);
  const [tenantMenu, setTenantMenu] = useState<HTMLElement | null>(null);
  const [profile, setProfile] = useState<HTMLElement | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [notifications, setNotifications] = useState(false);
  const facilities = useCollection('facilities');
  const patients = useCollection('patients');
  const orders = useCollection('orders');
  const workflows = useCollection('workflows');
  const jobs = useCollection('jobs');
  const realtime = useRealtime();
  const me = s.me!;
  const current = navigation
    .flatMap((x) => x.items)
    .find((x) => location.pathname === x.path || location.pathname.startsWith(x.path + '/'));
  const alerts = [
    ...jobs.items
      .filter((j) => j.status === 'dead')
      .map((j) => ({
        id: j.id,
        title: 'Job needs attention',
        detail: `${label(j.kind)} reached its retry limit.`,
        path: '/scheduler',
      })),
    ...workflows.items
      .filter((w) => w.status === 'rejected')
      .map((w) => ({
        id: w.id,
        title: 'Therapy prerequisite rejected',
        detail: 'Review the recorded workflow outcomes.',
        path: `/workflows/${w.id}`,
      })),
  ];
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setSearchOpen((v) => !v);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);
  useEffect(() => {
    setMobile(false);
    setSearch('');
    setSearchOpen(false);
  }, [location.pathname]);
  useEffect(() => {
    if (s.mode !== 'live' || !s.token) return;
    try {
      const payload = JSON.parse(atob(s.token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
      if (!payload.exp) return;
      const timer = setTimeout(
        () => {
          void changeConnection('signedout', '', null);
          s.notify('Your session expired. Sign in to continue.');
        },
        Math.max(0, Math.min(2 ** 31 - 1, payload.exp * 1000 - Date.now())),
      );
      return () => clearTimeout(timer);
    } catch {
      /* Backend validation remains authoritative. */
    }
  }, [s.token, s.mode]);
  const switchDemo = async (tenant: string) => {
    await changeConnection('demo', '', demoMe(tenant, me.roles[0]));
    setTenantMenu(null);
    navigate('/dashboard');
  };
  const sidebar = (
    <Box className={`sidebar ${s.sidebarCollapsed ? 'collapsed' : ''}`}>
      <NavLink to="/dashboard" className="brand">
        <Box className="brand-mark">
          <HeartPulse size={23} />
        </Box>
        {!s.sidebarCollapsed && (
          <Box>
            <span className="brand-name">
              CareFlow<span>®</span>
            </span>
            <span className="brand-subtitle">ENTERPRISE</span>
          </Box>
        )}
      </NavLink>
      <Box className="sidebar-scroll">
        {navigation.map((group) => {
          const items = group.items.filter((item) => !item.roles || hasRole(me.roles, ...item.roles));
          if (!items.length) return null;
          return (
            <Box key={group.title} className="nav-group">
              {!s.sidebarCollapsed && <Typography className="nav-group-title">{group.title}</Typography>}
              {items.map((item) => (
                <Tooltip key={item.path} title={s.sidebarCollapsed ? item.title : ''} placement="right">
                  <span className="nav-tooltip">
                    <NavLink
                      to={item.path}
                      className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}
                    >
                      <item.icon size={18} strokeWidth={1.65} />
                      {!s.sidebarCollapsed && (
                        <>
                          <span>{item.title}</span>
                          {item.path === '/insurance' &&
                            workflows.items.filter((w) => w.status === 'awaiting_approvals').length > 0 && (
                              <span className="nav-count">
                                {workflows.items.filter((w) => w.status === 'awaiting_approvals').length}
                              </span>
                            )}
                        </>
                      )}
                    </NavLink>
                  </span>
                </Tooltip>
              ))}
            </Box>
          );
        })}
      </Box>
      <Box className="sidebar-footer">
        {!s.sidebarCollapsed && (
          <Box className="workspace-status">
            <span className={`status-dot ${s.mode === 'demo' ? 'demo' : ''}`} />
            <span>
              {s.mode === 'demo'
                ? 'Synthetic demo workspace'
                : realtime === 'live'
                  ? 'Workspace connected'
                  : 'Reconnecting…'}
            </span>
          </Box>
        )}
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Tooltip title="Help and workspace guide">
            <IconButton size="small" onClick={() => navigate('/guide')} aria-label="Help and workspace guide">
              <CircleHelp size={18} />
            </IconButton>
          </Tooltip>
          <Tooltip title={s.sidebarCollapsed ? 'Expand navigation' : 'Collapse navigation'}>
            <IconButton
              size="small"
              onClick={s.toggleSidebar}
              aria-label={s.sidebarCollapsed ? 'Expand navigation' : 'Collapse navigation'}
            >
              {s.sidebarCollapsed ? <ChevronRight size={18} /> : <ChevronLeft size={18} />}
            </IconButton>
          </Tooltip>
        </Stack>
      </Box>
    </Box>
  );
  const searchResults = [
    ...patients.items
      .filter((p) => !s.maskPhi && `${p.name} ${p.external_id}`.toLowerCase().includes(search.toLowerCase()))
      .map((p) => ({
        id: p.id,
        name: displayName(p.name, s.maskPhi),
        sub: `Patient · ${p.external_id}`,
        path: `/patients/${p.id}`,
      })),
    ...orders.items
      .filter((o) => `${o.medication} ${o.id}`.toLowerCase().includes(search.toLowerCase()))
      .map((o) => ({
        id: o.id,
        name: o.medication,
        sub: `Therapy order · ${o.id.slice(0, 12)}`,
        path: `/therapy/${o.id}`,
      })),
  ].slice(0, 12);
  return (
    <Box className={`app-shell ${s.sidebarCollapsed ? 'nav-collapsed' : ''}`}>
      <a href="#main-content" className="skip-link">
        Skip to main content
      </a>
      <Box className="desktop-sidebar">{sidebar}</Box>
      <Drawer open={mobile} onClose={() => setMobile(false)} sx={{ '& .MuiDrawer-paper': { width: 254 } }}>
        {sidebar}
      </Drawer>
      <Box className="workspace">
        <Box component="header" className="topbar">
          <Stack direction="row" gap={1.4} alignItems="center">
            <IconButton className="mobile-menu" onClick={() => setMobile(true)} aria-label="Open navigation">
              <MenuIcon size={20} />
            </IconButton>
            <Button
              className="tenant-selector"
              onClick={(e) => setTenantMenu(e.currentTarget)}
              startIcon={
                <Box className="tenant-icon">
                  <Building2 size={17} />
                </Box>
              }
              endIcon={<ChevronDown size={13} />}
            >
              <Box textAlign="left">
                <Typography variant="body2" fontWeight={600}>
                  {me.tenant.name}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {s.mode === 'demo' ? 'Demo organization' : me.tenant.region}
                </Typography>
              </Box>
            </Button>
            <Box className="header-divider" />
            <Typography className="header-breadcrumb" variant="body2" color="text.secondary">
              Workspace <ChevronRight size={12} /> <span>{current?.title || 'Workspace guide'}</span>
            </Typography>
          </Stack>
          <Stack direction="row" spacing={1.2} alignItems="center">
            <Button
              className="global-search-trigger"
              onClick={() => setSearchOpen(true)}
              startIcon={<Search size={16} />}
            >
              <span>Search workspace</span>
              <kbd>⌘ K</kbd>
            </Button>
            <Chip
              className="environment-chip"
              label={s.mode === 'demo' ? 'DEMO' : 'LIVE'}
              size="small"
              variant="outlined"
            />
            <Tooltip title={s.maskPhi ? 'Reveal patient details' : 'Hide patient details'}>
              <IconButton
                onClick={s.toggleMask}
                aria-label={s.maskPhi ? 'Reveal patient details' : 'Hide patient details'}
              >
                {s.maskPhi ? <EyeOff size={18} /> : <Eye size={18} />}
              </IconButton>
            </Tooltip>
            <Tooltip title={s.theme === 'light' ? 'Switch to dark theme' : 'Switch to light theme'}>
              <IconButton onClick={s.setTheme} aria-label="Toggle theme">
                {s.theme === 'light' ? <Moon size={18} /> : <Sun size={18} />}
              </IconButton>
            </Tooltip>
            <IconButton onClick={() => setNotifications(true)} aria-label="Open notifications">
              <Badge badgeContent={alerts.length} color="error" variant={alerts.length ? 'dot' : 'standard'}>
                <Bell size={19} />
              </Badge>
            </IconButton>
            <IconButton onClick={(e) => setProfile(e.currentTarget)} aria-label="Open user menu">
              <Avatar
                sx={{
                  width: 32,
                  height: 32,
                  fontSize: 11,
                  fontWeight: 600,
                  bgcolor: '#edf0fa',
                  color: '#4664a0',
                }}
              >
                AM
              </Avatar>
            </IconButton>
          </Stack>
        </Box>
        <Box component="main" id="main-content" tabIndex={-1} className="main-content">
          <Outlet />
        </Box>
        <Stack component="footer" className="main-footer" direction="row" justifyContent="space-between">
          <Typography variant="caption">
            CareFlow Enterprise <span>·</span> Care, connected.
          </Typography>
          <Stack direction="row" gap={1} alignItems="center">
            <ShieldCheck size={12} />
            <Typography variant="caption">
              {s.mode === 'demo' ? 'Synthetic data only' : 'Tenant access enforced'} <span>·</span>{' '}
              {me.tenant.timezone}
            </Typography>
          </Stack>
        </Stack>
      </Box>
      <Menu anchorEl={tenantMenu} open={!!tenantMenu} onClose={() => setTenantMenu(null)}>
        {s.mode === 'demo' ? (
          ['north-health', 'south-health'].map((id) => (
            <MenuItem key={id} selected={id === s.tenantId} onClick={() => void switchDemo(id)}>
              <ListItemIcon>
                <Building2 size={17} />
              </ListItemIcon>
              {demoMe(id).tenant.name}
            </MenuItem>
          ))
        ) : (
          <MenuItem disabled>{me.tenant.name}</MenuItem>
        )}
        <MenuItem
          onClick={() => {
            setTenantMenu(null);
            void changeConnection('signedout', '', null);
          }}
        >
          <ListItemIcon>
            <LogOut size={17} />
          </ListItemIcon>
          Connect another workspace
        </MenuItem>
      </Menu>
      <Menu anchorEl={profile} open={!!profile} onClose={() => setProfile(null)}>
        <Box sx={{ px: 2, py: 1.3 }}>
          <Typography fontWeight={600}>{s.mode === 'demo' ? 'Alex Morgan' : me.user_id}</Typography>
          <Typography variant="caption" color="text.secondary">
            {label(me.roles[0])}
          </Typography>
        </Box>
        {s.mode === 'demo' &&
          (['tenant_admin', 'clinical', 'billing', 'integration', 'auditor'] as Role[]).map((role) => (
            <MenuItem
              key={role}
              selected={me.roles.includes(role)}
              onClick={() => {
                void changeConnection('demo', '', demoMe(s.tenantId, role));
                setProfile(null);
                navigate('/dashboard');
              }}
            >
              Preview {label(role)} role
            </MenuItem>
          ))}
        <MenuItem
          onClick={() => {
            setProfile(null);
            void changeConnection('signedout', '', null);
          }}
        >
          <ListItemIcon>
            <LogOut size={16} />
          </ListItemIcon>
          Sign out
        </MenuItem>
      </Menu>
      <Dialog open={searchOpen} onClose={() => setSearchOpen(false)} fullWidth maxWidth="sm">
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            placeholder="Search patients or therapy orders…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={18} />
                  </InputAdornment>
                ),
              },
              htmlInput: { 'aria-label': 'Search workspace records' },
            }}
          />
          <Typography variant="caption" color="text.secondary" display="block" sx={{ my: 1.5 }}>
            Searches authorized, loaded records in this workspace.
          </Typography>
          {search.trim() &&
            searchResults.map((result) => (
              <Button
                key={result.id}
                fullWidth
                onClick={() => navigate(result.path)}
                sx={{ justifyContent: 'space-between', p: 1.5, textAlign: 'left' }}
              >
                <Person name={result.name} sub={result.sub} small />
                <ArrowRight size={15} />
              </Button>
            ))}
          {search.trim() && !searchResults.length && (
            <Typography color="text.secondary" sx={{ p: 3, textAlign: 'center' }}>
              No matching records.
            </Typography>
          )}
        </DialogContent>
      </Dialog>
      <Drawer
        anchor="right"
        open={notifications}
        onClose={() => setNotifications(false)}
        sx={{ '& .MuiDrawer-paper': { width: { xs: '100%', sm: 380 }, p: 3 } }}
      >
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Typography variant="h2">Notifications</Typography>
          <IconButton aria-label="Close notifications" onClick={() => setNotifications(false)}>
            <X size={18} />
          </IconButton>
        </Stack>
        <Typography color="text.secondary" sx={{ mt: 1, mb: 3 }}>
          Items that may need your attention.
        </Typography>
        {alerts.length ? (
          alerts.map((a) => (
            <Box key={a.id} className="notification-item">
              <Box className="notification-icon">
                <Bell size={17} />
              </Box>
              <Box>
                <Typography variant="body2" fontWeight={600}>
                  {a.title}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {a.detail}
                </Typography>
                <Button
                  size="small"
                  sx={{ display: 'block', px: 0 }}
                  onClick={() => {
                    setNotifications(false);
                    navigate(a.path);
                  }}
                >
                  Review <ArrowRight size={12} />
                </Button>
              </Box>
            </Box>
          ))
        ) : (
          <Typography color="text.secondary">You're all caught up.</Typography>
        )}
        <Button
          onClick={() => {
            setNotifications(false);
            navigate('/alerts');
          }}
          sx={{ mt: 2 }}
        >
          View all alerts
        </Button>
      </Drawer>
      <Snackbar
        open={!!s.notice}
        autoHideDuration={6000}
        onClose={() => s.notify('')}
        message={s.notice}
        action={
          <IconButton color="inherit" aria-label="Dismiss notification" onClick={() => s.notify('')}>
            <X size={16} />
          </IconButton>
        }
      />
    </Box>
  );
}
