import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';

import { Environment, EnvironmentApi } from '../core/backend/environment-api';
import { SessionsStore } from '../sessions/sessions.store';
import { WorkspacesStore } from '../sessions/workspaces.store';
import { TunnelsStore } from '../tunnels/tunnels.store';
import { PersonalValues } from '../core/personal-values';
import { EnvironmentsStore, expand, routeFor } from './environments.store';
import { BrowserPrefs } from '../sessions/browser-prefs';

const env = (over: Partial<Environment> = {}): Environment => ({
  id: 'e1',
  name: 'GKE',
  description: '',
  variables: [],
  login: { check: '', login: '', prepare: '' },
  remote: { hostId: '', terminal: false, script: [], kubeContext: '', kubeNamespace: '' },
  tunnelIds: [],
  localKube: { context: '', namespace: '', tunnelId: '', terminals: 0 },
  workspaceId: '',
  ...over,
});

describe('EnvironmentsStore', () => {
  let checks: boolean[];
  let opened: { kind: string; extra: unknown }[];
  let store: EnvironmentsStore;
  let panes: Map<number, Record<string, unknown>>;
  let kubeSet: number[];
  let kubeChecked: number[];
  let kubeFails: boolean;
  let stopped: string[];
  let api: { list: () => Promise<Environment[]> };
  const tunnels = signal([
    { profile: { id: 't1', kind: 'dynamic' }, status: { state: 'stopped', addr: '' } },
  ] as unknown as ReturnType<TunnelsStore['tunnels']>);

  beforeEach(() => {
    checks = [];
    opened = [];
    let key = 0;
    panes = new Map();
    kubeSet = [];
    kubeChecked = [];
    kubeFails = false;
    stopped = [];
    tunnels.set([{ profile: { id: 't1', kind: 'dynamic' }, status: { state: 'stopped', addr: '' } }] as never);
    api = { list: async () => [] };
    TestBed.configureTestingModule({
      providers: [
        { provide: EnvironmentApi, useValue: { checkLogin: async () => ({ ok: checks.shift() ?? true, detail: '' }), list: () => api.list() } },
        {
          provide: SessionsStore,
          useValue: {
            openLocal: async (_p: unknown, extra: { env?: unknown; kube?: unknown }) => {
              opened.push({ kind: 'local', extra });
              panes.set(++key, { key, state: 'connected', local: true, source: { kind: 'local' }, env: extra.env ?? null, kubeOverride: extra.kube ?? null });
              return key;
            },
            openHost: async (id: string, _t: string, extra: { env?: unknown }) => {
              opened.push({ kind: 'host:' + id, extra });
              panes.set(++key, { key, state: 'connected', local: false, source: { kind: 'host', hostId: id }, env: extra.env ?? null });
              return key;
            },
            pane: (k: number) => panes.get(k),
            panes: () => [...panes.values()],
            movedAway: new Set<number>(),
            setKube: async (k: number) => {
              kubeSet.push(k);
            },
            checkKube: async (k: number) => {
              kubeChecked.push(k);
              if (kubeFails) {
                throw { cause: { code: 'kube.kubectl', params: { detail: 'dial tcp 10.0.0.12:443: connect: connection refused' } } };
              }
            },
            closePane: async (k: number) => panes.delete(k),
          },
        },
        {
          provide: TunnelsStore,
          useValue: {
            tunnels,
            load: async () => undefined,
            start: async (id: string) =>
              tunnels.update((list) => list.map((t) => (t.profile.id === id && t.status.state !== 'running' ? ({ ...t, status: { state: 'running', addr: '127.0.0.1:1080' } } as never) : t))),
            stop: async (id: string) => {
              stopped.push(id);
            },
          },
        },
        { provide: WorkspacesStore, useValue: { load: async () => undefined, list: signal([]), open: async () => true } },
      ],
    });
    store = TestBed.inject(EnvironmentsStore);
  });

  it('expands variables and builds routes', () => {
    expect(expand('get-credentials {{cluster}} --project {{ p }} {{x}}', { cluster: 'c1', p: 'acme' })).toBe('get-credentials c1 --project acme {{x}}');
    expect(routeFor('dynamic', '127.0.0.1:1080')).toEqual({ proxyUrl: 'socks5://127.0.0.1:1080', server: '' });
    expect(routeFor('local', '127.0.0.1:6443')).toEqual({ proxyUrl: '', server: 'https://127.0.0.1:6443' });
  });

  it('logs in when the check fails, then runs every step in order', async () => {
    checks = [false, false, true];
    await store.start(
      env({
        login: { check: 'gcloud auth print-access-token', login: 'gcloud auth login --project {{p}}', prepare: '' },
        remote: { hostId: 'h1', terminal: true, script: ['echo {{p}}'], kubeContext: 'gke_{{p}}', kubeNamespace: '' },
        tunnelIds: ['t1'],
        localKube: { context: 'gke_{{p}}', namespace: 'web', tunnelId: 't1', terminals: 1 },
      }),
      { p: 'acme' },
    );
    const run = store.runOf('e1')!;
    expect(run.steps.map((s) => s.state)).toEqual(['done', 'done', 'done', 'done', 'skipped']);
    expect(opened[0]).toMatchObject({ kind: 'local', extra: { script: ['gcloud auth login --project acme'] } });
    const tag = { id: 'e1', name: 'GKE', values: { p: 'acme' } };
    expect(opened[1]).toEqual({ kind: 'host:h1', extra: { script: ['echo acme'], kube: { context: 'gke_acme', namespace: '' }, env: tag } });
    expect(opened[2].extra).toEqual({
      kube: { context: 'gke_acme', namespace: 'web', route: { proxyUrl: 'socks5://127.0.0.1:1080', server: '' } },
      env: tag,
    });
    expect(run.panes).toHaveLength(3);
    expect(run.tunnels).toEqual(['t1']);

    await store.stop('e1');
    expect(store.runOf('e1')).toBeUndefined();
  }, 15000);

  it('gives restored panes their context again, without reopening them', async () => {
    // A restored local pane of the environment, without its context.
    panes.set(7, { key: 7, state: 'connected', local: true, source: { kind: 'local' }, env: { id: 'e1', name: 'GKE', values: {} }, kubeOverride: null });
    // One that remembers it gets it again too: restored before kubectl
    // could be asked, its file may hold nothing.
    panes.set(8, { key: 8, state: 'connected', local: true, source: { kind: 'local' }, env: { id: 'e1', name: 'GKE', values: {} }, kubeOverride: { context: 'gke_prd', namespace: 'web' } });
    await store.start(env({ localKube: { context: 'gke_prd', namespace: 'web', tunnelId: '', terminals: 1 } }), {}, 'login', { adopt: [7, 8] });
    expect(opened).toEqual([]);
    expect(kubeSet).toEqual([7, 8]);
    expect(kubeChecked).toEqual([7]); // and a terminal must reach the cluster
    expect(store.stateOf('e1')).toBe('ready');
  });

  it('is not ready when a restored terminal does not reach its cluster', async () => {
    panes.set(7, { key: 7, state: 'connected', local: true, source: { kind: 'local' }, env: { id: 'e1', name: 'GKE', values: {} }, kubeOverride: null });
    kubeFails = true;
    await store.start(env({ localKube: { context: 'shop-prd', namespace: '', tunnelId: '', terminals: 1 } }), {}, 'login', { adopt: [7] });
    const step = store.runOf('e1')!.steps.find((x) => x.id === 'localKube')!;
    expect(step.state).toBe('failed');
    expect(step.error).toEqual({ key: 'errors.environments.kubeUnreachable', params: { detail: 'dial tcp 10.0.0.12:443: connect: connection refused' } });
    expect(store.stateOf('e1')).toBe('failed');
  });

  it('fails a login step without a login command', async () => {
    checks = [false];
    await store.start(env({ login: { check: 'false', login: '', prepare: '' } }), {});
    const step = store.runOf('e1')!.steps[0];
    expect(step.state).toBe('failed');
    expect(step.error?.key).toBe('errors.environments.loginFailed');
  });

  it('runs the preparation after the login and stops when it fails', async () => {
    checks = [true, false]; // logged in, then the preparation fails
    await store.start(env({ login: { check: 'gcloud auth print-access-token', login: '', prepare: 'gcloud container clusters get-credentials x' } }), {});
    const step = store.runOf('e1')!.steps[0];
    expect(step.state).toBe('failed');
    expect(step.error?.key).toBe('errors.environments.prepareFailed');
  });

  it('copies the login password before the login and uses the gcloud profile everywhere', async () => {
    checks = [false, true];
    const calls: string[] = [];
    const api = TestBed.inject(EnvironmentApi) as unknown as Record<string, unknown>;
    api['checkLogin'] = async (c: string, _v: unknown, gcloud: string) => {
      calls.push(`check ${c} @${gcloud}`);
      return { ok: checks.shift() ?? true, detail: '' };
    };
    TestBed.inject(BrowserPrefs).set({ browser: 'firefox', seconds: 90 });
    api['copyLoginSecret'] = async (id: string, _v: unknown, seconds: number) => {
      calls.push(`copy ${id} ${seconds}s`);
      return { user: 'alice-admin@example.com', seconds: 45 };
    };
    await store.start(
      env({
        gcloud: 'prod-admin',
        login: { check: 'gcloud auth print-access-token', login: 'gcloud auth login --update-adc', prepare: '', secret: { source: 'cfg-1', address: 'accounts.google.com', user: 'alice-admin@example.com', safe: '' }, privateBrowser: true },
        localKube: { context: 'gke', namespace: '', tunnelId: '', terminals: 1 },
      }),
      {},
    );
    expect(calls.slice(0, 2)).toEqual(['check gcloud auth print-access-token @prod-admin', 'copy e1 90s']);
    expect(opened.map((o) => (o.extra as { gcloud?: string }).gcloud)).toEqual(['prod-admin', 'prod-admin']);
    expect(opened.map((o) => (o.extra as { browser?: unknown }).browser)).toEqual([{ id: 'firefox', command: '', private: true }, undefined]);
    expect((opened[0].extra as { openLinks?: boolean }).openLinks).toBe(false);
    TestBed.inject(BrowserPrefs).set({ browser: '', seconds: 45 });
    expect(store.runOf('e1')!.steps.every((st) => st.state === 'done' || st.state === 'skipped')).toBe(true);
  });

  it('logs in anyway when the password cannot be read', async () => {
    checks = Array(100).fill(false); // still waiting for the login
    const api = TestBed.inject(EnvironmentApi) as unknown as Record<string, unknown>;
    api['copyLoginSecret'] = async () => {
      throw { cause: { code: 'environments.loginSecretEmpty', params: { user: 'alice' } } };
    };
    const started = store.start(env({ login: { check: 'c', login: 'gcloud auth login', prepare: '', secret: { source: 'cfg-1', address: 'a', user: 'alice', safe: '' } } }), {});
    await new Promise((r) => setTimeout(r, 20));
    const a = store.runOf('e1')!.steps[0].activity;
    expect(a?.key).toBe('environments.activity.loggingIn');
    expect(a?.detail?.key).toBe('errors.environments.loginSecretEmpty');
    store.continueWithoutLogin('e1');
    await started;
    expect(opened.length).toBe(1);
  }, 15000);

  it('goes on without the password when the user skips a sign-in in the browser', async () => {
    checks = Array(100).fill(false);
    const api = TestBed.inject(EnvironmentApi) as unknown as Record<string, unknown>;
    let cancelled = false;
    api['copyLoginSecret'] = () => {
      let reject: (e: unknown) => void = () => undefined;
      const p = new Promise((_, rej) => (reject = rej)) as Promise<unknown> & { cancel?: () => void };
      p.cancel = () => {
        cancelled = true;
        reject(new Error('cancelled'));
      };
      return p;
    };
    const started = store.start(env({ login: { check: 'c', login: 'gcloud auth login', prepare: '', secret: { source: 'cfg-1', address: '', user: 'alice', safe: '' } } }), {});
    await new Promise((r) => setTimeout(r, 20));
    expect(store.runOf('e1')!.steps[0].activity?.key).toBe('environments.activity.fetchingSecret');
    store.skipSecret('e1');
    await new Promise((r) => setTimeout(r, 20));
    const a = store.runOf('e1')!.steps[0].activity;
    expect(cancelled).toBe(true);
    expect(a?.key).toBe('environments.activity.loggingIn');
    expect(a?.detail?.key).toBe('environments.secretSkipped');
    store.continueWithoutLogin('e1');
    await started;
  }, 15000);

  it('keeps personal values on this computer and adds them to every start', async () => {
    const calls: string[] = [];
    const api = TestBed.inject(EnvironmentApi) as unknown as Record<string, unknown>;
    api['checkLogin'] = async (c: string) => {
      calls.push(c);
      return { ok: true, detail: '' };
    };
    const e = env({
      variables: [
        { name: 'googleEmail', default: '', choices: [], personal: true },
        { name: 'region', default: 'europe-west8', choices: [] },
      ],
      login: { check: 'gcloud auth print-access-token --account {{googleEmail}}', login: '', prepare: '' },
    });
    expect(store.needsValues(e)).toBe(true);
    store.setPersonalValues(e, { googleEmail: 'alice@example.com', region: 'europe-west1' });
    // Only personal values are kept; a shared variable still asks.
    expect(store.personalValues(e)).toEqual({ googleEmail: 'alice@example.com' });
    expect(store.needsValues(e)).toBe(true);
    expect(store.needsValues({ ...e, variables: [e.variables![0]] })).toBe(false);
    await store.start(e, { region: 'europe-west8' });
    expect(calls[0]).toBe('gcloud auth print-access-token --account {{googleEmail}}');
    expect(store.runOf('e1')!.values).toEqual({ googleEmail: 'alice@example.com', region: 'europe-west8' });
    // "Only here": this environment keeps its own value; the global one stays for the others.
    store.setPersonalValues(e, { googleEmail: 'alice@client.example' }, { googleEmail: 'here' });
    expect(store.personalValues(e)).toEqual({ googleEmail: 'alice@client.example' });
    expect(store.personalSources(e)).toEqual({ googleEmail: 'here' });
    expect(store.personalValues({ ...e, id: 'e2' })).toEqual({ googleEmail: 'alice@example.com' });
    // Deleting the environment forgets its own value only.
    await store.delete('e1').catch(() => undefined);
    expect(store.personalValues(e)).toEqual({ googleEmail: 'alice@example.com' });
    localStorage.clear();
  });

  it('shows why the login check fails and can go on without it', async () => {
    checks = Array(100).fill(false); // the check never passes
    const api = TestBed.inject(EnvironmentApi) as unknown as { checkLogin: (c: string) => Promise<{ ok: boolean; detail: string }> };
    api.checkLogin = async () => ({ ok: checks.shift() ?? false, detail: 'gcloud: command not found' });
    const started = store.start(env({ login: { check: 'gcloud auth print-access-token', login: 'gcloud auth login', prepare: '' } }), {});
    await new Promise((r) => setTimeout(r, 50));
    const waiting = store.runOf('e1')!.steps[0];
    expect(waiting.state).toBe('running');
    expect(waiting.waiting).toBe('gcloud: command not found');
    store.continueWithoutLogin('e1');
    await started;
    expect(store.runOf('e1')!.steps[0].state).toBe('done');
  }, 15000);

  it('starts the environments of restored panes again without opening them twice', async () => {
    const e = env({
      login: { check: 'gcloud auth print-access-token', login: '', prepare: '' },
      remote: { hostId: 'h1', terminal: true, script: [], kubeContext: '', kubeNamespace: '' },
      tunnelIds: ['t1'],
      localKube: { context: 'gke', namespace: '', tunnelId: 't1', terminals: 2 },
      workspaceId: 'w1',
    });
    api.list = async () => [e];
    const tag = { id: 'e1', name: 'GKE', values: { p: 'acme' } };
    // As restored: the old tunnel port in the kubectl route.
    panes.set(10, { key: 10, state: 'connected', local: true, source: { kind: 'local' }, env: tag, kubeOverride: { context: 'gke', namespace: '', route: { proxyUrl: 'socks5://127.0.0.1:9999', server: '' } } });
    panes.set(11, { key: 11, state: 'connected', local: false, source: { kind: 'host', hostId: 'h1' }, env: tag });
    panes.set(12, { key: 12, state: 'connected', local: true, source: { kind: 'local' }, env: null });
    expect(store.stateOf('e1')).toBe('stopped');

    await store.resumeRestored();
    await new Promise((r) => setTimeout(r, 400));
    const run = store.runOf('e1')!;
    expect(run.steps.map((s) => s.state)).toEqual(['done', 'done', 'done', 'done', 'skipped']);
    expect(opened).toEqual([]); // nothing opened again
    expect(run.panes.sort()).toEqual([10, 11]);
    expect(run.values).toEqual({ p: 'acme' });
    expect(kubeSet).toEqual([10]); // pointed at the tunnel's new port
    expect(store.stateOf('e1')).toBe('ready');

    // One more terminal, with the same context and route.
    const key = await store.openTerminal('e1');
    expect(opened[0].extra).toEqual({ kube: { context: 'gke', namespace: '', route: { proxyUrl: 'socks5://127.0.0.1:1080', server: '' } }, env: tag });
    expect(run.panes).toContain(key);

    // The login expires later: the state shows it.
    checks = [false];
    await store.recheckLogins();
    expect(store.stateOf('e1')).toBe('expired');
  }, 15000);

  it('opens one more kubectl terminal of an environment that is up, or starts it', async () => {
    const e = env({ gcloud: 'prd', tunnelIds: ['t1'], localKube: { context: 'shop-prd', namespace: 'argocd', tunnelId: 't1', terminals: 1 } });
    api.list = async () => [e];
    await store.load();
    expect(store.isUp('e1')).toBe(false);
    await store.openKubeTerminal(e); // not up: started, with its terminal
    expect(store.isUp('e1')).toBe(true);
    expect(opened.filter((o) => o.kind === 'local')).toHaveLength(1);
    await store.openKubeTerminal(e);
    const local = opened.filter((o) => o.kind === 'local');
    expect(local).toHaveLength(2);
    expect(local[1].extra).toMatchObject({ kube: { context: 'shop-prd', namespace: 'argocd' }, gcloud: 'prd', env: { id: 'e1' } });
    expect(store.runOf('e1')!.panes).toHaveLength(2);
  }, 15000);

  it('lists the personal values still missing, by name, with their environments', async () => {
    localStorage.clear();
    const v = (name: string) => ({ name, default: '', choices: [], personal: true });
    api.list = async () => [
      env({ id: 'a', name: 'GKE shop prd', variables: [v('googleEmail'), v('cyberarkUser')] }),
      env({ id: 'b', name: 'GKE pay prd', variables: [v('googleEmail'), { name: 'region', default: 'x', choices: [] }] }),
    ];
    await store.load();
    expect(store.missingPersonal()).toEqual([
      { name: 'googleEmail', envs: ['GKE shop prd', 'GKE pay prd'] },
      { name: 'cyberarkUser', envs: ['GKE shop prd'] },
    ]);
    TestBed.inject(PersonalValues).setGlobal('googleEmail', 'me@corp.example');
    expect(store.missingPersonal()).toEqual([{ name: 'cyberarkUser', envs: ['GKE shop prd'] }]);
    localStorage.clear();
  });

  it('stops with its last terminal, only the tunnels it started, and can undo', async () => {
    const e = env({ tunnelIds: ['t1', 't2'], localKube: { context: 'gke', namespace: '', tunnelId: 't1', terminals: 1 } });
    api.list = async () => [e];
    await store.load();
    // t2 was already running before the environment.
    tunnels.set([
      { profile: { id: 't1', kind: 'dynamic' }, status: { state: 'stopped', addr: '' } },
      { profile: { id: 't2', kind: 'local' }, status: { state: 'running', addr: '127.0.0.1:5432' } },
    ] as never);
    await store.start(e, {});
    const run = store.runOf('e1')!;
    expect(run.tunnels).toEqual(['t1']);
    await store.stopUnused();
    expect(store.runOf('e1')).toBe(run); // its terminal is open

    panes.clear(); // the user closed the last terminal
    await store.stopUnused();
    expect(store.runOf('e1')).toBeUndefined();
    expect(stopped).toEqual(['t1']);
    expect(store.notice()?.name).toBe('GKE');

    await store.undoStop();
    expect(store.notice()).toBeNull();
    expect(store.runOf('e1')!.steps.map((x) => x.state)).toEqual(['skipped', 'skipped', 'done', 'skipped', 'skipped']);
    expect(opened.filter((o) => o.kind === 'local')).toHaveLength(1); // no new terminal
    await store.stopUnused();
    expect(store.runOf('e1')).toBeDefined(); // no terminal opened since: it stays
  }, 15000);

  it('keeps environments without terminals, or with the preference off', async () => {
    const tunnelsOnly = env({ tunnelIds: ['t1'] });
    api.list = async () => [tunnelsOnly];
    await store.load();
    await store.start(tunnelsOnly, {});
    store.runOf('e1')!.hadPanes = true;
    await store.stopUnused();
    expect(store.runOf('e1')).toBeDefined();

    const e = env({ localKube: { context: '', namespace: '', tunnelId: '', terminals: 1 } });
    api.list = async () => [e];
    await store.load();
    await store.start(e, {});
    store.setAutoStop(false);
    await store.stopUnused();
    panes.clear();
    await store.stopUnused();
    expect(store.runOf('e1')).toBeDefined();
    store.setAutoStop(true);
  }, 15000);
});
