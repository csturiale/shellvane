import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { TranslatableError, toTranslatableError } from '../../core/backend/errors';
import { NetworkApi, ProxyConfig, ReachResult } from '../../core/backend/network-api';
import { HelpLink } from '../../guide/help-link';

/** Settings › Network: the proxy of the app's own requests, and a check. */
@Component({
  selector: 'app-network-settings',
  imports: [FormsModule, TranslocoDirective, HelpLink],
  templateUrl: './network-settings.html',
  styleUrls: ['../../shared/form.scss', './network-settings.scss'],
})
export class NetworkSettings {
  private readonly api = inject(NetworkApi);
  protected config: ProxyConfig = { mode: 'system', url: '', noProxy: '' };
  protected readonly saved = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);
  protected readonly testing = signal(false);
  protected readonly results = signal<ReachResult[] | null>(null);

  constructor() {
    this.api
      .proxy()
      .then((c) => (this.config = { ...c }))
      .catch(() => undefined);
  }

  protected async save(): Promise<void> {
    this.error.set(null);
    this.saved.set(false);
    try {
      this.config = { ...(await this.api.setProxy(this.config)) };
      this.saved.set(true);
      this.results.set(null);
    } catch (err) {
      this.error.set(toTranslatableError(err));
    }
  }

  protected async test(): Promise<void> {
    this.testing.set(true);
    this.results.set(null);
    try {
      this.results.set(await this.api.test());
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.testing.set(false);
    }
  }

  protected host(url: string): string {
    try {
      return new URL(url).host;
    } catch {
      return url;
    }
  }
}
