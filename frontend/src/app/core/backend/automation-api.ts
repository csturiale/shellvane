import { Injectable } from '@angular/core';
import { Events } from '@wailsio/runtime';

import { AutomationService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { TriggerEvent } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Entry as AuditEntry } from '../../../../bindings/github.com/csturiale/shellvane/core/audit';
import type { Rule } from '../../../../bindings/github.com/csturiale/shellvane/core/automation';
import { SessionTrigger } from '../../../../bindings/events';

export type { AuditEntry, Rule, TriggerEvent };

/** Output rules and the audit log. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class AutomationApi {
  async rules(): Promise<Rule[]> {
    return (await AutomationService.Rules()) ?? [];
  }
  saveRule(rule: Rule): Promise<Rule> {
    return AutomationService.SaveRule(rule);
  }
  deleteRule(id: string): Promise<void> {
    return AutomationService.DeleteRule(id);
  }
  async audit(limit: number): Promise<AuditEntry[]> {
    return (await AutomationService.Audit(limit)) ?? [];
  }
  exportAudit(format: 'csv' | 'json'): Promise<string> {
    return AutomationService.ExportAudit(format);
  }
  clearAudit(): Promise<void> {
    return AutomationService.ClearAudit();
  }
  onTrigger(handler: (e: TriggerEvent) => void): () => void {
    return Events.On(SessionTrigger, (ev) => handler(ev.data));
  }
}
