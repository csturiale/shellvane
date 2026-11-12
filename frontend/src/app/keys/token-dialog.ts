import { Component, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { TranslatableError, toTranslatableError } from '../core/backend/errors';
import { KeyApi, Token, TokenKey } from '../core/backend/key-api';
import { HelpLink } from '../guide/help-link';
import { Modal } from '../shared/modal';
import { VaultGate } from '../vault/vault-gate';
import { PathPicker } from '../shared/path-picker';

/** Adds a key that lives on a hardware token (YubiKey, smart card) through its PKCS#11 module. */
@Component({
  selector: 'app-token-dialog',
  imports: [FormsModule, HelpLink, Modal, TranslocoDirective, PathPicker],
  templateUrl: './token-dialog.html',
  styleUrls: ['../shared/form.scss', './token-dialog.scss'],
})
export class TokenDialog {
  private readonly vaultGate = inject(VaultGate);
  /** Emits the new key's name, or null when cancelled. */
  readonly closed = output<string | null>();

  private readonly api = inject(KeyApi);
  protected readonly supported = signal(true);
  protected readonly modules = signal<string[]>([]);
  protected readonly tokens = signal<Token[] | null>(null);
  protected readonly keys = signal<TokenKey[] | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);
  protected module = '';
  protected serial = '';
  protected keyId = '';
  protected name = '';

  constructor() {
    void this.api
      .tokenSupport()
      .then((s) => {
        this.supported.set(s.supported);
        const modules = s.modules ?? [];
        this.modules.set(modules);
        this.module = modules[0] ?? '';
      })
      .catch(() => undefined);
  }

  protected async findTokens(): Promise<void> {
    await this.run(async () => {
      this.keys.set(null);
      const list = await this.api.tokens(this.module.trim());
      this.tokens.set(list);
      this.serial = list[0]?.serial ?? '';
      if (list.length > 0) {
        await this.loadKeys();
      }
    });
  }

  protected async findKeys(): Promise<void> {
    await this.run(() => this.loadKeys());
  }

  private async loadKeys(): Promise<void> {
    const list = await this.api.tokenKeys(this.module.trim(), this.serial);
    this.keys.set(list);
    this.keyId = list[0]?.id ?? '';
    const token = this.tokens()?.find((t) => t.serial === this.serial);
    this.name = token ? `${token.label || token.model} ${list[0]?.label ?? ''}`.trim() : '';
  }

  protected async add(): Promise<void> {
    const token = this.tokens()?.find((t) => t.serial === this.serial);
    const key = this.keys()?.find((k) => k.id === this.keyId);
    if (!token || !key) {
      return;
    }
    await this.run(async () => {
      const added = await this.api.addTokenKey(this.name.trim(), this.module.trim(), token, key);
      this.closed.emit(added.name);
    });
  }

  private async run(action: () => Promise<void>): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    try {
      await this.vaultGate.run(action);
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.busy.set(false);
    }
  }
}
