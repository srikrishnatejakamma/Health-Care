import { describe, expect, it } from 'vitest';
import { starterNodes, starterEdges, validateGraph } from './graph';
describe('workflow draft validation', () => {
  it('accepts a connected parallel DAG', () => expect(validateGraph(starterNodes, starterEdges)).toEqual([]));
  it('rejects loops', () =>
    expect(
      validateGraph(starterNodes, [...starterEdges, { id: 'loop', source: 'billing', target: 'order' }]),
    ).toContain('Workflow connections must not contain cycles.'));
  it('rejects disconnected steps and missing handlers', () => {
    const nodes = [
      ...starterNodes,
      { ...starterNodes[0], id: 'orphan', data: { ...starterNodes[0].data, handler: '' } },
    ];
    const errors = validateGraph(nodes, starterEdges);
    expect(errors).toContain('Connect all workflow steps.');
    expect(errors).toContain('Every step needs a name and handler.');
  });
});
