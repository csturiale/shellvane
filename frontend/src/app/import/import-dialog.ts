import { Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { ImportApi, ImportPreview, ImportSource, ImportSummary, ImportedHost } from '../core/backend/import-api';
import { HostsStore } from '../hosts/hosts.store';
import { TunnelsStore } from '../tunnels/tunnels.store';
import { Modal } from '../shared/modal';
import { ImportRow, importRows, withDefaultUser } from './import-tree';
import { UsageStore } from '../core/usage/usage.store';
import { VaultGate } from '../vault/vault-gate';

type Step = 'source' | 'preview' | 'done';

/** Import wizard: source → preview → import → summary (plan §8). */
@Component({
  selector: 'app-import-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './import-dialog.html',
  styleUrls: ['../shared/form.scss', './import-dialog.scss'],
})
export class ImportDialog {
  private readonly vaultGate = inject(VaultGate);
  private readonly usage = inject(UsageStore);
  readonly closed = output<void>();

  private readonly api = inject(ImportApi);
  private readonly hosts = inject(HostsStore);
  private readonly tunnels = inject(TunnelsStore);
  private readonly transloco = inject(TranslocoService);

  protected readonly step = signal<Step>('source');
  protected readonly sources = signal<ImportSource[]>([]);
  protected readonly selected = signal<number>(0);
  protected path = '';
  protected group = '';
  /** Put everything in a new top-level group (named group) or not. */
  protected useGroup = true;
  /** User for imported hosts that have none. */
  protected defaultUser = '';
  /** Groups folded in the preview, by key. */
  protected readonly collapsed = signal<ReadonlySet<string>>(new Set());
  protected readonly preview = signal<ImportPreview | null>(null);
  protected readonly chosen = signal<boolean[]>([]);
  protected readonly summary = signal<ImportSummary | null>(null);
  protected readonly error = signal<TranslatableError | null>(null);
  protected readonly busy = signal(false);
  protected readonly chosenCount = computed(() => this.chosen().filter(Boolean).length);
  protected readonly rows = computed<ImportRow[]>(() => importRows(this.preview()?.hosts ?? [], this.collapsed()));
  /** Chosen hosts without a user: the default user applies to them. */
  protected readonly withoutUser = computed(() => (this.preview()?.hosts ?? []).filter((h, i) => this.chosen()[i] && !h.user).length);

  async ngOnInit(): Promise<void> {
    try {
      const list = await this.api.sources();
      this.sources.set(list);
      const found = list.findIndex((s) => s.found);
      this.select(found >= 0 ? found : 0);
    } catch (err) {
      this.error.set(toTranslatableError(err));
    }
  }

  protected source(): ImportSource | null {
    return this.sources()[this.selected()] ?? null;
  }

  protected select(i: number): void {
    this.selected.set(i);
    const s = this.sources()[i];
    this.path = s?.path ?? '';
    this.group = s ? this.transloco.translate('import.defaultGroup', { tool: this.transloco.translate('import.tools.' + s.id) }) : '';
  }

  protected async read(): Promise<void> {
    const s = this.source();
    if (!s) {
      return;
    }
    await this.run(async () => {
      const res = await this.api.preview(s.id, s.kind, this.path);
      this.preview.set(res);
      this.chosen.set((res.hosts ?? []).map(() => true));
      this.collapsed.set(new Set());
      this.defaultUser = '';
      this.step.set('preview');
    });
  }

  protected toggle(i: number): void {
    this.chosen.update((list) => list.map((v, j) => (j === i ? !v : v)));
  }

  /** "all", "some" or "none" of a group's hosts are chosen. */
  protected groupState(indexes: number[]): 'all' | 'some' | 'none' {
    const n = indexes.filter((i) => this.chosen()[i]).length;
    return n === 0 ? 'none' : n === indexes.length ? 'all' : 'some';
  }

  /** Chooses, or leaves out, every host of a group and its subgroups. */
  protected toggleGroup(indexes: number[]): void {
    const on = this.groupState(indexes) !== 'all';
    const set = new Set(indexes);
    this.chosen.update((list) => list.map((v, i) => (set.has(i) ? on : v)));
  }

  protected fold(key: string): void {
    this.collapsed.update((s) => {
      const next = new Set(s);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }

  protected toggleAll(on: boolean): void {
    this.chosen.update((list) => list.map(() => on));
  }

  protected async import(): Promise<void> {
    this.usage.count('import');
    const hosts = withDefaultUser(
      (this.preview()?.hosts ?? []).filter((_, i) => this.chosen()[i]),
      this.defaultUser,
    );
    await this.run(async () => {
      this.summary.set(await this.api.import(hosts, this.useGroup ? this.group.trim() : ''));
      this.step.set('done');
      await Promise.all([this.hosts.load(), this.tunnels.load()]);
    });
  }

  protected address(h: ImportedHost): string {
    const user = h.user || this.defaultUser.trim();
    return `${user ? user + '@' : ''}${h.hostname}${h.port ? ':' + h.port : ''}`;
  }

  private async run(action: () => Promise<void>): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    try {
      await this.vaultGate.run(action);
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.busy.set(false);
    }
  }
}
