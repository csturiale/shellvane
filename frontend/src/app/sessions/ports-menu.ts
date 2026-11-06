import { Component, ElementRef, inject, input, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideEthernetPort } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { RemotePort } from '../core/backend/tunnel-api';
import { openUrl } from '../core/browser';
import { TunnelsStore } from '../tunnels/tunnels.store';

/** Ports listening on the pane's server, each with a one-click forward. */
@Component({
  selector: 'app-ports-menu',
  imports: [TranslocoDirective, LucideEthernetPort],
  templateUrl: './ports-menu.html',
  styleUrl: './ports-menu.scss',
  host: { '(document:pointerdown)': 'onOutside($event)', '(keydown.escape)': 'open.set(false)' },
})
export class PortsMenu {
  readonly hostId = input.required<string>();

  protected readonly tunnels = inject(TunnelsStore);
  private readonly transloco = inject(TranslocoService);
  private readonly el = inject(ElementRef<HTMLElement>);

  protected readonly open = signal(false);
  protected readonly pos = signal({ top: 0, left: 0 });
  protected readonly loading = signal(false);
  protected readonly ports = signal<RemotePort[]>([]);
  protected readonly error = signal<string | null>(null);

  protected async toggle(ev: MouseEvent): Promise<void> {
    const box = (ev.currentTarget as HTMLElement).getBoundingClientRect();
    this.pos.set({ top: box.bottom + 6, left: Math.max(8, Math.min(box.left - 200, globalThis.innerWidth - 336)) });
    this.open.update((o) => !o);
    if (this.open()) {
      await this.refresh();
    }
  }

  protected async refresh(): Promise<void> {
    this.error.set(null);
    this.loading.set(true);
    try {
      this.ports.set(await this.tunnels.listeningPorts(this.hostId()));
    } catch (err) {
      this.ports.set([]);
      this.fail(err);
    } finally {
      this.loading.set(false);
    }
  }

  protected local(port: number): string | null {
    const t = this.tunnels.forwardOf(this.hostId(), port);
    return t ? t.status.addr : null;
  }

  protected async forward(p: RemotePort): Promise<void> {
    try {
      await this.tunnels.forward(this.hostId(), p);
    } catch (err) {
      this.fail(err);
    }
  }

  protected browse(addr: string): void {
    void openUrl(`http://${addr}`);
  }

  protected onOutside(ev: PointerEvent): void {
    if (this.open() && !this.el.nativeElement.contains(ev.target as Node)) {
      this.open.set(false);
    }
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.error.set(this.transloco.translate(e.key, e.params));
  }
}
