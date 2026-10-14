import { demoDatabase } from './demoData';
import type { Entity, Kind, Me, Operations, Overview, ResourceMap, Workflow } from '../shared/types';
import { canRead, hasRole } from '../shared/types';

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public correlationId?: string,
  ) {
    super(message);
  }
}
const replays = new Map<string, { body: string; result: unknown }>();
function emit(me: Me, action: string, id: string) {
  const db = demoDatabase(me.tenant.id);
  const at = new Date().toISOString();
  const correlation_id = crypto.randomUUID();
  db.audit.unshift({
    id: crypto.randomUUID(),
    tenant_id: me.tenant.id,
    actor: me.user_id,
    action,
    resource_id: id,
    correlation_id,
    at,
  });
  const event_id = crypto.randomUUID();
  db.events.push({
    id: event_id,
    event_id,
    tenant_id: me.tenant.id,
    event_type: action,
    aggregate_id: id,
    schema_version: 1,
    configuration_version: me.tenant.version,
    correlation_id,
    occurred_at: at,
    published: false,
    sequence: db.events.length + 1,
  });
}
function advance(me: Me, w: Workflow) {
  const db = demoDatabase(me.tenant.id);
  if (w.status === 'awaiting_approvals') {
    if (
      w.outcomes.insurance === 'rejected' ||
      w.outcomes.inventory === 'unavailable' ||
      w.outcomes.clearance === 'rejected'
    )
      w.status = 'rejected';
    else if (
      w.outcomes.insurance === 'approved' &&
      w.outcomes.inventory === 'available' &&
      w.outcomes.clearance === 'approved'
    )
      w.status = 'ready_to_schedule';
  } else if (w.status === 'scheduled' && w.outcomes.administration === 'completed') {
    w.status = 'awaiting_billing';
    db.appointments.filter((a) => a.order_id === w.order_id).forEach((a) => (a.status = 'completed'));
  } else if (w.status === 'awaiting_billing' && w.outcomes.billing === 'submitted') w.status = 'completed';
  w.updated_at = new Date().toISOString();
  if (w.checkpoints.at(-1) !== w.status) w.checkpoints.push(w.status);
  db.orders.find((o) => o.id === w.order_id)!.status = w.status;
}
export function overview(me: Me, days = 7, facility = 'all'): Overview {
  const db = demoDatabase(me.tenant.id);
  const filtered = <T extends Entity>(items: T[]) =>
    items.filter((x) => facility === 'all' || x.facility_id === facility);
  const orders = canRead('orders', me.roles) ? filtered(db.orders) : [];
  const workflows = canRead('workflows', me.roles) ? filtered(db.workflows) : [];
  const appts = canRead('appointments', me.roles) ? filtered(db.appointments) : [];
  const counts: Record<string, number> = {};
  if (canRead('patients', me.roles)) counts.patients = filtered(db.patients).length;
  if (canRead('orders', me.roles)) {
    counts.active_orders = orders.filter(
      (o) => !['completed', 'cancelled', 'rejected'].includes(o.status),
    ).length;
    counts.pending_authorizations = workflows.filter(
      (w) => w.status === 'awaiting_approvals' && !w.outcomes.insurance,
    ).length;
    counts.completed_orders = orders.filter((o) => o.status === 'completed').length;
  }
  if (canRead('appointments', me.roles))
    counts.appointments = appts.filter((a) => a.status === 'confirmed').length;
  if (canRead('workflows', me.roles))
    counts.active_workflows = workflows.filter(
      (w) => !['completed', 'cancelled', 'rejected'].includes(w.status),
    ).length;
  if (canRead('jobs', me.roles)) {
    counts.failed_jobs = db.jobs.filter((j) => j.status === 'dead').length;
    counts.queued_jobs = db.jobs.filter((j) => j.status === 'queued').length;
    counts.running_jobs = db.jobs.filter((j) => j.status === 'running').length;
  }
  const series = Array.from({ length: days }, (_, i) => {
    const d = new Date();
    d.setDate(d.getDate() - (days - 1 - i));
    const date = d.toISOString().slice(0, 10);
    return {
      date,
      orders: orders.filter((o) => o.created_at.startsWith(date)).length,
      completed: workflows.filter((w) => w.status === 'completed' && w.updated_at.startsWith(date)).length,
    };
  });
  const statuses = [
    'awaiting_approvals',
    'ready_to_schedule',
    'scheduled',
    'awaiting_billing',
    'completed',
    'rejected',
  ].map((name) => ({ name, count: workflows.filter((w) => w.status === name).length }));
  return { counts, series, statuses, generated_at: new Date().toISOString(), days };
}
export const demoOperations = (): Operations => ({
  scope: 'Shared service · synthetic metrics',
  store: { status: 'healthy' },
  databases: [
    { name: 'PostgreSQL', status: 'healthy', description: 'Primary orchestration store' },
    { name: 'SQL Server', status: 'not_configured', description: 'Optional billing adapter' },
    { name: 'MongoDB', status: 'not_configured', description: 'Optional clinical metadata adapter' },
    { name: 'Redis', status: 'not_configured', description: 'Optional cache adapter' },
  ],
  workers: { configured: 8, completed: 124, failed: 2, reserved_mb: 24 },
  memory: {
    heap_bytes: 46 * 1024 * 1024,
    heap_sys_bytes: 96 * 1024 * 1024,
    gc_cycles: 32,
    gc_pause_ns: 840000,
    goroutines: 28,
    soft_limit_bytes: 256 * 1024 * 1024,
  },
  capabilities: {
    designer_publish: false,
    external_inventory: false,
    claims_submission: false,
    callbacks: false,
    feature_flags: false,
  },
});
export function demoRequest<T>(me: Me, path: string, method = 'GET', body?: unknown, key = ''): T {
  const url = new URL(path, 'http://demo');
  const parts = url.pathname.split('/').filter(Boolean);
  const kind = parts[0] as Kind;
  const id = parts[1];
  const action = parts[2];
  const db = demoDatabase(me.tenant.id);
  if (path === '/me') return structuredClone(me) as T;
  if (parts[0] === 'portal' && parts[1] === 'overview')
    return overview(
      me,
      Number(url.searchParams.get('days') || 7),
      url.searchParams.get('facility') || 'all',
    ) as T;
  if (parts[0] === 'operations') {
    if (!hasRole(me.roles, 'tenant_admin', 'auditor'))
      throw new ApiError(403, 'Your role does not allow infrastructure monitoring.');
    return demoOperations() as T;
  }
  const payload = (body as Record<string, unknown>) || {};
  const replayKey = `${me.tenant.id}/${method}/${url.pathname}/${key}`;
  if (method !== 'GET' && key && replays.has(replayKey)) {
    const prior = replays.get(replayKey)!;
    if (prior.body !== JSON.stringify(body))
      throw new ApiError(409, 'This request key was already used with different details.');
    return structuredClone(prior.result) as T;
  }
  if (method === 'GET') {
    if (!(kind in db) || !canRead(kind, me.roles))
      throw new ApiError(403, 'Your role does not allow access to these records.');
    const rows = db[kind] as Entity[];
    if (id) {
      const row = rows.find((x) => x.id === id);
      if (!row) throw new ApiError(404, 'Record not found.');
      return structuredClone(row) as T;
    }
    const after = url.searchParams.get('after');
    const offset = after ? rows.findIndex((x) => x.id === after) + 1 : 0;
    const limit = Number(url.searchParams.get('limit') || 100);
    return structuredClone({
      items: rows.slice(offset, offset + limit),
      next_cursor: offset + limit < rows.length ? rows[offset + limit - 1].id : undefined,
    }) as T;
  }
  const admin = hasRole(me.roles, 'tenant_admin');
  const clinical = hasRole(me.roles, 'tenant_admin', 'clinical');
  let result: unknown;
  if (parts[0] === 'memberships' && method === 'PUT') {
    if (!admin) throw new ApiError(403, 'Administrator access required.');
    result = { tenant_id: me.tenant.id, user_id: id, ...payload };
    emit(me, 'membership.updated', id);
  } else if (kind === 'workflows' && action === 'outcomes') {
    const w = db.workflows.find((x) => x.id === id);
    if (!w) throw new ApiError(404, 'Workflow not found.');
    const step = String(payload.step);
    const outcome = String(payload.outcome);
    const allowed =
      step === 'billing'
        ? hasRole(me.roles, 'tenant_admin', 'billing', 'integration')
        : ['insurance', 'inventory'].includes(step)
          ? hasRole(me.roles, 'tenant_admin', 'clinical', 'integration')
          : clinical;
    if (!allowed) throw new ApiError(403, 'Your role cannot record this outcome.');
    const expected =
      step === 'billing'
        ? 'awaiting_billing'
        : step === 'administration'
          ? 'scheduled'
          : 'awaiting_approvals';
    if (w.status !== expected)
      throw new ApiError(409, 'The workflow has moved to another stage. Refresh and try again.');
    w.outcomes[step] = outcome;
    advance(me, w);
    result = w;
    emit(me, 'workflow.outcome_recorded', w.id);
  } else if (kind === 'workflows' && action === 'cancel') {
    if (!clinical) throw new ApiError(403, 'Clinical access required.');
    const w = db.workflows.find((x) => x.id === id);
    if (!w) throw new ApiError(404, 'Workflow not found.');
    if (['completed', 'awaiting_billing'].includes(w.status) || w.outcomes.administration === 'completed')
      throw new ApiError(409, 'A workflow with recorded administration cannot be cancelled.');
    w.status = 'cancelled';
    db.orders.find((o) => o.id === w.order_id)!.status = 'cancelled';
    db.appointments
      .filter((a) => a.order_id === w.order_id && a.status === 'confirmed')
      .forEach((a) => (a.status = 'cancelled'));
    result = w;
    emit(me, 'workflow.cancelled', w.id);
  } else if (kind === 'schedules' && action === 'actions') {
    if (!admin) throw new ApiError(403, 'Administrator access required.');
    const schedule = db.schedules.find((x) => x.id === id);
    if (!schedule) throw new ApiError(404, 'Schedule not found.');
    if (payload.action === 'pause') schedule.paused = true;
    else if (payload.action === 'resume') schedule.paused = false;
    else if (payload.action === 'trigger')
      db.jobs.unshift({
        id: crypto.randomUUID(),
        tenant_id: me.tenant.id,
        kind: schedule.kind,
        target_id: id,
        status: 'queued',
        attempt: 0,
        max_attempts: 5,
        memory_mb: 8,
        due_at: new Date().toISOString(),
        version: me.tenant.version,
      });
    else throw new ApiError(400, 'Unknown schedule action.');
    schedule.version++;
    result = schedule;
    emit(me, `schedule.${payload.action}`, id);
  } else if (kind === 'jobs' && action === 'actions') {
    if (!admin) throw new ApiError(403, 'Administrator access required.');
    const job = db.jobs.find((x) => x.id === id);
    if (!job) throw new ApiError(404, 'Job not found.');
    if (payload.action === 'retry') {
      if (!['dead', 'cancelled'].includes(job.status))
        throw new ApiError(409, 'Only failed or cancelled jobs can be retried.');
      job.status = 'queued';
      job.attempt = 0;
    } else if (payload.action === 'cancel') job.status = 'cancelled';
    else throw new ApiError(400, 'Unknown job action.');
    result = job;
    emit(me, `job.${payload.action}`, id);
  } else if (kind === 'patients' && method === 'PATCH') {
    if (!clinical) throw new ApiError(403, 'Clinical access required.');
    const p = db.patients.find((x) => x.id === id);
    if (!p) throw new ApiError(404, 'Patient not found.');
    Object.assign(p, payload);
    result = p;
    emit(me, 'patient.updated', id);
  } else if (kind === 'appointments' && action === 'cancel') {
    if (!clinical) throw new ApiError(403, 'Clinical access required.');
    const a = db.appointments.find((x) => x.id === id);
    if (!a) throw new ApiError(404, 'Appointment not found.');
    const order = db.orders.find((o) => o.id === a.order_id)!;
    const w = db.workflows.find((x) => x.id === order.workflow_id)!;
    if (w.outcomes.administration === 'completed')
      throw new ApiError(409, 'Administration has already been recorded.');
    a.status = 'cancelled';
    w.status = 'ready_to_schedule';
    order.status = 'ready_to_schedule';
    result = a;
    emit(me, 'appointment.cancelled', id);
  } else if (method === 'POST' && kind in db) {
    if (['facilities', 'staff', 'care-resources', 'schedules'].includes(kind) && !admin)
      throw new ApiError(403, 'Administrator access required.');
    if (['patients', 'orders', 'appointments'].includes(kind) && !clinical)
      throw new ApiError(403, 'Clinical access required.');
    const entity = {
      id: crypto.randomUUID(),
      tenant_id: me.tenant.id,
      ...payload,
      created_at: new Date().toISOString(),
    } as Entity;
    if (kind === 'patients') {
      if (db.patients.some((p) => p.external_id === payload.external_id))
        throw new ApiError(409, 'That patient reference already exists in this workspace.');
      if (!db.facilities.some((f) => f.id === payload.facility_id))
        throw new ApiError(400, 'Choose a valid facility.');
    }
    if (kind === 'orders') {
      const p = db.patients.find((x) => x.id === payload.patient_id);
      if (!p) throw new ApiError(404, 'Patient not found.');
      entity.facility_id = p.facility_id;
      entity.status = 'awaiting_approvals';
      entity.workflow_id = crypto.randomUUID();
      db.workflows.unshift({
        id: String(entity.workflow_id),
        tenant_id: me.tenant.id,
        facility_id: p.facility_id,
        order_id: entity.id,
        status: 'awaiting_approvals',
        outcomes: {},
        checkpoints: ['patient_and_prescription_reference_validated'],
        updated_at: new Date().toISOString(),
        version: me.tenant.version,
        definition_version: 1,
      });
    }
    if (kind === 'appointments') {
      const o = db.orders.find((x) => x.id === payload.order_id);
      const w = db.workflows.find((x) => x.id === o?.workflow_id);
      if (!o || !w || w.status !== 'ready_to_schedule')
        throw new ApiError(409, 'This order is not ready to schedule.');
      const n = db.staff.find((x) => x.id === payload.nurse_id);
      const r = db['care-resources'].find((x) => x.id === payload.resource_id);
      if (
        !n?.active ||
        !n.qualifications.includes('infusion') ||
        n.facility_id !== o.facility_id ||
        r?.facility_id !== o.facility_id
      )
        throw new ApiError(400, 'Choose an infusion-qualified nurse and resource at the order facility.');
      const start = String(payload.starts_at),
        end = String(payload.ends_at);
      if (end <= start || new Date(start) < new Date())
        throw new ApiError(400, 'Choose a future appointment with a valid end time.');
      if (
        db.appointments.some(
          (a) =>
            a.status === 'confirmed' &&
            a.starts_at < end &&
            a.ends_at > start &&
            (a.nurse_id === payload.nurse_id ||
              a.resource_id === payload.resource_id ||
              a.patient_id === o.patient_id),
        )
      )
        throw new ApiError(409, 'The patient, nurse, or chair is already reserved during this time.');
      Object.assign(entity, { patient_id: o.patient_id, facility_id: o.facility_id, status: 'confirmed' });
      w.status = 'scheduled';
      w.checkpoints.push('scheduled');
      o.status = 'scheduled';
    }
    if (kind === 'schedules')
      Object.assign(entity, {
        paused: false,
        timezone: me.tenant.timezone,
        next_run: new Date(Date.now() + 15 * 60000).toISOString(),
        version: 1,
      });
    if (kind === 'staff') entity.active = true;
    (db[kind] as Entity[]).unshift(entity);
    result = entity;
    emit(me, `${kind}.created`, entity.id);
  } else throw new ApiError(400, 'This action is not available.');
  if (key) replays.set(replayKey, { body: JSON.stringify(body), result: structuredClone(result) });
  return structuredClone(result) as T;
}
