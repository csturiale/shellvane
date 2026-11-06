import { Component, computed, inject, input, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideSend } from '@lucide/angular';

import { HostsStore } from '../hosts/hosts.store';
import { panesOf } from '../sessions/layout';
import { Pane, SessionsStore, Tab } from '../sessions/sessions.store';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { toTerminalInput } from '../snippets/variables';
import { DangerousCommands } from './dangerous-commands';

/** A way to choose the receiving panes of a tab. */
export interface TargetOption {
  /** "all", "g:<group path>" or "t:<tag>". */
  value: string;
  kind: 'all' | 'group' | 'tag';
  name: string;
  /** Panes it selects. */
  keys: number[];
}

/** Groups and tags of the panes of a tab, each with the panes they select. */
export function targetOptions(panes: Pane[], tagsOf: (hostId: string) => string[]): TargetOption[] {
  const groups = new Map<string, number[]>();
  const tags = new Map<string, number[]>();
  for (const p of panes) {
    if (p.group.length > 0) {
      const path = p.group.join(' / ');
      groups.set(path, [...(groups.get(path) ?? []), p.key]);
    }
    if (p.source.kind === 'host' || p.source.kind === 'mosh') {
      for (const tag of tagsOf(p.source.hostId)) {
        tags.set(tag, [...(tags.get(tag) ?? []), p.key]);
      }
    }
  }
  const sorted = (m: Map<string, number[]>) => [...m.entries()].sort(([a], [b]) => a.localeCompare(b));
  return [
    { value: 'all', kind: 'all', name: '', keys: panes.map((p) => p.key) },
    ...sorted(groups).map(([name, keys]): TargetOption => ({ value: 'g:' + name, kind: 'group', name, keys })),
    ...sorted(tags).map(([name, keys]): TargetOption => ({ value: 't:' + name, kind: 'tag', name, keys })),
  ];
}

/** Input bar of a tab in broadcast mode: one line goes to every receiving pane. */
@Component({
  selector: 'app-broadcast-bar',
  imports: [TranslocoDirective, ConfirmDialog, LucideSend],
  templateUrl: './broadcast-bar.html',
  styleUrl: './broadcast-bar.scss',
})
export class BroadcastBar {
  readonly tab = input.required<Tab>();

  private readonly sessions = inject(SessionsStore);
  private readonly hosts = inject(HostsStore);
  private readonly danger = inject(DangerousCommands);
  private readonly transloco = inject(TranslocoService);

  protected readonly text = signal('');
  protected readonly enter = signal(true);
  protected readonly confirming = signal<string | null>(null);
  protected readonly selected = signal('all');
  private readonly history: string[] = [];
  private historyAt = -1;

  protected readonly options = computed(() => {
    const keys = panesOf(this.tab().layout);
    const panes = this.sessions.panes().filter((p) => keys.includes(p.key));
    const hosts = new Map(this.hosts.hosts().map((h) => [h.id, h.tags ?? []]));
    return targetOptions(panes, (id) => hosts.get(id) ?? []);
  });
  protected readonly count = computed(() => this.sessions.tabReceivers(this.tab().key).length);

  protected select(value: string): void {
    const opt = this.options().find((o) => o.value === value);
    if (!opt) {
      return;
    }
    this.selected.set(value);
    const all = panesOf(this.tab().layout);
    this.sessions.setExcluded(this.tab().key, all.filter((k) => !opt.keys.includes(k)));
  }

  protected submit(): void {
    const line = this.text();
    if (!line.trim() && !this.enter()) {
      return;
    }
    if (this.count() > 1 && this.danger.test(line)) {
      this.confirming.set(line);
      return;
    }
    this.deliver(line);
  }

  protected deliver(line: string): void {
    this.confirming.set(null);
    this.sessions.sendToTab(this.tab().key, toTerminalInput(line, this.enter()));
    if (line.trim() && this.history[0] !== line) {
      this.history.unshift(line);
      this.history.length = Math.min(this.history.length, 100);
    }
    this.historyAt = -1;
    this.text.set('');
  }

  protected interrupt(): void {
    this.sessions.sendToTab(this.tab().key, '\x03');
  }

  protected browse(ev: Event, step: number): void {
    const next = this.historyAt + step;
    if (next < -1 || next >= this.history.length) {
      return;
    }
    ev.preventDefault();
    this.historyAt = next;
    this.text.set(next === -1 ? '' : this.history[next]);
  }

  protected confirmMessage(line: string): string {
    return this.transloco.translate('snippets.dangerous', { count: this.count(), command: line });
  }
}
