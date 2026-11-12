import { Component, ElementRef, computed, inject, signal, viewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucidePlus, LucidePuzzle, LucideRefreshCw, LucideTrash2 } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { DEFAULT_STORE_URL, PluginApi, PluginConfig, PluginInfo, StoreItem } from '../core/backend/plugin-api';
import { readPreference, writePreference } from '../core/preferences';
import { HelpLink } from '../guide/help-link';
import { Navigation } from '../layout/navigation';
import { HostsStore } from '../hosts/hosts.store';
import { PathPicker } from '../shared/path-picker';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { UsageStore } from '../core/usage/usage.store';
import { VaultGate } from '../vault/vault-gate';

/** What the form edits: a configuration of one plugin. */
interface Draft {
  id: string;
  pluginId: string;
  name: string;
  settings: Record<string, string>;
  secrets: Record<string, string>;
}

/**
 * Plugins: inventories (clouds, Kubernetes, Ansible, Terraform, Proxmox,
 * NetBox) set up once and refreshed into a host group.
 */
@Component({
  selector: 'app-plugins-view',
  imports: [TranslocoDirective, FormsModule, ConfirmDialog, HelpLink, LucidePuzzle, LucidePlus, LucideRefreshCw, LucideTrash2, PathPicker],
  templateUrl: './plugins-view.html',
  styleUrl: './plugins-view.scss',
})
export class PluginsView {
  private readonly vaultGate = inject(VaultGate);
  private readonly usage = inject(UsageStore);
  private readonly api = inject(PluginApi);
  private readonly hosts = inject(HostsStore);
  private readonly transloco = inject(TranslocoService);
  private readonly nav = inject(Navigation);

  private readonly storeHeading = viewChild<ElementRef<HTMLElement>>('storeHeading');
  protected readonly plugins = signal<PluginInfo[]>([]);
  protected readonly configs = signal<PluginConfig[]>([]);
  /** The configuration being edited; null shows the catalog. */
  protected readonly draft = signal<Draft | null>(null);
  protected readonly busy = signal(false);
  protected readonly message = signal<string | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly deleting = signal<PluginConfig | null>(null);

  protected readonly removing = signal<PluginInfo | null>(null);
  protected readonly store = signal<StoreItem[]>([]);
  protected readonly storeLoading = signal(false);
  protected readonly storeError = signal<string | null>(null);
  protected storeUrl = readPreference('plugins.storeUrl') || DEFAULT_STORE_URL;
  protected filePath = '';

  protected readonly builtIns = computed(() => this.plugins().filter((p) => p.builtIn));
  /** External plugins installed, whether from the store or a file. */
  protected readonly externals = computed(() => this.plugins().filter((p) => !p.builtIn && !this.store().some((s) => s.id === p.id)));

  protected readonly plugin = computed(() => this.plugins().find((p) => p.id === this.draft()?.pluginId) ?? null);
  protected readonly saved = computed(() => this.configs().find((c) => c.id === this.draft()?.id) ?? null);

  constructor() {
    void this.load();
    void this.loadStore();
  }

  /** How to get a plugin into the store: the guide's Contribute topic. */
  protected publish(): void {
    this.nav.openGuide('contribute', 'plugins');
  }

  protected async loadStore(): Promise<void> {
    const url = this.storeUrl.trim() || DEFAULT_STORE_URL;
    writePreference('plugins.storeUrl', url === DEFAULT_STORE_URL ? '' : url);
    this.storeError.set(null);
    this.storeLoading.set(true);
    try {
      this.store.set(await this.api.store(url));
    } catch (err) {
      this.store.set([]);
      const e = toTranslatableError(err);
      this.storeError.set(this.transloco.translate('plugins.store.unreachable', { detail: e.params['detail'] ?? e.detail ?? '' }));
    } finally {
      this.storeLoading.set(false);
    }
  }

  private async reload(): Promise<void> {
    this.plugins.set(await this.api.plugins());
    this.configs.set(await this.api.configs());
  }

  /** Runs an install or uninstall, then shows the result. */
  private async change(work: () => Promise<unknown>, done: string, name: string): Promise<void> {
    this.clear();
    this.busy.set(true);
    try {
      await this.vaultGate.run(work);
      await this.reload();
      if (this.store().length > 0) {
        await this.loadStore();
      }
      this.message.set(this.transloco.translate(done, { name }));
    } catch (err) {
      this.showError(err);
    } finally {
      this.busy.set(false);
    }
  }

  protected install(p: PluginInfo): Promise<void> {
    this.usage.count('plugin.install');
    return this.change(() => this.api.install(p.id), 'plugins.store.done', p.name);
  }

  protected uninstall(p: PluginInfo): void {
    this.removing.set(p);
  }

  protected uninstallId(id: string): void {
    const p = this.plugins().find((x) => x.id === id);
    if (p) {
      this.removing.set(p);
    }
  }

  protected confirmUninstall(): void {
    const p = this.removing();
    this.removing.set(null);
    if (p) {
      void this.change(() => this.api.uninstall(p.id), 'plugins.store.removed', p.name);
    }
  }

  protected installFromStore(id: string): Promise<void> {
    const name = this.store().find((s) => s.id === id)?.name ?? id;
    return this.change(() => this.api.installFromStore(this.storeUrl.trim() || DEFAULT_STORE_URL, id), 'plugins.store.done', name);
  }

  protected installFile(): Promise<void> {
    const path = this.filePath.trim();
    return this.change(
      async () => {
        await this.api.installFile(path);
        this.filePath = '';
      },
      'plugins.store.done',
      path,
    );
  }

  private async load(): Promise<void> {
    try {
      const [plugins, configs] = await Promise.all([this.api.plugins(), this.api.configs()]);
      this.plugins.set(plugins);
      this.configs.set(configs);
    } catch (err) {
      this.showError(err);
    }
  }

  protected pluginOf(c: PluginConfig): PluginInfo | undefined {
    return this.plugins().find((p) => p.id === c.pluginId);
  }

  /** Built-in plugins name their texts with translation keys. */
  protected text(p: PluginInfo | null | undefined, value: string): string {
    return p?.builtIn && value ? this.transloco.translate(value) : value;
  }

  /** Plugins with something to set up; panels work without it. */
  protected configurable(p: PluginInfo): boolean {
    return !!p.capabilities?.some((c) => c === 'inventory' || c === 'credentials') || (p.settings?.length ?? 0) > 0;
  }

  protected add(p: PluginInfo): void {
    this.clear();
    let name = p.name;
    for (let i = 2; this.configs().some((c) => c.name.toLowerCase() === name.toLowerCase()); i++) {
      name = `${p.name} ${i}`;
    }
    this.draft.set({ id: '', pluginId: p.id, name, settings: {}, secrets: {} });
  }

  protected edit(c: PluginConfig): void {
    this.clear();
    this.draft.set({ id: c.id, pluginId: c.pluginId, name: c.name, settings: { ...(c.settings ?? {}) } as Record<string, string>, secrets: {} });
  }

  /** The + button: back to the catalog, scrolled to the store. */
  protected showCatalog(): void {
    this.clear();
    this.draft.set(null);
    if (this.store().length === 0 && !this.storeLoading()) {
      void this.loadStore();
    }
    setTimeout(() => this.storeHeading()?.nativeElement.scrollIntoView({ behavior: 'smooth', block: 'start' }));
  }

  /** Picks a plugin's .zip with the system's chooser and installs it. */
  protected async chooseFile(): Promise<void> {
    const path = await this.api.chooseFile(this.transloco.translate('plugins.store.chooseTitle')).catch((err) => {
      this.showError(err);
      return '';
    });
    if (path) {
      this.filePath = path;
      await this.installFile();
    }
  }

  protected async save(): Promise<PluginConfig | null> {
    const d = this.draft();
    if (!d) {
      return null;
    }
    this.clear();
    this.busy.set(true);
    try {
      const saved = await this.vaultGate.run(() => this.api.save({ id: d.id, pluginId: d.pluginId, name: d.name, settings: d.settings, groupId: '', lastSync: '' }, d.secrets));
      this.configs.set(await this.api.configs());
      this.draft.set({ ...d, id: saved.id, secrets: {} });
      this.message.set(this.transloco.translate('plugins.saved'));
      return saved;
    } catch (err) {
      this.showError(err);
      return null;
    } finally {
      this.busy.set(false);
    }
  }

  /** Saves, then refreshes the inventory into its host group. */
  protected async sync(): Promise<void> {
    const saved = await this.save();
    if (!saved) {
      return;
    }
    this.clear();
    this.busy.set(true);
    try {
      const r = await this.vaultGate.run(() => this.api.sync(saved.id));
      this.message.set(this.transloco.translate('plugins.result', { ...r }));
      this.configs.set(await this.api.configs());
      await this.hosts.load();
    } catch (err) {
      this.showError(err);
    } finally {
      this.busy.set(false);
    }
  }

  protected async confirmDelete(): Promise<void> {
    const c = this.deleting();
    this.deleting.set(null);
    if (!c) {
      return;
    }
    try {
      await this.api.delete(c.id);
      this.configs.set(await this.api.configs());
      if (this.draft()?.id === c.id) {
        this.draft.set(null);
      }
    } catch (err) {
      this.showError(err);
    }
  }

  protected readonly capability = (c: string) => this.transloco.translate('plugins.capabilities.' + c);

  protected secretSet(key: string): boolean {
    return this.saved()?.secretsSet?.includes(key) ?? false;
  }

  protected lastSync(c: PluginConfig): string {
    return c.lastSync ? new Date(c.lastSync).toLocaleString(this.transloco.getActiveLang()) : '';
  }

  private clear(): void {
    this.message.set(null);
    this.error.set(null);
  }

  private showError(err: unknown): void {
    const e = toTranslatableError(err);
    const params: Record<string, unknown> = { ...e.params, detail: e.params['detail'] ?? e.detail ?? '' };
    // A missing setting is named by its label, as in the form.
    const setting = this.plugin()?.settings?.find((s) => s.key === params['key']);
    if (setting) {
      params['key'] = this.text(this.plugin(), setting.label);
    }
    this.error.set(this.transloco.translate(e.key, params));
  }
}
