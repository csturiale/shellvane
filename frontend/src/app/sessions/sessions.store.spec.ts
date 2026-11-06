import { TestBed } from '@angular/core/testing';

import type { Prompt, SessionInfo, SessionState } from '../core/backend/session-api';
import { SessionApi } from '../core/backend/session-api';
import { arrange, panesOf } from './layout';
import { RETRY_MAX_ATTEMPTS, SessionsStore, addressOf, labelOf, retryDelay } from './sessions.store';

class FakeSessionApi {
  private n = 0;
  stateHandler: (s: SessionState) => void = () => undefined;
  promptHandler: (p: Prompt) => void = () => undefined;
  closed: string[] = [];
  opened: string[] = [];
  answers: { id: string; cancel: boolean; value: string }[] = [];
  /** Lets a test emit events while OpenHost is still in flight. */
  beforeReturn: (id: string) => void = () => undefined;

  openOptions: { persist: string; resume: boolean }[] = [];

  async openHost(hostId: string, _cols = 0, _rows = 0, opts = { persist: '', resume: false }): Promise<SessionInfo> {
    const id = `s${++this.n}`;
    this.opened.push(`host:${hostId}`);
    this.openOptions.push(opts);
    this.beforeReturn(id);
    return {
      id,
      title: 'web-1',
      target: { user: 'deploy', host: '10.0.0.1', port: 22 },
      group: ['Prod', 'web'],
      color: 'red',
      terminalTheme: '',
      fontSize: 0,
      via: [],
      multiplexer: 'tmux',
      highlights: [],
      logging: false,
      recording: false,
      local: false,
      protocol: '',
      autoReconnect: true,
    };
  }
  async openTarget(spec: string): Promise<SessionInfo> {
    this.opened.push(`target:${spec}`);
    return {
      id: `s${++this.n}`,
      title: spec,
      target: { user: 'me', host: spec, port: 22 },
      group: [],
      color: '',
      terminalTheme: '',
      fontSize: 0,
      via: [],
      multiplexer: '',
      highlights: [],
      logging: false,
      recording: false,
      local: false,
      protocol: '',
      autoReconnect: true,
    };
  }
  async answer(id: string, a: { cancel: boolean; value: string }) {
    this.answers.push({ id, ...a });
  }
  async resize() {}
  async setBroadcast() {}
  kubeSet: { id: string; context: string }[] = [];
  localOptions: { program: string; kube?: { context: string; namespace: string } }[] = [];
  async openLocal(_cols: number, _rows: number, opts: { program: string; kube?: { context: string; namespace: string } }): Promise<SessionInfo> {
    this.localOptions.push(opts);
    return {
      id: `s${++this.n}`,
      title: 'zsh',
      target: { user: 'me', host: 'laptop', port: 0 },
      group: [],
      color: '',
      terminalTheme: '',
      fontSize: 0,
      via: [],
      multiplexer: '',
      highlights: [],
      logging: false,
      recording: false,
      local: true,
      protocol: '',
      autoReconnect: false,
      kube: { context: opts.kube?.context ?? '', namespace: opts.kube?.namespace ?? '', live: true },
    };
  }
  async setKube(id: string, sel: { context: string }) {
    this.kubeSet.push({ id, context: sel.context });
  }
  async close(id: string) {
    this.closed.push(id);
  }
  onState(h: (s: SessionState) => void) {
    this.stateHandler = h;
    return () => undefined;
  }
  onPrompt(h: (p: Prompt) => void) {
    this.promptHandler = h;
    return () => undefined;
  }
  detached = new Map<string, string>();
  async detach(payload: string, ids: string[]): Promise<void> {
    this.detached.set('tok', payload);
    this.moved = ids;
  }
  moved: string[] = [];
  async takeDetached(token: string): Promise<string> {
    return this.detached.get(token)!;
  }
}

describe('SessionsStore', () => {
  let api: FakeSessionApi;
  let store: SessionsStore;

  beforeEach(() => {
    api = new FakeSessionApi();
    TestBed.configureTestingModule({ providers: [{ provide: SessionApi, useValue: api }] });
    store = TestBed.inject(SessionsStore);
  });

  it('opens a tab with one pane and tracks its state', async () => {
    await store.openHost('h1', 'fallback');
    expect(store.tabs()).toHaveLength(1);
    expect(store.focused()?.title).toBe('web-1');
    expect(store.focused()?.state).toBe('connecting');

    api.stateHandler({ id: 's1', state: 'connected', exitCode: 0, lost: false });
    expect(store.focused()?.state).toBe('connected');
  });

  it('describes each pane so it can be recognized', async () => {
    await store.openHost('h1', 'fallback');
    const pane = store.focused()!;
    expect(pane).toMatchObject({ title: 'web-1', group: ['Prod', 'web'], color: 'red' });
    expect(addressOf(pane)).toBe('deploy@10.0.0.1:22');
    expect(addressOf({ target: { user: 'u', host: '::1', port: 2200 } })).toBe('u@[::1]:2200');
    expect(addressOf({ target: { user: '', host: 'switch1', port: 23 }, protocol: 'telnet' })).toBe('telnet://switch1:23');
    expect(addressOf({ target: { user: '', host: '/dev/ttyUSB0', port: 0 }, protocol: 'serial' })).toBe('/dev/ttyUSB0');
    expect(labelOf(pane)).toBe('web-1 · deploy@10.0.0.1:22');
    expect(labelOf({ title: 'me@h:22', target: { user: 'me', host: 'h', port: 22 } })).toBe('me@h:22');
  });

  it('keeps a failure that arrives before the pane exists', async () => {
    // Codes are a generated enum on the TS side; any Go code may arrive.
    const error = { code: 'ssh.dialFailed', params: { host: 'x' } } as unknown as SessionState['error'];
    api.beforeReturn = (id) => api.stateHandler({ id, state: 'failed', exitCode: 0, lost: false, error });
    await store.openHost('h1', 'x');
    expect(store.focused()?.state).toBe('failed');
    expect(store.focused()?.error?.key).toBe('errors.ssh.dialFailed');
  });

  it('queues prompts and drops them when the connection ends', async () => {
    await store.openTarget('me@host');
    const target = { host: 'host', port: 22, user: 'me' };
    api.promptHandler({ id: 'p1', sessionId: 's1', kind: 'password', target, attempt: 1, canSave: false });
    expect(store.prompts()).toHaveLength(1);

    await store.answer(store.prompts()[0], { value: 'pw', decision: '', cancel: false, save: false, answers: [] });
    expect(api.answers).toEqual([{ id: 'p1', value: 'pw', decision: '', cancel: false, save: false, answers: [] }]);
    expect(store.prompts()).toHaveLength(0);

    api.promptHandler({ id: 'p2', sessionId: 's1', kind: 'password', target, attempt: 2, canSave: false });
    api.stateHandler({ id: 's1', state: 'failed', exitCode: 0, lost: false });
    expect(store.prompts()).toHaveLength(0);
  });

  it('splits the focused pane with a new session to the same host', async () => {
    await store.openHost('h1', 'web-1');
    await store.split('row');
    await store.split('column');

    const tab = store.active()!;
    expect(api.opened).toEqual(['host:h1', 'host:h1', 'host:h1']);
    expect(panesOf(tab.layout)).toHaveLength(3);
    const rects = arrange(tab.layout).panes;
    const [first, second, third] = panesOf(tab.layout);
    expect(rects.get(first)).toEqual({ x: 0, y: 0, w: 50, h: 100 });
    expect(rects.get(second)).toEqual({ x: 50, y: 0, w: 50, h: 50 });
    expect(rects.get(third)).toEqual({ x: 50, y: 50, w: 50, h: 50 });
    expect(tab.focus).toBe(third);

    store.moveFocus('left');
    expect(store.active()!.focus).toBe(first);
  });

  it('moves a tab to another window without closing its sessions', async () => {
    await store.openHost('h1', 'web-1');
    await store.split('row');
    const tab = store.active()!;
    await store.detachTab(tab.key);
    expect(api.moved).toEqual(['s1', 's2']);
    expect(api.closed).toEqual([]);
    expect(store.tabs()).toHaveLength(0);
    expect(store.panes()).toHaveLength(0);

    // The other window rebuilds the same layout on the same sessions.
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [{ provide: SessionApi, useValue: api }] });
    const other = TestBed.inject(SessionsStore);
    await other.adopt('tok');
    const moved = other.active()!;
    expect(other.panes().map((p) => [p.sessionId, p.replay])).toEqual([
      ['s1', true],
      ['s2', true],
    ]);
    expect(panesOf(moved.layout)).toEqual(other.panes().map((p) => p.key));
    expect(moved.focus).toBe(other.panes()[1].key);
    api.stateHandler({ id: 's2', state: 'closed', exitCode: 0, lost: false });
    expect(other.panes()[1].state).toBe('closed');
  });

  it('closing a pane keeps the tab until the last pane closes', async () => {
    await store.openTarget('a');
    await store.split('row');
    const [first, second] = panesOf(store.active()!.layout);

    await store.closePane(second);
    expect(api.closed).toEqual(['s2']);
    expect(store.tabs()).toHaveLength(1);
    expect(store.active()!.focus).toBe(first);

    await store.closePane(first);
    expect(store.tabs()).toHaveLength(0);
    expect(store.panes()).toHaveLength(0);
  });

  it('closing a tab ends all its sessions and activates a neighbour', async () => {
    await store.openTarget('a');
    await store.openTarget('b');
    await store.split('row');
    await store.openTarget('c');
    store.activeKey.set(store.tabs()[1].key);

    await store.closeTab(store.tabs()[1].key);
    expect(api.closed.sort()).toEqual(['s2', 's3']);
    expect(store.tabs()).toHaveLength(2);
    expect(store.focused()?.title).toBe('c');
  });

  it('reconnects in the same pane with a new session', async () => {
    await store.openHost('h1', 'web-1');
    const key = store.focused()!.key;
    api.stateHandler({ id: 's1', state: 'closed', exitCode: 0, lost: false });

    await store.reconnect(key);
    expect(store.panes()).toHaveLength(1);
    expect(store.pane(key)).toMatchObject({ sessionId: 's2', state: 'connecting' });
  });

  it('mirrors input to every broadcasting pane except excluded ones', async () => {
    await store.openTarget('a');
    await store.split('row');
    await store.split('column');
    const tab = store.active()!;
    const [p1, p2, p3] = panesOf(tab.layout);
    const got: Record<number, string> = { [p1]: '', [p2]: '', [p3]: '' };
    for (const k of [p1, p2, p3]) {
      store.registerSender(k, (d) => (got[k] += d));
    }

    store.typed(p1, 'x');
    expect(got).toEqual({ [p1]: '', [p2]: '', [p3]: '' }); // no broadcast yet

    store.toggleBroadcast(tab.key);
    store.typed(p1, 'ls');
    expect(got).toEqual({ [p1]: '', [p2]: 'ls', [p3]: 'ls' }); // the source writes itself

    store.toggleExcluded(tab.key, p3);
    store.focusPane(tab.key, p1);
    expect(store.sendToFocused('pwd')).toBe(2);
    expect(got[p3]).toBe('ls');
    expect(store.receivers(p3)).toEqual([p3]); // an excluded pane types only to itself
  });

  it('moves panes between positions and tabs without closing sessions', async () => {
    await store.openTarget('a');
    await store.split('row');
    const [p1, p2] = panesOf(store.active()!.layout);

    store.movePane(p2, p1, 'top');
    let rects = arrange(store.active()!.layout).panes;
    expect(rects.get(p2)).toEqual({ x: 0, y: 0, w: 100, h: 50 });

    store.movePane(p1, p2, 'center');
    expect(panesOf(store.active()!.layout)).toEqual([p1, p2]);

    store.movePaneToNewTab(p2);
    expect(store.tabs()).toHaveLength(2);
    expect(panesOf(store.active()!.layout)).toEqual([p2]);

    store.movePaneToTab(p2, store.tabs()[0].key);
    expect(store.tabs()).toHaveLength(1); // the emptied tab is gone
    rects = arrange(store.tabs()[0].layout).panes;
    expect(rects.get(p2)?.x).toBe(50);
    expect(api.closed).toEqual([]);
  });

  it('swaps panes across tabs', async () => {
    await store.openTarget('a');
    await store.openTarget('b');
    const [t1, t2] = store.tabs();
    store.movePane(t2.focus, t1.focus, 'center');
    expect(panesOf(store.tabs()[0].layout)).toEqual([t2.focus]);
    expect(panesOf(store.tabs()[1].layout)).toEqual([t1.focus]);
  });

  describe('automatic reconnection', () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    const lost = (id: string) => api.stateHandler({ id, state: 'closed', exitCode: -1, lost: true });
    const failed = (id: string, code = 'ssh.dialFailed') =>
      api.stateHandler({ id, state: 'failed', exitCode: 0, lost: false, error: { code, params: {} } as unknown as SessionState['error'] });

    it('reconnects with growing pauses and reattaches to the same tmux session', async () => {
      await store.openHost('h1', 'x');
      const persist = api.openOptions[0].persist;
      expect(persist).toMatch(/^shellvane-[0-9a-f]{8}$/);
      api.stateHandler({ id: 's1', state: 'connected', exitCode: 0, lost: false });

      lost('s1');
      expect(store.focused()?.retry?.attempt).toBe(1);
      await vi.advanceTimersByTimeAsync(1000);
      expect(api.openOptions[1]).toMatchObject({ persist, resume: true });

      failed('s2');
      expect(store.focused()?.retry?.attempt).toBe(2);
      await vi.advanceTimersByTimeAsync(1999);
      expect(api.opened).toHaveLength(2);
      await vi.advanceTimersByTimeAsync(1);
      expect(api.opened).toHaveLength(3);

      api.stateHandler({ id: 's3', state: 'connected', exitCode: 0, lost: false });
      expect(store.focused()?.retry).toBeNull();
      expect(store.focused()?.sessionId).toBe('s3');
    });

    it('stops on failures a retry cannot fix, on a normal exit and when cancelled', async () => {
      await store.openHost('h1', 'x');
      lost('s1');
      await vi.advanceTimersByTimeAsync(1000);
      failed('s2', 'ssh.authFailed');
      expect(store.focused()?.retry).toBeNull();

      await store.reconnect(store.focused()!.key);
      api.stateHandler({ id: 's3', state: 'closed', exitCode: 0, lost: false });
      expect(store.focused()?.retry).toBeNull();

      await store.reconnect(store.focused()!.key);
      lost('s4');
      store.cancelRetry(store.focused()!.key);
      await vi.advanceTimersByTimeAsync(60_000);
      expect(api.opened).toHaveLength(4);
    });

    it('gives up after the last attempt', async () => {
      await store.openHost('h1', 'x');
      lost('s1');
      for (let i = 1; i <= RETRY_MAX_ATTEMPTS; i++) {
        await vi.advanceTimersByTimeAsync(retryDelay(i));
        failed(`s${i + 1}`);
      }
      expect(store.focused()?.retry).toBeNull();
      expect(store.focused()?.state).toBe('failed');
      expect(api.opened).toHaveLength(RETRY_MAX_ATTEMPTS + 1);
    });
  });

  it('saves and restores a workspace with its layout and tmux sessions', async () => {
    await store.openHost('h1', 'x');
    await store.split('row');
    await store.openTarget('me@box');
    store.activeKey.set(store.tabs()[0].key);
    const snap = store.snapshot();
    expect(snap.tabs).toHaveLength(2);
    expect(snap.tabs[0].layout).toMatchObject({ kind: 'split', a: { pane: 0 }, b: { pane: 1 } });

    const persisted = snap.tabs[0].panes.map((p) => p.persist);
    await store.restore(JSON.parse(JSON.stringify(snap)));
    expect(store.tabs()).toHaveLength(4);
    expect(api.openOptions.slice(-2)).toMatchObject(persisted.map((persist) => ({ persist, resume: true })));
    expect(store.active()?.key).toBe(store.tabs()[2].key);
    expect(panesOf(store.tabs()[2].layout)).toHaveLength(2);
  });

  it('opens local terminals and switches their kubectl context live', async () => {
    await store.openLocal('/bin/zsh');
    const pane = store.focused()!;
    expect(pane.local).toBe(true);
    expect(addressOf(pane)).toBe('me@laptop');
    expect(api.localOptions[0].program).toBe('/bin/zsh');

    await store.setKube(pane.key, { context: 'prod', namespace: 'web' });
    expect(api.kubeSet).toEqual([{ id: pane.sessionId, context: 'prod' }]);
    expect(store.focused()?.kube).toMatchObject({ context: 'prod', namespace: 'web' });
    expect(store.snapshot().tabs[0].panes[0].kube).toEqual({ context: 'prod', namespace: 'web' });
  });

  it('reopens a pane without its own kubectl file to set a context', async () => {
    await store.openHost('h1', 'x');
    const key = store.focused()!.key;
    await store.setKube(key, { context: 'dev', namespace: '' });
    expect(api.kubeSet).toEqual([]);
    expect(api.openOptions[1]).toMatchObject({ resume: true, kube: { context: 'dev', namespace: '' } });
  });

  it('maximizes a pane until the layout changes, renames tabs and marks bells', async () => {
    await store.openTarget('a');
    await store.split('row');
    const tab = store.active()!;
    const [p1, p2] = panesOf(tab.layout);
    store.toggleMaximized(p1);
    expect(store.active()?.maximized).toBe(p1);
    store.toggleMaximized(p1);
    expect(store.active()?.maximized).toBeNull();
    store.toggleMaximized(p2);
    await store.split('column');
    expect(store.active()?.maximized).toBeNull(); // a new layout shows every pane

    store.renameTab(tab.key, '  prod web  ');
    expect(store.active()?.title).toBe('prod web');
    expect(store.snapshot().tabs[0].title).toBe('prod web');

    store.bell(p1, false);
    expect(store.pane(p1)?.bell).toBe(false);
    store.bell(p1, true);
    expect(store.pane(p1)?.bell).toBe(true);
    store.activate(tab.key);
    expect(store.pane(p1)?.bell).toBe(false);
  });
});
