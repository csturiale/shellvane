import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { TunnelView } from '../core/backend/tunnel-api';
import { HostsStore } from '../hosts/hosts.store';
import { Modal } from '../shared/modal';
import { TunnelsStore } from './tunnels.store';

type Kind = 'local' | 'remote' | 'dynamic';

@Component({
  selector: 'app-tunnel-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './tunnel-dialog.html',
  styleUrls: ['../shared/form.scss', './tunnel-dialog.scss'],
})
export class TunnelDialog {
  readonly tunnel = input<TunnelView | null>(null);
  readonly closed = output<void>();

  private readonly store = inject(TunnelsStore);
  private readonly hostsStore = inject(HostsStore);
  protected readonly hosts = computed(() =>
    [...this.hostsStore.hosts()].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
  );
  protected readonly kinds: Kind[] = ['local', 'remote', 'dynamic'];

  protected readonly kind = signal<Kind>('local');
  protected hostId = '';
  protected name = '';
  protected bindAddr = '127.0.0.1';
  protected bindPort: number | null = null;
  protected destHost = 'localhost';
  protected destPort: number | null = null;
  protected autostart = false;
  protected readonly error = signal<TranslatableError | null>(null);
  protected readonly saving = signal(false);

  ngOnInit(): void {
    const t = this.tunnel()?.profile;
    if (t) {
      this.kind.set(t.kind as Kind);
      this.hostId = t.hostId;
      this.name = t.name;
      this.bindAddr = t.bindAddr;
      this.bindPort = t.bindPort;
      this.destHost = t.destHost || 'localhost';
      this.destPort = t.destPort || null;
      this.autostart = t.autostart;
    } else {
      this.hostId = this.hosts()[0]?.id ?? '';
    }
  }

  protected async save(): Promise<void> {
    this.saving.set(true);
    this.error.set(null);
    try {
      await this.store.save(this.tunnel()?.profile.id ?? null, {
        hostId: this.hostId,
        name: this.name,
        autostart: this.autostart,
        spec: {
          kind: this.kind() as never,
          bindAddr: this.bindAddr,
          bindPort: Number(this.bindPort ?? 0),
          destHost: this.kind() === 'dynamic' ? '' : this.destHost,
          destPort: this.kind() === 'dynamic' ? 0 : Number(this.destPort ?? 0),
        },
      });
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.saving.set(false);
    }
  }
}
