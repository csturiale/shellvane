import { Injectable, computed, inject, signal } from '@angular/core';

import { Doc, ProcedureApi, Source } from '../core/backend/procedure-api';
import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { readPreference, writePreference } from '../core/preferences';

/** A source of procedures as saved: a Git repository or a folder. */
export interface SavedSource extends Source {
  id: string;
  name: string;
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
  /** Every source: the user's, then the received pages. */
  readonly all = computed(() => [...this.sources(), ...(this.inbox() ? [this.inbox()!] : [])]);
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

function parse(raw: string | null): SavedSource[] {
  try {
    const list = JSON.parse(raw ?? '[]');
    return Array.isArray(list) ? list : [];
  } catch {
    return [];
  }
}
