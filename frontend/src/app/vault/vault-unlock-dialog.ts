import { Component, computed, effect, inject, signal } from '@angular/core';
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
  /** Touch ID / Windows Hello can open the vault from now on. */
  protected enableBiometric = true;

  /** Offered when this computer has Touch ID or Windows Hello but the
   * vault does not use it yet (it guards the keychain unlock, which is
   * turned on with it). */
  protected readonly offerBiometric = computed(() => {
    const b = this.vault.biometric();
    return b?.available && b.kind && !(this.vault.hasKeychain() && b.required) ? b.kind : null;
  });
  protected readonly busy = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);

  constructor() {
    // Opened elsewhere meanwhile (Touch ID, a pane's prompt): nothing to ask.
    effect(() => {
      if (this.vault.state() === 'unlocked' && !this.busy()) {
        this.gate.finish(true);
      }
    });
  }

  protected unlock(): Promise<void> {
    return this.attempt(async () => {
      await this.vault.unlock(this.password);
      if (this.offerBiometric() && this.enableBiometric) {
        // Asks for Touch ID / Windows Hello once, to check it works; a
        // refusal leaves the vault open and the setting off.
        await this.vault.setKeychain(true).then(() => this.vault.setBiometric(true)).catch(() => undefined);
      }
    });
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
