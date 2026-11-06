import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { Rule } from '../core/backend/automation-api';
import { HostsStore } from '../hosts/hosts.store';
import { groupPaths } from '../hosts/tree';
import { Modal } from '../shared/modal';
import { HIGHLIGHT_COLORS, compileRules, findSpans } from '../terminal/highlighter';
import { VaultStore } from '../vault/vault.store';
import { AutomationStore } from './automation.store';
import { HelpLink } from '../guide/help-link';

/** Creates or edits an output rule, with a live test of the pattern. */
@Component({
  selector: 'app-rule-dialog',
  imports: [NgComponentOutlet, HelpLink, FormsModule, TranslocoDirective, Modal],
  templateUrl: './rule-dialog.html',
  styleUrls: ['../shared/form.scss', './rule-dialog.scss'],
})
export class RuleDialog {
  protected readonly sharing = inject(Sharing);
  /** null creates a rule. */
  readonly rule = input<Rule | null>(null);
  readonly closed = output<void>();

  private readonly store = inject(AutomationStore);
  private readonly hosts = inject(HostsStore);
  protected readonly vault = inject(VaultStore);
  protected readonly colorNames = Object.keys(HIGHLIGHT_COLORS);
  protected readonly colors = HIGHLIGHT_COLORS;
  protected readonly groups = computed(() => groupPaths(this.hosts.groups()));
  protected readonly hostList = computed(() =>
    [...this.hosts.hosts()].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
  );

  protected model: Rule = {
    id: '',
    name: '',
    pattern: '',
    ignoreCase: false,
    action: 'highlight',
    color: 'yellow',
    response: '',
    secretScope: '',
    enter: true,
    sound: false,
    scopeKind: 'global',
    scopeId: '',
    enabled: true,
  };
  protected sample = '';
  protected readonly saving = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);

  ngOnInit(): void {
    const r = this.rule();
    if (r) {
      this.model = { ...r };
    }
  }

  /** "match", "noMatch", "invalid" (for JavaScript) or "" before typing. */
  protected test(): string {
    if (!this.sample || !this.model.pattern) {
      return '';
    }
    const rules = compileRules([{ pattern: this.model.pattern, ignoreCase: this.model.ignoreCase, color: 'yellow' }]);
    if (rules.length === 0) {
      return 'invalid';
    }
    return findSpans(this.sample, rules).length ? 'match' : 'noMatch';
  }

  /** Responders cannot be global: pick a sensible scope when switching. */
  protected onAction(): void {
    if (this.model.action === 'respond' && this.model.scopeKind === 'global') {
      this.model.scopeKind = 'host';
      this.onScope();
    }
    if (this.model.action === 'notify' && 'Notification' in globalThis && Notification.permission === 'default') {
      void Notification.requestPermission().catch(() => undefined);
    }
  }

  protected onScope(): void {
    if (this.model.scopeKind === 'group') {
      this.model.scopeId = this.groups()[0]?.id ?? '';
    } else if (this.model.scopeKind === 'host') {
      this.model.scopeId = this.hostList()[0]?.id ?? '';
    } else {
      this.model.scopeId = '';
    }
  }

  protected async save(): Promise<void> {
    this.saving.set(true);
    this.error.set(null);
    try {
      await this.store.save(this.model);
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.saving.set(false);
    }
  }
}
