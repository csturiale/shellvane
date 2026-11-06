import { Component, effect, inject, signal, untracked } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideCopy, LucideDownload, LucidePencil, LucidePlay, LucidePlus, LucideSquare, LucideTrash2, LucideUpload } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { Cell, Runbook, RunbookPreview } from '../core/backend/runbook-api';
import { writeClipboard } from '../core/clipboard';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { Modal } from '../shared/modal';
import { RunbookDialog } from './runbook-dialog';
import { RunView, RunbooksStore, summary } from './runbooks.store';

/** Runbooks: run steps on several hosts and see the results per host. */
@Component({
  selector: 'app-runbooks-view',
  imports: [
    FormsModule,
    TranslocoDirective,
    Modal,
    ConfirmDialog,
    RunbookDialog,
    LucidePlus,
    LucidePlay,
    LucidePencil,
    LucideTrash2,
    LucideCopy,
    LucideUpload,
    LucideDownload,
    LucideSquare,
  ],
  templateUrl: './runbooks-view.html',
  styleUrl: './runbooks-view.scss',
})
export class RunbooksView {
  protected readonly store = inject(RunbooksStore);
  private readonly transloco = inject(TranslocoService);

  protected readonly editing = signal<Runbook | 'new' | null>(null);
  protected readonly deleting = signal<Runbook | null>(null);
  protected readonly starting = signal<Runbook | null>(null);
  protected readonly preview = signal<RunbookPreview | null>(null);
  protected readonly importing = signal(false);
  protected readonly selectedRun = signal<string | null>(null);
  protected readonly output = signal<{ host: string; step: string; cell: Cell } | null>(null);
  protected readonly message = signal<string | null>(null);
  protected readonly summary = summary;
  protected values: Record<string, string> = {};
  protected importText = '';

  constructor() {
    void this.store.load().catch((err) => this.fail(err));
    // A runbook prepared elsewhere (a snippet "run on hosts") opens here.
    effect(() => {
      const draft = this.store.draft();
      if (draft) {
        untracked(() => {
          this.store.draft.set(null);
          this.editing.set(draft);
        });
      }
    });
  }

  protected run(): RunView | null {
    const id = this.selectedRun();
    return this.store.runs().find((r) => r.id === id) ?? this.store.runs()[0] ?? null;
  }

  protected askStart(rb: Runbook): void {
    this.values = Object.fromEntries((rb.variables ?? []).map((v) => [v.name, v.default ?? '']));
    this.preview.set(null);
    this.starting.set(rb);
  }

  protected async showPreview(): Promise<void> {
    const rb = this.starting();
    if (rb) {
      try {
        this.preview.set(await this.store.preview(rb, this.values));
      } catch (err) {
        this.fail(err);
      }
    }
  }

  protected async start(): Promise<void> {
    const rb = this.starting();
    this.starting.set(null);
    if (!rb) {
      return;
    }
    try {
      const view = await this.store.start(rb, { ...this.values });
      this.selectedRun.set(view.id);
    } catch (err) {
      this.fail(err);
    }
  }

  protected stepName(rb: Runbook, i: number): string {
    return rb.steps[i]?.name || String(i + 1);
  }

  protected waitingAt(run: RunView, step: number): boolean {
    return Object.values(run.cells).some((cells) => cells[step]?.state === 'waiting');
  }

  protected confirm(run: RunView, hostId: string, step: number, ok: boolean): void {
    void this.store.confirm(run.id, hostId, step, ok).catch((err) => this.fail(err));
  }

  protected async exportResults(run: RunView, format: 'csv' | 'json'): Promise<void> {
    try {
      const text = await this.store.exportResults(run.id, format);
      const url = URL.createObjectURL(new Blob([text], { type: format === 'csv' ? 'text/csv' : 'application/json' }));
      const a = document.createElement('a');
      a.href = url;
      a.download = `${run.runbook.name || 'runbook'}-results.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      this.fail(err);
    }
  }

  protected async copyYAML(rb: Runbook): Promise<void> {
    try {
      await writeClipboard(await this.store.exportYAML(rb.id));
      this.message.set(this.transloco.translate('runbooks.copied', { name: rb.name }));
    } catch (err) {
      this.fail(err);
    }
  }

  protected async doImport(): Promise<void> {
    try {
      await this.store.importYAML(this.importText);
      this.importing.set(false);
      this.importText = '';
    } catch (err) {
      this.fail(err);
    }
  }

  protected async confirmDelete(): Promise<void> {
    const rb = this.deleting();
    this.deleting.set(null);
    if (rb) {
      await this.store.delete(rb.id).catch((err) => this.fail(err));
    }
  }

  protected errorText(code: string): string {
    const key = 'errors.' + code;
    const text = this.transloco.translate(key);
    return text === key ? code : text;
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.message.set(this.transloco.translate(e.key, e.params));
  }
}
