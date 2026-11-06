import { Injectable, inject, signal } from '@angular/core';

import { readPreference, writePreference } from '../core/preferences';

import { BiometricStatus, Credential, VaultApi, VaultStateName } from '../core/backend/vault-api';

@Injectable({ providedIn: 'root' })
export class VaultStore {
  private readonly api = inject(VaultApi);

  /** null until the first status arrives (or outside the desktop app). */
  readonly state = signal<VaultStateName | null>(null);
  readonly credentials = signal<Credential[]>([]);
  readonly hasRecoveryKey = signal(false);
  /** The vault can open with a secret kept in the system keychain. */
  readonly hasKeychain = signal(false);
  /** Touch ID / Windows Hello for the keychain unlock. */
  readonly biometric = signal<BiometricStatus | null>(null);
  private triedKeychain = false;
  /** Minutes of inactivity before locking; 0 never. */
  readonly autoLockMinutes = signal(Number(readPreference('vault.autoLock') ?? 15));
  readonly lockOnSleep = signal(readPreference('vault.lockOnSleep') !== 'false');
  /** Password prompts never offer to save the password. */
  readonly neverSave = signal(readPreference('vault.neverSave') === 'true');

  constructor() {
    try {
      this.api.onState((s) => {
        this.state.set(s);
        void this.loadCredentials();
      });
    } catch {
      // No event bus outside the desktop app.
    }
    void this.refresh();
    void this.applyAutoLock();
  }

  setAutoLock(minutes: number, onSleep: boolean): void {
    this.autoLockMinutes.set(minutes);
    this.lockOnSleep.set(onSleep);
    writePreference('vault.autoLock', String(minutes));
    writePreference('vault.lockOnSleep', String(onSleep));
    void this.applyAutoLock();
  }

  setNeverSave(on: boolean): void {
    this.neverSave.set(on);
    writePreference('vault.neverSave', String(on));
  }

  private async applyAutoLock(): Promise<void> {
    try {
      await this.api.setAutoLock(this.autoLockMinutes(), this.lockOnSleep());
    } catch {
      // Outside the desktop app.
    }
  }

  async createRecoveryKey(): Promise<string> {
    const code = await this.api.createRecoveryKey();
    this.hasRecoveryKey.set(true);
    return code;
  }

  async setKeychain(on: boolean): Promise<void> {
    await (on ? this.api.enableKeychainUnlock() : this.api.disableKeychainUnlock());
    this.hasKeychain.set(on);
    this.biometric.set(await this.api.biometric().catch(() => null));
  }

  async setBiometric(on: boolean): Promise<void> {
    await this.api.setBiometric(on);
    this.biometric.set(await this.api.biometric().catch(() => null));
  }

  async unlockWithKeychain(): Promise<void> {
    await this.api.unlockWithKeychain();
    await this.refresh();
  }

  async removeRecoveryKey(): Promise<void> {
    await this.api.removeRecoveryKey();
    this.hasRecoveryKey.set(false);
  }

  async recover(recoveryKey: string, newPassword: string): Promise<void> {
    await this.api.recover(recoveryKey, newPassword);
    await this.refresh();
  }

  async refresh(): Promise<void> {
    try {
      this.state.set(await this.api.status());
      this.hasRecoveryKey.set(await this.api.hasRecoveryKey());
      this.hasKeychain.set(await this.api.hasKeychainUnlock().catch(() => false));
      this.biometric.set(await this.api.biometric().catch(() => null));
      // At startup the keychain opens the vault without the master password.
      if (!this.triedKeychain && this.state() === 'locked' && this.hasKeychain()) {
        this.triedKeychain = true;
        if (await this.api.unlockWithKeychain().then(() => true, () => false)) {
          this.state.set(await this.api.status());
        }
      }
      this.triedKeychain = true;
      await this.loadCredentials();
    } catch {
      this.state.set(null);
    }
  }

  async setup(password: string): Promise<void> {
    await this.api.setup(password);
    await this.refresh();
  }

  async unlock(password: string): Promise<void> {
    await this.api.unlock(password);
    await this.refresh();
  }

  async lock(): Promise<void> {
    await this.api.lock();
    await this.refresh();
  }

  changePassword(oldPassword: string, newPassword: string): Promise<void> {
    return this.api.changePassword(oldPassword, newPassword);
  }

  async deleteCredential(scope: string): Promise<void> {
    await this.api.deleteCredential(scope);
    await this.loadCredentials();
  }

  private async loadCredentials(): Promise<void> {
    this.credentials.set(await this.api.credentials());
  }
}
