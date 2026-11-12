import { TestBed } from '@angular/core/testing';

import { ProcedureApi, Source } from '../core/backend/procedure-api';
import { ProceduresStore } from './procedures.store';

describe('ProceduresStore', () => {
  let calls: string[];
  let store: ProceduresStore;

  beforeEach(() => {
    localStorage.removeItem('shellvane.procedures.sources');
    localStorage.removeItem('shellvane.procedures.current');
    calls = [];
    TestBed.configureTestingModule({
      providers: [
        {
          provide: ProcedureApi,
          useValue: {
            list: async (s: Source) => {
              calls.push('list ' + s.url);
              return { docs: [{ path: 'db/restore.md', title: 'Restore' }], synced: '2026-11-12T10:00:00Z' };
            },
            sync: async (s: Source) => {
              calls.push('sync ' + s.url);
              return { docs: [], synced: '' };
            },
            read: async (_s: Source, path: string) => `# ${path}\n`,
          },
        },
      ],
    });
    store = TestBed.inject(ProceduresStore);
  });

  it('keeps sources, lists, opens and tracks the steps done', async () => {
    store.addSource({ name: 'Ops', kind: 'git', url: 'git@example.com:ops.git', branch: '', folder: '' });
    await store.load();
    expect(calls).toContain('list git@example.com:ops.git');
    expect(store.docs()).toEqual([{ path: 'db/restore.md', title: 'Restore' }]);
    expect(JSON.parse(localStorage.getItem('shellvane.procedures.sources')!)[0].name).toBe('Ops');

    await store.open('db/restore.md');
    expect(store.doc()).toEqual({ path: 'db/restore.md', title: 'Restore', markdown: '# db/restore.md\n' });
    store.markDone(0);
    store.markDone(2);
    store.markDone(2, false);
    expect([...store.done()]).toEqual([0]);
    store.setValues({ namespace: 'web' });
    expect(store.values()).toEqual({ namespace: 'web' });
    store.resetDone();
    expect(store.done().size).toBe(0);

    await store.load(true);
    expect(calls).toContain('sync git@example.com:ops.git');
    store.removeSource(store.current()!.id);
    expect(store.sources()).toEqual([]);
  });
});
