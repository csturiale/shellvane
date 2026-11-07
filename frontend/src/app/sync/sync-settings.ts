import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { TranslatableError, toTranslatableError } from '../core/backend/errors';
import { CloudFolder, SyncApi, SyncReport, SyncStatus } from '../core/backend/sync-api';
import { LanguageService } from '../core/i18n/language.service';
import { HelpLink } from '../guide/help-link';
import { VaultStore } from '../vault/vault.store';
import { UsageStore } from '../core/usage/usage.store';

/** Sync section of the settings page. */
@Component({
  selector: 'app-sync-settings',
  imports: [FormsModule, TranslocoDirective, HelpLink],
  templateUrl: './sync-settings.html',
  styleUrls: ['../shared/form.scss', './sync-settings.scss'],
})
export class SyncSettings {
  private readonly usage = inject(UsageStore);
  private readonly api = inject(SyncApi);
  private readonly language = inject(LanguageService);
  protected readonly vault = inject(VaultStore);

  protected readonly status = signal<SyncStatus | null>(null);
  protected readonly report = signal<SyncReport | null>(null);
  protected readonly error = signal<TranslatableError | null>(null);
  protected readonly busy = signal(false);
  /** Cloud drive folders found on this computer, offered as shortcuts. */
  protected readonly clouds = signal<CloudFolder[]>([]);
  protected kind: 'folder' | 'webdav' | 'git' | 's3' = 'folder';
  protected region = '';
  protected bucket = '';
  protected folder = '';
  protected url = '';
  protected user = '';
  protected password = '';
  protected passphrase = '';
  protected confirm = '';

  constructor() {
    void this.refresh();
    void this.api
      .cloudFolders()
      .then((list) => {
        this.clouds.set(list);
        if (list.length > 0 && !this.folder) {
          this.folder = list[0].path;
        }
      })
      .catch(() => undefined);
  }

  protected async refresh(): Promise<void> {
    try {
      this.status.set(await this.api.status());
    } catch {
      this.status.set(null);
    }
  }

  protected when(ts: string): string {
    return ts ? new Intl.DateTimeFormat(this.language.current(), { dateStyle: 'short', timeStyle: 'short' }).format(new Date(ts)) : '';
  }

  protected setup(): Promise<void> {
    if (this.passphrase !== this.confirm) {
      this.error.set({ key: 'vault.mismatch', params: {} });
      return Promise.resolve();
    }
    return this.run(async () => {
      this.report.set(
        await this.api.setup({
          kind: this.kind,
          folder: this.folder.trim(),
          url: this.url.trim(),
          user: this.user.trim(),
          password: this.password,
          region: this.region.trim(),
          bucket: this.bucket.trim(),
          passphrase: this.passphrase,
        }),
      );
      this.passphrase = this.confirm = this.password = '';
    });
  }

  protected ready(): boolean {
    if (this.kind === 's3') {
      return !!(this.url.trim() && this.bucket.trim() && this.user.trim() && this.password);
    }
    return this.kind === 'folder' ? !!this.folder.trim() : !!this.url.trim();
  }

  protected syncNow(): Promise<void> {
    this.usage.count('sync');
    return this.run(async () => this.report.set(await this.api.syncNow()));
  }

  protected setAuto(on: boolean): Promise<void> {
    return this.run(() => this.api.setAuto(on));
  }

  protected disable(): Promise<void> {
    return this.run(async () => {
      await this.api.disable();
      this.report.set(null);
    });
  }

  private async run(action: () => Promise<unknown>): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    try {
      await action();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.busy.set(false);
      await this.refresh();
    }
  }
}
