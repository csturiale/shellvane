import { Component, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { openUrl } from '../browser';
import { SITE_URL } from '../site';
import { UsageStore } from './usage.store';

/** Settings › Privacy: the usage statistics switch and what they send. */
@Component({
  selector: 'app-usage-settings',
  imports: [TranslocoDirective],
  templateUrl: './usage-settings.html',
  styleUrl: './usage-settings.scss',
})
export class UsageSettings {
  protected readonly usage = inject(UsageStore);
  protected readonly preview = signal<string | null>(null);

  protected async toggle(on: boolean): Promise<void> {
    this.usage.setConsent(on);
    if (this.preview() !== null) {
      await this.show();
    }
  }

  protected async show(): Promise<void> {
    this.preview.set(JSON.stringify(await this.usage.preview(), null, 2));
  }

  protected when(ms: number): string {
    return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(ms));
  }

  protected privacy(): void {
    void openUrl(`${SITE_URL}/privacy/#usage`);
  }
}
