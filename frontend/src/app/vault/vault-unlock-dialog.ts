import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { TranslatableError, toTranslatableError } from '../core/backend/errors';
import { Modal } from '../shared/modal';
import { VaultGate } from './vault-gate';
import { VaultStore } from './vault.store';

/** The master password dialog of VaultGate, shown wherever the vault is needed. */
@Component({
  selector: 'app-vault-unlock-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './vault-unlock-dialog.html',
  styleUrls: ['../shared/form.scss', './vault-unlock-dialog.scss'],
})
export class VaultUnlockDialog {
  protected readonly gate = inject(VaultGate);
  protected readonly vault = inject(VaultStore);
  protected password = '';
  protected readonly busy = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);

  protected unlock(): Promise<void> {
    return this.attempt(() => this.vault.unlock(this.password));
  }

  protected unlockWithKeychain(): Promise<void> {
    return this.attempt(() => this.vault.unlockWithKeychain());
  }

  protected cancel(): void {
    this.password = '';
    this.gate.finish(false);
  }

  private async attempt(work: () => Promise<void>): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    try {
      await work();
      this.password = '';
      this.gate.finish(true);
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.busy.set(false);
    }
  }
}
