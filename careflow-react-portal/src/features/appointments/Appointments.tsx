import {
  Alert,
  Box,
  Button,
  Card,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@mui/material';
import { CalendarDays, Plus, Users, X } from 'lucide-react';
import { useState } from 'react';
import FullCalendar from '@fullcalendar/react';
import dayGridPlugin from '@fullcalendar/daygrid';
import timeGridPlugin from '@fullcalendar/timegrid';
import interactionPlugin from '@fullcalendar/interaction';
import { useCollection, useAction } from '../../shared/hooks';
import { ConfirmDialog, PageHeader, PermissionState, Person, StatusBadge } from '../../shared/components';
import { EntityForm } from '../../shared/EntityForm';
import { DataTable } from '../../shared/DataTable';
import { useSession } from '../../stores/session';
import { date, displayName, time } from '../../shared/format';
import type { Appointment } from '../../shared/types';
import { hasRole } from '../../shared/types';

export default function Appointments() {
  const appointments = useCollection('appointments');
  const patients = useCollection('patients');
  const orders = useCollection('orders');
  const staff = useCollection('staff');
  const resources = useCollection('care-resources');
  const facilities = useCollection('facilities');
  const s = useSession();
  const action = useAction();
  const [tab, setTab] = useState('calendar');
  const [form, setForm] = useState(false);
  const [selected, setSelected] = useState<Appointment | null>(null);
  const [cancel, setCancel] = useState(false);
  const [rebook, setRebook] = useState(false);
  if (!appointments.allowed) return <PermissionState />;
  const clinical = hasRole(s.me!.roles, 'tenant_admin', 'clinical');
  const patient = (id: string) => displayName(patients.all.find((p) => p.id === id)?.name, s.maskPhi);
  return (
    <>
      <PageHeader
        eyebrow="CARE OPERATIONS"
        title="Appointment scheduling"
        description="Bring the right people, resources, and therapies together."
        actions={
          clinical && (
            <Button variant="contained" startIcon={<Plus size={16} />} onClick={() => setForm(true)}>
              Book appointment
            </Button>
          )
        }
      />
      <Tabs value={tab} onChange={(_, v) => setTab(v)} className="page-tabs">
        <Tab label="Calendar" value="calendar" />
        <Tab label="Staff availability" value="staff" />
        <Tab label="Facility capacity" value="capacity" />
      </Tabs>
      {tab === 'calendar' && (
        <Card sx={{ p: { xs: 1.5, md: 3 } }}>
          <Stack direction="row" gap={1} alignItems="center" sx={{ mb: 2 }}>
            <CalendarDays size={16} color="#3459e6" />
            <Typography variant="body2" color="text.secondary">
              {appointments.items.filter((a) => a.status === 'confirmed').length} confirmed appointments ·
              Times shown in {Intl.DateTimeFormat().resolvedOptions().timeZone}
            </Typography>
          </Stack>
          <FullCalendar
            plugins={[dayGridPlugin, timeGridPlugin, interactionPlugin]}
            initialView="dayGridMonth"
            headerToolbar={{
              left: 'prev,next today',
              center: 'title',
              right: 'dayGridMonth,timeGridWeek,timeGridDay',
            }}
            buttonText={{ today: 'Today', month: 'Month', week: 'Week', day: 'Day' }}
            height="auto"
            dayMaxEvents={3}
            nowIndicator
            selectable={clinical}
            dateClick={() => clinical && setForm(true)}
            events={appointments.items
              .filter((a) => a.status !== 'cancelled')
              .map((a) => ({
                id: a.id,
                title: `${patient(a.patient_id)} · ${orders.all.find((o) => o.id === a.order_id)?.medication || 'Infusion'}`,
                start: a.starts_at,
                end: a.ends_at,
                backgroundColor: a.status === 'completed' ? '#e9f6f1' : '#edf1ff',
                borderColor: a.status === 'completed' ? '#38a28a' : '#5879e8',
                textColor: a.status === 'completed' ? '#128970' : '#3459ce',
              }))}
            eventClick={(info) => setSelected(appointments.items.find((a) => a.id === info.event.id) || null)}
            eventTimeFormat={{ hour: 'numeric', minute: '2-digit', meridiem: 'short' }}
          />
        </Card>
      )}
      {tab === 'staff' && (
        <DataTable
          rows={staff.items}
          name="staff"
          columns={[
            {
              field: 'name',
              headerName: 'CLINICAL STAFF',
              minWidth: 200,
              flex: 1.5,
              renderCell: ({ value }) => <Person name={String(value)} sub="Clinical staff" />,
            },
            {
              field: 'facility_id',
              headerName: 'FACILITY',
              minWidth: 200,
              flex: 1,
              renderCell: ({ value }) => facilities.all.find((f) => f.id === value)?.name || value,
            },
            {
              field: 'qualifications',
              headerName: 'QUALIFICATIONS',
              minWidth: 170,
              renderCell: ({ value }) => String(value).replaceAll(',', ' · '),
            },
            {
              field: 'active',
              headerName: 'STATUS',
              width: 140,
              renderCell: ({ value }) => <StatusBadge status={value ? 'active' : 'paused'} />,
            },
            {
              field: 'bookings',
              headerName: 'CONFIRMED BOOKINGS',
              minWidth: 180,
              renderCell: ({ row }) =>
                appointments.all.filter((a) => a.nurse_id === row.id && a.status === 'confirmed').length,
            },
          ]}
        />
      )}
      {tab === 'capacity' && (
        <Box className="two-column-grid">
          {facilities.items.map((f) => {
            const booked = appointments.all.filter(
              (a) =>
                a.facility_id === f.id &&
                a.status === 'confirmed' &&
                new Date(a.starts_at) <= new Date() &&
                new Date(a.ends_at) > new Date(),
            ).length;
            return (
              <Card key={f.id} sx={{ p: 3 }}>
                <Stack direction="row" justifyContent="space-between" alignItems="center">
                  <Typography variant="h2">{f.name}</Typography>
                  <StatusBadge status="active" />
                </Stack>
                <Typography sx={{ mt: 3, fontSize: 30, fontWeight: 600 }}>
                  {booked}
                  <Typography component="span" color="text.secondary" sx={{ ml: 1, fontSize: 15 }}>
                    / {f.capacity} concurrent slots
                  </Typography>
                </Typography>
                <LinearProgress
                  aria-label={`${f.name} appointment capacity`}
                  value={(booked / f.capacity) * 100}
                  variant="determinate"
                  sx={{ height: 7, borderRadius: 5, my: 2 }}
                />
                <Typography color="text.secondary">
                  {resources.all.filter((r) => r.facility_id === f.id).length} registered chairs/rooms ·{' '}
                  {staff.all.filter((n) => n.facility_id === f.id && n.active).length} active staff
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Current occupancy is calculated from confirmed appointment times.
                </Typography>
              </Card>
            );
          })}
        </Box>
      )}
      {selected && (
        <Dialog open onClose={() => setSelected(null)} fullWidth maxWidth="sm">
          <DialogTitle>
            <Stack direction="row" justifyContent="space-between">
              Appointment details
              <IconButton aria-label="Close appointment" onClick={() => setSelected(null)}>
                <X size={18} />
              </IconButton>
            </Stack>
          </DialogTitle>
          <DialogContent>
            <Person
              name={patient(selected.patient_id)}
              sub={orders.all.find((o) => o.id === selected.order_id)?.medication}
            />
            <Stack spacing={1.5} sx={{ mt: 3 }}>
              <Typography>
                <strong>When:</strong> {date(selected.starts_at)} · {time(selected.starts_at)}–
                {time(selected.ends_at)}
              </Typography>
              <Typography>
                <strong>Nurse:</strong>{' '}
                {staff.all.find((n) => n.id === selected.nurse_id)?.name || selected.nurse_id}
              </Typography>
              <Typography>
                <strong>Resource:</strong>{' '}
                {resources.all.find((r) => r.id === selected.resource_id)?.name || selected.resource_id}
              </Typography>
              <Typography>
                <strong>Facility:</strong> {facilities.all.find((f) => f.id === selected.facility_id)?.name}
              </Typography>
              <Box>
                <StatusBadge status={selected.status} />
              </Box>
            </Stack>
          </DialogContent>
          <DialogActions sx={{ p: 2.5 }}>
            {selected.status === 'confirmed' && clinical && (
              <>
                <Button
                  color="error"
                  onClick={() => {
                    setRebook(false);
                    setCancel(true);
                  }}
                >
                  Cancel appointment
                </Button>
                <Button
                  variant="outlined"
                  onClick={() => {
                    setRebook(true);
                    setCancel(true);
                  }}
                >
                  Release & rebook
                </Button>
              </>
            )}
            <Button onClick={() => setSelected(null)}>Close</Button>
          </DialogActions>
        </Dialog>
      )}
      <ConfirmDialog
        open={cancel}
        title={rebook ? 'Release this slot and rebook?' : 'Cancel this appointment?'}
        description={
          rebook
            ? 'The existing reservation will be released first. You can then choose a new time, nurse, and resource; a new slot is not reserved until your new booking is confirmed.'
            : 'The nurse and care resource will be released. The therapy will return to ready-to-schedule, unless administration has already been recorded.'
        }
        confirm={rebook ? 'Release & choose new slot' : 'Cancel appointment'}
        danger
        pending={action.isPending}
        onClose={() => setCancel(false)}
        onConfirm={() => {
          if (selected)
            void action
              .mutateAsync({ path: `/appointments/${selected.id}/cancel` })
              .then(() => {
                setCancel(false);
                setSelected(null);
                if (rebook) setForm(true);
              })
              .catch(() => {});
        }}
      />
      {form && <EntityForm kind="appointment" onClose={() => setForm(false)} />}
    </>
  );
}
