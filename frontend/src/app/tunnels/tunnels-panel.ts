import { Component, computed, inject, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideCable, LucideExternalLink, LucideSave, LucidePencil, LucidePlay, LucidePlus, LucideSquare, LucideTrash2 } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { TunnelView } from '../core/backend/tunnel-api';
import { formatBytes } from '../core/format';
import { LanguageService } from '../core/i18n/language.service';
import { HostsStore } from '../hosts/hosts.store';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { describeTunnel } from './describe';
import { openUrl } from '../core/browser';
import { TunnelDialog } from './tunnel-dialog';
import { TunnelsStore } from './tunnels.store';
import { KubeForwardsStore } from './kube-forwards.store';
import type { KubeForward } from '../core/backend/kube-forward-api';
import { HelpLink } from '../guide/help-link';

@Component({
  selector: 'app-tunnels-panel',
  imports: [HelpLink, TranslocoDirective, TunnelDialog, ConfirmDialog, LucidePlus, LucidePlay, LucideSquare, LucidePencil, LucideTrash2, LucideCable, LucideExternalLink, LucideSave],
  templateUrl: './tunnels-panel.html',
  styleUrl: './tunnels-panel.scss',
})
export class TunnelsPanel {
  protected readonly store = inject(TunnelsStore);
  protected readonly kube = inject(KubeForwardsStore);
  private readonly hosts = inject(HostsStore);
  private readonly language = inject(LanguageService);
  private readonly transloco = inject(TranslocoService);

  protected readonly editing = signal<{ tunnel: TunnelView | null } | null>(null);
  protected readonly deleting = signal<TunnelView | null>(null);
  protected readonly message = signal<string | null>(null);
  protected readonly hostNames = computed(() => new Map(this.hosts.hosts().map((h) => [h.id, h.name])));
  protected readonly describe = describeTunnel;

  constructor() {
    void this.store.load().catch((err) => this.showError(err));
    void this.kube.load().catch(() => undefined);
    if (!this.hosts.loaded()) {
      void this.hosts.load().catch(() => undefined);
    }
  }

  /** Opens a forwarded web port: https for 443 and 8443, http otherwise. */
  protected browseKube(f: KubeForward): void {
    const https = f.remotePort === 443 || f.remotePort === 8443;
    void openUrl(`${https ? 'https' : 'http'}://127.0.0.1:${f.localPort}/`);
  }

  protected stopKube(id: string): void {
    void this.kube.stop(id).catch((err) => this.showError(err));
  }

  protected bytes(n: number): string {
    return formatBytes(n, this.language.current());
  }

  protected toggle(t: TunnelView): void {
    const running = this.active(t);
    void (running ? this.store.stop(t.profile.id) : this.store.start(t.profile.id)).catch((err) => this.showError(err));
  }

  protected newTunnel(): void {
    if (this.hosts.hosts().length === 0) {
      this.message.set(this.transloco.translate('tunnels.needHost'));
      return;
    }
    this.editing.set({ tunnel: null });
  }

  protected async confirmDelete(): Promise<void> {
    const t = this.deleting();
    this.deleting.set(null);
    if (t) {
      await this.store.delete(t.profile.id).catch((err) => this.showError(err));
    }
  }

  private showError(err: unknown): void {
    const e = toTranslatableError(err);
    this.message.set(this.transloco.translate(e.key, e.params));
  }

  /** Starting, running or waiting to reconnect: the button stops it. */
  /** Opens the local end of a forward in the browser. */
  protected browse(t: TunnelView): void {
    const addr = t.status.addr || `127.0.0.1:${t.profile.bindPort}`;
    void openUrl(`http://${addr.replace(/^0\.0\.0\.0:/, '127.0.0.1:')}`);
  }

  protected keep(t: TunnelView): void {
    void this.store.keep(t).catch((err) => this.showError(err));
  }

  protected active(t: TunnelView): boolean {
    return t.status.state === 'running' || t.status.state === 'starting' || t.status.state === 'reconnecting';
  }
}
