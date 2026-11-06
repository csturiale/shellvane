import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { Runbook, Step } from '../core/backend/runbook-api';
import { HelpLink } from '../guide/help-link';
import { HostsStore } from '../hosts/hosts.store';
import { groupPaths } from '../hosts/tree';
import { Modal } from '../shared/modal';
import { RunbooksStore, newRunbook } from './runbooks.store';
import { Literals } from '../core/i18n/literals';

interface VariableRow {
  name: string;
  default: string;
  choices: string;
}

/** Creates or edits a runbook: targets, how hosts run, and the steps. */
@Component({
  selector: 'app-runbook-dialog',
  imports: [FormsModule, TranslocoDirective, Modal, HelpLink],
  templateUrl: './runbook-dialog.html',
  styleUrls: ['../shared/form.scss', './runbook-dialog.scss'],
})
export class RunbookDialog {
  /** Literal placeholders shown in the hint. */
  protected readonly hint = new Literals({ example: '{{name}}', output: '{{steps.<step>.output}}' });
  /** null creates one. */
  readonly runbook = input<Runbook | null>(null);
  readonly closed = output<void>();
  /** Emitted with the saved runbook. */
  readonly saved = output<Runbook>();

  private readonly store = inject(RunbooksStore);
  private readonly hosts = inject(HostsStore);
  protected readonly groups = computed(() => groupPaths(this.hosts.groups()));
  protected readonly hostList = computed(() =>
    [...this.hosts.hosts()].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
  );

  protected model: Runbook = newRunbook();
  protected variables: VariableRow[] = [];
  protected tags = '';
  protected readonly saving = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);

  ngOnInit(): void {
    if (!this.hosts.loaded()) {
      void this.hosts.load().catch(() => undefined);
    }
    const rb = this.runbook();
    if (rb) {
      this.model = structuredClone(rb);
      this.model.steps = (this.model.steps ?? []).map((s) => ({ ...s }));
      this.variables = (rb.variables ?? []).map((v) => ({ name: v.name, default: v.default, choices: (v.choices ?? []).join(', ') }));
      this.tags = (rb.targets.tags ?? []).join(', ');
    }
  }

  protected toggle(list: 'hostIds' | 'groupIds', id: string, on: boolean): void {
    const t = this.model.targets;
    const current = t[list] ?? [];
    t[list] = on ? [...current, id] : current.filter((x) => x !== id);
  }

  protected addStep(): void {
    this.model.steps.push({ name: '', command: '', when: '', pattern: '', continueOnError: false, confirm: false, timeoutSec: 0 });
  }

  protected move(i: number, delta: number): void {
    const s = this.model.steps;
    const j = i + delta;
    if (j >= 0 && j < s.length) {
      [s[i], s[j]] = [s[j], s[i]];
    }
  }

  protected trackStep(_: number, s: Step): Step {
    return s;
  }

  protected async save(): Promise<void> {
    this.model.variables = this.variables
      .filter((v) => v.name.trim())
      .map((v) => ({
        name: v.name.trim(),
        default: v.default,
        choices: v.choices
          .split(',')
          .map((c) => c.trim())
          .filter(Boolean),
      }));
    this.model.targets.tags = this.tags.split(/[,\s]+/).filter(Boolean);
    this.model.concurrency = Number(this.model.concurrency) || 0;
    this.model.steps = this.model.steps.map((s) => ({ ...s, timeoutSec: Number(s.timeoutSec) || 0 }));
    this.saving.set(true);
    this.error.set(null);
    try {
      this.saved.emit(await this.store.save(this.model));
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.saving.set(false);
    }
  }
}
