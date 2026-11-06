import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { Group, Resolved } from '../core/backend/host-api';
import { Modal } from '../shared/modal';
import { ConnectionModel, ConnectionOptions, connectionModel, connectionSettings } from './connection-options';
import { GROUP_COLORS, colorVar } from './colors';
import { inheritedCommands, inheritedValue, optionalText, parseLines, parsePort } from './form-utils';
import { KeysStore } from '../keys/keys.store';
import { allSchemes } from '../terminal/schemes';
import { HostsStore } from './hosts.store';
import { groupPaths, subtreeIds } from './tree';
import { HelpLink } from '../guide/help-link';

export type GroupDialogTarget = { mode: 'create'; parentId: string } | { mode: 'edit'; group: Group };

@Component({
  selector: 'app-group-dialog',
  imports: [NgComponentOutlet, HelpLink, FormsModule, TranslocoDirective, Modal, ConnectionOptions],
  templateUrl: './group-dialog.html',
  styleUrls: ['../shared/form.scss', './group-dialog.scss'],
})
export class GroupDialog {
  protected readonly sharing = inject(Sharing);
  readonly target = input.required<GroupDialogTarget>();
  readonly closed = output<void>();
  /** The group itself, or the parent of a new one, for the shared-group note. */
  protected readonly noteGroup = computed(() => {
    const t = this.target();
    return t.mode === 'edit' ? t.group.id : t.parentId;
  });

  private readonly store = inject(HostsStore);
  protected readonly colors = GROUP_COLORS;
  protected readonly colorVar = colorVar;

  protected name = '';
  protected parentId = '';
  protected color = '';
  protected user = '';
  protected port: number | string | null = null;

  protected startup = '';
  protected identityId = '';
  protected terminalTheme = '';
  protected conn: ConnectionModel = connectionModel(undefined);
  protected readonly schemes = allSchemes;
  protected readonly keys = inject(KeysStore);
  protected readonly saving = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);
  private readonly inherited = signal<Resolved | null>(null);

  /** Valid parents: every group except the edited one and its descendants. */
  protected readonly parents = computed(() => {
    const t = this.target();
    const excluded = t.mode === 'edit' ? subtreeIds(this.store.groups(), t.group.id) : new Set<string>();
    return groupPaths(this.store.groups()).filter((p) => !excluded.has(p.id));
  });

  ngOnInit(): void {
    if (!this.keys.loaded()) {
      void this.keys.load().catch(() => undefined);
    }
    const t = this.target();
    if (t.mode === 'create') {
      this.parentId = t.parentId;
    } else {
      this.name = t.group.name;
      this.parentId = t.group.parentId;
      this.color = t.group.color;
      this.user = t.group.settings.user ?? '';
      this.startup = (t.group.settings.startupCommands ?? []).join('\n');
      this.identityId = t.group.settings.identityId ?? '';
      this.terminalTheme = t.group.settings.terminalThemeId ?? '';
      this.conn = connectionModel(t.group.settings);
      this.port = t.group.settings.port ?? null;
    }
    void this.loadInherited();
  }

  /** Loads what the group would inherit from its parent chain. */
  protected async loadInherited(): Promise<void> {
    try {
      this.inherited.set(await this.store.inherited(this.parentId));
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
    const base = t.mode === 'edit' ? t.group.settings : {};
    const input = {
      parentId: this.parentId,
      name: this.name,
      color: this.color,
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
      if (t.mode === 'create') {
        await this.store.createGroup(input);
      } else {
        await this.store.updateGroup(t.group.id, input);
      }
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.saving.set(false);
    }
  }
}
