import { beforeEach, describe, expect, it } from 'vitest';
import { demoDatabase, demoMe, resetDemo } from './demoData';
import { ApiError, demoRequest, overview } from './demoClient';
import type { Patient, Workflow } from '../shared/types';

describe('isolated synthetic transport', () => {
  beforeEach(resetDemo);
  it('keeps identifiers and mutations in their owning tenant', () => {
    const north = demoMe();
    const south = demoMe('south-health');
    const input = {
      name: 'Synthetic isolation case',
      external_id: 'isolation-case',
      birth_date: '1992-01-01',
      facility_id: demoDatabase(north.tenant.id).facilities[0].id,
    };
    const patient = demoRequest<Patient>(north, '/patients', 'POST', input, crypto.randomUUID());
    expect(demoDatabase(south.tenant.id).patients.some((p) => p.id === patient.id)).toBe(false);
    expect(() => demoRequest(south, `/patients/${patient.id}`)).toThrow('Record not found');
  });
  it('deduplicates retries and rejects changed inputs', () => {
    const me = demoMe();
    const input = {
      name: 'Synthetic retry case',
      external_id: 'retry-case',
      birth_date: '1992-01-01',
      facility_id: demoDatabase(me.tenant.id).facilities[0].id,
    };
    const key = crypto.randomUUID();
    const first = demoRequest<Patient>(me, '/patients', 'POST', input, key);
    expect(demoRequest<Patient>(me, '/patients', 'POST', input, key).id).toBe(first.id);
    expect(() => demoRequest(me, '/patients', 'POST', { ...input, name: 'Changed' }, key)).toThrow(ApiError);
  });
  it('denies clinical records to auditor and billing roles', () => {
    expect(() => demoRequest(demoMe('north-health', 'auditor'), '/patients')).toThrow('Your role');
    expect(() => demoRequest(demoMe('north-health', 'billing'), '/patients')).toThrow('Your role');
    expect(overview(demoMe('north-health', 'auditor')).counts).not.toHaveProperty('patients');
  });
  it('requires explicit prerequisites and guards cancellation after administration', () => {
    const me = demoMe();
    const db = demoDatabase(me.tenant.id);
    const w = db.workflows[0];
    const path = `/workflows/${w.id}/outcomes`;
    demoRequest(me, path, 'POST', { step: 'insurance', outcome: 'approved' }, crypto.randomUUID());
    expect(w.status).toBe('awaiting_approvals');
    demoRequest(me, path, 'POST', { step: 'inventory', outcome: 'available' }, crypto.randomUUID());
    expect(w.status).toBe('ready_to_schedule');
    w.status = 'scheduled';
    demoRequest(me, path, 'POST', { step: 'administration', outcome: 'completed' }, crypto.randomUUID());
    expect(w.status).toBe('awaiting_billing');
    expect(() =>
      demoRequest(me, `/workflows/${w.id}/cancel`, 'POST', undefined, crypto.randomUUID()),
    ).toThrow('cannot be cancelled');
  });
});
