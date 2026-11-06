import { Injectable, inject, signal } from '@angular/core';

import { Cell, Runbook, RunbookApi, RunbookCell, RunbookHost } from '../core/backend/runbook-api';

/** A run shown in the results grid. */
export interface RunView {
  id: string;
  runbook: Runbook;
  hosts: RunbookHost[];
  /** Host ID → one cell per step. */
  cells: Record<string, Cell[]>;
  done: boolean;
  startedAt: string;
}

/** An empty runbook, for the editor. */
export function newRunbook(): Runbook {
  return {
    id: '',
    name: '',
    description: '',
    variables: [],
    steps: [{ name: '', command: '', when: '', pattern: '', continueOnError: false, confirm: false, timeoutSec: 0 }],
    targets: { hostIds: [], groupIds: [], tags: [] },
    parallel: true,
    concurrency: 0,
    stopOnHostFailure: false,
  };
}

/** Per-state counts of a run, for its summary. */
export function summary(run: RunView): Record<string, number> {
  const out: Record<string, number> = {};
  for (const cells of Object.values(run.cells)) {
    for (const c of cells) {
      out[c.state] = (out[c.state] ?? 0) + 1;
    }
  }
  return out;
}

/** Applies a cell event to a run. */
export function applyCell(r: RunView, c: RunbookCell): RunView {
  const row = [...(r.cells[c.hostId] ?? [])];
  row[c.step] = c.cell;
  return { ...r, cells: { ...r.cells, [c.hostId]: row } };
}

@Injectable({ providedIn: 'root' })
export class RunbooksStore {
  private readonly api = inject(RunbookApi);
  readonly list = signal<Runbook[]>([]);
  readonly runs = signal<RunView[]>([]);
  /** A runbook to open in the editor (from a snippet). */
  readonly draft = signal<Runbook | null>(null);

  /** Events of runs not yet in the list (they can arrive before start returns). */
  private early = new Map<string, { cells: RunbookCell[]; done: boolean }>();

  constructor() {
    try {
      this.api.onCell((c) => {
        if (!this.runs().some((r) => r.id === c.runId)) {
          this.earlyOf(c.runId).cells.push(c);
          return;
        }
        this.runs.update((list) => list.map((r) => (r.id === c.runId ? applyCell(r, c) : r)));
      });
      this.api.onDone((d) => {
        if (!this.runs().some((r) => r.id === d.runId)) {
          this.earlyOf(d.runId).done = true;
          return;
        }
        this.runs.update((list) => list.map((r) => (r.id === d.runId ? { ...r, done: true } : r)));
      });
    } catch {
      // No event bus outside the desktop app.
    }
  }

  private earlyOf(runId: string) {
    let e = this.early.get(runId);
    if (!e) {
      e = { cells: [], done: false };
      this.early.set(runId, e);
    }
    return e;
  }

  async load(): Promise<void> {
    this.list.set(await this.api.list());
  }

  async save(rb: Runbook): Promise<Runbook> {
    const saved = await this.api.save(rb);
    await this.load();
    return saved;
  }

  async delete(id: string): Promise<void> {
    await this.api.delete(id);
    await this.load();
  }

  preview(rb: Runbook, values: Record<string, string>) {
    return this.api.preview(rb, values);
  }

  /** Starts a run; it appears first in the list of runs. */
  async start(rb: Runbook, values: Record<string, string>): Promise<RunView> {
    const r = await this.api.start(rb, values);
    const cells: Record<string, Cell[]> = {};
    for (const h of r.hosts) {
      cells[h.id] = rb.steps.map(() => ({ state: 'pending', command: '' }));
    }
    let view: RunView = { id: r.id, runbook: rb, hosts: r.hosts, cells, done: false, startedAt: new Date().toISOString() };
    const early = this.early.get(r.id);
    this.early.delete(r.id);
    for (const c of early?.cells ?? []) {
      view = applyCell(view, c);
    }
    view.done = early?.done ?? false;
    this.runs.update((list) => [view, ...list]);
    return view;
  }

  confirm(runId: string, hostId: string, step: number, ok: boolean): Promise<void> {
    return this.api.confirm(runId, hostId, step, ok);
  }

  cancel(runId: string): Promise<void> {
    return this.api.cancel(runId);
  }

  forget(runId: string): void {
    this.runs.update((list) => list.filter((r) => r.id !== runId));
  }

  exportResults(runId: string, format: 'csv' | 'json'): Promise<string> {
    return this.api.exportResults(runId, format);
  }

  exportYAML(id: string): Promise<string> {
    return this.api.exportYAML(id);
  }

  async importYAML(text: string): Promise<void> {
    await this.api.importYAML(text);
    await this.load();
  }
}
