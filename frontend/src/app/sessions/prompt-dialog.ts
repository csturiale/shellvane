import { Component, computed, effect, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { toTranslatableError } from '../core/backend/errors';
import { Modal } from '../shared/modal';
import { VaultStore } from '../vault/vault.store';
import { SessionsStore } from './sessions.store';

/**
 * Shows the oldest pending prompt: a password, a 2FA challenge, a forwarded
 * agent signature or an unknown host key.
 */
@Component({
  selector: 'app-prompt-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './prompt-dialog.html',
  styleUrls: ['../shared/form.scss', './prompt-dialog.scss'],
})
export class PromptDialog {
  protected readonly store = inject(SessionsStore);
  protected readonly vault = inject(VaultStore);
  protected readonly prompt = computed(() => this.store.prompts()[0] ?? null);
  protected password = '';
  protected answers: string[] = [];
  /** Index of the first question the user has to answer. */
  protected readonly firstOpen = computed(() => {
    const p = this.prompt();
    const qs = p?.challenge?.questions ?? [];
    return qs.findIndex((_, i) => !p?.autofilled?.[i]);
  });
  protected save = false;
  /** Master password typed to unlock or create the vault for saving. */
  protected master = '';
  protected confirm = '';
  protected readonly vaultError = signal<string | null>(null);
  private readonly transloco = inject(TranslocoService);

  constructor() {
    // Opened elsewhere (Touch ID, another pane, the unlock dialog): the
    // panes still waiting for the master password go on without it.
    effect(() => {
      if (this.vault.state() !== 'unlocked') {
        return;
      }

      for (const p of this.store.prompts()) {
        if (p.kind === 'vaultUnlock') {
          void this.store.answer(p, { value: '', decision: '', cancel: false, save: false, answers: [] });
        }
      }
    });
  }

  protected async submitPassword(): Promise<void> {
    const p = this.prompt();
    if (!p) {
      return;
    }
    // Saving needs an open vault: unlock or create it here, without
    // leaving the login.
    if (this.save && p.kind !== 'vaultUnlock' && !(await this.openVault())) {
      return;
    }
    void this.store.answer(p, { value: this.password, decision: '', cancel: false, save: this.save, answers: [] });
    this.password = '';
    this.save = false;
    this.master = this.confirm = '';
    this.vaultError.set(null);
  }

  /** Makes the vault ready to save; false (with a message) when it is not. */
  private async openVault(): Promise<boolean> {
    this.vaultError.set(null);
    try {
      switch (this.vault.state()) {
        case 'locked':
          await this.vault.unlock(this.master);
          break;
        case 'notSetUp':
          if (this.master !== this.confirm) {
            this.vaultError.set(this.transloco.translate('vault.mismatch'));
            return false;
          }
          await this.vault.setup(this.master);
          break;
      }
      return true;
    } catch (err) {
      this.vaultError.set(this.message(err));
      return false;
    }
  }

  private message(err: unknown): string {
    const e = toTranslatableError(err);
    return this.transloco.translate(e.key, e.params);
  }

  protected async unlockWithKeychain(): Promise<void> {
    this.vaultError.set(null);
    try {
      await this.vault.unlockWithKeychain();
    } catch (err) {
      this.vaultError.set(this.message(err));
    }
  }

  protected submitChallenge(): void {
    const p = this.prompt();
    if (p) {
      const n = p.challenge?.questions?.length ?? 0;
      const answers = Array.from({ length: n }, (_, i) => this.answers[i] ?? '');
      void this.store.answer(p, { value: '', decision: '', cancel: false, save: false, answers });
    }
    this.answers = [];
  }

  protected cancel(): void {
    const p = this.prompt();
    if (p) {
      void this.store.answer(p, { value: '', decision: '', cancel: true, save: false, answers: [] });
    }
    this.password = '';
    this.answers = [];
  }

  protected decide(decision: 'reject' | 'once' | 'save' | 'session'): void {
    const p = this.prompt();
    if (p) {
      void this.store.answer(p, { value: '', decision, cancel: false, save: false, answers: [] });
    }
  }
}
