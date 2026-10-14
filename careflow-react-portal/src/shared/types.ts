export type Role = 'tenant_admin' | 'clinical' | 'billing' | 'integration' | 'auditor';
export interface Entity {
  id: string;
  tenant_id: string;
  facility_id?: string;
  [key: string]: unknown;
}
export interface Facility extends Entity {
  name: string;
  capacity: number;
}
export interface Patient extends Entity {
  name: string;
  external_id: string;
  birth_date: string;
  facility_id: string;
  created_at: string;
}
export interface Order extends Entity {
  patient_id: string;
  medication: string;
  prescription: string;
  workflow_id: string;
  status: string;
  facility_id: string;
  created_at: string;
}
export interface Workflow extends Entity {
  order_id: string;
  status: string;
  outcomes: Record<string, string>;
  checkpoints: string[];
  updated_at: string;
  version: number;
  definition_version: number;
}
export interface Appointment extends Entity {
  order_id: string;
  patient_id: string;
  nurse_id: string;
  resource_id: string;
  starts_at: string;
  ends_at: string;
  status: string;
}
export interface Job extends Entity {
  kind: string;
  target_id: string;
  status: string;
  attempt: number;
  max_attempts: number;
  due_at: string;
  memory_mb: number;
  version: number;
  last_error?: string;
}
export interface Schedule extends Entity {
  kind: string;
  expression: string;
  timezone: string;
  paused: boolean;
  next_run: string;
  version: number;
}
export interface Staff extends Entity {
  name: string;
  active: boolean;
  qualifications: string[];
}
export interface CareResource extends Entity {
  name: string;
  kind: string;
}
export interface Audit extends Entity {
  actor: string;
  action: string;
  resource_id: string;
  correlation_id: string;
  at: string;
}
export interface Event extends Entity {
  event_id: string;
  event_type: string;
  aggregate_id: string;
  schema_version: number;
  configuration_version: number;
  correlation_id: string;
  occurred_at: string;
  published: boolean;
  sequence: number;
}
export interface ResourceMap {
  facilities: Facility;
  patients: Patient;
  orders: Order;
  workflows: Workflow;
  appointments: Appointment;
  jobs: Job;
  schedules: Schedule;
  staff: Staff;
  'care-resources': CareResource;
  audit: Audit;
  events: Event;
}
export type Kind = keyof ResourceMap;
export interface Page<T> {
  items: T[];
  next_cursor?: string;
}
export interface Tenant {
  id: string;
  name: string;
  status: string;
  timezone: string;
  region: string;
  version: number;
  isolation_mode: string;
  quotas: { concurrent_jobs: number; queued_jobs: number; memory_mb: number; requests_per_minute: number };
}
export interface Me {
  tenant: Tenant;
  user_id: string;
  roles: Role[];
  facilities: string[];
}
export interface Overview {
  counts: Record<string, number>;
  series: { date: string; orders: number; completed: number }[];
  statuses: { name: string; count: number }[];
  generated_at: string;
  days: number;
}
export interface Operations {
  scope: string;
  store: { status: string };
  databases: { name: string; status: string; description: string }[];
  workers: { configured: number; completed: number; failed: number; reserved_mb: number };
  memory: {
    heap_bytes: number;
    heap_sys_bytes: number;
    gc_cycles: number;
    gc_pause_ns: number;
    goroutines: number;
    soft_limit_bytes: number;
  };
  capabilities: Record<string, boolean>;
}
export const kindRoles: Record<Kind, Role[]> = {
  facilities: ['tenant_admin', 'clinical', 'billing', 'auditor', 'integration'],
  patients: ['tenant_admin', 'clinical'],
  orders: ['tenant_admin', 'clinical', 'billing', 'integration'],
  workflows: ['tenant_admin', 'clinical', 'billing', 'integration'],
  appointments: ['tenant_admin', 'clinical'],
  staff: ['tenant_admin', 'clinical'],
  'care-resources': ['tenant_admin', 'clinical'],
  jobs: ['tenant_admin', 'auditor'],
  schedules: ['tenant_admin', 'auditor'],
  audit: ['tenant_admin', 'auditor'],
  events: ['tenant_admin', 'auditor'],
};
export const canRead = (kind: Kind, roles: Role[]) => roles.some((role) => kindRoles[kind].includes(role));
export const hasRole = (roles: Role[], ...allowed: Role[]) => roles.some((role) => allowed.includes(role));
