import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideCopy, LucideLayers, LucidePencil, LucidePlay, LucidePlus, LucideSquare, LucideSquareTerminal, LucideTrash2, LucideUpload, LucideUserRound } from '@lucide/angular';

import { Environment } from '../core/backend/environment-api';
import { toTranslatableError } from '../core/backend/errors';
import { writeClipboard } from '../core/clipboard';
import { HelpLink } from '../guide/help-link';
import { HostsStore } from '../hosts/hosts.store';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { Modal } from '../shared/modal';
import { RememberSelect } from '../shared/remember-select';
import { Remember } from '../core/personal-values';
import { EnvironmentDialog } from './environment-dialog';
import { EnvironmentsStore, StepId } from './environments.store';

/** Environments: start everything a system needs in one click. */
@Component({
  selector: 'app-environments-panel',
  imports: [NgComponentOutlet, FormsModule, TranslocoDirective, Modal, ConfirmDialog, EnvironmentDialog, HelpLink, LucidePlus, LucidePlay, LucideSquare, LucideSquareTerminal, LucidePencil, LucideTrash2, LucideCopy, LucideUpload, LucideLayers, LucideUserRound, RememberSelect],
  templateUrl: './environments-panel.html',
  styleUrls: ['../shared/form.scss', './environments-panel.scss'],
})
export class EnvironmentsPanel {
  protected readonly store = inject(EnvironmentsStore);
  /** Ticks every second while a step runs, for the elapsed time. */
  private readonly now = signal(Date.now());
  private readonly ticker = setInterval(() => {
    if (this.store.runs().some((r) => r.steps.some((s) => s.state === 'running'))) {
      this.now.set(Date.now());
    }
  }, 1000);

  protected elapsed(since: number): number {
    return Math.max(0, Math.round((this.now() - since) / 1000));
  }

  ngOnDestroy(): void {
    clearInterval(this.ticker);
  }
  private readonly hosts = inject(HostsStore);
  protected readonly transloco = inject(TranslocoService);
  protected readonly sharing = inject(Sharing);
  protected readonly shareFailed = (key: string) => this.message.set(this.transloco.translate(key));
  protected readonly closeReview = () => this.reviewing.set(null);
  /** The reviewed environment's commands are approved: start it. */
  protected readonly approveReview = () => {
    const env = this.reviewing();
    this.reviewing.set(null);
    if (env) {
      this.start(env);
    }
  };
  /** A shared environment whose commands wait for approval. */
  protected readonly reviewing = signal<Environment | null>(null);

  protected readonly editing = signal<Environment | 'new' | null>(null);
  protected readonly deleting = signal<Environment | null>(null);
  /** Values asked before a start, or only this person's values (mine: nothing starts). */
  protected readonly asking = signal<{ env: Environment; from: StepId; mine?: boolean } | null>(null);
  protected readonly importing = signal(false);
  protected readonly message = signal<string | null>(null);
  protected readonly hostNames = computed(() => new Map(this.hosts.hosts().map((h) => [h.id, h.name])));
  protected values: Record<string, string> = {};
  /** Where each personal value typed in the dialog is remembered. */
  protected where: Record<string, Remember> = {};
  protected importText = '';
  /** Values used by the last start, for "retry from here". */
  private readonly lastValues = new Map<string, Record<string, string>>();

  constructor() {
    void this.store.load().catch((err) => this.fail(err));
    if (!this.hosts.loaded()) {
      void this.hosts.load().catch(() => undefined);
    }
  }

  /** Starts, asking for the variables first when there are any. */
  protected start(env: Environment, from: StepId = 'login'): void {
    if (this.sharing.needsReview('environment', env.id)) {
      this.reviewing.set(env); // its commands changed in the team: approve them first
      return;
    }
    // Personal values already known are not asked again.
    if (from === 'login' && this.store.needsValues(env)) {
      this.askValues(env, from);
      return;
    }
    this.run(env, from === 'login' ? {} : (this.lastValues.get(env.id) ?? {}), from);
  }

  /** This person's values of the personal variables, to see or change. */
  protected editMine(env: Environment): void {
    this.askValues(env, 'login', true);
  }

  protected personal(env: Environment): boolean {
    return (env.variables ?? []).some((v) => v.personal);
  }

  private askValues(env: Environment, from: StepId, mine = false): void {
    const last = { ...(this.lastValues.get(env.id) ?? {}), ...this.store.personalValues(env) };
    this.values = Object.fromEntries((env.variables ?? []).map((v) => [v.name, last[v.name] ?? v.default ?? '']));
    const sources = this.store.personalSources(env);
    this.where = Object.fromEntries(Object.entries(sources).map(([n, src]) => [n, src === 'here' ? 'here' : 'global']));
    this.asking.set({ env, from, mine });
  }

  protected confirmValues(): void {
    const a = this.asking();
    this.asking.set(null);
    if (!a) {
      return;
    }
    this.store.setPersonalValues(a.env, this.values, this.where);
    if (!a.mine) {
      this.run(a.env, { ...this.values }, a.from);
    }
  }

  private run(env: Environment, values: Record<string, string>, from: StepId): void {
    this.lastValues.set(env.id, values);
    this.message.set(null);
    void this.store.start(env, values, from);
  }

  protected async exportEnv(env: Environment): Promise<void> {
    try {
      await writeClipboard(await this.store.export(env.id));
      this.message.set(this.transloco.translate('environments.exported', { name: env.name }));
    } catch (err) {
      this.fail(err);
    }
  }

  protected async doImport(): Promise<void> {
    try {
      await this.store.import(this.importText);
      this.importing.set(false);
      this.importText = '';
    } catch (err) {
      this.fail(err);
    }
  }

  protected async confirmDelete(): Promise<void> {
    const e = this.deleting();
    this.deleting.set(null);
    if (e) {
      await this.store.delete(e.id).catch((err) => this.fail(err));
    }
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.message.set(this.transloco.translate(e.key, e.params));
  }
}
