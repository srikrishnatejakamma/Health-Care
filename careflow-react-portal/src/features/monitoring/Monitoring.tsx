import {
  Alert,
  Box,
  Button,
  Card,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  Stack,
  Typography,
} from '@mui/material';
import {
  Activity,
  ArrowUpRight,
  CheckCheck,
  Copy,
  Cpu,
  Database,
  Gauge,
  GitBranch,
  HardDrive,
  MemoryStick,
  Radio,
  RefreshCw,
  Server,
  ShieldCheck,
  Webhook,
  X,
} from 'lucide-react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip as ChartTooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useCollection, useOperations } from '../../shared/hooks';
import {
  EmptyState,
  ErrorState,
  LoadingState,
  MetricCard,
  PageHeader,
  Panel,
  PermissionState,
  StatusBadge,
} from '../../shared/components';
import { DataTable } from '../../shared/DataTable';
import { useSession } from '../../stores/session';
import { bytes, date, label, shortId, time } from '../../shared/format';
import { hasRole, type Entity } from '../../shared/types';
import { queryClient } from '../../services/apiClient';

export default function Monitoring() {
  const route = useLocation().pathname.split('/')[1];
  const s = useSession();
  const operations = useOperations();
  const events = useCollection('events');
  const audit = useCollection('audit');
  const jobs = useCollection('jobs');
  const [selected, setSelected] = useState<Entity | null>(null);
  const [history, setHistory] = useState<{ at: string; heap: number; reserved: number }[]>([]);
  useEffect(() => {
    if (operations.data)
      setHistory((h) =>
        [
          ...h,
          {
            at: new Date().toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit' }),
            heap: Math.round((operations.data.memory.heap_bytes / 1024 / 1024) * 10) / 10,
            reserved: operations.data.workers.reserved_mb,
          },
        ].slice(-36),
      );
  }, [operations.dataUpdatedAt]);
  if (!hasRole(s.me!.roles, 'tenant_admin', 'auditor')) return <PermissionState />;
  const titles: Record<string, [string, string]> = {
    databases: ['Database connections', 'Understand the stores behind your care workflows.'],
    concurrency: ['Worker pools & concurrency', 'Keep workloads moving within safe resource boundaries.'],
    memory: ['Memory & resource governor', 'Observe runtime memory and estimated processing reservations.'],
    events: ['Kafka event delivery', 'Trace durable events from transaction to delivery.'],
    callbacks: ['Callback monitoring', 'Inspect event callbacks when a callback service is connected.'],
    health: ['System health', 'A clear view of your connected platform services.'],
    audit: ['Audit logs', 'A traceable record of authorized workspace activity.'],
  };
  const [title, description] = titles[route] || titles.health;
  const data = operations.data;
  return (
    <>
      <PageHeader
        eyebrow={route === 'audit' ? 'ADMINISTRATION' : 'INFRASTRUCTURE'}
        title={title}
        description={description}
        actions={
          <IconButton
            className="outlined-icon"
            onClick={() => void queryClient.invalidateQueries()}
            aria-label="Refresh monitoring"
          >
            <RefreshCw size={16} />
          </IconButton>
        }
      />
      {s.mode === 'demo' && (
        <Alert severity="info" sx={{ mb: 2.5 }}>
          This workspace uses synthetic records and clearly simulated infrastructure readings.
        </Alert>
      )}
      {operations.error && <ErrorState error={operations.error} retry={() => void operations.refetch()} />}
      {operations.isLoading && <LoadingState />}
      {data && ['databases', 'health'].includes(route) && (
        <>
          <Stack direction="row" gap={1} alignItems="center" className="monitor-scope">
            <ShieldCheck size={15} />
            <Typography variant="body2" color="text.secondary">
              {data.scope} · Updated {time(new Date(operations.dataUpdatedAt).toISOString())}
            </Typography>
          </Stack>
          <Box className="database-grid">
            {data.databases.map((db, i) => (
              <Card key={db.name} className="database-card">
                <Stack direction="row" alignItems="center" justifyContent="space-between">
                  <Box className={`database-icon database-${i}`}>
                    <Database size={24} />
                  </Box>
                  <StatusBadge status={db.status} />
                </Stack>
                <Typography variant="h2" sx={{ mt: 2.5 }}>
                  {db.name}
                </Typography>
                <Typography color="text.secondary" sx={{ mt: 1, minHeight: 44 }}>
                  {db.description}
                </Typography>
                <Box sx={{ mt: 2.5, pt: 2, borderTop: 1, borderColor: 'divider' }}>
                  <Typography variant="caption" color="text.secondary">
                    {i === 0
                      ? 'Readiness checked through the API'
                      : 'Pool and query metrics are not instrumented'}
                  </Typography>
                </Box>
              </Card>
            ))}
          </Box>
          {route === 'health' && (
            <Panel title="Workspace checks" subtitle="Identity and infrastructure status">
              <Stack divider={<Box sx={{ borderBottom: 1, borderColor: 'divider' }} />}>
                {[
                  {
                    name: 'API connection',
                    detail: s.mode === 'demo' ? 'Synthetic demo transport' : 'Authenticated API requests',
                    status: 'healthy',
                  },
                  {
                    name: 'Tenant access',
                    detail: `${s.me!.tenant.name} · ${s.me!.tenant.status}`,
                    status: s.me!.tenant.status === 'active' ? 'healthy' : 'paused',
                  },
                  {
                    name: 'Orchestration store',
                    detail: 'Primary repository readiness',
                    status: data.store.status,
                  },
                  {
                    name: 'External clinical systems',
                    detail: 'Provider contracts and connections have not been bound',
                    status: 'not_configured',
                  },
                ].map((check) => (
                  <Stack
                    key={check.name}
                    direction="row"
                    justifyContent="space-between"
                    alignItems="center"
                    sx={{ px: 3, py: 2.2 }}
                  >
                    <Box>
                      <Typography fontWeight={550}>{check.name}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {check.detail}
                      </Typography>
                    </Box>
                    <StatusBadge status={check.status} />
                  </Stack>
                ))}
              </Stack>
            </Panel>
          )}
        </>
      )}
      {data && route === 'concurrency' && (
        <>
          <Box className="kpi-grid">
            <MetricCard
              title="Configured workers"
              value={data.workers.configured}
              detail="Shared API process capacity"
              icon={<Cpu size={18} />}
            />
            <MetricCard
              title="Tenant concurrency limit"
              value={s.me!.tenant.quotas.concurrent_jobs}
              detail="Maximum concurrent workspace jobs"
              icon={<Gauge size={18} />}
              tone="teal"
            />
            <MetricCard
              title="Queued jobs"
              value={jobs.items.filter((j) => j.status === 'queued').length}
              detail={`${s.me!.tenant.quotas.queued_jobs} allowed in this workspace`}
              icon={<GitBranch size={18} />}
              tone="amber"
            />
            <MetricCard
              title="Memory reservations"
              value={`${data.workers.reserved_mb} MB`}
              detail="Shared process admission estimates"
              icon={<MemoryStick size={18} />}
              tone="violet"
            />
          </Box>
          <Box className="two-column-grid">
            <Panel title="Job state distribution" subtitle="Loaded tenant jobs by execution status">
              <Box sx={{ height: 260, p: 2 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart
                    data={['queued', 'running', 'completed', 'dead', 'cancelled'].map((status) => ({
                      name: label(status),
                      jobs: jobs.items.filter((j) => j.status === status).length,
                    }))}
                  >
                    <CartesianGrid stroke="var(--border)" vertical={false} />
                    <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{ fontSize: 10 }} />
                    <YAxis allowDecimals={false} axisLine={false} tickLine={false} tick={{ fontSize: 10 }} />
                    <ChartTooltip />
                    <Bar dataKey="jobs" name="Jobs" fill="#5474e4" radius={[5, 5, 0, 0]} maxBarSize={50} />
                  </BarChart>
                </ResponsiveContainer>
              </Box>
            </Panel>
            <Panel title="Admission boundaries" subtitle="Current workspace resource quotas">
              <Stack spacing={3} sx={{ px: 3, pb: 3 }}>
                {[
                  {
                    label: 'Queued job budget',
                    used: jobs.items.filter((j) => ['queued', 'running'].includes(j.status)).length,
                    total: s.me!.tenant.quotas.queued_jobs,
                    unit: 'jobs',
                  },
                  {
                    label: 'Concurrent job budget',
                    used: jobs.items.filter((j) => j.status === 'running').length,
                    total: s.me!.tenant.quotas.concurrent_jobs,
                    unit: 'jobs',
                  },
                ].map((q) => (
                  <Box key={q.label}>
                    <Stack direction="row" justifyContent="space-between" sx={{ mb: 1 }}>
                      <Typography color="text.secondary">{q.label}</Typography>
                      <Typography fontWeight={600}>
                        {q.used} / {q.total} {q.unit}
                      </Typography>
                    </Stack>
                    <LinearProgress
                      variant="determinate"
                      value={Math.min(100, (q.used / q.total) * 100)}
                      sx={{ height: 7, borderRadius: 5 }}
                    />
                  </Box>
                ))}
                <Alert severity="info">
                  Workers are bounded and tenant dispatch is fair. Adaptive scaling is not enabled in this
                  release.
                </Alert>
              </Stack>
            </Panel>
          </Box>
        </>
      )}
      {data && route === 'memory' && (
        <>
          <Box className="kpi-grid">
            <MetricCard
              title="Heap allocated"
              value={bytes(data.memory.heap_bytes)}
              detail="Shared Go process memory"
              icon={<MemoryStick size={18} />}
            />
            <MetricCard
              title="Runtime soft limit"
              value={bytes(data.memory.soft_limit_bytes)}
              detail="Go runtime memory target"
              icon={<HardDrive size={18} />}
              tone="teal"
            />
            <MetricCard
              title="Goroutines"
              value={data.memory.goroutines}
              detail="Current shared process count"
              icon={<GitBranch size={18} />}
              tone="violet"
            />
            <MetricCard
              title="Latest GC pause"
              value={`${(data.memory.gc_pause_ns / 1e6).toFixed(2)} ms`}
              detail={`${data.memory.gc_cycles} collection cycles`}
              icon={<Activity size={18} />}
              tone="amber"
            />
          </Box>
          <Panel
            title="Memory observed this session"
            subtitle="Rolling observations; no historical server data is assumed"
          >
            <Box sx={{ height: 280, p: 2 }}>
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={history}>
                  <CartesianGrid vertical={false} stroke="var(--border)" />
                  <XAxis dataKey="at" tick={{ fontSize: 10 }} axisLine={false} tickLine={false} />
                  <YAxis unit=" MB" tick={{ fontSize: 10 }} axisLine={false} tickLine={false} />
                  <ChartTooltip />
                  <Line
                    name="Heap allocated (MB)"
                    dataKey="heap"
                    stroke="#3459e6"
                    strokeWidth={2.2}
                    dot={{ r: 3 }}
                  />
                  <Line name="Reservations (MB)" dataKey="reserved" stroke="#35ae94" strokeWidth={2.2} />
                </LineChart>
              </ResponsiveContainer>
            </Box>
            <Alert severity="info" sx={{ mx: 3, mb: 3 }}>
              Memory reservations are admission estimates, not a hard heap limit. Shared runtime measurements
              do not expose another tenant's clinical records.
            </Alert>
          </Panel>
        </>
      )}
      {route === 'events' && (
        <>
          <Box className="kpi-grid">
            <MetricCard
              title="Loaded events"
              value={events.items.length}
              detail="Authorized tenant events"
              icon={<Radio size={18} />}
            />
            <MetricCard
              title="Acknowledged delivery"
              value={events.items.filter((e) => e.published).length}
              detail="Published to the broker"
              icon={<CheckCheck size={18} />}
              tone="teal"
            />
            <MetricCard
              title="Pending delivery"
              value={events.items.filter((e) => !e.published).length}
              detail="Persisted in the transactional outbox"
              icon={<RefreshCw size={18} />}
              tone="amber"
            />
            <MetricCard
              title="Schema version"
              value={events.items[0]?.schema_version ? `v${events.items[0].schema_version}` : '—'}
              detail="Versioned event envelopes"
              icon={<GitBranch size={18} />}
              tone="violet"
            />
          </Box>
          <DataTable
            rows={events.items}
            name="events"
            loading={events.isLoading}
            onRow={setSelected}
            hasMore={events.hasNextPage}
            loadMore={() => void events.fetchNextPage()}
            columns={[
              {
                field: 'event_type',
                headerName: 'EVENT TYPE',
                minWidth: 250,
                flex: 1.5,
                renderCell: ({ value }) => <Typography fontWeight={500}>{label(value)}</Typography>,
              },
              { field: 'sequence', headerName: 'SEQUENCE', width: 110 },
              {
                field: 'published',
                headerName: 'DELIVERY',
                width: 160,
                renderCell: ({ value }) => <StatusBadge status={value ? 'completed' : 'queued'} />,
              },
              {
                field: 'correlation_id',
                headerName: 'CORRELATION',
                minWidth: 170,
                renderCell: ({ value }) => <code>{shortId(value)}</code>,
              },
              {
                field: 'occurred_at',
                headerName: 'OCCURRED',
                minWidth: 180,
                renderCell: ({ value }) => `${date(value)} · ${time(value)}`,
              },
            ]}
            exportable
          />
          <Alert severity="info" sx={{ mt: 2.5 }}>
            Broker topic, partition and consumer-lag metrics require a connected Kafka monitoring service.
            Pending outbox records remain durable until acknowledged.
          </Alert>
        </>
      )}
      {route === 'callbacks' && (
        <Card>
          <EmptyState
            title="No callback service connected"
            description="Callback registrations and delivery attempts will appear when your organization connects a callback registry."
            icon={<Webhook size={32} />}
          />
          <Alert severity="info" sx={{ mx: 3, mb: 3 }}>
            Critical workflow events already use the durable outbox. Callback handler counts and delivery
            health are not currently instrumented.
          </Alert>
        </Card>
      )}
      {route === 'audit' && (
        <DataTable
          rows={audit.items}
          name="audit entries"
          loading={audit.isLoading}
          hasMore={audit.hasNextPage}
          loadMore={() => void audit.fetchNextPage()}
          onRow={setSelected}
          exportable
          columns={[
            {
              field: 'action',
              headerName: 'ACTION',
              minWidth: 240,
              flex: 1.4,
              renderCell: ({ value }) => <Typography fontWeight={500}>{label(value)}</Typography>,
            },
            { field: 'actor', headerName: 'ACTOR', minWidth: 180, flex: 1 },
            {
              field: 'resource_id',
              headerName: 'RESOURCE',
              minWidth: 150,
              renderCell: ({ value }) => <code>{shortId(value)}</code>,
            },
            {
              field: 'correlation_id',
              headerName: 'CORRELATION',
              minWidth: 160,
              renderCell: ({ value }) => <code>{shortId(value)}</code>,
            },
            {
              field: 'at',
              headerName: 'TIMESTAMP',
              minWidth: 170,
              renderCell: ({ value }) => `${date(value)} · ${time(value)}`,
            },
          ]}
        />
      )}
      {selected && (
        <Dialog open fullWidth maxWidth="sm" onClose={() => setSelected(null)}>
          <DialogTitle>
            <Stack direction="row" justifyContent="space-between">
              {route === 'audit' ? 'Audit entry' : 'Event metadata'}
              <IconButton onClick={() => setSelected(null)} aria-label="Close metadata">
                <X size={18} />
              </IconButton>
            </Stack>
          </DialogTitle>
          <DialogContent>
            <Box component="pre" className="metadata-block">
              {JSON.stringify(selected, null, 2)}
            </Box>
            <Button
              startIcon={<Copy size={15} />}
              onClick={() =>
                void navigator.clipboard
                  .writeText(JSON.stringify(selected, null, 2))
                  .then(() => s.notify('Metadata copied.'))
                  .catch(() => s.notify('Copy is unavailable in this browser.'))
              }
            >
              Copy metadata
            </Button>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
