import {
  Alert,
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Skeleton,
  Stack,
  Typography,
} from '@mui/material';
import { ArrowDownLeft, ArrowUpRight, Circle, Inbox, RefreshCw, ShieldCheck } from 'lucide-react';
import type { ReactNode } from 'react';
import { Component } from 'react';
import { friendlyError } from '../services/apiClient';
import { initials, label } from './format';

const colors: Record<string, { fg: string; bg: string }> = {
  completed: { fg: '#10715d', bg: '#e9f6f1' },
  approved: { fg: '#10715d', bg: '#e9f6f1' },
  available: { fg: '#10715d', bg: '#e9f6f1' },
  healthy: { fg: '#10715d', bg: '#e9f6f1' },
  confirmed: { fg: '#3459ce', bg: '#edf1ff' },
  scheduled: { fg: '#3459ce', bg: '#edf1ff' },
  ready_to_schedule: { fg: '#156b83', bg: '#eaf6fa' },
  running: { fg: '#6e55b5', bg: '#f1ecfc' },
  awaiting_approvals: { fg: '#8a5c17', bg: '#fff6e5' },
  awaiting_billing: { fg: '#6e55b5', bg: '#f1ecfc' },
  queued: { fg: '#8a5c17', bg: '#fff6e5' },
  dead: { fg: '#c34b56', bg: '#fceef0' },
  rejected: { fg: '#c34b56', bg: '#fceef0' },
  cancelled: { fg: '#69758a', bg: '#eff2f6' },
  not_configured: { fg: '#69758a', bg: '#eff2f6' },
  paused: { fg: '#8a5c17', bg: '#fff6e5' },
  active: { fg: '#10715d', bg: '#e9f6f1' },
};
export function StatusBadge({ status, compact = false }: { status: string; compact?: boolean }) {
  const color = colors[status] || colors.cancelled;
  const text: Record<string, string> = {
    awaiting_approvals: 'Pending approval',
    ready_to_schedule: 'Ready to schedule',
    awaiting_billing: 'Awaiting billing',
    dead: 'Failed',
    not_configured: 'Not connected',
  };
  return (
    <Chip
      className="status-chip"
      size="small"
      icon={<Circle size={6} fill="currentColor" strokeWidth={0} />}
      label={compact ? text[status] || label(status) : text[status] || label(status)}
      sx={{
        color: color.fg,
        bgcolor: color.bg,
        '& .MuiChip-icon': { color: color.fg, ml: 1.1, width: 7 },
        '& .MuiChip-label': { px: 1 },
      }}
    />
  );
}
export function Person({ name, sub, small = false }: { name: string; sub?: string; small?: boolean }) {
  return (
    <Stack direction="row" spacing={1.3} alignItems="center">
      <Avatar
        sx={{
          width: small ? 30 : 36,
          height: small ? 30 : 36,
          fontSize: 11,
          fontWeight: 600,
          color: '#48618f',
          bgcolor: '#edf1fa',
          borderRadius: small ? 1.8 : 2,
        }}
      >
        {initials(name)}
      </Avatar>
      <Box sx={{ minWidth: 0 }}>
        <Typography variant="body2" fontWeight={550} noWrap>
          {name}
        </Typography>
        {sub && (
          <Typography variant="caption" color="text.secondary" noWrap>
            {sub}
          </Typography>
        )}
      </Box>
    </Stack>
  );
}
export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  actions?: ReactNode;
}) {
  return (
    <Stack
      className="page-heading"
      direction={{ xs: 'column', sm: 'row' }}
      justifyContent="space-between"
      alignItems={{ xs: 'flex-start', sm: 'center' }}
      gap={2}
    >
      <Box>
        {eyebrow && <Typography className="eyebrow">{eyebrow}</Typography>}
        <Typography variant="h1">{title}</Typography>
        <Typography color="text.secondary" sx={{ mt: 0.65 }}>
          {description}
        </Typography>
      </Box>
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        {actions}
      </Stack>
    </Stack>
  );
}
export function Panel({
  title,
  subtitle,
  action,
  children,
  sx,
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
  children: ReactNode;
  sx?: object;
}) {
  return (
    <Card sx={{ height: '100%', ...sx }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ p: 2.5, pb: 2 }}>
        <Box>
          <Typography variant="h3">{title}</Typography>
          {subtitle && (
            <Typography variant="caption" color="text.secondary">
              {subtitle}
            </Typography>
          )}
        </Box>
        {action}
      </Stack>
      {children}
    </Card>
  );
}
export function MetricCard({
  title,
  value,
  detail,
  icon,
  trend,
  tone = 'blue',
}: {
  title: string;
  value: string | number;
  detail: string;
  icon: ReactNode;
  trend?: number;
  tone?: string;
}) {
  return (
    <Card className="metric-card">
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="body2" color="text.secondary" fontWeight={500}>
          {title}
        </Typography>
        <Box className={`metric-icon ${tone}`}>{icon}</Box>
      </Stack>
      <Typography className="metric-number">{value}</Typography>
      <Stack direction="row" gap={0.7} alignItems="center">
        {trend !== undefined && (
          <Box className={`metric-trend ${trend < 0 ? 'negative' : ''}`}>
            {trend < 0 ? <ArrowDownLeft size={12} /> : <ArrowUpRight size={12} />} {Math.abs(trend)}%
          </Box>
        )}
        <Typography variant="caption" color="text.secondary">
          {detail}
        </Typography>
      </Stack>
    </Card>
  );
}
export function EmptyState({
  title = 'No records yet',
  description = 'New records will appear here.',
  action,
  icon,
}: {
  title?: string;
  description?: string;
  action?: ReactNode;
  icon?: ReactNode;
}) {
  return (
    <Box className="empty-state">
      <Box className="empty-icon">{icon || <Inbox size={26} />}</Box>
      <Typography variant="h3">{title}</Typography>
      <Typography color="text.secondary" sx={{ mt: 0.7, mb: 2, maxWidth: 380 }}>
        {description}
      </Typography>
      {action}
    </Box>
  );
}
export function ErrorState({ error, retry }: { error: unknown; retry?: () => void }) {
  return (
    <Alert
      severity="error"
      action={
        retry ? (
          <Button size="small" color="inherit" startIcon={<RefreshCw size={14} />} onClick={retry}>
            Retry
          </Button>
        ) : undefined
      }
    >
      {friendlyError(error)}
    </Alert>
  );
}
export function LoadingState() {
  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={2}>
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} variant="rounded" height={130} sx={{ flex: 1 }} />
        ))}
      </Stack>
      <Skeleton variant="rounded" height={320} />
    </Stack>
  );
}
export function ConfirmDialog({
  open,
  title,
  description,
  onClose,
  onConfirm,
  pending = false,
  confirm = 'Confirm',
  danger = false,
}: {
  open: boolean;
  title: string;
  description: string;
  onClose: () => void;
  onConfirm: () => void;
  pending?: boolean;
  confirm?: string;
  danger?: boolean;
}) {
  return (
    <Dialog open={open} onClose={pending ? undefined : onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Typography color="text.secondary">{description}</Typography>
      </DialogContent>
      <DialogActions sx={{ p: 2.5 }}>
        <Button onClick={onClose} disabled={pending}>
          Keep current state
        </Button>
        <Button
          variant="contained"
          color={danger ? 'error' : 'primary'}
          onClick={onConfirm}
          disabled={pending}
          startIcon={pending ? <CircularProgress size={14} color="inherit" /> : undefined}
        >
          {confirm}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
export function PermissionState() {
  return (
    <EmptyState
      title="This area is restricted"
      description="Your current role does not have access. Contact your workspace administrator if you need additional permissions."
      icon={<ShieldCheck size={28} />}
    />
  );
}
export class ErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    if (this.state.failed)
      return (
        <Box sx={{ p: 5 }}>
          <EmptyState
            title="We couldn't load this view"
            description="Refresh the page to reconnect to your workspace."
            action={
              <Button variant="contained" onClick={() => location.reload()}>
                Reload workspace
              </Button>
            }
          />
        </Box>
      );
    return this.props.children;
  }
}
