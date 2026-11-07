import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { VaultGate } from './vault-gate';
import { VaultStore } from './vault.store';

const locked = () => Object.assign(new Error('locked'), { cause: { code: 'vault.locked' } });

describe('VaultGate', () => {
  let state: ReturnType<typeof signal<'notSetUp' | 'locked' | 'unlocked'>>;
  let keychain: boolean;
  let gate: VaultGate;

  beforeEach(() => {
    state = signal<'notSetUp' | 'locked' | 'unlocked'>('locked');
    keychain = false;
    const store = {
      state,
      hasKeychain: () => keychain,
      unlockWithKeychain: async () => {
        if (!keychain) throw new Error('no keychain');
        state.set('unlocked');
      },
    };
    TestBed.configureTestingModule({ providers: [{ provide: VaultStore, useValue: store }] });
    gate = TestBed.inject(VaultGate);
  });

  it('asks to unlock and runs the work again', async () => {
    let calls = 0;
    const work = async () => {
      calls++;
      if (state() !== 'unlocked') throw locked();
      return 'done';
    };
    const result = gate.run(work, 'why');
    await Promise.resolve();
    await Promise.resolve();
    expect(gate.asking()).toEqual({ reason: 'why' });
    state.set('unlocked');
    gate.finish(true);
    expect(await result).toBe('done');
    expect(calls).toBe(2);
    expect(gate.asking()).toBeNull();
  });

  it('keeps the error when the dialog is cancelled', async () => {
    const result = gate.run(async () => {
      throw locked();
    });
    await Promise.resolve();
    await Promise.resolve();
    gate.finish(false);
    await expect(result).rejects.toThrow('locked');
  });

  it('uses the keychain without asking, and passes other errors through', async () => {
    keychain = true;
    expect(await gate.ensureUnlocked()).toBe(true);
    expect(gate.asking()).toBeNull();
    await expect(gate.run(async () => Promise.reject(new Error('offline')))).rejects.toThrow('offline');
  });

  it('does not ask when there is no vault', async () => {
    state.set('notSetUp');
    expect(await gate.ensureUnlocked()).toBe(false);
    expect(gate.asking()).toBeNull();
  });
});
