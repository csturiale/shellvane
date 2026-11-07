import { Injectable, effect, inject, signal, untracked } from '@angular/core';

import { Workspace, WorkspaceApi } from '../core/backend/workspace-api';
import { SessionPrefs } from './session-prefs';
import { SavedWorkspace, SessionsStore } from './sessions.store';
import { UsageStore } from '../core/usage/usage.store';

/** The open tabs are saved this long after the last change. */
const AUTOSAVE_MS = 1000;

/** Named workspaces, and the open tabs restored at the next start. */
@Injectable({ providedIn: 'root' })
export class WorkspacesStore {
  private readonly usage = inject(UsageStore);
  private readonly api = inject(WorkspaceApi);
  private readonly sessions = inject(SessionsStore);
  private readonly prefs = inject(SessionPrefs);
  readonly list = signal<Workspace[]>([]);
  private timer?: ReturnType<typeof setTimeout>;
  /** Autosave starts after the restore, so it cannot overwrite it with nothing. */
  private started = false;

  constructor() {
    effect(() => {
      const on = this.prefs.restoreOnStart();
      // Layout, focus and pane sources all live in these two signals.
      this.sessions.tabs();
      this.sessions.panes();
      untracked(() => this.scheduleSave(on));
    });
  }

  /** Restores the tabs of the previous run, once, if the preference is on. */
  async start(): Promise<void> {
    if (this.started) {
      return;
    }
    try {
      if (this.prefs.restoreOnStart() && this.sessions.tabs().length === 0) {
        const last = parse(await this.api.last());
        if (last) {
          await this.sessions.restore(last);
        }
      }
    } catch {
      // Outside the desktop app, or nothing to restore.
    } finally {
      this.started = true;
      this.scheduleSave(this.prefs.restoreOnStart());
    }
  }

  async load(): Promise<void> {
    this.list.set(await this.api.list());
  }

  async save(name: string): Promise<void> {
    await this.api.save(name, JSON.stringify(this.sessions.snapshot()));
    await this.load();
  }

  async open(w: Workspace): Promise<boolean> {
    this.usage.count('workspace.open');
    const ws = parse(w.layout);
    if (ws) {
      await this.sessions.restore(ws);
    }
    return !!ws;
  }

  async delete(id: string): Promise<void> {
    await this.api.delete(id);
    await this.load();
  }

  private scheduleSave(on: boolean): void {
    if (!this.started) {
      return;
    }
    clearTimeout(this.timer);
    this.timer = setTimeout(() => {
      const layout = on && this.sessions.tabs().length > 0 ? JSON.stringify(this.sessions.snapshot()) : '';
      void this.api.saveLast(layout).catch(() => undefined);
    }, AUTOSAVE_MS);
  }
}

function parse(json: string): SavedWorkspace | null {
  if (!json) {
    return null;
  }
  try {
    const ws = JSON.parse(json) as SavedWorkspace;
    return ws.version === 1 && Array.isArray(ws.tabs) ? ws : null;
  } catch {
    return null;
  }
}
