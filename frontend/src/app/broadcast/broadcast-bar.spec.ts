import { Pane } from '../sessions/sessions.store';
import { targetOptions } from './broadcast-bar';

const pane = (key: number, group: string[], hostId = ''): Pane =>
  ({ key, group, source: hostId ? { kind: 'host', hostId } : { kind: 'local', program: '' } }) as unknown as Pane;

describe('targetOptions', () => {
  it('offers all panes, each group and each tag', () => {
    const tags: Record<string, string[]> = { a: ['web', 'prod'], b: ['web'] };
    const opts = targetOptions([pane(1, ['Prod', 'EU'], 'a'), pane(2, ['Prod', 'EU'], 'b'), pane(3, [])], (id) => tags[id] ?? []);
    expect(opts.map((o) => [o.value, o.keys])).toEqual([
      ['all', [1, 2, 3]],
      ['g:Prod / EU', [1, 2]],
      ['t:prod', [1]],
      ['t:web', [1, 2]],
    ]);
  });
});
