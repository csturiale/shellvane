import type { Group, Host } from '../core/backend/host-api';
import { buildRows, groupPaths, subtreeIds } from './tree';

const group = (id: string, name: string, parentId = ''): Group => ({ id, name, parentId, color: '', settings: {} });
const host = (id: string, name: string, groupId = '', hostname = name): Host => ({
  id,
  name,
  hostname,
  groupId,
  notes: '',
  settings: {},
  tags: [],
  favorite: false,
  lastUsed: '',
});

const groups = [group('prod', 'Production'), group('web', 'web', 'prod'), group('dev', 'dev')];
const hosts = [
  host('h1', 'web-1', 'web', '10.0.0.1'),
  host('h2', 'db', 'prod'),
  host('h3', 'bastion'),
  host('h4', 'Alpha', 'web'),
];

const labels = (rows: ReturnType<typeof buildRows>) =>
  rows.map((r) => (r.kind === 'group' ? `${'  '.repeat(r.depth)}[${r.group.name}]` : `${'  '.repeat(r.depth)}${r.host.name}`));

describe('buildRows', () => {
  it('shows only top-level items when nothing is expanded', () => {
    expect(labels(buildRows(groups, hosts, new Set()))).toEqual(['[dev]', '[Production]', 'bastion']);
  });

  it('lists subgroups before hosts, sorted by name', () => {
    expect(labels(buildRows(groups, hosts, new Set(['prod', 'web'])))).toEqual([
      '[dev]',
      '[Production]',
      '  [web]',
      '    Alpha',
      '    web-1',
      '  db',
      'bastion',
    ]);
  });

  it('counts hosts recursively', () => {
    const prod = buildRows(groups, hosts, new Set()).find((r) => r.kind === 'group' && r.group.id === 'prod');
    expect(prod?.kind === 'group' && prod.hostCount).toBe(3);
  });

  it('filters by host name or address and opens the ancestors', () => {
    expect(labels(buildRows(groups, hosts, new Set(), '10.0.0'))).toEqual(['[Production]', '  [web]', '    web-1']);
  });

  it('shows the whole content of a group whose name matches', () => {
    expect(labels(buildRows(groups, hosts, new Set(), 'web'))).toEqual([
      '[Production]',
      '  [web]',
      '    Alpha',
      '    web-1',
    ]);
  });

  it('returns nothing when no item matches', () => {
    expect(buildRows(groups, hosts, new Set(), 'nothing')).toEqual([]);
  });
});

describe('subtreeIds', () => {
  it('includes the group and all descendants', () => {
    expect([...subtreeIds(groups, 'prod')].sort()).toEqual(['prod', 'web']);
  });
});

describe('groupPaths', () => {
  it('builds sorted full paths', () => {
    expect(groupPaths(groups).map((p) => p.path)).toEqual(['dev', 'Production', 'Production / web']);
  });

  it('filters by tag with # and by notes and tags with text', () => {
    const tagged = { ...host('h1', 'web-1'), tags: ['prod', 'eu'], notes: 'runs nginx' };
    const other = host('h2', 'db-1');
    expect(buildRows([], [tagged, other], new Set(), '#pro').map((r) => (r.kind === 'host' ? r.host.id : ''))).toEqual(['h1']);
    expect(buildRows([], [tagged, other], new Set(), 'nginx')).toHaveLength(1);
    expect(buildRows([], [tagged, other], new Set(), 'eu')).toHaveLength(1);
    expect(buildRows([], [tagged, other], new Set(), '#web')).toHaveLength(0);
  });
});
