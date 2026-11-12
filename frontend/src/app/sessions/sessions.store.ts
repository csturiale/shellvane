import { Injectable, computed, effect, inject, signal, untracked } from '@angular/core';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { BrowserChoice, Highlight, KubeInfo, KubeSelection, LocalOptions, Prompt, PromptAnswer, SessionApi, SessionInfo, SessionState } from '../core/backend/session-api';
import { BrowserPrefs } from './browser-prefs';
import { LaunchApi } from '../core/backend/launch-api';
import { DropZone, Layout, SplitDir, grid, leaf, mapPanes, neighbour, panesOf, remove, replace, setRatio, split, swap } from './layout';
import { writePreference } from '../core/preferences';
import { SessionPrefs } from './session-prefs';
import { UsageStore } from '../core/usage/usage.store';
import { PersonalValues } from '../core/personal-values';

export type PaneState = 'connecting' | 'connected' | 'closed' | 'failed';

/** How a pane was opened, so it can reconnect or be duplicated. */
export type PaneSource =
  | { kind: 'host'; hostId: string }
  | { kind: 'target'; spec: string }
  /** gcloud: the pane's gcloud profile (its own gcloud configuration). */
  | { kind: 'local'; program: string; gcloud?: string; browser?: BrowserChoice; openLinks?: boolean }
  | { kind: 'mosh'; hostId: string }
  /** Measures the terminal: generated output, timed until drawn. */
  | { kind: 'benchmark'; megabytes: number; transportOnly?: boolean };

/** One terminal bound to one SSH session. */
export interface Pane {
  /** Stable for the pane's lifetime; the session ID changes on reconnect. */
  key: number;
  sessionId: string;
  title: string;
  /** Where the pane is connected; shown so panes can be told apart. */
  target: { user: string; host: string; port: number };
  /** Group path from the root; empty for quick connections. */
  group: string[];
  /** Nearest group color name, or '' for none. */
  color: string;
  /** Terminal scheme and font size set on the host or its groups; '' / 0 when not set. */
  terminalTheme: string;
  fontSize: number;
  /** Jump hosts on the way, first hop first. */
  via: string[];
  source: PaneSource;
  /** Name of the tmux/screen session a reconnection attaches to again. */
  persist: string;
  /** Highlight rules of the host, applied by the terminal. */
  highlights: Highlight[];
  /** "tmux" or "screen" when the shell runs in a persistent session. */
  multiplexer: string;
  autoReconnect: boolean;
  /** A shell of this computer. */
  local: boolean;
  /** "telnet" or "serial" for panes that are not SSH; '' otherwise. */
  protocol: string;
  /** The terminal was moved from another window: its stream replays the recent output. */
  replay?: boolean;
  /** The pane's kubectl context; null when it has none and cannot switch live. */
  kube: KubeInfo | null;
  /** Context chosen for this pane, kept across reconnections and restores. */
  kubeOverride: KubeSelection | null;
  /** The environment that opened the pane, kept across restores. */
  env: PaneEnv | null;
  /** The session is written to a log / recorded. */
  logging: boolean;
  recording: boolean;
  /** Current folder reported by the shell (OSC 7), '' when unknown. */
  cwd: string;
  /** The pane rang its bell while it was not visible. */
  bell: boolean;
  /** Pending automatic reconnection: attempt number and when it starts. */
  retry: { attempt: number; at: number } | null;
  state: PaneState;
  error: TranslatableError | null;
  exitCode: number;
}

/** What a window sends to the window a tab moves to. */
interface DetachedTab {
  tab: Tab;
  panes: Pane[];
}

/** A tab holds one or more panes arranged by splits. */
export interface Tab {
  key: number;
  layout: Layout;
  focus: number;
  /** Input typed in one pane goes to every pane of the tab not excluded. */
  broadcast: boolean;
  excluded: number[];
  /** Name given by the user; '' shows the focused pane's title. */
  title: string;
  /** Pane shown alone in the tab, or null. */
  maximized: number | null;
}

/** The environment a pane belongs to, with the variables it ran with. */
export interface PaneEnv {
  id: string;
  name: string;
  values: Record<string, string>;
}

/** Extra settings of a pane opened by an environment. */
export interface OpenExtra {
  env?: PaneEnv;
  /** Typed after login (after the host's own startup script). */
  script?: string[];
  kube?: KubeSelection | null;
  /** gcloud profile of a local pane ('' or absent: gcloud's own configuration). */
  gcloud?: string;
  /** How the shell's tools (gcloud, az, aws sso) open the browser. */
  browser?: BrowserChoice;
  /** Sign-in links printed in the terminal open in that browser. */
  openLinks?: boolean;
}

/** A pane as saved in a workspace. */
export interface SavedPane {
  source: PaneSource;
  title: string;
  persist: string;
  kube?: KubeSelection | null;
  env?: PaneEnv | null;
}

/** A tab as saved in a workspace; layout leaves index `panes`. */
export interface SavedTab {
  layout: Layout;
  panes: SavedPane[];
  focus: number;
  broadcast: boolean;
  title?: string;
}

export interface SavedWorkspace {
  version: 1;
  tabs: SavedTab[];
  active: number;
}

/** Automatic reconnection: 1, 2, 4… seconds, at most 30, 8 attempts. */
export const RETRY_MAX_ATTEMPTS = 8;
export function retryDelay(attempt: number): number {
  return Math.min(30_000, 1000 * 2 ** (attempt - 1));
}

/** Failures that another attempt cannot fix. */
const FINAL_ERRORS = new Set([
  'errors.ssh.authFailed',
  'errors.ssh.hostKeyChanged',
  'errors.ssh.hostKeyRejected',
  'errors.ssh.cancelled',
  'errors.ssh.passphraseFailed',
  'errors.ssh.jumpLoop',
  'errors.ssh.invalidTarget',
  'errors.hosts.notFound',
]);

/** Size used to open a session before the terminal has measured itself. */
const INITIAL_COLS = 120;
const INITIAL_ROWS = 32;

@Injectable({ providedIn: 'root' })
export class SessionsStore {
  private readonly usage = inject(UsageStore);
  private readonly api = inject(SessionApi);
  private readonly launch = inject(LaunchApi);
  private nextKey = 1;
  /** States that arrived before their pane existed (fast failures). */
  private readonly early = new Map<string, SessionState>();
  /** Writes input to a pane's terminal; registered by each terminal view. */
  private readonly senders = new Map<number, (data: string) => void>();
  private readonly retryTimers = new Map<number, ReturnType<typeof setTimeout>>();
  /** Log format of panes logged by hand, to log again after a reconnection. */
  private readonly logFormats = new Map<number, string>();
  private readonly prefs = inject(SessionPrefs);
  private readonly browserPrefs = inject(BrowserPrefs);
  private readonly personal = inject(PersonalValues);
  /** Sessions last reported to Go as receiving broadcast input. */
  private broadcasting = new Set<string>();

  readonly panes = signal<Pane[]>([]);
  readonly tabs = signal<Tab[]>([]);
  readonly activeKey = signal<number | null>(null);
  readonly active = computed(() => this.tabs().find((t) => t.key === this.activeKey()) ?? null);
  readonly prompts = signal<Prompt[]>([]);

  constructor() {
    try {
      this.api.onState((s) => {
        if (!this.panes().some((p) => p.sessionId === s.id)) {
          this.early.set(s.id, s);
          return;
        }
        this.apply(s);
        if (s.state !== 'connecting') {
          // Prompts of a finished connection can no longer be answered.
          this.prompts.update((list) => list.filter((p) => p.sessionId !== s.id));
        }
      });
      this.api.onPrompt((p) => this.prompts.update((list) => [...list, p]));
    } catch {
      // Outside the desktop app there is no event bus.
    }
    // Automatic responses are suspended in sessions that receive broadcast.
    effect(() => {
      const now = new Set<string>();
      for (const t of this.tabs()) {
        if (t.broadcast) {
          for (const k of panesOf(t.layout)) {
            const p = this.panes().find((x) => x.key === k);
            if (p && !t.excluded.includes(k)) {
              now.add(p.sessionId);
            }
          }
        }
      }
      untracked(() => this.syncBroadcast(now));
    });
    // Back online: pending reconnections start now instead of waiting.
    globalThis.addEventListener?.('online', () => {
      for (const p of this.panes()) {
        if (p.retry) {
          void this.attemptReconnect(p.key, p.retry.attempt);
        }
      }
    });
  }

  pane(key: number): Pane | undefined {
    return this.panes().find((p) => p.key === key);
  }

  /** The pane that has focus in the active tab. */
  readonly focused = computed(() => {
    const tab = this.active();
    return tab ? (this.panes().find((p) => p.key === tab.focus) ?? null) : null;
  });

  /** Opens a session to a saved host in a new tab; returns the pane key. */
  async openHost(hostId: string, title: string, extra: OpenExtra = {}): Promise<number> {
    this.usage.count('session.ssh');
    const pane = await this.createPane({ kind: 'host', hostId }, title, undefined, false, extra.kube ?? null, extra.script, extra.env);
    this.addTab(pane.key);
    return pane.key;
  }

  /**
   * Opens several hosts in one tab, in a grid; with broadcast on, typing
   * goes to all of them.
   */
  async openMany(hosts: { id: string; name: string }[], broadcast = false): Promise<void> {
    this.usage.count('session.many');
    const keys: number[] = [];
    for (const h of hosts) {
      try {
        keys.push((await this.createPane({ kind: 'host', hostId: h.id }, h.name)).key);
      } catch {
        // a host that cannot be opened is left out
      }
    }
    if (keys.length === 0) {
      return;
    }
    const tab: Tab = { key: this.nextKey++, layout: grid(keys), focus: keys[0], broadcast, excluded: [], title: '', maximized: null };
    this.tabs.update((list) => [...list, tab]);
    this.activeKey.set(tab.key);
  }

  /** Opens a shell of this computer ('' = the preferred one); returns the pane key. */
  /** Opens a tab that measures how fast the terminal shows output. */
  async openBenchmark(megabytes = 8, transportOnly = false): Promise<number> {
    const pane = await this.createPane({ kind: 'benchmark', megabytes, transportOnly }, 'benchmark');
    this.addTab(pane.key);
    return pane.key;
  }

  async openLocal(program = this.prefs.localShell(), extra: OpenExtra = {}): Promise<number> {
    this.usage.count('session.local');
    const source: PaneSource = { kind: 'local', program, gcloud: extra.gcloud, browser: extra.browser, openLinks: extra.openLinks };
    const pane = await this.createPane(source, '', undefined, false, extra.kube ?? null, extra.script, extra.env);
    this.addTab(pane.key);
    return pane.key;
  }

  /**
   * Switches a pane's kubectl context. Panes with their own kubectl file
   * switch live; the others reopen with the new context.
   */
  async setKube(paneKey: number, sel: KubeSelection): Promise<void> {
    const pane = this.pane(paneKey);
    if (!pane) {
      return;
    }
    if (pane.kube?.live) {
      await this.api.setKube(pane.sessionId, sel);
      this.patchPane(paneKey, { kube: { ...pane.kube, context: sel.context, namespace: sel.namespace }, kubeOverride: sel });
      return;
    }
    this.patchPane(paneKey, { kubeOverride: sel });
    await this.reconnect(paneKey);
  }

  /** Opens a host with Mosh (roaming, survives sleep and network changes). */
  async openMosh(hostId: string, title: string): Promise<void> {
    this.usage.count('session.mosh');
    const pane = await this.createPane({ kind: 'mosh', hostId }, 'mosh ' + title);
    this.addTab(pane.key);
  }

  async openTarget(spec: string): Promise<void> {
    this.usage.count('session.quick');
    const pane = await this.createPane({ kind: 'target', spec }, spec);
    this.addTab(pane.key);
  }

  /** Opens a new session to the focused pane's host next to it. */
  split(dir: SplitDir): Promise<void> {
    return this.splitWith(dir);
  }

  /**
   * Opens a session next to the focused pane: to the same host, or to
   * another source (a saved host, an address, a local terminal). Without
   * a tab open it opens in a new tab.
   */
  async splitWith(dir: SplitDir, other?: { source: PaneSource; title: string }): Promise<void> {
    this.usage.count('pane.split');
    const tab = this.active();
    const from = tab && this.pane(tab.focus);
    const source = other?.source ?? from?.source;
    if (!source) {
      return;
    }
    const pane = await this.createPane(source, other?.title ?? from?.title ?? '');
    const current = this.active();
    if (!current || !this.pane(current.focus)) {
      this.addTab(pane.key);
      return;
    }
    this.updateTab(current.key, (t) => ({ ...t, layout: split(t.layout, t.focus, pane.key, dir), focus: pane.key }));
  }

  /** The default local shell, for a split with a local terminal. */
  localSource(): PaneSource {
    return { kind: 'local', program: this.prefs.localShell() };
  }

  focusPane(tabKey: number, paneKey: number): void {
    this.updateTab(tabKey, (t) => ({ ...t, focus: paneKey }));
    if (this.pane(paneKey)?.bell) {
      this.patchPane(paneKey, { bell: false });
    }
  }

  /** Shows a pane alone in its tab, or the whole layout again. */
  toggleMaximized(paneKey: number): void {
    const tab = this.tabOf(paneKey);
    if (tab) {
      this.updateTab(tab.key, (t) => ({ ...t, maximized: t.maximized === paneKey ? null : paneKey, focus: paneKey }));
    }
  }

  /** Starts or stops the pane's log; returns the file path when started. */
  async toggleLogging(paneKey: number, format: string): Promise<string | null> {
    const pane = this.pane(paneKey);
    if (!pane) {
      return null;
    }
    if (pane.logging) {
      this.logFormats.delete(paneKey);
      await this.api.stopLogging(pane.sessionId);
      this.patchPane(paneKey, { logging: false });
      return null;
    }
    const path = await this.api.startLogging(pane.sessionId, format);
    this.logFormats.set(paneKey, format);
    this.patchPane(paneKey, { logging: true });
    return path;
  }

  /** Starts or stops the pane's recording; returns the file path when started. */
  async toggleRecording(paneKey: number): Promise<string | null> {
    this.usage.count('recording');
    const pane = this.pane(paneKey);
    if (!pane) {
      return null;
    }
    if (pane.recording) {
      await this.api.stopRecording(pane.sessionId);
      this.patchPane(paneKey, { recording: false });
      return null;
    }
    const path = await this.api.startRecording(pane.sessionId);
    this.patchPane(paneKey, { recording: true });
    return path;
  }

  setCwd(paneKey: number, cwd: string): void {
    if (this.pane(paneKey)?.cwd !== cwd) {
      this.patchPane(paneKey, { cwd });
    }
  }

  renameTab(tabKey: number, title: string): void {
    this.updateTab(tabKey, (t) => ({ ...t, title: title.trim() }));
  }

  /** A pane rang its bell: mark it when it cannot be seen. */
  bell(paneKey: number, hidden: boolean): void {
    if (!hidden) {
      return;
    }
    this.patchPane(paneKey, { bell: true });
    const pane = this.pane(paneKey);
    if (pane && typeof document !== 'undefined' && !document.hasFocus() && globalThis.Notification?.permission === 'granted') {
      try {
        new Notification(pane.title, { body: '🔔' });
      } catch {
        // not supported
      }
    }
  }

  /**
   * A notification from a pane (OSC 9/777, a long command that ended): the
   * tab is marked when the pane is hidden, and a system notification is
   * shown when Shellvane is not in front.
   */
  notify(paneKey: number, title: string, body: string, hidden: boolean): void {
    if (hidden) {
      this.patchPane(paneKey, { bell: true });
    }
    if (typeof document === 'undefined' || (document.hasFocus() && !hidden) || !globalThis.Notification) {
      return;
    }
    const show = () => {
      try {
        new Notification(title, { body });
      } catch {
        // not supported
      }
    };
    if (Notification.permission === 'granted') {
      show();
    } else if (Notification.permission === 'default') {
      void Notification.requestPermission().then((p) => p === 'granted' && show(), () => undefined);
    }
  }

  /** Clears the bell marks of the panes of a tab. */
  activate(tabKey: number): void {
    this.activeKey.set(tabKey);
    const tab = this.tabs().find((t) => t.key === tabKey);
    for (const k of tab ? panesOf(tab.layout) : []) {
      if (this.pane(k)?.bell) {
        this.patchPane(k, { bell: false });
      }
    }
  }

  /** Moves focus to the neighbouring pane of the active tab. */
  moveFocus(dir: 'left' | 'right' | 'up' | 'down'): void {
    const tab = this.active();
    const next = tab && neighbour(tab.layout, tab.focus, dir);
    if (tab && next !== null && next !== undefined) {
      this.focusPane(tab.key, next);
    }
  }

  registerSender(paneKey: number, send: (data: string) => void): () => void {
    this.senders.set(paneKey, send);
    return () => this.senders.delete(paneKey);
  }

  toggleBroadcast(tabKey: number): void {
    this.usage.count('broadcast');
    this.updateTab(tabKey, (t) => ({ ...t, broadcast: !t.broadcast }));
  }

  toggleExcluded(tabKey: number, paneKey: number): void {
    this.updateTab(tabKey, (t) => ({
      ...t,
      excluded: t.excluded.includes(paneKey) ? t.excluded.filter((k) => k !== paneKey) : [...t.excluded, paneKey],
    }));
  }

  /** Sets which panes of a tab are kept out of its broadcast. */
  setExcluded(tabKey: number, excluded: number[]): void {
    this.updateTab(tabKey, (t) => ({ ...t, excluded }));
  }

  /** Panes of a tab that receive its broadcast. */
  tabReceivers(tabKey: number): number[] {
    const tab = this.tabs().find((t) => t.key === tabKey);
    return tab ? panesOf(tab.layout).filter((k) => !tab.excluded.includes(k)) : [];
  }

  /** Sends text to every receiving pane of a tab (the broadcast bar). */
  sendToTab(tabKey: number, text: string): number {
    const targets = this.tabReceivers(tabKey);
    for (const k of targets) {
      this.senders.get(k)?.(text);
    }
    return targets.length;
  }

  /** Whether a pane takes part in its tab's broadcast. */
  isBroadcasting(paneKey: number): boolean {
    const tab = this.tabOf(paneKey);
    return !!tab && tab.broadcast && !tab.excluded.includes(paneKey);
  }

  /** Panes that receive input written to `paneKey` (itself included). */
  receivers(paneKey: number): number[] {
    const tab = this.tabOf(paneKey);
    if (!tab || !this.isBroadcasting(paneKey)) {
      return [paneKey];
    }
    return panesOf(tab.layout).filter((k) => !tab.excluded.includes(k));
  }

  /** Called by a terminal for what the user typed there: mirrors it. */
  typed(paneKey: number, data: string): void {
    for (const k of this.receivers(paneKey)) {
      if (k !== paneKey) {
        this.senders.get(k)?.(data);
      }
    }
  }

  /** Sends text to the focused pane, and to the others when broadcasting. */
  sendToFocused(text: string): number {
    const pane = this.focused();
    if (!pane) {
      return 0;
    }
    const targets = this.receivers(pane.key);
    for (const k of targets) {
      this.senders.get(k)?.(text);
    }
    return targets.length;
  }

  private tabOf(paneKey: number): Tab | undefined {
    return this.tabs().find((t) => panesOf(t.layout).includes(paneKey));
  }

  /**
   * Moves a pane next to another (left/right/top/bottom) or swaps the two
   * (center). The panes may be in different tabs; sessions are untouched.
   */
  movePane(paneKey: number, targetKey: number, zone: DropZone): void {
    if (paneKey === targetKey) {
      return;
    }
    const from = this.tabOf(paneKey);
    const to = this.tabOf(targetKey);
    if (!from || !to) {
      return;
    }
    if (zone === 'center') {
      if (from.key === to.key) {
        this.updateTab(to.key, (t) => ({ ...t, layout: swap(t.layout, paneKey, targetKey), focus: paneKey }));
      } else {
        this.updateTab(from.key, (t) => ({ ...t, layout: replace(t.layout, paneKey, targetKey), focus: targetKey }));
        this.updateTab(to.key, (t) => ({ ...t, layout: replace(t.layout, targetKey, paneKey), focus: paneKey }));
      }
      this.activeKey.set(to.key);
      return;
    }
    this.detach(paneKey);
    const dir: SplitDir = zone === 'left' || zone === 'right' ? 'row' : 'column';
    const before = zone === 'left' || zone === 'top';
    this.updateTab(to.key, (t) => ({ ...t, layout: split(t.layout, targetKey, paneKey, dir, before), focus: paneKey }));
    this.activeKey.set(to.key);
  }

  /** Moves a pane into another tab, to the right of its focused pane. */
  movePaneToTab(paneKey: number, tabKey: number): void {
    const target = this.tabs().find((t) => t.key === tabKey);
    if (!target || panesOf(target.layout).includes(paneKey)) {
      return;
    }
    this.movePane(paneKey, target.focus, 'right');
  }

  /** Moves a pane out of its tab into a new one. */
  movePaneToNewTab(paneKey: number): void {
    const from = this.tabOf(paneKey);
    if (!from || panesOf(from.layout).length === 1) {
      return; // already alone in its tab
    }
    this.detach(paneKey);
    this.addTab(paneKey);
  }

  /** Takes a pane out of its tab's layout, dropping the tab if it empties. */
  private detach(paneKey: number): void {
    const tab = this.tabOf(paneKey);
    if (!tab) {
      return;
    }
    const layout = remove(tab.layout, paneKey);
    if (!layout) {
      this.dropTab(tab.key);
      return;
    }
    const focus = tab.focus === paneKey ? panesOf(layout)[0] : tab.focus;
    this.updateTab(tab.key, (t) => ({ ...t, layout, focus, excluded: t.excluded.filter((k) => k !== paneKey) }));
  }

  resizeSplit(tabKey: number, path: string, ratio: number): void {
    this.updateTab(tabKey, (t) => ({ ...t, layout: setRatio(t.layout, path, ratio) }));
  }

  /** Reconnects now, ending any automatic reconnection in progress. */
  async reconnect(paneKey: number): Promise<void> {
    this.cancelRetry(paneKey);
    await this.attemptReconnect(paneKey, 0);
  }

  /** Stops automatic reconnection; the pane stays closed or failed. */
  cancelRetry(paneKey: number): void {
    clearTimeout(this.retryTimers.get(paneKey));
    this.retryTimers.delete(paneKey);
    this.patchPane(paneKey, { retry: null });
  }

  /**
   * Opens a new session for the pane. A multiplexed pane attaches to its
   * tmux/screen session again, so its programs are still running.
   */
  private async attemptReconnect(paneKey: number, attempt: number): Promise<void> {
    clearTimeout(this.retryTimers.get(paneKey));
    this.retryTimers.delete(paneKey);
    const pane = this.pane(paneKey);
    if (!pane) {
      return;
    }
    let info: SessionInfo;
    try {
      info = await this.start(pane.source, pane.persist, true, pane.kubeOverride);
    } catch (err) {
      this.patchPane(paneKey, { state: 'failed', error: toTranslatableError(err), retry: null });
      return;
    }
    this.panes.update((list) =>
      list.map((p) =>
        p.key === paneKey
          ? { ...p, ...describe(info), sessionId: info.id, state: 'connecting', error: null, retry: attempt ? { attempt, at: 0 } : null }
          : p,
      ),
    );
    this.applyEarly(info.id);
    // A log or recording started by hand goes on in the new session.
    const format = this.logFormats.get(paneKey);
    if (format && !info.logging) {
      void this.api.startLogging(info.id, format).then(() => this.patchPane(paneKey, { logging: true }), () => undefined);
    }
    if (pane.recording && !info.recording) {
      void this.api.startRecording(info.id).then(() => this.patchPane(paneKey, { recording: true }), () => undefined);
    }
  }

  private scheduleRetry(paneKey: number, attempt: number): void {
    if (attempt > RETRY_MAX_ATTEMPTS) {
      this.patchPane(paneKey, { retry: null });
      return;
    }
    const delay = retryDelay(attempt);
    this.patchPane(paneKey, { retry: { attempt, at: Date.now() + delay } });
    clearTimeout(this.retryTimers.get(paneKey));
    this.retryTimers.set(
      paneKey,
      setTimeout(() => void this.attemptReconnect(paneKey, attempt), delay),
    );
  }

  private syncBroadcast(now: Set<string>): void {
    const on = [...now].filter((id) => !this.broadcasting.has(id));
    const off = [...this.broadcasting].filter((id) => !now.has(id));
    this.broadcasting = now;
    if (on.length) {
      void this.api.setBroadcast(on, true).catch(() => undefined);
    }
    if (off.length) {
      void this.api.setBroadcast(off, false).catch(() => undefined);
    }
  }

  private patchPane(paneKey: number, change: Partial<Pane>): void {
    this.panes.update((list) => list.map((p) => (p.key === paneKey ? { ...p, ...change } : p)));
  }

  /** Closes one pane; closing the last pane of a tab closes the tab. */
  async closePane(paneKey: number): Promise<void> {
    const tab = this.tabs().find((t) => panesOf(t.layout).includes(paneKey));
    if (!tab) {
      return;
    }
    const layout = remove(tab.layout, paneKey);
    if (!layout) {
      this.dropTab(tab.key);
    } else {
      const focus = tab.focus === paneKey ? panesOf(layout)[0] : tab.focus;
      this.updateTab(tab.key, (t) => ({ ...t, layout, focus }));
    }
    await this.endPanes([paneKey]);
  }

  async closeTab(tabKey: number): Promise<void> {
    const tab = this.tabs().find((t) => t.key === tabKey);
    if (!tab) {
      return;
    }
    this.dropTab(tabKey);
    await this.endPanes(panesOf(tab.layout));
  }

  async answer(prompt: Prompt, answer: PromptAnswer): Promise<void> {
    this.prompts.update((list) => list.filter((p) => p.id !== prompt.id));
    await this.api.answer(prompt.id, answer).catch(() => undefined);
  }

  titleOf(sessionId: string): string {
    return this.panes().find((p) => p.sessionId === sessionId)?.title ?? '';
  }

  private async createPane(
    source: PaneSource,
    title: string,
    persist = newPersistName(),
    resume = false,
    kubeOverride: KubeSelection | null = null,
    script: string[] = [],
    env: PaneEnv | null = null,
  ): Promise<Pane> {
    const info = await this.start(source, persist, resume, kubeOverride, script);
    const pane: Pane = {
      key: this.nextKey++,
      sessionId: info.id,
      ...describe(info),
      title: info.title || title,
      source,
      persist,
      kubeOverride,
      env,
      cwd: '',
      bell: false,
      retry: null,
      state: 'connecting',
      error: null,
      exitCode: 0,
    };
    this.panes.update((list) => [...list, pane]);
    this.applyEarly(info.id);
    return pane;
  }

  /** Moves a tab, with its sessions still open, to a window of its own. */
  detachTab(tabKey: number): Promise<void> {
    return this.handOver(tabKey, (payload, ids) => this.api.detach(payload, ids));
  }

  /** Gives a detached window's tab back to the main window. */
  reattachTab(tabKey: number): Promise<void> {
    return this.handOver(tabKey, (payload, ids) => this.api.reattach(payload, ids));
  }

  /** Panes moved to another window: gone from here, but still open. */
  readonly movedAway = new Set<number>();

  private async handOver(tabKey: number, send: (payload: string, sessionIds: string[]) => Promise<void>): Promise<void> {
    const tab = this.tabs().find((t) => t.key === tabKey);
    if (!tab) {
      return;
    }
    const keys = panesOf(tab.layout);
    const panes = this.panes().filter((p) => keys.includes(p.key));
    await send(JSON.stringify({ tab, panes } satisfies DetachedTab), panes.map((p) => p.sessionId));
    // Forgotten here without closing: the new window takes the sessions over.
    for (const k of keys) {
      this.movedAway.add(k);
      this.logFormats.delete(k);
      clearTimeout(this.retryTimers.get(k));
      this.retryTimers.delete(k);
    }
    const sessions = new Set(panes.map((p) => p.sessionId));
    this.panes.update((list) => list.filter((p) => !keys.includes(p.key)));
    this.prompts.update((list) => list.filter((p) => !sessions.has(p.sessionId)));
    this.dropTab(tabKey);
  }

  /** Shows, in this window, the tab another window moved here. */
  async adopt(token: string): Promise<void> {
    const moved = JSON.parse(await this.api.takeDetached(token)) as DetachedTab;
    const keyOf = new Map<number, number>();
    const panes = moved.panes.map((p) => {
      const key = this.nextKey++;
      keyOf.set(p.key, key);
      return { ...p, key, replay: true, retry: null };
    });
    const map = (k: number) => keyOf.get(k) ?? panes[0].key;
    this.panes.update((list) => [...list, ...panes]);
    const tab: Tab = {
      ...moved.tab,
      key: this.nextKey++,
      layout: mapPanes(moved.tab.layout, map),
      focus: map(moved.tab.focus),
      excluded: moved.tab.excluded.filter((k) => keyOf.has(k)).map(map),
      maximized: moved.tab.maximized === null ? null : map(moved.tab.maximized),
    };
    this.tabs.update((list) => [...list, tab]);
    this.activeKey.set(tab.key);
    for (const p of panes) {
      this.applyEarly(p.sessionId);
    }
  }

  private addTab(paneKey: number): void {
    const tab: Tab = { key: this.nextKey++, layout: leaf(paneKey), focus: paneKey, broadcast: false, excluded: [], title: '', maximized: null };
    this.tabs.update((list) => [...list, tab]);
    this.activeKey.set(tab.key);
  }

  private dropTab(tabKey: number): void {
    const list = this.tabs();
    const index = list.findIndex((t) => t.key === tabKey);
    this.tabs.set(list.filter((t) => t.key !== tabKey));
    if (this.activeKey() === tabKey) {
      const next = this.tabs()[Math.min(index, this.tabs().length - 1)];
      this.activeKey.set(next?.key ?? null);
    }
  }

  private async endPanes(keys: number[]): Promise<void> {
    for (const k of keys) {
      this.logFormats.delete(k);
      clearTimeout(this.retryTimers.get(k));
      this.retryTimers.delete(k);
    }
    const ended = this.panes().filter((p) => keys.includes(p.key));
    this.panes.update((list) => list.filter((p) => !keys.includes(p.key)));
    const sessions = new Set(ended.map((p) => p.sessionId));
    this.prompts.update((list) => list.filter((p) => !sessions.has(p.sessionId)));
    await Promise.all(ended.map((p) => this.api.close(p.sessionId).catch(() => undefined)));
  }

  private updateTab(key: number, change: (t: Tab) => Tab): void {
    this.tabs.update((list) =>
      list.map((t) => {
        if (t.key !== key) {
          return t;
        }
        const next = change(t);
        // A new layout (split, move, close) shows every pane again.
        return next.layout !== t.layout ? { ...next, maximized: null } : next;
      }),
    );
  }

  private start(source: PaneSource, persist: string, resume: boolean, kube: KubeSelection | null, script: string[] = []): Promise<SessionInfo> {
    switch (source.kind) {
      case 'host':
        return this.api.openHost(source.hostId, INITIAL_COLS, INITIAL_ROWS, { persist, resume, kube: kube ?? undefined, script });
      case 'local': {
        const context = this.prefs.localKubeContext();
        const fallback = context ? { context, namespace: this.prefs.localKubeNamespace() } : undefined;
        return this.api.openLocal(INITIAL_COLS, INITIAL_ROWS, { program: source.program, dir: '', persist, kube: kube ?? fallback, script, ...this.browserOf(source) });
      }
      case 'mosh':
        return this.launch.openMosh(source.hostId, INITIAL_COLS, INITIAL_ROWS);
      case 'benchmark':
        return this.api.openBenchmark(INITIAL_COLS, INITIAL_ROWS, source.megabytes);
      default:
        return this.api.openTarget(source.spec, INITIAL_COLS, INITIAL_ROWS);
    }
  }

  /** The browser of a local terminal: its own (an environment's login), else the settings' when they apply to every terminal. */
  private browserOf(source: { gcloud?: string; browser?: BrowserChoice; openLinks?: boolean }): Partial<LocalOptions> {
    const s = this.browserPrefs.settings();
    const all = s.allTerminals && !source.browser;
    return {
      gcloud: source.gcloud ?? '',
      browser: source.browser ?? (all ? this.browserPrefs.choice('', s.allPrivate) : null),
      openLinks: !!source.openLinks || (all && s.allLinks),
    };
  }

  private applyEarly(sessionId: string): void {
    const s = this.early.get(sessionId);
    if (s) {
      this.early.delete(sessionId);
      this.apply(s);
    }
  }

  private apply(s: SessionState): void {
    const pane = this.panes().find((p) => p.sessionId === s.id);
    if (!pane) {
      return;
    }
    const error = s.error ? toTranslatableError({ cause: s.error }) : null;
    const state = s.state as PaneState;
    this.patchPane(pane.key, { state, exitCode: s.exitCode, error, retry: state === 'connected' ? null : pane.retry });
    if (state === 'connected') {
      writePreference('guide.connected', 'true'); // first steps of the guide
    }
    if (!pane.autoReconnect || !this.prefs.autoReconnect()) {
      return;
    }
    if (state === 'closed' && s.lost) {
      this.scheduleRetry(pane.key, 1);
    } else if (state === 'failed' && pane.retry && !(error && FINAL_ERRORS.has(error.key))) {
      // A reconnection failed: the network may still be down.
      this.scheduleRetry(pane.key, pane.retry.attempt + 1);
    } else if (state === 'failed') {
      this.patchPane(pane.key, { retry: null });
    }
  }

  /**
   * A shared workspace carries the names of an environment pane's values
   * but not this person's values: fill the empty ones from their own.
   */
  private withPersonal(env: PaneEnv | null): PaneEnv | null {
    if (!env) {
      return null;
    }
    const values = { ...env.values };
    for (const [name, value] of Object.entries(values)) {
      if (value === '') {
        values[name] = this.personal.get(name, `env:${env.id}`) ?? '';
      }
    }
    return { ...env, values };
  }

  /** The open tabs, to save as a workspace. */
  snapshot(): SavedWorkspace {
    const tabs = this.tabs().map((t): SavedTab => {
      const keys = panesOf(t.layout);
      const panes = keys.map((k) => {
        const p = this.pane(k)!;
        return { source: p.source, title: p.title, persist: p.persist, kube: p.kubeOverride, env: p.env };
      });
      return { layout: mapPanes(t.layout, (k) => keys.indexOf(k)), panes, focus: Math.max(0, keys.indexOf(t.focus)), broadcast: t.broadcast, title: t.title };
    });
    const active = Math.max(0, this.tabs().findIndex((t) => t.key === this.activeKey()));
    return { version: 1, tabs, active };
  }

  /**
   * Opens a saved workspace next to the open tabs. Multiplexed panes
   * attach to their tmux/screen sessions again.
   */
  async restore(ws: SavedWorkspace): Promise<void> {
    const before = this.tabs().length;
    for (const saved of ws.tabs) {
      const keys: number[] = [];
      for (const sp of saved.panes) {
        try {
          if (sp.source.kind === 'benchmark') {
            throw new Error('not restored'); // a measurement, not a session to resume
          }
          keys.push((await this.createPane(sp.source, sp.title, sp.persist || newPersistName(), true, sp.kube ?? null, [], this.withPersonal(sp.env ?? null))).key);
        } catch {
          keys.push(-1); // the host was deleted: leave it out
        }
      }
      const layout = remove(
        mapPanes(saved.layout, (i) => keys[i] ?? -1),
        -1,
      );
      if (!layout) {
        continue;
      }
      const focus = keys[saved.focus] >= 0 ? keys[saved.focus] : panesOf(layout)[0];
      const tab: Tab = { key: this.nextKey++, layout, focus, broadcast: saved.broadcast, excluded: [], title: saved.title ?? '', maximized: null };
      this.tabs.update((list) => [...list, tab]);
    }
    const active = this.tabs()[before + ws.active] ?? this.tabs()[before];
    if (active) {
      this.activeKey.set(active.key);
    }
  }
}

/** Connection details of a session, as shown on its pane. */
function describe(
  info: SessionInfo,
): Pick<
  Pane,
  | 'title'
  | 'target'
  | 'group'
  | 'color'
  | 'terminalTheme'
  | 'fontSize'
  | 'via'
  | 'multiplexer'
  | 'autoReconnect'
  | 'highlights'
  | 'local'
  | 'protocol'
  | 'kube'
  | 'logging'
  | 'recording'
> {
  return {
    multiplexer: info.multiplexer ?? '',
    highlights: info.highlights ?? [],
    local: info.local ?? false,
    protocol: info.protocol ?? '',
    logging: info.logging ?? false,
    recording: info.recording ?? false,
    kube: info.kube ?? null,
    autoReconnect: info.autoReconnect ?? false,
    title: info.title,
    target: info.target,
    group: info.group ?? [],
    color: info.color ?? '',
    terminalTheme: info.terminalTheme ?? '',
    fontSize: info.fontSize ?? 0,
    via: info.via ?? [],
  };
}

/** A new tmux/screen session name: stable for the pane's lifetime. */
function newPersistName(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(4));
  return 'shellvane-' + Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

/** "user@host:port", with brackets around IPv6 addresses. */
export function addressOf(p: Pick<Pane, 'target'> & { local?: boolean; protocol?: string }): string {
  if (p.local) {
    return `${p.target.user}@${p.target.host}`;
  }
  if (p.protocol === 'serial') {
    return p.target.host;
  }
  const host = p.target.host.includes(':') ? `[${p.target.host}]` : p.target.host;
  if (p.protocol === 'telnet') {
    return `telnet://${p.target.user ? p.target.user + '@' : ''}${host}:${p.target.port}`;
  }
  return `${p.target.user}@${host}:${p.target.port}`;
}

/** Title and address, without repeating the address when it is the title. */
export function labelOf(p: Pick<Pane, 'title' | 'target'> & { local?: boolean; protocol?: string }): string {
  const address = addressOf(p);
  return p.title === address ? address : `${p.title} · ${address}`;
}
