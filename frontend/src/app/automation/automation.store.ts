import { Injectable, inject, signal } from '@angular/core';

import { AuditEntry, AutomationApi, Rule } from '../core/backend/automation-api';

/** How many audit entries the view shows; export includes all of them. */
const AUDIT_PAGE = 500;

@Injectable({ providedIn: 'root' })
export class AutomationStore {
  private readonly api = inject(AutomationApi);
  readonly rules = signal<Rule[]>([]);
  readonly audit = signal<AuditEntry[]>([]);

  async load(): Promise<void> {
    this.rules.set(await this.api.rules());
  }

  async loadAudit(): Promise<void> {
    this.audit.set(await this.api.audit(AUDIT_PAGE));
  }

  async save(rule: Rule): Promise<void> {
    await this.api.saveRule(rule);
    await this.load();
  }

  async toggle(rule: Rule): Promise<void> {
    await this.save({ ...rule, enabled: !rule.enabled });
  }

  async delete(id: string): Promise<void> {
    await this.api.deleteRule(id);
    await this.load();
  }

  exportAudit(format: 'csv' | 'json'): Promise<string> {
    return this.api.exportAudit(format);
  }

  async clearAudit(): Promise<void> {
    await this.api.clearAudit();
    await this.loadAudit();
  }
}
