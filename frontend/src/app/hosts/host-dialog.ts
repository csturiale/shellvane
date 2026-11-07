import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { Host, Resolved } from '../core/backend/host-api';
import { Modal } from '../shared/modal';
import { ConnectionModel, ConnectionOptions, connectionModel, connectionSettings } from './connection-options';
import { inheritedCommands, inheritedValue, optionalText, parseLines, parsePort } from './form-utils';
import { KeysStore } from '../keys/keys.store';
import { VaultApi } from '../core/backend/vault-api';
import { VaultStore } from '../vault/vault.store';
import { allSchemes } from '../terminal/schemes';
import { HostsStore } from './hosts.store';
import { groupPaths } from './tree';
import { HelpLink } from '../guide/help-link';
import { VaultUnlockButton } from '../vault/vault-unlock-button';

export type HostDialogTarget = { mode: 'create'; groupId: string } | { mode: 'edit'; host: Host };

@Component({
  selector: 'app-host-dialog',
  imports: [VaultUnlockButton, NgComponentOutlet, HelpLink, FormsModule, TranslocoDirective, Modal, ConnectionOptions],
  templateUrl: './host-dialog.html',
  styleUrls: ['../shared/form.scss', './host-dialog.scss'],
})
export class HostDialog {
  protected readonly sharing = inject(Sharing);
  readonly target = input.required<HostDialogTarget>();
  readonly closed = output<void>();
  /** The group the host is in, for the shared-group note. */
  protected readonly noteGroup = computed(() => {
    const t = this.target();
    return t.mode === 'edit' ? t.host.groupId : t.groupId;
  });

  private readonly store = inject(HostsStore);
  protected readonly groups = computed(() => groupPaths(this.store.groups()));

  protected name = '';
  protected hostname = '';
  protected port: number | string | null = null;
  protected user = '';
  protected groupId = '';
  protected notes = '';
  /** Comma or space separated. */
  protected tags = '';

  protected startup = '';
  protected identityId = '';
  protected terminalTheme = '';
  protected conn: ConnectionModel = connectionModel(undefined);
  /** The TOTP seed is write-only: the dialog only knows whether one is set. */
  protected totpSeed = '';
  protected replaceTotp = false;
  protected removeTotp = false;
  protected readonly hasTotp = signal(false);
  protected readonly vault = inject(VaultStore);
  private readonly vaultApi = inject(VaultApi);
  protected readonly selfId = computed(() => {
    const t = this.target();
    return t.mode === 'edit' ? t.host.id : '';
  });
  protected readonly schemes = allSchemes;
  protected readonly keys = inject(KeysStore);
  protected readonly saving = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);
  private readonly inherited = signal<Resolved | null>(null);

  ngOnInit(): void {
    if (!this.keys.loaded()) {
      void this.keys.load().catch(() => undefined);
    }
    const t = this.target();
    if (t.mode === 'create') {
      this.groupId = t.groupId;
    } else {
      const h = t.host;
      this.name = h.name === h.hostname ? '' : h.name;
      this.hostname = h.hostname;
      this.port = h.settings.port ?? null;
      this.user = h.settings.user ?? '';
      this.startup = (h.settings.startupCommands ?? []).join('\n');
      this.identityId = h.settings.identityId ?? '';
      this.terminalTheme = h.settings.terminalThemeId ?? '';
      this.conn = connectionModel(h.settings);
      if (this.vault.state() === 'unlocked') {
        void this.vaultApi.hasTOTP(h.id).then((v) => this.hasTotp.set(v), () => undefined);
      }
      this.groupId = h.groupId;
      this.notes = h.notes;
      this.tags = (h.tags ?? []).join(', ');
    }
    void this.loadInherited();
  }

  /** Loads what a host in the selected group inherits. */
  protected async loadInherited(): Promise<void> {
    try {
      this.inherited.set(await this.store.inherited(this.groupId));
    } catch {
      this.inherited.set(null);
    }
  }

  protected placeholder(t: (key: string, params?: object) => string, field: 'user' | 'port'): string {
    const v = inheritedValue(this.inherited(), field, this.store.groups());
    if (!v) {
      return t('hosts.notSet');
    }
    return v.from ? t('hosts.inheritedFrom', { value: v.value, group: v.from }) : t('hosts.defaultValue', { value: v.value });
  }

  protected keyPlaceholder(t: (key: string, params?: object) => string): string {
    const r = this.inherited();
    const id = r?.settings.identityId;
    if (!id) {
      return t('hosts.keyDefault');
    }
    const name = this.keys.keys().find((k) => k.id === id)?.name ?? '?';
    const group = this.store.groups().find((g) => g.id === r?.source?.['identityId'])?.name ?? '';
    return t('hosts.keyInherited', { name, group });
  }

  protected startupPlaceholder(t: (key: string, params?: object) => string): string {
    const v = inheritedCommands(this.inherited(), this.store.groups());
    if (!v) {
      return t('hosts.startupHint');
    }
    return t('hosts.startupInherited', { group: v.from ?? '' }) + '\n' + v.value;
  }

  protected async save(): Promise<void> {
    const t = this.target();
    const base = t.mode === 'edit' ? t.host.settings : {};
    const input = {
      groupId: this.groupId,
      name: this.name,
      hostname: this.hostname,
      notes: this.notes,
      tags: this.tags.split(/[,\s]+/).filter(Boolean),
      favorite: t.mode === 'edit' ? t.host.favorite : false,
      settings: {
        ...base,
        user: optionalText(this.user),
        port: parsePort(this.port),
        startupCommands: parseLines(this.startup),
        identityId: this.identityId || null,
        terminalThemeId: this.terminalTheme || null,
        ...connectionSettings(this.conn),
      },
    };
    this.saving.set(true);
    this.error.set(null);
    try {
      let id: string;
      if (t.mode === 'create') {
        id = (await this.store.createHost(input)).id;
      } else {
        id = t.host.id;
        await this.store.updateHost(id, input);
      }
      if (this.totpSeed.trim()) {
        await this.vaultApi.setTOTP(id, this.totpSeed.trim());
      } else if (this.removeTotp) {
        await this.vaultApi.setTOTP(id, '');
      }
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.saving.set(false);
    }
  }
}
