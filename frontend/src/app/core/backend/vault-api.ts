import { Injectable } from '@angular/core';
import { Events } from '@wailsio/runtime';

import { VaultService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { BiometricStatus } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Credential } from '../../../../bindings/github.com/csturiale/shellvane/core/vault';
import { VaultState } from '../../../../bindings/events';

export type { Credential };
export type VaultStateName = 'notSetUp' | 'locked' | 'unlocked';

/** Vault operations. Secrets never reach the frontend. */
export type { BiometricStatus };

@Injectable({ providedIn: 'root' })
export class VaultApi {
  async status(): Promise<VaultStateName> {
    return (await VaultService.Status()).state as VaultStateName;
  }
  setup(password: string): Promise<void> {
    return VaultService.Setup(password);
  }
  unlock(password: string): Promise<void> {
    return VaultService.Unlock(password);
  }
  lock(): Promise<void> {
    return VaultService.Lock();
  }
  changePassword(oldPassword: string, newPassword: string): Promise<void> {
    return VaultService.ChangePassword(oldPassword, newPassword);
  }
  async credentials(): Promise<Credential[]> {
    return (await VaultService.Credentials()) ?? [];
  }
  deleteCredential(scope: string): Promise<void> {
    return VaultService.DeleteCredential(scope);
  }
  setAutoLock(minutes: number, onSleep: boolean): Promise<void> {
    return VaultService.SetAutoLock(minutes, onSleep);
  }
  createRecoveryKey(): Promise<string> {
    return VaultService.CreateRecoveryKey();
  }
  enableKeychainUnlock(): Promise<void> {
    return VaultService.EnableKeychainUnlock();
  }
  disableKeychainUnlock(): Promise<void> {
    return VaultService.DisableKeychainUnlock();
  }
  hasKeychainUnlock(): Promise<boolean> {
    return VaultService.HasKeychainUnlock();
  }
  biometric(): Promise<BiometricStatus> {
    return VaultService.Biometric();
  }
  setBiometric(on: boolean): Promise<void> {
    return VaultService.SetBiometric(on);
  }
  unlockWithKeychain(): Promise<void> {
    return VaultService.UnlockWithKeychain();
  }
  hasRecoveryKey(): Promise<boolean> {
    return VaultService.HasRecoveryKey();
  }
  removeRecoveryKey(): Promise<void> {
    return VaultService.RemoveRecoveryKey();
  }
  recover(recoveryKey: string, newPassword: string): Promise<void> {
    return VaultService.Recover(recoveryKey, newPassword);
  }
  setTOTP(hostId: string, seed: string): Promise<void> {
    return VaultService.SetTOTP(hostId, seed);
  }
  hasTOTP(hostId: string): Promise<boolean> {
    return VaultService.HasTOTP(hostId);
  }
  onState(handler: (state: VaultStateName) => void): () => void {
    return Events.On(VaultState, (ev) => handler(ev.data.state as VaultStateName));
  }
}
