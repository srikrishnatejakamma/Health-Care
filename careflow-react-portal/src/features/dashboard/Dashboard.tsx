import {
  Box,
  Button,
  Card,
  Chip,
  IconButton,
  LinearProgress,
  MenuItem,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  ArrowRight,
  ArrowUpRight,
  CalendarDays,
  ChevronRight,
  CircleCheck,
  ClipboardList,
  Clock3,
  GitBranch,
  MoreHorizontal,
  Plus,
  RefreshCw,
  ShieldCheck,
  TriangleAlert,
  Users,
} from 'lucide-react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip as ChartTooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useCollection, useOverview } from '../../shared/hooks';
import {
  EmptyState,
  ErrorState,
  LoadingState,
  MetricCard,
  PageHeader,
  Panel,
  Person,
  StatusBadge,
} from '../../shared/components';
import { EntityForm, type FormKind } from '../../shared/EntityForm';
import { useSession } from '../../stores/session';
import { date, displayName, time } from '../../shared/format';
import { hasRole } from '../../shared/types';
import { queryClient } from '../../services/apiClient';

export default function Dashboard() {
  const [days, setDays] = useState(7);
  const [tab, setTab] = useState('active');
  const [form, setForm] = useState<FormKind | null>(null);
  const s = useSession();
  const navigate = useNavigate();
  const summary = useOverview(days);
  const patients = useCollection('patients');
  const orders = useCollection('orders');
  const appointments = useCollection('appointments');
  const facilities = useCollection('facilities');
  const workflows = useCollection('workflows');
  const me = s.me!;
  const name = (id: string) => displayName(patients.all.find((p) => p.id === id)?.name, s.maskPhi);
  if (summary.isLoading) return <LoadingState />;
  if (summary.error) return <ErrorState error={summary.error} retry={() => void summary.refetch()} />;
  const data = summary.data!;
  const c = data.counts;
  const active = orders.items.filter((o) => !['completed', 'cancelled', 'rejected'].includes(o.status));
  const recent = (
    tab === 'active'
      ? active
      : tab === 'attention'
        ? orders.items.filter((o) => ['rejected', 'awaiting_approvals'].includes(o.status))
        : orders.items
  )
    .slice()
    .sort((a, b) => b.created_at.localeCompare(a.created_at))
    .slice(0, 5);
  const upcoming = appointments.items
    .filter((a) => a.status === 'confirmed' && new Date(a.ends_at) >= new Date())
    .sort((a, b) => a.starts_at.localeCompare(b.starts_at))
    .slice(0, 3);
  const totals = data.statuses.reduce((sum, item) => sum + item.count, 0);
  const completed = data.statuses.find((x) => x.name === 'completed')?.count || 0;
  const success = totals ? Math.round((completed / totals) * 100) : 0;
  const pending = c.pending_authorizations || 0;
  return (
    <>
      <PageHeader
        eyebrow="YOUR WORKSPACE, AT A GLANCE"
        title="Operations overview"
        description="A clear view of your care teams, therapies, and workflows."
        actions={
          <>
            <Tooltip title="Refresh all workspace data">
              <IconButton
                className="outlined-icon"
                onClick={() => void queryClient.invalidateQueries()}
                aria-label="Refresh dashboard"
              >
                <RefreshCw size={16} />
              </IconButton>
            </Tooltip>
            {hasRole(me.roles, 'tenant_admin', 'clinical') && (
              <Button variant="contained" startIcon={<Plus size={16} />} onClick={() => setForm('order')}>
                New therapy order
              </Button>
            )}
          </>
        }
      />
      <Stack
        className="overview-toolbar"
        direction={{ xs: 'column', sm: 'row' }}
        justifyContent="space-between"
        gap={1.5}
      >
        <Stack direction="row" spacing={1} alignItems="center" color="text.secondary">
          <CalendarDays size={15} />
          <Typography variant="body2">
            {new Intl.DateTimeFormat('en', {
              weekday: 'long',
              month: 'long',
              day: 'numeric',
              year: 'numeric',
            }).format(new Date())}
          </Typography>
          <span className="toolbar-date-dot">·</span>
          <Typography variant="caption" className="updated-label">
            Updated {time(data.generated_at)}
          </Typography>
        </Stack>
        <Stack direction="row" gap={1}>
          <TextField
            select
            value={s.facility}
            onChange={(e) => s.setFacility(e.target.value)}
            className="compact-select"
            slotProps={{ htmlInput: { 'aria-label': 'Filter by facility' } }}
          >
            <MenuItem value="all">All facilities</MenuItem>
            {facilities.all.map((f) => (
              <MenuItem key={f.id} value={f.id}>
                {f.name}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
            className="compact-select"
            slotProps={{ htmlInput: { 'aria-label': 'Dashboard date range' } }}
          >
            {[7, 30, 90].map((n) => (
              <MenuItem key={n} value={n}>
                Last {n} days
              </MenuItem>
            ))}
          </TextField>
        </Stack>
      </Stack>
      <Box className="kpi-grid">
        <MetricCard
          title="Active therapy orders"
          value={c.active_orders ?? '—'}
          detail={`${c.completed_orders ?? 0} completed across this workspace`}
          icon={<ClipboardList size={18} />}
          tone="blue"
        />
        <MetricCard
          title="Scheduled appointments"
          value={c.appointments ?? '—'}
          detail="Confirmed and ready for care"
          icon={<CalendarDays size={18} />}
          tone="teal"
        />
        <MetricCard
          title="Pending authorizations"
          value={c.pending_authorizations ?? '—'}
          detail={pending ? 'Awaiting insurance review' : 'No pending insurance reviews'}
          icon={<ShieldCheck size={18} />}
          tone="amber"
        />
        <MetricCard
          title="Active workflows"
          value={c.active_workflows ?? '—'}
          detail={`${c.queued_jobs ?? 0} background jobs in queue`}
          icon={<GitBranch size={18} />}
          tone="violet"
        />
      </Box>
      <Box className="dashboard-middle">
        <Panel
          title="Therapy activity"
          subtitle="Orders received and workflows completed"
          action={
            <Stack direction="row" gap={2} className="chart-legend">
              <span>
                <i style={{ background: '#3459e6' }} />
                New orders
              </span>
              <span>
                <i style={{ background: '#38b69b' }} />
                Completed
              </span>
            </Stack>
          }
        >
          <Box sx={{ px: 1.5, pt: 1, height: 252 }}>
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart
                data={data.series}
                margin={{ top: 10, right: 20, bottom: 8, left: -21 }}
                accessibilityLayer
              >
                <defs>
                  <linearGradient id="orderFill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#3459e6" stopOpacity={0.16} />
                    <stop offset="100%" stopColor="#3459e6" stopOpacity={0.005} />
                  </linearGradient>
                  <linearGradient id="completeFill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#38b69b" stopOpacity={0.1} />
                    <stop offset="100%" stopColor="#38b69b" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid vertical={false} stroke="var(--border)" strokeDasharray="3 4" />
                <XAxis
                  dataKey="date"
                  tickFormatter={(value) => date(value, { month: 'short', day: 'numeric' })}
                  tick={{ fontSize: 10, fill: '#8390a7' }}
                  axisLine={false}
                  tickLine={false}
                  minTickGap={25}
                  dy={9}
                />
                <YAxis
                  allowDecimals={false}
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 10, fill: '#8390a7' }}
                />
                <ChartTooltip
                  labelFormatter={(value) => date(String(value))}
                  contentStyle={{
                    borderRadius: 10,
                    borderColor: 'var(--border)',
                    fontSize: 12,
                    background: 'var(--paper)',
                  }}
                />
                <Area
                  isAnimationActive={false}
                  type="monotone"
                  name="New orders"
                  dataKey="orders"
                  stroke="#3459e6"
                  strokeWidth={2.5}
                  fill="url(#orderFill)"
                  dot={false}
                  activeDot={{ r: 5, strokeWidth: 3, stroke: '#fff' }}
                />
                <Area
                  isAnimationActive={false}
                  type="monotone"
                  name="Completed"
                  dataKey="completed"
                  stroke="#38b69b"
                  strokeWidth={2.2}
                  fill="url(#completeFill)"
                  dot={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </Box>
          <Stack
            direction="row"
            alignItems="center"
            gap={0.7}
            sx={{ px: 2.7, py: 1.5, borderTop: 1, borderColor: 'divider' }}
          >
            <CircleCheck size={14} color="#128970" />
            <Typography variant="caption" color="text.secondary">
              {data.series.reduce((a, b) => a + b.completed, 0)} workflows completed in this period
            </Typography>
            <Box sx={{ flex: 1 }} />
            <Typography variant="caption" color="text.secondary">
              {s.facility === 'all' ? 'All facilities' : 'Selected facility'}
            </Typography>
          </Stack>
        </Panel>
        <Panel
          title="Care pipeline"
          subtitle="Where your therapies stand today"
          action={
            <Tooltip title="Open workflow monitoring">
              <IconButton size="small" aria-label="Open workflows" onClick={() => navigate('/workflows')}>
                <ArrowUpRight size={16} />
              </IconButton>
            </Tooltip>
          }
        >
          <Box sx={{ px: 2.5, pb: 1.5 }}>
            {[
              { status: 'awaiting_approvals', title: 'Awaiting approval', color: '#e1ae59' },
              { status: 'ready_to_schedule', title: 'Ready to schedule', color: '#5aa4bf' },
              { status: 'scheduled', title: 'Scheduled for care', color: '#5879e8' },
              { status: 'awaiting_billing', title: 'Awaiting billing', color: '#9a80cb' },
            ].map((stage) => {
              const count = data.statuses.find((x) => x.name === stage.status)?.count || 0;
              return (
                <Box key={stage.status} className="pipeline-stage">
                  <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 0.85 }}>
                    <Typography variant="body2" color="text.secondary">
                      {stage.title}
                    </Typography>
                    <Typography variant="body2" fontWeight={600}>
                      {count.toString().padStart(2, '0')}
                    </Typography>
                  </Stack>
                  <LinearProgress
                    aria-label={stage.title}
                    variant="determinate"
                    value={totals ? (count / totals) * 100 : 0}
                    sx={{
                      height: 5,
                      borderRadius: 4,
                      bgcolor: 'var(--subtle)',
                      '& .MuiLinearProgress-bar': { borderRadius: 4, bgcolor: stage.color },
                    }}
                  />
                </Box>
              );
            })}
            <Box className="pipeline-summary">
              <Box className="pipeline-check">
                <CircleCheck size={18} />
              </Box>
              <Box>
                <Typography variant="body2" fontWeight={600}>
                  {completed} completed workflows
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {success}% of the current care pipeline
                </Typography>
              </Box>
            </Box>
          </Box>
        </Panel>
      </Box>
      <Box className="dashboard-bottom">
        <Panel
          title="Recent therapy orders"
          subtitle="Keep the next step in care moving"
          action={
            <Button size="small" endIcon={<ArrowRight size={13} />} onClick={() => navigate('/therapy')}>
              View all orders
            </Button>
          }
        >
          <Stack direction="row" className="table-tabs" gap={2.5}>
            {[
              { id: 'active', title: 'Active orders', count: active.length },
              { id: 'all', title: 'All orders', count: orders.items.length },
              {
                id: 'attention',
                title: 'Needs attention',
                count: orders.items.filter((o) => ['rejected', 'awaiting_approvals'].includes(o.status))
                  .length,
              },
            ].map((t) => (
              <button key={t.id} className={tab === t.id ? 'selected' : ''} onClick={() => setTab(t.id)}>
                {t.title}
                <span>{t.count}</span>
              </button>
            ))}
          </Stack>
          {recent.length ? (
            <Box sx={{ overflowX: 'auto' }}>
              <Table size="small" aria-label="Recent therapy orders">
                <TableHead>
                  <TableRow>
                    <TableCell>PATIENT</TableCell>
                    <TableCell>THERAPY</TableCell>
                    <TableCell>STATUS</TableCell>
                    <TableCell sx={{ width: 35 }} />
                  </TableRow>
                </TableHead>
                <TableBody>
                  {recent.map((o) => (
                    <TableRow
                      key={o.id}
                      hover
                      sx={{ cursor: 'pointer' }}
                      onClick={() => navigate(`/therapy/${o.id}`)}
                    >
                      <TableCell>
                        <Person
                          name={name(o.patient_id)}
                          sub={
                            s.maskPhi
                              ? 'Protected reference'
                              : patients.all.find((p) => p.id === o.patient_id)?.external_id || o.patient_id
                          }
                          small
                        />
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>
                          {o.medication}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {date(o.created_at)} · Infusion therapy
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <StatusBadge status={o.status} />
                      </TableCell>
                      <TableCell>
                        <IconButton
                          size="small"
                          aria-label={`Review order ${o.id}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            navigate(`/therapy/${o.id}`);
                          }}
                        >
                          <ChevronRight size={15} />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Box>
          ) : (
            <EmptyState
              title={orders.allowed ? 'No matching therapy orders' : 'Therapy orders are restricted'}
              description={
                orders.allowed
                  ? 'Create a therapy order to start coordinating care.'
                  : 'Your current role can monitor operational data from the navigation.'
              }
            />
          )}
          <Stack direction="row" justifyContent="space-between" sx={{ px: 2.5, py: 1.7 }}>
            <Typography variant="caption" color="text.secondary">
              Showing {recent.length} of {orders.items.length} loaded orders
            </Typography>
            <Typography variant="caption" color="text.secondary">
              Workspace-isolated records
            </Typography>
          </Stack>
        </Panel>
        <Stack spacing={2}>
          <Panel
            title="Upcoming appointments"
            action={
              <IconButton
                size="small"
                onClick={() => navigate('/appointments')}
                aria-label="Open appointment calendar"
              >
                <CalendarDays size={16} />
              </IconButton>
            }
          >
            <Box sx={{ px: 2.5, pb: 1 }}>
              {upcoming.length ? (
                upcoming.map((a, i) => (
                  <Box key={a.id} className="appointment-preview">
                    <Box className={`appointment-time color-${i}`}>
                      <strong>{date(a.starts_at, { hour: 'numeric', minute: '2-digit' })}</strong>
                      <span>{date(a.starts_at)}</span>
                    </Box>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography variant="body2" fontWeight={550} noWrap>
                        {name(a.patient_id)}
                      </Typography>
                      <Typography variant="caption" color="text.secondary" noWrap display="block">
                        {orders.all.find((o) => o.id === a.order_id)?.medication || 'Infusion appointment'}
                      </Typography>
                    </Box>
                    <Box sx={{ ml: 'auto', color: '#9aa6ba' }}>
                      <ChevronRight size={14} />
                    </Box>
                  </Box>
                ))
              ) : (
                <Box sx={{ py: 2 }}>
                  <Typography variant="body2" color="text.secondary">
                    No upcoming appointments in this view.
                  </Typography>
                </Box>
              )}
            </Box>
            <Button
              fullWidth
              component={Link}
              to="/appointments"
              endIcon={<ArrowRight size={13} />}
              sx={{ borderTop: 1, borderColor: 'divider', borderRadius: 0, py: 1.4 }}
            >
              Open calendar
            </Button>
          </Panel>
          <Card className="attention-card">
            <Box className="attention-icon">
              <TriangleAlert size={17} />
            </Box>
            <Box>
              <Typography variant="body2" fontWeight={600}>
                {pending ? `${pending} authorizations need review` : 'Keep your care queue moving'}
              </Typography>
              <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 0.4 }}>
                {pending
                  ? 'Review insurance outcomes to move eligible therapies to scheduling.'
                  : 'Check upcoming appointments and workflow prerequisites.'}
              </Typography>
              <Button
                size="small"
                endIcon={<ArrowRight size={12} />}
                onClick={() => navigate('/insurance')}
                sx={{ p: 0, mt: 1, minHeight: 22, color: '#97661d' }}
              >
                Review authorizations
              </Button>
            </Box>
          </Card>
        </Stack>
      </Box>
      {form && <EntityForm kind={form} onClose={() => setForm(null)} />}
    </>
  );
}
