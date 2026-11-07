import { Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideBadgeCheck, LucideCopy, LucideFileKey, LucideKeyRound, LucideNetwork, LucidePlus, LucideTrash2, LucideUpload, LucideUsb } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { KeyUsage, RotationResult, SshKey } from '../core/backend/key-api';
import { writeClipboard } from '../core/clipboard';
import { HostsStore } from '../hosts/hosts.store';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { Modal } from '../shared/modal';
import { VaultStore } from '../vault/vault.store';
import { KeysStore } from './keys.store';
import { HelpLink } from '../guide/help-link';
import { TokenDialog } from './token-dialog';
import { VaultUnlockButton } from '../vault/vault-unlock-button';

type Dialog =
  | { kind: 'generate' }
  | { kind: 'import' }
  | { kind: 'deploy' | 'certificate' | 'usage' | 'export'; key: SshKey };

@Component({
  selector: 'app-keys-panel',
  imports: [VaultUnlockButton, HelpLink, FormsModule, TranslocoDirective, Modal, ConfirmDialog, LucidePlus, LucideKeyRound, LucideCopy, LucideTrash2, LucideUpload, LucideBadgeCheck, LucideNetwork, LucideFileKey, LucideUsb, TokenDialog],
  templateUrl: './keys-panel.html',
  styleUrls: ['../shared/form.scss', './keys-panel.scss'],
})
export class KeysPanel {
  /** The dialog that adds a key from a hardware token is open. */
  protected readonly addingToken = signal(false);

  protected tokenAdded(name: string | null): void {
    this.addingToken.set(false);
    if (name) {
      this.message.set(this.transloco.translate('keys.token.added', { name }));
      void this.store.load();
    }
  }

  protected readonly store = inject(KeysStore);
  protected readonly vault = inject(VaultStore);
  private readonly hostsStore = inject(HostsStore);
  private readonly transloco = inject(TranslocoService);

  protected readonly dialog = signal<Dialog | null>(null);
  protected readonly deleting = signal<SshKey | null>(null);
  protected readonly message = signal<string | null>(null);
  protected readonly dialogError = signal<string | null>(null);
  protected readonly busy = signal(false);
  protected readonly hosts = computed(() =>
    [...this.hostsStore.hosts()].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
  );

  protected name = '';
  protected kind = 'ed25519';
  protected comment = '';
  protected keyData = '';
  protected passphrase = '';
  protected hostId = '';
  protected format: 'openssh' | 'ppk' = 'openssh';
  protected removeOld = true;
  protected readonly usage = signal<KeyUsage[] | null>(null);
  protected readonly rotation = signal<RotationResult | null>(null);
  protected readonly exported = signal<string | null>(null);
  protected readonly deleteUsage = signal(0);

  constructor() {
    void this.store.initAgent().catch(() => undefined);
    void this.store.load().catch((err) => this.message.set(this.translate(err)));
    if (!this.hostsStore.loaded()) {
      void this.hostsStore.load().catch(() => undefined);
    }
  }

  protected open(d: Dialog): void {
    this.name = this.comment = this.keyData = this.passphrase = '';
    this.kind = 'ed25519';
    this.hostId = this.hosts()[0]?.id ?? '';
    if (d.kind === 'certificate') {
      this.keyData = d.key.certificate;
    }
    this.dialogError.set(null);
    this.usage.set(null);
    this.rotation.set(null);
    this.exported.set(null);
    this.format = 'openssh';
    this.dialog.set(d);
    if (d.kind === 'usage') {
      void this.store.usage(d.key.id).then(
        (u) => this.usage.set(u),
        (err) => this.dialogError.set(this.translate(err)),
      );
    }
  }

  protected askDelete(k: SshKey): void {
    this.deleteUsage.set(0);
    this.deleting.set(k);
    void this.store.usage(k.id).then((u) => this.deleteUsage.set(u.filter((x) => x.kind === 'host').length), () => undefined);
  }

  protected toggleAgent(on: boolean): void {
    void this.store.setAgent(on).catch((err) => this.message.set(this.translate(err)));
  }

  /** Copies the shell line that points programs at Shellvane's agent. */
  protected async copyAgent(path: string): Promise<void> {
    const line = path.startsWith('\\\\.\\pipe\\') ? `$env:SSH_AUTH_SOCK = "${path}"` : `export SSH_AUTH_SOCK='${path}'`;
    await writeClipboard(line).catch(() => undefined);
    this.message.set(this.transloco.translate('keys.agentCopied', { line }));
  }

  protected async copy(k: SshKey): Promise<void> {
    try {
      await navigator.clipboard.writeText(k.publicKey);
      this.message.set(this.transloco.translate('keys.copied', { name: k.name }));
    } catch {
      this.message.set(this.transloco.translate('keys.copyFailed'));
    }
  }

  protected async submit(): Promise<void> {
    const d = this.dialog();
    if (!d) {
      return;
    }
    this.busy.set(true);
    this.dialogError.set(null);
    try {
      if (d.kind === 'generate') {
        const [type, bits] = this.kind.split(':');
        await this.store.generate(this.name, type, Number(bits ?? 0), this.comment);
      } else if (d.kind === 'import') {
        await this.store.import(this.name, this.keyData, this.passphrase);
      } else if (d.kind === 'certificate') {
        await this.store.setCertificate(d.key.id, this.keyData);
      } else if (d.kind === 'usage') {
        this.rotation.set(await this.store.rotate(d.key.id, this.removeOld));
        return; // the results stay on screen
      } else if (d.kind === 'export') {
        const text = await this.store.export(d.key.id, this.format, this.passphrase);
        this.exported.set(text);
        await writeClipboard(text).catch(() => undefined);
        this.message.set(this.transloco.translate('keys.exported', { name: d.key.name }));
        return;
      } else {
        await this.store.deploy(d.key.id, this.hostId);
        const host = this.hosts().find((h) => h.id === this.hostId)?.name ?? '';
        this.message.set(this.transloco.translate('keys.deployed', { name: d.key.name, host }));
      }
      this.dialog.set(null);
    } catch (err) {
      this.dialogError.set(this.translate(err));
    } finally {
      this.busy.set(false);
      this.passphrase = '';
    }
  }

  protected async confirmDelete(): Promise<void> {
    const k = this.deleting();
    this.deleting.set(null);
    if (k) {
      await this.store.delete(k.id).catch((err) => this.message.set(this.translate(err)));
    }
  }

  protected certExpired(k: SshKey): boolean {
    return !!k.certValidBefore && new Date(k.certValidBefore).getTime() < Date.now();
  }

  protected certUntil(k: SshKey): string {
    if (!k.certValidBefore) {
      return '';
    }
    return new Intl.DateTimeFormat(this.transloco.getActiveLang(), { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(k.certValidBefore));
  }

  private translate(err: unknown): string {
    const e = toTranslatableError(err);
    return this.transloco.translate(e.key, e.params);
  }
}
