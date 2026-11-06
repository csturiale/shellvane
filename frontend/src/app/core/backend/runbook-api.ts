import { Injectable } from '@angular/core';
import { Events } from '@wailsio/runtime';

import { RunbookService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { RunbookCell, RunbookDone } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type * as gen from '../../../../bindings/github.com/csturiale/shellvane/core/runbooks';
import type { Cell, Host as RunbookHost, Step } from '../../../../bindings/github.com/csturiale/shellvane/core/runbooks';
import { RunbookCell as CellEvent, RunbookDone as DoneEvent } from '../../../../bindings/events';

export type { Cell, RunbookCell, RunbookHost, Step };

/** Generated types allow null arrays (Go nil slices); these do not. */
export interface Targets {
  hostIds: string[];
  groupIds: string[];
  tags: string[];
}
export interface Variable extends Omit<gen.Variable, 'choices'> {
  choices: string[];
}
export interface Runbook extends Omit<gen.Runbook, 'variables' | 'steps' | 'targets'> {
  variables: Variable[];
  steps: Step[];
  targets: Targets;
}
export interface RunbookPreview {
  hosts: RunbookHost[];
  commands: Record<string, string[]>;
}
export interface RunbookRun {
  id: string;
  runbook: Runbook;
  hosts: RunbookHost[];
}

export function normalize(rb: gen.Runbook): Runbook {
  return {
    ...rb,
    variables: (rb.variables ?? []).map((v) => ({ ...v, choices: v.choices ?? [] })),
    steps: rb.steps ?? [],
    targets: { hostIds: rb.targets?.hostIds ?? [], groupIds: rb.targets?.groupIds ?? [], tags: rb.targets?.tags ?? [] },
  };
}

/** Runbooks. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class RunbookApi {
  async list(): Promise<Runbook[]> {
    return ((await RunbookService.List()) ?? []).map(normalize);
  }
  async save(rb: Runbook): Promise<Runbook> {
    return normalize(await RunbookService.Save(rb));
  }
  delete(id: string): Promise<void> {
    return RunbookService.Delete(id);
  }
  async preview(rb: Runbook, values: Record<string, string>): Promise<RunbookPreview> {
    const p = await RunbookService.Preview(rb, values);
    const commands: Record<string, string[]> = {};
    for (const [k, v] of Object.entries(p.commands ?? {})) {
      commands[k] = v ?? [];
    }
    return { hosts: p.hosts ?? [], commands };
  }
  async start(rb: Runbook, values: Record<string, string>): Promise<RunbookRun> {
    const r = await RunbookService.Start(rb, values);
    return { id: r.id, runbook: normalize(r.runbook), hosts: r.hosts ?? [] };
  }
  confirm(runId: string, hostId: string, step: number, ok: boolean): Promise<void> {
    return RunbookService.Confirm(runId, hostId, step, ok);
  }
  cancel(runId: string): Promise<void> {
    return RunbookService.Cancel(runId);
  }
  exportResults(runId: string, format: 'csv' | 'json'): Promise<string> {
    return RunbookService.ExportResults(runId, format);
  }
  exportYAML(id: string): Promise<string> {
    return RunbookService.ExportYAML(id);
  }
  async importYAML(text: string): Promise<Runbook> {
    return normalize(await RunbookService.ImportYAML(text));
  }
  onCell(handler: (c: RunbookCell) => void): () => void {
    return Events.On(CellEvent, (ev) => handler(ev.data));
  }
  onDone(handler: (d: RunbookDone) => void): () => void {
    return Events.On(DoneEvent, (ev) => handler(ev.data));
  }
}
