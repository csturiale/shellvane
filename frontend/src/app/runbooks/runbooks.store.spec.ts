import { TestBed } from '@angular/core/testing';

import { RunbookApi, RunbookCell } from '../core/backend/runbook-api';
import { RunbooksStore, newRunbook, summary } from './runbooks.store';

describe('RunbooksStore', () => {
  let onCell: (c: RunbookCell) => void;
  let onDone: (d: { runId: string }) => void;
  let store: RunbooksStore;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: RunbookApi,
          useValue: {
            onCell: (h: typeof onCell) => ((onCell = h), () => undefined),
            onDone: (h: typeof onDone) => ((onDone = h), () => undefined),
            start: async () => {
              // Events can arrive before Start returns.
              onCell({ runId: 'r1', hostId: 'h1', step: 0, cell: { state: 'running', command: 'uptime' } });
              return { id: 'r1', runbook: newRunbook(), hosts: [{ id: 'h1', name: 'a' }, { id: 'h2', name: 'b' }] };
            },
          },
        },
      ],
    });
    store = TestBed.inject(RunbooksStore);
  });

  it('keeps events that arrive before the run is known', async () => {
    const rb = { ...newRunbook(), name: 'x' };
    const run = await store.start(rb, {});
    expect(run.cells['h1'][0].state).toBe('running');
    expect(run.cells['h2'][0].state).toBe('pending');
  });

  it('applies cells and completion', async () => {
    await store.start(newRunbook(), {});
    onCell({ runId: 'r1', hostId: 'h2', step: 0, cell: { state: 'failed', command: 'uptime' } });
    onDone({ runId: 'r1' });
    const run = store.runs()[0];
    expect(run.done).toBe(true);
    expect(summary(run)).toEqual({ running: 1, failed: 1 });
  });
});
