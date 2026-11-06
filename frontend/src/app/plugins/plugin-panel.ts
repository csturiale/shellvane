import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideRefreshCw } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { Action, Panel, PluginApi, PluginInfo } from '../core/backend/plugin-api';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { Modal } from '../shared/modal';
import { SessionsStore } from '../sessions/sessions.store';
import { Navigation } from '../layout/navigation';

/** An action waiting for confirmation. */
interface Pending {
  action: Action;
  item: string;
}

/**
 * The panels of plugins (Docker, systemd, logs…) for a connected host:
 * tables with actions per row. Actions that need a terminal open a new
 * pane on the host with the command typed.
 */
@Component({
  selector: 'app-plugin-panel',
  imports: [TranslocoDirective, FormsModule, Modal, ConfirmDialog, LucideRefreshCw],
  templateUrl: './plugin-panel.html',
  styleUrl: './plugin-panel.scss',
})
export class PluginPanel {
  readonly hostId = input.required<string>();
  /** The host's name, for the title and the new panes. */
  readonly hostName = input.required<string>();
  readonly closed = output<void>();

  private readonly api = inject(PluginApi);
  private readonly sessions = inject(SessionsStore);
  private readonly transloco = inject(TranslocoService);
  private readonly nav = inject(Navigation);
  /** The list of installed panels has been read. */
  protected readonly ready = signal(false);

  protected readonly plugins = signal<PluginInfo[]>([]);
  protected readonly selected = signal<string>('');
  protected readonly panel = signal<Panel | null>(null);
  protected readonly actions = signal<Action[]>([]);
  protected readonly loading = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly output = signal<string | null>(null);
  protected readonly pending = signal<Pending | null>(null);
  protected filter = '';
  protected readonly filterText = signal('');

  protected readonly plugin = computed(() => this.plugins().find((p) => p.id === this.selected()) ?? null);

  constructor() {
    void this.init();
  }

  private async init(): Promise<void> {
    try {
      const list = await this.api.panelPlugins();
      this.plugins.set(list);
      this.ready.set(true);
      if (list.length > 0) {
        await this.select(list[0].id);
      }
    } catch (err) {
      this.fail(err);
    }
  }

  protected openStore(): void {
    this.nav.view.set('plugins');
    this.closed.emit();
  }

  protected async select(id: string): Promise<void> {
    this.selected.set(id);
    this.filter = '';
    this.filterText.set('');
    await this.refresh();
  }

  protected async refresh(): Promise<void> {
    const id = this.selected();
    this.error.set(null);
    this.output.set(null);
    this.loading.set(true);
    try {
      const plugin = this.plugin();
      const [panel, actions] = await Promise.all([
        plugin?.capabilities?.includes('panel') ? this.api.panel(id, this.hostId()) : Promise.resolve(null),
        this.api.hostActions(id, this.hostId()),
      ]);
      if (this.selected() === id) {
        this.panel.set(panel);
        this.actions.set(actions);
      }
    } catch (err) {
      this.panel.set(null);
      this.fail(err);
    } finally {
      this.loading.set(false);
    }
  }

  /** Built-in plugins name their texts with translation keys. */
  protected text(value: string): string {
    return this.plugin()?.builtIn && value.startsWith('plugins.') ? this.transloco.translate(value) : value;
  }

  protected rows(rows: (string[] | null)[] | null | undefined): string[][] {
    const words = this.filterText().toLowerCase().split(/\s+/).filter(Boolean);
    return (rows ?? []).map((r) => r ?? []).filter((r) => words.every((w) => r.join(' ').toLowerCase().includes(w)));
  }

  protected act(action: Action, item = ''): void {
    if (action.confirm) {
      this.pending.set({ action, item });
    } else {
      void this.run(action, item);
    }
  }

  protected confirmed(): void {
    const p = this.pending();
    this.pending.set(null);
    if (p) {
      void this.run(p.action, p.item);
    }
  }

  private async run(action: Action, item: string): Promise<void> {
    this.error.set(null);
    this.output.set(null);
    try {
      const res = await this.api.run(this.selected(), this.hostId(), action.id, item);
      if (res.command) {
        // Typed in a new pane on the host, where it can ask for input.
        await this.sessions.openHost(this.hostId(), this.hostName(), { script: [res.command] });
        this.closed.emit();
        return;
      }
      this.output.set(res.output || this.transloco.translate('plugins.panel.done'));
      await this.refresh();
      if (res.output) {
        this.output.set(res.output);
      }
    } catch (err) {
      this.fail(err);
    }
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.error.set(this.transloco.translate(e.key, { ...e.params, detail: e.params['detail'] ?? e.detail ?? '' }));
  }
}
