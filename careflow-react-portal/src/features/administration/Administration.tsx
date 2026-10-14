import {
  Alert,
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  FormControlLabel,
  IconButton,
  Stack,
  Switch,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material';
import {
  Bell,
  Building2,
  CheckCheck,
  ChevronRight,
  Cpu,
  FileText,
  HardDrive,
  KeyRound,
  LockKeyhole,
  Plus,
  Save,
  Settings2,
  ShieldCheck,
  SlidersHorizontal,
  TriangleAlert,
  Users,
} from 'lucide-react';
import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useSession } from '../../stores/session';
import { useCollection, useAction } from '../../shared/hooks';
import {
  ConfirmDialog,
  EmptyState,
  PageHeader,
  Panel,
  PermissionState,
  Person,
  StatusBadge,
} from '../../shared/components';
import { EntityForm, type FormKind } from '../../shared/EntityForm';
import { DataTable } from '../../shared/DataTable';
import { friendlyError, queryClient } from '../../services/apiClient';
import { label } from '../../shared/format';
import { hasRole, type Tenant } from '../../shared/types';

export default function Administration() {
  const route = useLocation().pathname.split('/')[1];
  const s = useSession();
  const navigate = useNavigate();
  const [tab, setTab] = useState('quotas');
  const [form, setForm] = useState<FormKind | null>(null);
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState('');
  const [acknowledged, setAcknowledged] = useState<string[]>([]);
  const action = useAction();
  const facilities = useCollection('facilities');
  const staff = useCollection('staff');
  const resources = useCollection('care-resources');
  const jobs = useCollection('jobs');
  const workflows = useCollection('workflows');
  const [quotas, setQuotas] = useState({ ...s.me!.tenant.quotas });
  const tenant = s.me!.tenant;
  const admin = hasRole(s.me!.roles, 'tenant_admin');
  if (route !== 'alerts' && !admin) return <PermissionState />;
  const alerts = [
    ...jobs.items
      .filter((j) => j.status === 'dead')
      .map((j) => ({
        id: j.id,
        title: 'Job execution needs review',
        description: `${label(j.kind)} exhausted its retry budget.`,
        severity: 'High',
        path: '/scheduler',
      })),
    ...workflows.items
      .filter((w) => w.status === 'rejected')
      .map((w) => ({
        id: w.id,
        title: 'Therapy prerequisite rejected',
        description: 'A recorded authorization, readiness, or clearance outcome requires review.',
        severity: 'High',
        path: `/workflows/${w.id}`,
      })),
    ...workflows.items
      .filter((w) => w.status === 'awaiting_approvals' && !w.outcomes.insurance)
      .map((w) => ({
        id: w.id,
        title: 'Insurance approval pending',
        description: 'This therapy is waiting for an insurance decision.',
        severity: 'Review',
        path: `/insurance/${w.id}`,
      })),
  ];
  const save = async () => {
    setError('');
    try {
      let next: Tenant;
      if (s.mode === 'demo') next = { ...tenant, quotas, version: tenant.version + 1 };
      else
        next = (await action.mutateAsync({
          path: '/configuration',
          method: 'PATCH',
          body: { version: tenant.version, quotas },
        })) as Tenant;
      useSession.setState({ me: { ...s.me!, tenant: next } });
      void queryClient.invalidateQueries();
      s.notify(s.mode === 'demo' ? 'Demo quotas updated for this session.' : 'Workspace quotas updated.');
      setConfirm(false);
    } catch (e) {
      setError(friendlyError(e));
      setConfirm(false);
    }
  };
  if (route === 'alerts')
    return (
      <>
        <PageHeader
          eyebrow="WORKSPACE"
          title="Alerts & notifications"
          description="Focus on the operational exceptions that need your attention."
        />
        <Box className="alert-summary">
          <Bell size={20} />
          <Typography>
            {alerts.filter((a) => !acknowledged.includes(a.id)).length} open items in your loaded workspace
            records
          </Typography>
          <Chip label={`${acknowledged.length} acknowledged this session`} size="small" variant="outlined" />
        </Box>
        <Stack spacing={2}>
          {alerts.length ? (
            alerts.map((a) => (
              <Card key={a.id} className="alert-row">
                <Box className={`alert-indicator ${acknowledged.includes(a.id) ? 'acknowledged' : ''}`}>
                  <TriangleAlert size={21} />
                </Box>
                <Box sx={{ flex: 1 }}>
                  <Stack direction="row" gap={1} alignItems="center">
                    <Typography fontWeight={600}>{a.title}</Typography>
                    <Chip
                      size="small"
                      label={acknowledged.includes(a.id) ? 'Acknowledged' : a.severity}
                      variant="outlined"
                    />
                  </Stack>
                  <Typography color="text.secondary" sx={{ mt: 0.5 }}>
                    {a.description}
                  </Typography>
                </Box>
                <Stack direction={{ xs: 'column', sm: 'row' }} gap={1}>
                  <Button onClick={() => navigate(a.path)}>Review</Button>
                  <Button
                    variant="outlined"
                    disabled={acknowledged.includes(a.id)}
                    onClick={() => setAcknowledged((ids) => [...ids, a.id])}
                  >
                    Acknowledge
                  </Button>
                </Stack>
              </Card>
            ))
          ) : (
            <Card>
              <EmptyState
                title="You're all caught up"
                description="No alerts were derived from your authorized loaded records."
                icon={<CheckCheck size={30} />}
              />
            </Card>
          )}
        </Stack>
        <Alert severity="info" sx={{ mt: 2.5 }}>
          Acknowledgments are local to this browser session. A shared escalation and notification service is
          not connected.
        </Alert>
      </>
    );
  if (route === 'users')
    return (
      <>
        <PageHeader
          eyebrow="ADMINISTRATION"
          title="Users & access"
          description="Give each team member the access they need within this workspace."
          actions={
            <Button variant="contained" startIcon={<Plus size={16} />} onClick={() => setForm('membership')}>
              Manage member access
            </Button>
          }
        />
        <Box className="two-column-grid">
          <Panel title="Your membership" subtitle="Validated workspace permissions">
            <Box sx={{ px: 3, pb: 3 }}>
              <Person name={s.mode === 'demo' ? 'Alex Morgan' : s.me!.user_id} sub={s.me!.user_id} />
              <Stack direction="row" spacing={1} sx={{ mt: 3 }}>
                {s.me!.roles.map((role) => (
                  <Chip key={role} label={label(role)} />
                ))}
              </Stack>
              <Typography color="text.secondary" sx={{ mt: 2 }}>
                Facility scope:{' '}
                {s.me!.facilities.includes('*')
                  ? 'All tenant facilities'
                  : `${s.me!.facilities.length} assigned facilities`}
              </Typography>
              <StatusBadge status="active" />
            </Box>
          </Panel>
          <Panel title="Access principles" subtitle="Security enforced by the healthcare API">
            <Stack spacing={2} sx={{ px: 3, pb: 3 }}>
              {[
                'Membership is checked on each API request.',
                'Role and facility permissions apply to every resource.',
                'Platform operators receive no implicit clinical access.',
                'Revoked memberships lose access without waiting for token expiry.',
              ].map((text) => (
                <Stack direction="row" gap={1.2} key={text}>
                  <ShieldCheck size={17} color="#128970" />
                  <Typography variant="body2" color="text.secondary">
                    {text}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </Panel>
        </Box>
        <Alert severity="info" sx={{ mt: 2.5 }}>
          Manage a member using their identity-provider user ID. This release supports membership updates, not
          a searchable identity-provider directory.
        </Alert>
        {form && <EntityForm kind={form} onClose={() => setForm(null)} />}
      </>
    );
  return (
    <>
      <PageHeader
        eyebrow="ADMINISTRATION"
        title="Workspace configuration"
        description="Manage care resources, approved budgets, and workspace preferences."
      />
      <Tabs value={tab} onChange={(_, v) => setTab(v)} className="page-tabs">
        <Tab value="quotas" label="Resource budgets" />
        <Tab value="resources" label="Facilities & resources" />
        <Tab value="preferences" label="Preferences" />
        <Tab value="features" label="Feature availability" />
      </Tabs>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      {tab === 'quotas' && (
        <Panel
          title="Tenant resource budgets"
          subtitle={`Configuration v${tenant.version} · ${tenant.name}`}
          action={
            <Chip
              icon={<ShieldCheck size={13} />}
              label="Versioned changes"
              size="small"
              variant="outlined"
            />
          }
        >
          <Box sx={{ px: 3, pb: 3 }}>
            <Alert severity="info" sx={{ mb: 3 }}>
              You may reduce assigned limits. A platform administrator must approve increases. Active
              workflows retain their original configuration version.
            </Alert>
            <Box className="form-grid">
              {[
                {
                  key: 'concurrent_jobs',
                  label: 'Concurrent jobs',
                  hint: 'Upper bound for active workspace jobs.',
                  min: 1,
                },
                {
                  key: 'queued_jobs',
                  label: 'Queued job capacity',
                  hint: 'Backpressure applies when the queue budget is full.',
                  min: 1,
                },
                {
                  key: 'memory_mb',
                  label: 'Memory reservation budget (MB)',
                  hint: 'Admission estimate for this tenant.',
                  min: 8,
                },
                {
                  key: 'requests_per_minute',
                  label: 'API requests per minute',
                  hint: 'Shared fixed-window budget across protocols.',
                  min: 1,
                },
              ].map((field) => (
                <TextField
                  key={field.key}
                  label={field.label}
                  type="number"
                  value={quotas[field.key as keyof typeof quotas]}
                  onChange={(e) => setQuotas((q) => ({ ...q, [field.key]: Number(e.target.value) }))}
                  helperText={field.hint}
                  slotProps={{
                    htmlInput: { min: field.min, max: tenant.quotas[field.key as keyof typeof quotas] },
                  }}
                />
              ))}
            </Box>
            <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mt: 3 }}>
              <Typography variant="caption" color="text.secondary">
                Changes are tenant-scoped and checked against the current version.
              </Typography>
              <Button variant="contained" startIcon={<Save size={15} />} onClick={() => setConfirm(true)}>
                Save budgets
              </Button>
            </Stack>
          </Box>
        </Panel>
      )}
      {tab === 'resources' && (
        <Stack spacing={3}>
          <Panel
            title="Facilities"
            subtitle="Care locations and concurrent appointment capacity"
            action={
              <Button size="small" startIcon={<Plus size={15} />} onClick={() => setForm('facility')}>
                Add facility
              </Button>
            }
          >
            <Box sx={{ px: 3, pb: 3 }}>
              {facilities.items.map((f) => (
                <Stack
                  key={f.id}
                  direction="row"
                  justifyContent="space-between"
                  alignItems="center"
                  className="resource-row"
                >
                  <Stack direction="row" gap={1.3} alignItems="center">
                    <Box className="row-icon">
                      <Building2 size={18} />
                    </Box>
                    <Box>
                      <Typography fontWeight={550}>{f.name}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {f.capacity} concurrent appointment slots
                      </Typography>
                    </Box>
                  </Stack>
                  <StatusBadge status="active" />
                </Stack>
              ))}
              {!facilities.items.length && (
                <EmptyState
                  title="Create your first care facility"
                  description="Register a facility before adding patients, staff, and care resources."
                />
              )}
            </Box>
          </Panel>
          <Box className="two-column-grid">
            <Panel
              title="Clinical staff"
              subtitle="Recorded qualifications and facility assignments"
              action={
                <Button size="small" startIcon={<Plus size={14} />} onClick={() => setForm('staff')}>
                  Add staff
                </Button>
              }
            >
              <Stack sx={{ px: 3, pb: 3 }}>
                {staff.items.map((n) => (
                  <Box key={n.id} className="resource-row">
                    <Person
                      name={n.name}
                      sub={`${n.qualifications.join(' · ')} · ${facilities.all.find((f) => f.id === n.facility_id)?.name || 'Facility'}`}
                    />
                  </Box>
                ))}
                {!staff.items.length && (
                  <Typography color="text.secondary">No clinical staff registered.</Typography>
                )}
              </Stack>
            </Panel>
            <Panel
              title="Chairs & rooms"
              subtitle="Resources available for care reservations"
              action={
                <Button size="small" startIcon={<Plus size={14} />} onClick={() => setForm('resource')}>
                  Add resource
                </Button>
              }
            >
              <Stack sx={{ px: 3, pb: 3 }}>
                {resources.items.map((r) => (
                  <Box key={r.id} className="resource-row">
                    <Typography fontWeight={550}>{r.name}</Typography>
                    <Typography variant="caption" color="text.secondary">
                      {facilities.all.find((f) => f.id === r.facility_id)?.name} · {label(r.kind)}
                    </Typography>
                  </Box>
                ))}
                {!resources.items.length && (
                  <Typography color="text.secondary">No care resources registered.</Typography>
                )}
              </Stack>
            </Panel>
          </Box>
        </Stack>
      )}
      {tab === 'preferences' && (
        <Panel title="Display preferences" subtitle="These settings affect only your browser">
          <Stack sx={{ px: 3, pb: 3 }} spacing={2}>
            <FormControlLabel
              control={<Switch checked={s.theme === 'dark'} onChange={s.setTheme} />}
              label="Use dark appearance"
            />
            <FormControlLabel
              control={<Switch checked={s.maskPhi} onChange={s.toggleMask} />}
              label="Mask patient names and demographic details"
            />
            <Typography color="text.secondary">
              Workspace timezone: {tenant.timezone}. Appointment entry follows your device timezone.
            </Typography>
            <Typography variant="caption" color="text.secondary">
              Only appearance preferences are persisted. Live access tokens and clinical records are kept out
              of browser storage.
            </Typography>
          </Stack>
        </Panel>
      )}
      {tab === 'features' && (
        <Panel
          title="Feature availability"
          subtitle="Availability follows connected services, not browser-only toggles"
        >
          <Stack sx={{ px: 3, pb: 3 }}>
            {[
              {
                name: 'Healthcare workflows',
                enabled: true,
                detail: 'Version 1 infusion workflow with explicit outcome recording.',
              },
              {
                name: 'Tenant isolation',
                enabled: true,
                detail: 'Membership, role and facility authorization on the API.',
              },
              {
                name: 'Custom workflow publishing',
                enabled: false,
                detail: 'Local drafts are available; an approved publishing service is required.',
              },
              {
                name: 'External pharmacy inventory',
                enabled: false,
                detail: 'Stock feeds and preparation systems are not connected.',
              },
              {
                name: 'Payer claim submission',
                enabled: false,
                detail: 'The portal records billing outcome state only.',
              },
              {
                name: 'Callback registry',
                enabled: false,
                detail: 'No callback registry service is connected.',
              },
            ].map((feature) => (
              <Stack
                key={feature.name}
                className="resource-row"
                direction="row"
                justifyContent="space-between"
                gap={2}
              >
                <Box>
                  <Typography fontWeight={550}>{feature.name}</Typography>
                  <Typography variant="caption" color="text.secondary">
                    {feature.detail}
                  </Typography>
                </Box>
                <StatusBadge status={feature.enabled ? 'active' : 'not_configured'} />
              </Stack>
            ))}
          </Stack>
        </Panel>
      )}
      {form && <EntityForm kind={form} onClose={() => setForm(null)} />}
      <ConfirmDialog
        open={confirm}
        title="Update workspace budgets?"
        description="This may reduce processing capacity or trigger backpressure. Existing workflow configuration versions remain pinned."
        confirm="Apply budgets"
        pending={action.isPending}
        onClose={() => setConfirm(false)}
        onConfirm={() => void save()}
      />
    </>
  );
}
