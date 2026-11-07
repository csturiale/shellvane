import { Injectable, inject, signal } from '@angular/core';

import { toTranslatableError } from '../core/backend/errors';
import { VaultStore } from './vault.store';

/** The error of a call that needed the vault while it was locked. */
export function isVaultLocked(err: unknown): boolean {
  return toTranslatableError(err).key === 'errors.vault.locked';
}

/**
 * Opens the vault when something needs it: instead of failing with "the
 * vault is locked", actions ask for the master password (or the system
 * keychain) and carry on.
 */
@Injectable({ providedIn: 'root' })
export class VaultGate {
  private readonly vault = inject(VaultStore);

  /** The unlock dialog is open; reason says what needs the vault. */
  readonly asking = signal<{ reason: string | null } | null>(null);
  private waiting: ((unlocked: boolean) => void)[] = [];

  /** Resolves true once the vault is open: already, with the keychain, or
   * with the dialog; false if the user cancels or there is no vault. */
  async ensureUnlocked(reason?: string): Promise<boolean> {
    if (this.vault.state() === 'unlocked') {
      return true;
    }
    if (this.vault.state() === 'notSetUp') {
      return false;
    }
    if (this.vault.hasKeychain() && (await this.vault.unlockWithKeychain().then(() => true, () => false))) {
      return true;
    }
    return new Promise((resolve) => {
      this.waiting.push(resolve);
      if (!this.asking()) {
        this.asking.set({ reason: reason ?? null });
      }
    });
  }

  /** Runs work; if it fails because the vault is locked, asks to unlock
   * it and runs it once more. */
  async run<T>(work: () => Promise<T>, reason?: string): Promise<T> {
    try {
      return await work();
    } catch (err) {
      if (!isVaultLocked(err) || !(await this.ensureUnlocked(reason))) {
        throw err;
      }
      return work();
    }
  }

  /** Called by the dialog when it closes. */
  finish(unlocked: boolean): void {
    this.asking.set(null);
    const waiting = this.waiting;
    this.waiting = [];
    waiting.forEach((resolve) => resolve(unlocked));
  }
}
