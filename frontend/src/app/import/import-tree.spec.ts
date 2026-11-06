import { ImportedHost } from '../core/backend/import-api';
import { importRows, withDefaultUser } from './import-tree';

const host = (name: string, group: string[] = [], user = ''): ImportedHost => ({ name, hostname: name + '.example', user, group }) as ImportedHost;

describe('importRows', () => {
  const hosts = [host('top'), host('qa1', ['QA']), host('nex1', ['QA', 'NEX']), host('prod1', ['PROD']), host('nex2', ['qa', 'nex'])];

  it('nests groups, groups first, and lists every host below a group', () => {
    const rows = importRows(hosts);
    const shape = rows.map((r) => (r.kind === 'group' ? `${'  '.repeat(r.depth)}[${r.name}] ${r.hosts.length}` : `${'  '.repeat(r.depth)}${hosts[r.index].name}`));
    expect(shape).toEqual(['[PROD] 1', '  prod1', '[QA] 3', '  [NEX] 2', '    nex1', '    nex2', '  qa1', 'top']);
  });

  it('hides the rows of collapsed groups', () => {
    const rows = importRows(hosts, new Set(['/qa']));
    expect(rows.map((r) => (r.kind === 'group' ? r.name : hosts[r.index].name))).toEqual(['PROD', 'prod1', 'QA', 'top']);
  });
});

describe('withDefaultUser', () => {
  it('fills only the hosts without a user', () => {
    const out = withDefaultUser([host('a'), host('b', [], 'root')], ' deploy ');
    expect(out.map((h) => h.user)).toEqual(['deploy', 'root']);
    expect(withDefaultUser([host('a')], '')[0].user).toBe('');
  });
});
