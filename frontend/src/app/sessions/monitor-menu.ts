import { Component, ElementRef, inject, input, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideActivity } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { MonitorApi, ServerStats } from '../core/backend/monitor-api';
import { formatBytes } from '../core/format';
import { LanguageService } from '../core/i18n/language.service';

/** The server's load, CPU, memory and disk, refreshed while shown. */
@Component({
  selector: 'app-monitor-menu',
  imports: [TranslocoDirective, LucideActivity],
  templateUrl: './monitor-menu.html',
  styleUrl: './monitor-menu.scss',
  host: { '(document:pointerdown)': 'onOutside($event)', '(keydown.escape)': 'close()' },
})
export class MonitorMenu {
  readonly hostId = input.required<string>();

  private readonly api = inject(MonitorApi);
  private readonly language = inject(LanguageService);
  private readonly transloco = inject(TranslocoService);
  private readonly el = inject(ElementRef<HTMLElement>);

  protected readonly open = signal(false);
  protected readonly pos = signal({ top: 0, left: 0 });
  protected readonly stats = signal<ServerStats | null>(null);
  protected readonly error = signal<string | null>(null);
  private timer?: ReturnType<typeof setInterval>;

  protected toggle(ev: MouseEvent): void {
    if (this.open()) {
      this.close();
      return;
    }
    const box = (ev.currentTarget as HTMLElement).getBoundingClientRect();
    this.pos.set({ top: box.bottom + 6, left: Math.max(8, Math.min(box.left - 200, globalThis.innerWidth - 296)) });
    this.open.set(true);
    void this.sample();
    // A second sample soon gives the CPU use; then every 3 seconds.
    setTimeout(() => void this.sample(), 1000);
    this.timer = setInterval(() => void this.sample(), 3000);
  }

  protected close(): void {
    this.open.set(false);
    clearInterval(this.timer);
  }

  ngOnDestroy(): void {
    clearInterval(this.timer);
  }

  private async sample(): Promise<void> {
    try {
      this.stats.set(await this.api.sample(this.hostId()));
      this.error.set(null);
    } catch (err) {
      const e = toTranslatableError(err);
      this.error.set(this.transloco.translate(e.key, e.params));
    }
  }

  protected percent(used: number, total: number): number {
    return total > 0 ? Math.round((100 * used) / total) : 0;
  }

  protected bytes(n: number): string {
    return formatBytes(n, this.language.current());
  }

  protected uptime(seconds: number): string {
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return this.transloco.translate('monitor.uptimeValue', { d, h, m });
  }

  protected onOutside(ev: PointerEvent): void {
    if (this.open() && !this.el.nativeElement.contains(ev.target as Node)) {
      this.close();
    }
  }
}
