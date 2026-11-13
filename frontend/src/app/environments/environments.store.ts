import { Injectable, effect, inject, signal, untracked } from '@angular/core';

import { Environment, EnvironmentApi } from '../core/backend/environment-api';
import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { readPreference, writePreference } from '../core/preferences';
import { KubeSelection } from '../core/backend/session-api';
import { PaneEnv, SessionsStore } from '../sessions/sessions.store';
import { BrowserPrefs } from '../sessions/browser-prefs';
import { WorkspacesStore } from '../sessions/workspaces.store';
import { TunnelsStore } from '../tunnels/tunnels.store';
import { Sharing } from '../extensions/sharing';
import { UsageStore } from '../core/usage/usage.store';
import { PersonalValues, Remember } from '../core/personal-values';

export type StepId = 'login' | 'remote' | 'tunnels' | 'localKube' | 'workspace';
export type StepState = 'pending' | 'running' | 'done' | 'failed' | 'skipped';

export interface StepStatus {
  id: StepId;
  state: StepState;
  error: TranslatableError | null;
  /** While a step waits: why its check does not pass yet. */
  waiting?: string;
  /** What a running step is doing now (translation key and values). */
  activity?: { key: string; params: Record<string, unknown>; detail?: TranslatableError };
  /** When the step started (ms), to show how long it has been running. */
  since?: number;
}

/** A started environment: its steps and what it opened. */
export interface Run {
  envId: string;
  steps: StepStatus[];
  /** Panes the run opened or took over, closed by Stop. */
  panes: number[];
  /** Tunnels the run started (not those already running), stopped by Stop. */
  tunnels: string[];
  /** One of the environment's terminals has been open during the run. */
  hadPanes?: boolean;
  cancelled: boolean;
  /** The user chose to go on without the login check passing. */
  skipLogin?: boolean;
  /** The variables the run was started with. */
  values: Record<string, string>;
  /** The run took over panes already open (restored, or a new login): it does not open them again. */
  reuse: boolean;
  /** The last periodic login check failed: the login has expired. */
  loginExpired?: string;
  /** Stops fetching the login secret (the user skips the password). */
  skipSecret?: () => void;
}

/** How to start a run. */
export interface StartOptions {
  /** Panes already open to take over instead of opening them. */
  adopt?: number[];
  /** Only the login and the tunnels: no terminals, no workspace. */
  noPanes?: boolean;
}

/** An environment stopped by itself, shown with an Undo. */
export interface StopNotice {
  envId: string;
  name: string;
  values: Record<string, string>;
}

/** What a pane header shows of its environment. */
export type EnvState = 'stopped' | 'starting' | 'ready' | 'failed' | 'expired';

const STEPS: StepId[] = ['login', 'remote', 'tunnels', 'localKube', 'workspace'];
/** How long each wait may take before the step fails. */
const LOGIN_WAIT_MS = 10 * 60_000;
// Connections may ask for a password (or a 2FA code) first.
const CONNECT_WAIT_MS = 5 * 60_000;
const POLL_MS = 3000;
/** How long the notice of an automatic stop stays. */
const NOTICE_MS = 10_000;
/** How often a started environment's login is checked again. */
const RECHECK_MS = 5 * 60_000;

/** Whether an environment opens terminals (not only tunnels). */
export function hasTerminals(env: Environment): boolean {
  return (!!env.remote.hostId && env.remote.terminal) || env.localKube.terminals > 0;
}

/** A command shortened for a status line. */
function short(command: string): string {
  const c = command.replace(/\s+/g, ' ').trim();
  return c.length > 70 ? c.slice(0, 69) + '…' : c;
}

/** Replaces {{name}} with the variables' values. */
export function expand(text: string, values: Record<string, string>): string {
  return text.replace(/\{\{\s*([\w.-]+)\s*\}\}/g, (m, name: string) => values[name] ?? m);
}

/** The kubectl route through a tunnel: SOCKS proxy or forwarded API. */
export function routeFor(kind: string, addr: string): KubeSelection['route'] {
  if (!addr) {
    return undefined;
  }
  return kind === 'dynamic' ? { proxyUrl: `socks5://${addr}`, server: '' } : { proxyUrl: '', server: `https://${addr}` };
}

@Injectable({ providedIn: 'root' })
export class EnvironmentsStore {
  private readonly usage = inject(UsageStore);
  private readonly api = inject(EnvironmentApi);
  private readonly browserPrefs = inject(BrowserPrefs);
  private readonly sessions = inject(SessionsStore);
  private readonly tunnels = inject(TunnelsStore);
  private readonly workspaces = inject(WorkspacesStore);
  private readonly sharing = inject(Sharing);
  private readonly personal = inject(PersonalValues);

  readonly list = signal<Environment[]>([]);
  readonly runs = signal<Run[]>([]);
  /** Stop an environment when its last terminal closes. */
  readonly autoStop = signal(readPreference('environments.autoStop') !== 'false');
  /** The environment just stopped by closing its last terminal. */
  readonly notice = signal<StopNotice | null>(null);
  private recheck?: ReturnType<typeof setInterval>;
  private noticeTimer?: ReturnType<typeof setTimeout>;

  constructor() {
    effect(() => {
      this.sessions.panes();
      this.runs();
      untracked(() => void this.stopUnused());
    });
  }

  setAutoStop(on: boolean): void {
    this.autoStop.set(on);
    writePreference('environments.autoStop', String(on));
  }

  async load(): Promise<void> {
    this.list.set(await this.api.list());
  }

  async save(e: Environment): Promise<void> {
    await this.api.save(e);
    await this.load();
  }

  /**
   * This person's values of the environment's personal variables (a
   * sign-in email, a vault user): kept on this computer only, never in the
   * environment, so they are not exported or shared. The environment's own
   * value wins over the one remembered for every item.
   */
  personalValues(env: Environment): Record<string, string> {
    const names = (env.variables ?? []).filter((v) => v.personal).map((v) => v.name);
    return this.personal.resolve(names, scopeOf(env.id));
  }

  /** Where each personal value comes from: 'here', 'global' or 'none'. */
  personalSources(env: Environment): Record<string, Remember> {
    const out: Record<string, Remember> = {};
    for (const v of env.variables ?? []) {
      if (v.personal) {
        out[v.name] = this.personal.source(v.name, scopeOf(env.id));
      }
    }
    return out;
  }

  /** Remembers the personal ones among values, everywhere unless where says "here". */
  setPersonalValues(env: Environment, values: Record<string, string>, where: Record<string, Remember> = {}): void {
    for (const v of env.variables ?? []) {
      if (v.personal && values[v.name] !== undefined) {
        this.personal.remember(v.name, values[v.name], where[v.name] === 'here' ? 'here' : 'global', scopeOf(env.id));
      }
    }
  }

  /** Whether starting needs to ask something: a shared variable, or a personal one without a value yet. */
  needsValues(env: Environment): boolean {
    const mine = this.personalValues(env);
    return (env.variables ?? []).some((v) => !v.personal || !mine[v.name]);
  }

  async delete(id: string): Promise<void> {
    await this.stop(id);
    this.personal.clearScope(scopeOf(id));
    await this.api.delete(id);
    await this.load();
  }

  export(id: string): Promise<string> {
    return this.api.export(id);
  }

  /** Imports one environment, or several separated by "---" lines. */
  async import(text: string): Promise<void> {
    const docs = text.split(/^---[ \t]*$/m).filter((d) => d.split('\n').some((l) => l.trim() && !l.trim().startsWith('#')));
    for (const doc of docs) {
      await this.api.import(doc);
    }
    await this.load();
  }

  runOf(envId: string): Run | undefined {
    return this.runs().find((r) => r.envId === envId);
  }

  /**
   * Runs the steps in order; a failed step stops the run (what it opened
   * stays open, so the user can see why). {{variables}} are expanded.
   */
  async start(env: Environment, values: Record<string, string>, from: StepId = 'login', opts: StartOptions = {}): Promise<void> {
    this.usage.count('environment.start');
    values = { ...this.personalValues(env), ...values };
    const adopt = opts.adopt ?? [];
    const previous = this.runOf(env.id);
    const panes = [...new Set([...(previous?.panes ?? []), ...adopt])].filter((k) => !!this.sessions.pane(k));
    const run: Run = {
      envId: env.id,
      steps: STEPS.map((id) => ({ id, state: 'pending', error: null })),
      panes,
      tunnels: previous?.tunnels ?? [],
      cancelled: false,
      values,
      reuse: adopt.length > 0 || !!opts.noPanes,
    };
    this.watchLogins();
    this.runs.update((list) => [...list.filter((r) => r.envId !== env.id), run]);
    if (this.sharing.needsReview('environment', env.id)) {
      // Shared by a team, its commands changed: nothing runs before approval.
      this.set(run, 'login', 'failed', toTranslatableError({ cause: { code: 'environments.reviewNeeded', params: {} } }));
      return;
    }
    const startAt = STEPS.indexOf(from);
    for (const [i, id] of STEPS.entries()) {
      if (i < startAt) {
        this.set(run, id, 'skipped');
        continue;
      }
      if (this.current(env.id) !== run || run.cancelled) {
        return;
      }
      this.set(run, id, 'running');
      try {
        const ran = await this.step(id, env, values, run, opts);
        this.set(run, id, ran ? 'done' : 'skipped');
      } catch (err) {
        this.set(run, id, 'failed', toTranslatableError(err));
        return;
      }
    }
  }

  /**
   * Closes the panes and stops the tunnels the environment started — not
   * those already running before it, started at launch, or used by another
   * started environment.
   */
  async stop(envId: string): Promise<void> {
    const run = this.runOf(envId);
    if (!run) {
      return;
    }
    run.cancelled = true;
    this.runs.update((list) => list.filter((r) => r !== run));
    const others = new Set(this.runs().flatMap((r) => this.list().find((e) => e.id === r.envId)?.tunnelIds ?? []));
    const autostart = new Set(this.tunnels.tunnels().filter((t) => t.profile.autostart).map((t) => t.profile.id));
    const ours = run.tunnels.filter((id) => !others.has(id) && !autostart.has(id));
    await Promise.all(ours.map((id) => this.tunnels.stop(id).catch(() => undefined)));
    for (const key of run.panes) {
      await this.sessions.closePane(key).catch(() => undefined);
    }
  }

  /**
   * After a restore: panes of an environment come back as plain shells,
   * without its login or tunnels. Each such environment is started again
   * over its panes (login check, tunnels), without opening them twice.
   */
  async resumeRestored(): Promise<void> {
    const byEnv = new Map<string, { env: PaneEnv; keys: number[] }>();
    for (const p of this.sessions.panes()) {
      if (p.env && !this.runOf(p.env.id)) {
        const entry = byEnv.get(p.env.id) ?? { env: p.env, keys: [] };
        entry.keys.push(p.key);
        byEnv.set(p.env.id, entry);
      }
    }
    if (byEnv.size === 0) {
      return;
    }
    await this.load();
    for (const { env, keys } of byEnv.values()) {
      const e = this.list().find((x) => x.id === env.id);
      if (e) {
        void this.start(e, env.values, 'login', { adopt: keys });
      }
    }
  }

  /** An environment whose login just expired, offered a new login. */
  readonly expiredNotice = signal<{ envId: string; name: string } | null>(null);

  /** Starts an environment again over its open panes (login expired, a pane's header). */
  async resume(envId: string, values?: Record<string, string>): Promise<void> {
    if (this.expiredNotice()?.envId === envId) {
      this.expiredNotice.set(null);
    }
    if (this.list().length === 0) {
      await this.load();
    }
    const env = this.list().find((x) => x.id === envId);
    if (!env) {
      return;
    }
    const keys = this.sessions
      .panes()
      .filter((p) => p.env?.id === envId)
      .map((p) => p.key);
    const run = this.runOf(envId);
    const vals = values ?? run?.values ?? this.sessions.panes().find((p) => p.env?.id === envId)?.env?.values ?? {};
    await this.start(env, vals, 'login', { adopt: keys });
  }

  /**
   * Stops a started environment whose last terminal was closed (when the
   * preference is on). Environments without terminals, still starting, or
   * whose terminals moved to another window are left alone.
   */
  async stopUnused(): Promise<void> {
    if (!this.autoStop()) {
      return;
    }
    for (const run of this.runs()) {
      const open = this.sessions.panes().filter((p) => p.env?.id === run.envId);
      if (open.length > 0) {
        run.hadPanes = true;
        continue;
      }
      const state = this.stateOf(run.envId);
      const env = this.list().find((e) => e.id === run.envId);
      if (!run.hadPanes || !env || !hasTerminals(env) || (state !== 'ready' && state !== 'expired')) {
        continue;
      }
      if (run.panes.some((k) => this.sessions.movedAway.has(k))) {
        run.hadPanes = false; // its terminals live in another window now
        continue;
      }
      await this.stop(run.envId);
      this.notice.set({ envId: env.id, name: env.name, values: run.values });
      clearTimeout(this.noticeTimer);
      this.noticeTimer = setTimeout(() => this.notice.set(null), NOTICE_MS);
    }
  }

  /** Undoes an automatic stop: login and tunnels again, no terminals. */
  async undoStop(): Promise<void> {
    const n = this.notice();
    this.notice.set(null);
    const env = n && this.list().find((e) => e.id === n.envId);
    if (n && env) {
      await this.start(env, n.values, 'login', { noPanes: true });
    }
  }

  /**
   * Opens one more terminal of a started environment: a local shell with
   * its kubectl context (through its tunnel), or else its remote host.
   */
  async openTerminal(envId: string): Promise<number | null> {
    const run = this.runOf(envId);
    const env = this.list().find((x) => x.id === envId);
    if (!run || !env) {
      return null;
    }
    const tag: PaneEnv = { id: env.id, name: env.name, values: run.values };
    const k = env.localKube;
    let key: number;
    if (k.terminals || k.context || !env.remote.hostId) {
      key = await this.sessions.openLocal(undefined, { kube: this.localKube(env, run.values), env: tag, gcloud: env.gcloud });
    } else {
      const r = env.remote;
      const kube = r.kubeContext ? { context: expand(r.kubeContext, run.values), namespace: expand(r.kubeNamespace, run.values) } : null;
      key = await this.sessions.openHost(r.hostId, env.name, { script: (r.script ?? []).map((l) => expand(l, run.values)), kube, env: tag });
    }
    run.panes.push(key);
    this.runs.update((list) => [...list]);
    return key;
  }

  /** The state a pane header shows for its environment. */
  stateOf(envId: string): EnvState {
    const run = this.runOf(envId);
    if (!run) {
      return 'stopped';
    }
    if (run.steps.some((s) => s.state === 'failed')) {
      return 'failed';
    }
    if (run.steps.some((s) => s.state === 'running' || s.state === 'pending')) {
      return 'starting';
    }
    return run.loginExpired ? 'expired' : 'ready';
  }

  /** Checks again, every few minutes, the login of the started environments. */
  private watchLogins(): void {
    if (this.recheck) {
      return;
    }
    this.recheck = setInterval(() => void this.recheckLogins(), RECHECK_MS);
  }

  async recheckLogins(): Promise<void> {
    for (const run of this.runs()) {
      const env = this.list().find((x) => x.id === run.envId);
      if (!env?.login.check || this.stateOf(run.envId) === 'starting') {
        continue;
      }
      const res = await this.api.checkLogin(env.login.check, run.values, env.gcloud).catch(() => ({ ok: true, detail: '' }));
      if (this.runOf(run.envId) !== run) {
        continue;
      }
      const expired = res.ok ? undefined : res.detail || '?';
      if (expired !== run.loginExpired) {
        const justExpired = !!expired && !run.loginExpired;
        run.loginExpired = expired;
        this.runs.update((list) => [...list]);
        if (justExpired) {
          this.expiredNotice.set({ envId: env.id, name: env.name });
          systemNotification(env.name, expired);
        } else if (!expired && this.expiredNotice()?.envId === env.id) {
          this.expiredNotice.set(null);
        }
      }
    }
  }

  /** The kubectl selection of the environment's local terminals. */
  private localKube(env: Environment, values: Record<string, string>): KubeSelection | null {
    const k = env.localKube;
    if (!k.context) {
      return null;
    }
    const tunnel = this.tunnels.tunnels().find((t) => t.profile.id === k.tunnelId);
    return {
      context: expand(k.context, values),
      namespace: expand(k.namespace, values),
      route: tunnel ? routeFor(tunnel.profile.kind, tunnel.status.addr) : undefined,
    };
  }

  /** Goes on without waiting for the login check (it may never pass). */
  /**
   * Personal values not known yet, by name, with the environments that
   * use them: what a new member (or a newly shared environment) still needs.
   */
  missingPersonal(): { name: string; envs: string[] }[] {
    this.personal.global(); // follows changes
    this.personal.version();
    const out = new Map<string, string[]>();
    for (const env of this.list()) {
      const known = this.personalValues(env);
      for (const v of env.variables ?? []) {
        if (v.personal && !known[v.name]) {
          out.set(v.name, [...(out.get(v.name) ?? []), env.name]);
        }
      }
    }
    return [...out.entries()].map(([name, envs]) => ({ name, envs }));
  }

  /**
   * Checks that a terminal of the environment reaches its cluster (through
   * the tunnel, with the login): the step fails with kubectl's reason
   * instead of looking ready when it does not.
   */
  private async verifyKube(run: Run, paneKey: number): Promise<void> {
    this.act(run, 'localKube', 'verifying');
    try {
      await this.sessions.checkKube(paneKey);
    } catch (err) {
      const e = toTranslatableError(err);
      throw { cause: { code: 'environments.kubeUnreachable', params: { detail: (e.params as { detail?: string })?.detail ?? e.key } } };
    }
  }

  /** Whether the environment is up and ready for one more terminal. */
  isUp(envId: string): boolean {
    return this.stateOf(envId) === 'ready';
  }

  /**
   * One more terminal on the environment's cluster (openTerminal), or the
   * environment started when it is not up, which opens its terminals.
   */
  async openKubeTerminal(env: Environment): Promise<void> {
    if (this.isUp(env.id) && this.list().some((x) => x.id === env.id)) {
      await this.openTerminal(env.id);
      return;
    }
    await this.start(env, {});
  }

  /** Goes on without the login secret's password (typed or copied by hand). */
  skipSecret(envId: string): void {
    this.runOf(envId)?.skipSecret?.();
  }

  continueWithoutLogin(envId: string): void {
    const run = this.runOf(envId);
    if (run) {
      run.skipLogin = true;
    }
  }

  /**
   * Puts the login secret's password on the clipboard (from CyberArk or
   * another credential plugin, with its MFA when needed) for the sign-in
   * page the login opens. A failure is shown and the login goes on: the
   * password can still be typed.
   */
  private async copySecret(env: Environment, values: Record<string, string>, run: Run): Promise<void> {
    let note: TranslatableError | undefined;
    if (env.login.secret?.source) {
      this.act(run, 'login', 'fetchingSecret');
      let skipped = false;
      const copying = this.api.copyLoginSecret(env.id, values, this.browserPrefs.settings().seconds);
      run.skipSecret = () => {
        skipped = true;
        copying.cancel?.();
      };
      note = await copying.then(
        (c) => ({ key: 'environments.secretCopied', params: { ...c } }),
        (err) => (skipped ? { key: 'environments.secretSkipped', params: {} } : toTranslatableError(err)),
      );
      run.skipSecret = undefined;
    }
    this.act(run, 'login', 'loggingIn', {}, note);
  }

  /** Tells what a running step is doing. */
  private act(run: Run, id: StepId, key: string, params: Record<string, unknown> = {}, detail?: TranslatableError): void {
    run.steps = run.steps.map((s) => (s.id === id ? { ...s, activity: { key: 'environments.activity.' + key, params, detail } } : s));
    this.runs.update((list) => [...list]);
  }

  private wait(run: Run, id: StepId, why: string | undefined): void {
    run.steps = run.steps.map((s) => (s.id === id ? { ...s, waiting: why } : s));
    this.runs.update((list) => [...list]);
  }

  private current(envId: string): Run | undefined {
    return this.runOf(envId);
  }

  private set(run: Run, id: StepId, state: StepState, error: TranslatableError | null = null): void {
    run.steps = run.steps.map((s) =>
      s.id === id ? { ...s, state, error, since: state === 'running' ? Date.now() : s.since, activity: state === 'running' ? s.activity : undefined } : s,
    );
    this.runs.update((list) => [...list]); // the run keeps its identity
  }

  /** Runs one step; false when there was nothing to do. */
  private async step(id: StepId, env: Environment, values: Record<string, string>, run: Run, opts: StartOptions): Promise<boolean> {
    switch (id) {
      case 'login':
        return this.login(env, values, run);
      case 'remote': {
        const r = env.remote;
        if (!r.hostId || !r.terminal || opts.noPanes) {
          return false;
        }
        const open = run.reuse ? run.panes.find((k) => this.sessions.pane(k)?.source.kind === 'host' && (this.sessions.pane(k)?.source as { hostId?: string }).hostId === r.hostId) : undefined;
        if (open !== undefined) {
          this.act(run, 'remote', 'reusing');
          await this.waitConnected(open, run);
          return true;
        }
        const kube = r.kubeContext ? { context: expand(r.kubeContext, values), namespace: expand(r.kubeNamespace, values) } : null;
        const key = await this.sessions.openHost(r.hostId, env.name, { script: (r.script ?? []).map((l) => expand(l, values)), kube, env: this.tag(env, values) });
        run.panes.push(key);
        await this.waitConnected(key, run);
        return true;
      }
      case 'tunnels': {
        const tunnelIds = env.tunnelIds ?? [];
        if (tunnelIds.length === 0) {
          return false;
        }
        await this.tunnels.load();
        const names = tunnelIds.map((t) => this.tunnels.tunnels().find((x) => x.profile.id === t)?.profile.name || t).join(', ');
        this.act(run, 'tunnels', 'connecting', { names });
        for (const t of tunnelIds) {
          const state = this.tunnels.tunnels().find((x) => x.profile.id === t)?.status.state;
          const already = state === 'running' || state === 'starting';
          await this.tunnels.start(t);
          if (!already && !run.tunnels.includes(t)) {
            run.tunnels.push(t);
          }
        }
        await this.until(run, CONNECT_WAIT_MS, () => {
          const states = tunnelIds.map((t) => this.tunnels.tunnels().find((x) => x.profile.id === t)?.status);
          const failed = states.find((s) => s?.state === 'failed');
          if (failed) {
            throw { cause: failed.error };
          }
          return states.every((s) => s?.state === 'running');
        });
        return true;
      }
      case 'localKube': {
        const k = env.localKube;
        if (!k.terminals || opts.noPanes) {
          return false;
        }
        const kube = this.localKube(env, values);
        const open = run.reuse ? run.panes.filter((key) => this.sessions.pane(key)?.local && this.sessions.pane(key)?.env?.id === env.id) : [];
        if (open.length > 0) {
          // Restored panes come back without their context, and the tunnel
          // may listen on another port now: give each shell the
          // environment's context, namespace and route again, always (a
          // pane restored before kubectl could be asked has an empty
          // file even when it remembers the selection).
          this.act(run, 'localKube', 'reusing');
          for (const key of open) {
            if (kube && this.sessions.pane(key)) {
              await this.sessions.setKube(key, kube);
            }
          }
          await this.verifyKube(run, open[0]);
          return true;
        }
        this.act(run, 'localKube', 'opening', { count: k.terminals });
        for (let i = 0; i < k.terminals; i++) {
          run.panes.push(await this.sessions.openLocal(undefined, { kube, env: this.tag(env, values), gcloud: env.gcloud }));
        }
        const first = run.panes.at(-k.terminals);
        if (kube && first !== undefined) {
          await this.verifyKube(run, first);
        }
        return true;
      }
      case 'workspace': {
        if (!env.workspaceId || run.reuse) {
          return false;
        }
        await this.workspaces.load();
        const w = this.workspaces.list().find((x) => x.id === env.workspaceId);
        if (w) {
          await this.workspaces.open(w);
        }
        return !!w;
      }
    }
  }

  /**
   * Checks the login; when it fails, logs in in a local pane and waits.
   * Then runs the preparation command (e.g. get-credentials).
   */
  private async login(env: Environment, values: Record<string, string>, run: Run): Promise<boolean> {
    const { check, login, prepare } = env.login;
    if (!check && !login && !prepare) {
      return false;
    }
    if (check) {
      this.act(run, 'login', 'checking', { command: short(expand(check, values)) });
    }
    if (check && (await this.api.checkLogin(check, values, env.gcloud)).ok) {
      // Logged in already.
    } else if (login) {
      await this.copySecret(env, values, run);
      run.panes.push(await this.sessions.openLocal(undefined, {
          script: [expand(login, values)],
          gcloud: env.gcloud,
          browser: this.browserPrefs.choice(env.login.browser ?? '', !!env.login.privateBrowser),
          openLinks: !!env.login.openLinks,
        }));
      if (check) {
        await this.until(
          run,
          LOGIN_WAIT_MS,
          async () => {
            if (run.skipLogin) {
              return true;
            }
            const res = await this.api.checkLogin(check, values, env.gcloud);
            this.wait(run, 'login', res.ok ? undefined : res.detail);
            return res.ok;
          },
          POLL_MS,
        );
        this.wait(run, 'login', undefined);
      }
    } else if (check) {
      throw { cause: { code: 'environments.loginFailed', params: {} } };
    }
    if (prepare) {
      this.act(run, 'login', 'preparing', { command: short(expand(prepare, values)) });
      const res = await this.api.checkLogin(prepare, values, env.gcloud);
      if (!res.ok) {
        throw { cause: { code: 'environments.prepareFailed', params: { detail: res.detail } } };
      }
    }
    return true;
  }

  private tag(env: Environment, values: Record<string, string>): PaneEnv {
    return { id: env.id, name: env.name, values };
  }

  private waitConnected(key: number, run: Run): Promise<void> {
    return this.until(run, CONNECT_WAIT_MS, () => {
      const pane = this.sessions.pane(key);
      if (!pane) {
        throw { cause: { code: 'environments.cancelled', params: {} } };
      }
      if (pane.state === 'failed') {
        throw { cause: { code: pane.error?.key.replace(/^errors\./, '') ?? 'internal', params: pane.error?.params ?? {} } };
      }
      return pane.state === 'connected';
    });
  }

  /** Polls until done() is true, the run stops or the time is up. */
  private async until(run: Run, timeout: number, done: () => boolean | Promise<boolean>, interval = 300): Promise<void> {
    const end = Date.now() + timeout;
    for (;;) {
      if (run.cancelled) {
        throw { cause: { code: 'environments.cancelled', params: {} } };
      }
      if (await done()) {
        return;
      }
      if (Date.now() > end) {
        throw { cause: { code: 'environments.timeout', params: {} } };
      }
      await new Promise((r) => setTimeout(r, interval));
    }
  }
}

/** The scope of an environment's own personal values. */
function scopeOf(id: string): string {
  return `env:${id}`;
}

/**
 * Tells about an expired login with a system notification when Shellvane
 * is not in front (the in-app notice is there when it is).
 */
function systemNotification(name: string, detail: string): void {
  if (typeof document === 'undefined' || document.hasFocus() || !globalThis.Notification || Notification.permission !== 'granted') {
    return;
  }
  try {
    new Notification(name, { body: detail.split('\n')[0].slice(0, 160) });
  } catch {
    // Not supported here.
  }
}
