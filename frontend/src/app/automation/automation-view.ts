import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, computed, inject, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideDownload, LucidePencil, LucidePlus, LucideTrash2 } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { Rule } from '../core/backend/automation-api';
import { HostsStore } from '../hosts/hosts.store';
import { groupPaths } from '../hosts/tree';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { HIGHLIGHT_COLORS } from '../terminal/highlighter';
import { AutomationStore } from './automation.store';
import { RuleDialog } from './rule-dialog';
import { HelpLink } from '../guide/help-link';
import { RunbooksStore } from '../runbooks/runbooks.store';
import { RunbooksView } from '../runbooks/runbooks-view';

type Tab = 'rules' | 'runbooks' | 'audit';

/** Output rules (highlight, notify, respond) and the audit log. */
@Component({
  selector: 'app-automation-view',
  imports: [NgComponentOutlet, HelpLink, RunbooksView, TranslocoDirective, RuleDialog, ConfirmDialog, LucidePlus, LucidePencil, LucideTrash2, LucideDownload],
  templateUrl: './automation-view.html',
  styleUrl: './automation-view.scss',
})
export class AutomationView {
  protected readonly store = inject(AutomationStore);
  protected readonly sharing = inject(Sharing);
  private readonly hosts = inject(HostsStore);
  private readonly transloco = inject(TranslocoService);
  protected readonly colors = HIGHLIGHT_COLORS;

  protected readonly tab = signal<Tab>(inject(RunbooksStore).draft() ? 'runbooks' : 'rules');
  protected readonly editing = signal<Rule | 'new' | null>(null);
  protected readonly deleting = signal<Rule | null>(null);
  protected readonly clearing = signal(false);
  protected readonly message = signal<string | null>(null);

  private readonly groupNames = computed(() => new Map(groupPaths(this.hosts.groups()).map((g) => [g.id, g.path])));
  private readonly hostNames = computed(() => new Map(this.hosts.hosts().map((h) => [h.id, h.name])));

  constructor() {
    void this.store.load().catch((err) => this.fail(err));
    if (!this.hosts.loaded()) {
      void this.hosts.load().catch(() => undefined);
    }
  }

  protected show(tab: Tab): void {
    this.tab.set(tab);
    if (tab === 'audit') {
      void this.store.loadAudit().catch((err) => this.fail(err));
    }
  }

  protected scopeLabel(r: Rule): string {
    switch (r.scopeKind) {
      case 'group':
        return this.groupNames().get(r.scopeId) ?? '?';
      case 'host':
        return this.hostNames().get(r.scopeId) ?? '?';
      default:
        return this.transloco.translate('automation.scope.global');
    }
  }

  protected async toggle(r: Rule): Promise<void> {
    await this.store.toggle(r).catch((err) => this.fail(err));
  }

  protected async confirmDelete(): Promise<void> {
    const r = this.deleting();
    this.deleting.set(null);
    if (r) {
      await this.store.delete(r.id).catch((err) => this.fail(err));
    }
  }

  protected async confirmClear(): Promise<void> {
    this.clearing.set(false);
    await this.store.clearAudit().catch((err) => this.fail(err));
  }

  /** Saves the export through the browser's download. */
  protected async export(format: 'csv' | 'json'): Promise<void> {
    try {
      const text = await this.store.exportAudit(format);
      const url = URL.createObjectURL(new Blob([text], { type: format === 'csv' ? 'text/csv' : 'application/json' }));
      const a = document.createElement('a');
      a.href = url;
      a.download = `shellvane-audit.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      this.fail(err);
    }
  }

  protected formatTime(at: string): string {
    return new Intl.DateTimeFormat(this.transloco.getActiveLang(), { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(at));
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.message.set(this.transloco.translate(e.key, e.params));
  }
}
