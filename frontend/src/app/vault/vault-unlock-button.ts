import { Component, inject, output } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { VaultGate } from './vault-gate';
import { VaultStore } from './vault.store';

/** "Unlock the vault" next to a message about the locked vault; shown only
 * while it is locked. */
@Component({
  selector: 'app-vault-unlock-button',
  imports: [TranslocoDirective],
  templateUrl: './vault-unlock-button.html',
  styleUrl: './vault-unlock-button.scss',
})
export class VaultUnlockButton {
  protected readonly vault = inject(VaultStore);
  private readonly gate = inject(VaultGate);
  readonly unlocked = output<void>();

  protected async open(): Promise<void> {
    if (await this.gate.ensureUnlocked()) {
      this.unlocked.emit();
    }
  }
}
