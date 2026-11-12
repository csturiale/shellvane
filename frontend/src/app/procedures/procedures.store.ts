import { Injectable, computed, inject, signal } from '@angular/core';

import { Doc, ProcedureApi, Source } from '../core/backend/procedure-api';
import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { readPreference, writePreference } from '../core/preferences';
import type { SharedProcedureSource } from '../extensions/extension';

/** A source of procedures as saved: a Git repository or a folder. */
export interface SavedSource extends Source {
  id: string;
  name: string;
}

/** A procedure linked to an environment: shown when one of its terminals is focused. */
export interface ProcedureLink {
  sourceId: string;
  path: string;
  title: string;
}

export interface OpenDoc {
  path: string;
  title: string;
  markdown: string;
}

/**
 * Procedures: sources, the list of the chosen one, the open procedure and
 * the steps done. Placeholder values are kept per source while the app
 * runs, so a namespace typed once serves every procedure of that source.
 */
@Injectable({ providedIn: 'root' })
export class ProceduresStore {
  private readonly api = inject(ProcedureApi);

  /** The sources added by the user. */
  readonly sources = signal<SavedSource[]>(parse(readPreference('procedures.sources')));
  /** Pages received from the browser or pasted: a folder of the app's data. */
  readonly inbox = signal<SavedSource | null>(null);
  /** Repositories the user's teams share: read-only here, kept by the team. */
  readonly teamSources = signal<SharedProcedureSource[]>([]);
  /** Every source: the user's, the teams', then the received pages. */
  readonly all = computed(() => [
    ...this.sources(),
    ...this.teamSources()
      .filter((t) => !this.sources().some((s) => s.kind === 'git' && s.url === t.url && (s.folder ?? '') === t.folder))
      .map((t): SavedSource => ({ id: TEAM_PREFIX + t.key, name: `${t.name} · ${t.team}`, kind: 'git', url: t.url, branch: t.branch, folder: t.folder })),
    ...(this.inbox() ? [this.inbox()!] : []),
  ]);
  /** The current source comes from a team: it is changed there, not here. */
  readonly isTeam = computed(() => (this.current()?.id ?? '').startsWith(TEAM_PREFIX));
  readonly currentId = signal<string>(readPreference('procedures.current') ?? '');
  readonly current = computed(() => this.all().find((s) => s.id === this.currentId()) ?? this.all()[0] ?? null);
  readonly isInbox = computed(() => this.current()?.id === INBOX);
  readonly docs = signal<Doc[]>([]);
  /** When the repository was last updated (RFC 3339), '' for never or a folder. */
  readonly synced = signal('');
  readonly loading = signal(false);
  readonly error = signal<TranslatableError | null>(null);
  readonly doc = signal<OpenDoc | null>(null);
  /** Steps done in each procedure (source id + path → block indexes). */
  private readonly doneByDoc = signal<Record<string, number[]>>({});
  private readonly valuesBySource = signal<Record<string, Record<string, string>>>({});

  /** Procedures linked to each environment (environment id → links), on this computer. */
  readonly links = signal<Record<string, ProcedureLink[]>>(parseLinks(readPreference('procedures.envLinks')));

  readonly done = computed(() => new Set(this.doneByDoc()[this.docKey()] ?? []));
  readonly values = computed(() => this.valuesBySource()[this.current()?.id ?? ''] ?? {});

  /** Finds the folder of the received pages (named by the panel). */
  async loadInbox(name: string): Promise<void> {
    if (!this.inbox()) {
      try {
        this.inbox.set({ ...(await this.api.inbox()), id: INBOX, name });
      } catch {
        // Outside the desktop app.
      }
    }
  }

  /** Keeps a page as a received procedure and opens it. */
  async saveInbox(title: string, source: string, markdown: string): Promise<void> {
    const doc = await this.api.saveInbox(title, source, markdown);
    await this.openReceived(doc.path);
  }

  /** Shows a received page (from the browser or pasted). */
  async openReceived(path: string): Promise<void> {
    if (this.currentId() !== INBOX) {
      this.currentId.set(INBOX);
      writePreference('procedures.current', INBOX);
    }
    await this.load();
    await this.open(path);
  }

  async deleteReceived(path: string): Promise<void> {
    await this.api.deleteInbox(path);
    await this.load();
  }

  private docKey(): string {
    return `${this.current()?.id ?? ''}:${this.doc()?.path ?? ''}`;
  }

  private save(list: SavedSource[]): void {
    this.sources.set(list);
    writePreference('procedures.sources', JSON.stringify(list));
  }

  addSource(s: Omit<SavedSource, 'id'>): void {
    const id = crypto.randomUUID();
    this.save([...this.sources(), { ...s, id }]);
    void this.select(id);
  }

  updateSource(s: SavedSource): void {
    this.save(this.sources().map((x) => (x.id === s.id ? s : x)));
    void this.select(s.id);
  }

  removeSource(id: string): void {
    this.save(this.sources().filter((s) => s.id !== id));
    if (this.currentId() === id) {
      void this.select(this.all()[0]?.id ?? '');
    }
  }

  /** Links of an environment whose source still exists. */
  linksOf(envId: string): ProcedureLink[] {
    const ids = new Set(this.all().map((s) => s.id));
    return (this.links()[envId] ?? []).filter((l) => ids.has(l.sourceId));
  }

  isLinked(envId: string, path: string): boolean {
    const src = this.current()?.id;
    return (this.links()[envId] ?? []).some((l) => l.sourceId === src && l.path === path);
  }

  /** Links the open procedure to an environment, or unlinks it. */
  toggleLink(envId: string): void {
    const src = this.current();
    const doc = this.doc();
    if (!src || !doc) {
      return;
    }
    const list = this.links()[envId] ?? [];
    const next = this.isLinked(envId, doc.path)
      ? list.filter((l) => !(l.sourceId === src.id && l.path === doc.path))
      : [...list, { sourceId: src.id, path: doc.path, title: doc.title }];
    const all = { ...this.links(), [envId]: next };
    if (next.length === 0) {
      delete all[envId];
    }
    this.links.set(all);
    writePreference('procedures.envLinks', JSON.stringify(all));
  }

  /** Opens a linked procedure, in its source. */
  async openLink(link: ProcedureLink): Promise<void> {
    if (this.current()?.id !== link.sourceId) {
      await this.select(link.sourceId);
    }
    await this.open(link.path);
  }

  async select(id: string): Promise<void> {
    this.currentId.set(id);
    writePreference('procedures.current', id);
    this.doc.set(null);
    await this.load();
  }

  /** Lists the current source (cloning a repository the first time). */
  async load(sync = false): Promise<void> {
    const src = this.current();
    this.docs.set([]);
    this.synced.set('');
    this.error.set(null);
    if (!src) {
      return;
    }
    this.loading.set(true);
    try {
      const list = await (sync ? this.api.sync(src) : this.api.list(src));
      if (this.current()?.id === src.id) {
        this.docs.set(list.docs ?? []);
        this.synced.set(list.synced);
      }
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.loading.set(false);
    }
  }

  async open(path: string): Promise<void> {
    const src = this.current();
    if (!src) {
      return;
    }
    this.error.set(null);
    try {
      const markdown = await this.api.read(src, path);
      const title = this.docs().find((d) => d.path === path)?.title ?? path;
      this.doc.set({ path, title, markdown });
    } catch (err) {
      this.error.set(toTranslatableError(err));
    }
  }

  close(): void {
    this.doc.set(null);
  }

  markDone(index: number, on = true): void {
    const key = this.docKey();
    const set = new Set(this.doneByDoc()[key] ?? []);
    if (on) {
      set.add(index);
    } else {
      set.delete(index);
    }
    this.doneByDoc.update((m) => ({ ...m, [key]: [...set] }));
  }

  resetDone(): void {
    const key = this.docKey();
    this.doneByDoc.update((m) => ({ ...m, [key]: [] }));
  }

  setValues(values: Record<string, string>): void {
    const id = this.current()?.id ?? '';
    this.valuesBySource.update((m) => ({ ...m, [id]: { ...(m[id] ?? {}), ...values } }));
  }
}

/** Id of the source of received pages. */
export const INBOX = 'inbox';

function parseLinks(raw: string | null): Record<string, ProcedureLink[]> {
  try {
    const v = JSON.parse(raw ?? '{}');
    return v && typeof v === 'object' && !Array.isArray(v) ? v : {};
  } catch {
    return {};
  }
}

function parse(raw: string | null): SavedSource[] {
  try {
    const list = JSON.parse(raw ?? '[]');
    return Array.isArray(list) ? list : [];
  } catch {
    return [];
  }
}

/** Ids of the sources shared by a team. */
const TEAM_PREFIX = 'team:';
