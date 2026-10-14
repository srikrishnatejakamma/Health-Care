import type { Edge, Node } from '@xyflow/react';
export type StepData = {
  label: string;
  handler: string;
  retry: number;
  timeout: number;
  compensation: string;
  [key: string]: unknown;
};
export function validateGraph(nodes: Node<StepData>[], edges: Edge[]): string[] {
  const errors: string[] = [];
  if (!nodes.length) return ['Add at least one workflow step.'];
  const ids = new Set(nodes.map((n) => n.id));
  const outgoing = new Map(nodes.map((n) => [n.id, [] as string[]]));
  for (const e of edges) {
    if (!ids.has(e.source) || !ids.has(e.target)) {
      errors.push('A connection references a missing step.');
      continue;
    }
    outgoing.get(e.source)!.push(e.target);
  }
  const visiting = new Set<string>(),
    done = new Set<string>();
  let cycle = false;
  function visit(id: string) {
    if (visiting.has(id)) {
      cycle = true;
      return;
    }
    if (done.has(id)) return;
    visiting.add(id);
    for (const to of outgoing.get(id) || []) visit(to);
    visiting.delete(id);
    done.add(id);
  }
  nodes.forEach((n) => visit(n.id));
  if (cycle) errors.push('Workflow connections must not contain cycles.');
  const incoming = new Set(edges.map((e) => e.target));
  const roots = nodes.filter((n) => !incoming.has(n.id));
  if (roots.length !== 1) errors.push('Use exactly one starting step.');
  for (const node of nodes) {
    if (!node.data.label.trim() || !node.data.handler.trim())
      errors.push('Every step needs a name and handler.');
    if (node.data.retry < 0 || node.data.retry > 10 || node.data.timeout < 1)
      errors.push('Use 0–10 retries and a positive timeout.');
  }
  if (nodes.length > 1 && nodes.some((n) => !edges.some((e) => e.source === n.id || e.target === n.id)))
    errors.push('Connect all workflow steps.');
  return [...new Set(errors)];
}
export const starterNodes: Node<StepData>[] = [
  {
    id: 'order',
    position: { x: 260, y: 40 },
    type: 'careStep',
    data: { label: 'Order received', handler: 'therapy.order', retry: 0, timeout: 30, compensation: '' },
  },
  {
    id: 'validate',
    position: { x: 260, y: 170 },
    type: 'careStep',
    data: {
      label: 'Validate patient & reference',
      handler: 'patient.validate',
      retry: 2,
      timeout: 30,
      compensation: '',
    },
  },
  {
    id: 'insurance',
    position: { x: 100, y: 310 },
    type: 'careStep',
    data: {
      label: 'Insurance authorization',
      handler: 'insurance.wait',
      retry: 3,
      timeout: 3600,
      compensation: '',
    },
  },
  {
    id: 'inventory',
    position: { x: 420, y: 310 },
    type: 'careStep',
    data: {
      label: 'Medication readiness',
      handler: 'pharmacy.wait',
      retry: 3,
      timeout: 3600,
      compensation: '',
    },
  },
  {
    id: 'eligibility',
    position: { x: 260, y: 450 },
    type: 'careStep',
    data: {
      label: 'Clinical clearance',
      handler: 'clinical.approval',
      retry: 0,
      timeout: 3600,
      compensation: '',
    },
  },
  {
    id: 'appointment',
    position: { x: 260, y: 590 },
    type: 'careStep',
    data: {
      label: 'Confirm appointment',
      handler: 'appointment.reserve',
      retry: 2,
      timeout: 60,
      compensation: 'appointment.release',
    },
  },
  {
    id: 'billing',
    position: { x: 260, y: 730 },
    type: 'careStep',
    data: {
      label: 'Billing & reconciliation',
      handler: 'billing.submit',
      retry: 3,
      timeout: 60,
      compensation: 'billing.review',
    },
  },
];
export const starterEdges: Edge[] = [
  ['order', 'validate'],
  ['validate', 'insurance'],
  ['validate', 'inventory'],
  ['insurance', 'eligibility'],
  ['inventory', 'eligibility'],
  ['eligibility', 'appointment'],
  ['appointment', 'billing'],
].map(([source, target]) => ({
  id: `${source}-${target}`,
  source,
  target,
  type: 'smoothstep',
  animated: false,
  style: { stroke: '#aab8d5', strokeWidth: 1.5 },
}));
