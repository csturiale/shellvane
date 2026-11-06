import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideKeyRound, LucideLock, LucideLockOpen, LucideTrash2 } from '@lucide/angular';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { writeClipboard } from '../core/clipboard';
import { HelpLink } from '../guide/help-link';
import { Modal } from '../shared/modal';
import { VaultStore } from './vault.store';

/** Vault section of the settings page. */
@Component({
  selector: 'app-vault-settings',
  imports: [FormsModule, HelpLink, TranslocoDirective, Modal, LucideLock, LucideLockOpen, LucideKeyRound, LucideTrash2],
  templateUrl: './vault-settings.html',
  styleUrls: ['../shared/form.scss', './vault-settings.scss'],
})
export class VaultSettings {
  protected readonly vault = inject(VaultStore);

  protected password = '';
  protected confirm = '';
  protected oldPassword = '';
  protected newPassword = '';
  protected recoveryKey = '';
  protected readonly autoLockChoices = [0, 5, 15, 30, 60, 240];
  /** The recovery key just created, shown once. */
  protected readonly shownKey = signal<string | null>(null);
  protected readonly copied = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);
  protected readonly notice = signal<string | null>(null);
  protected readonly busy = signal(false);

  ngOnInit(): void {
    // Credentials are saved during logins, without a vault event.
    void this.vault.refresh();
  }

  protected setup(): Promise<void> {
    if (this.password !== this.confirm) {
      this.error.set({ key: 'vault.mismatch', params: {} });
      return Promise.resolve();
    }
    return this.run(() => this.vault.setup(this.password));
  }

  protected unlock(): Promise<void> {
    return this.run(() => this.vault.unlock(this.password));
  }

  protected changePassword(): Promise<void> {
    return this.run(async () => {
      await this.vault.changePassword(this.oldPassword, this.newPassword);
      this.notice.set('vault.changed');
    });
  }

  protected recover(): Promise<void> {
    if (this.newPassword !== this.confirm) {
      this.error.set({ key: 'vault.mismatch', params: {} });
      return Promise.resolve();
    }
    return this.run(async () => {
      await this.vault.recover(this.recoveryKey, this.newPassword);
      this.recoveryKey = '';
      this.notice.set('vault.recovered');
    });
  }

  protected createRecoveryKey(): Promise<void> {
    return this.run(async () => {
      this.copied.set(false);
      this.shownKey.set(await this.vault.createRecoveryKey());
    });
  }

  protected setKeychain(on: boolean): Promise<void> {
    return this.run(() => this.vault.setKeychain(on));
  }

  protected setBiometric(on: boolean): Promise<void> {
    return this.run(() => this.vault.setBiometric(on));
  }

  protected unlockWithKeychain(): Promise<void> {
    return this.run(() => this.vault.unlockWithKeychain());
  }

  protected removeRecoveryKey(): Promise<void> {
    return this.run(() => this.vault.removeRecoveryKey());
  }

  protected async copyKey(k: string): Promise<void> {
    await writeClipboard(k);
    this.copied.set(true);
  }

  protected deleteCredential(scope: string): Promise<void> {
    return this.run(() => this.vault.deleteCredential(scope));
  }

  private async run(action: () => Promise<void>): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    this.notice.set(null);
    try {
      await action();
      this.password = this.confirm = this.oldPassword = this.newPassword = '';
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.busy.set(false);
    }
  }
}
