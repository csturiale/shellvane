import { Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import {
  LucideChevronRight,
  LucideClipboardCopy,
  LucideClipboardPaste,
  LucideCopy,
  LucideHistory,
  LucideLayoutGrid,
  LucideNotebookText,
  LucideStar,
  LucideDownload,
  LucideFolder,
  LucideFolderOpen,
  LucideFolderPlus,
  LucideFolderSync,
  LucidePencil,
  LucidePlus,
  LucideSearch,
  LucideServer,
  LucidePower,
  LucideMonitor,
  LucideWifi,
  LucideSquareTerminal,
  LucideTrash2,
} from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { Group, Host } from '../core/backend/host-api';
import { ImportDialog } from '../import/import-dialog';
import { FULL_VIEWS } from '../layout/views';
import { Navigation } from '../layout/navigation';
import { SessionsStore } from '../sessions/sessions.store';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { colorVar } from './colors';
import { GroupDialog } from './group-dialog';
import type { GroupDialogTarget } from './group-dialog';
import { HostDialog } from './host-dialog';
import type { HostDialogTarget } from './host-dialog';
import { HostsStore } from './hosts.store';
import { TemplateDialog } from './template-dialog';
import { writeClipboard } from '../core/clipboard';
import { buildRows, subtreeIds } from './tree';
import { HelpLink } from '../guide/help-link';
import { ContextMenuService } from '../shared/context-menu.service';
import { Sharing } from '../extensions/sharing';
import { NgComponentOutlet } from '@angular/common';
import { Modal } from '../shared/modal';
import { VaultGate } from '../vault/vault-gate';

type DragItem = { kind: 'group' | 'host'; id: string };
type PendingDelete = { kind: 'group'; group: Group; message: string } | { kind: 'host'; host: Host; message: string };

/** Drop target id for the root level. */
const ROOT = '';

/** How many recent hosts are listed. */
const RECENT_COUNT = 5;

@Component({
  selector: 'app-hosts-panel',
  imports: [
    HelpLink,
    Modal,
    LucideCopy,
    LucideHistory,
    LucideLayoutGrid,
    LucideNotebookText,
    LucideStar,
    TranslocoDirective,
    GroupDialog,
    HostDialog,
    ConfirmDialog,
    ImportDialog,
    LucideChevronRight,
    LucideClipboardCopy,
    LucideClipboardPaste,
    LucideDownload,
    TemplateDialog,
    LucideFolder,
    LucideFolderOpen,
    LucideFolderPlus,
    LucideFolderSync,
    LucidePencil,
    LucidePlus,
    LucideSearch,
    LucideServer,
    LucidePower,
    LucideMonitor,
    LucideWifi,
    LucideSquareTerminal,
    LucideTrash2,
    NgComponentOutlet,
  ],
  templateUrl: './hosts-panel.html',
  styleUrl: './hosts-panel.scss',
})
export class HostsPanel {
  private readonly vaultGate = inject(VaultGate);
  protected readonly store = inject(HostsStore);
  private readonly sessions = inject(SessionsStore);
  protected readonly nav = inject(Navigation);
  private readonly transloco = inject(TranslocoService);

  protected readonly filter = signal('');
  protected readonly rows = computed(() =>
    buildRows(this.store.groups(), this.store.hosts(), this.store.expanded(), this.filter()),
  );
  protected readonly isEmpty = computed(
    () => this.store.loaded() && this.store.groups().length === 0 && this.store.hosts().length === 0,
  );

  protected readonly groupDialog = signal<GroupDialogTarget | null>(null);
  protected readonly hostDialog = signal<HostDialogTarget | null>(null);
  protected readonly pendingDelete = signal<PendingDelete | null>(null);
  protected readonly importing = signal(false);
  protected readonly importingTemplate = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly notice = signal<string | null>(null);

  private dragging: DragItem | null = null;
  protected readonly dropTarget = signal<string | null>(null);

  protected readonly colorVar = colorVar;

  ngOnDestroy(): void {
    clearInterval(this.statusTimer);
  }

  protected statusTitle(id: string): string {
    const st = this.store.status()[id];
    if (!st) {
      return '';
    }
    return st.state === 'up' ? this.transloco.translate('hosts.status.up', { ms: st.latencyMs }) : this.transloco.translate('hosts.status.' + st.state);
  }

  protected mosh(h: Host): void {
    if (FULL_VIEWS.includes(this.nav.view())) {
      this.nav.view.set('hosts');
    }
    void this.run(() => this.sessions.openMosh(h.id, h.name));
  }

  protected remoteDesktop(h: Host): Promise<void> {
    return this.run(() => this.store.remoteDesktop(h.id));
  }

  /** The group being exported, and whether its hosts go with it. */
  protected readonly exporting = signal<Group | null>(null);
  protected exportHosts = true;

  /** Copies the group, its subgroups and (optionally) their hosts for colleagues. */
  protected copyTemplate(): Promise<void> {
    const g = this.exporting();
    this.exporting.set(null);
    if (!g) {
      return Promise.resolve();
    }
    return this.run(async () => {
      await writeClipboard(await this.store.exportTemplate(g.id, this.exportHosts));
      this.notice.set(this.transloco.translate(this.exportHosts ? 'hosts.template.copiedWithHosts' : 'hosts.template.copied', { name: g.name }));
    });
  }

  protected templateClosed(message: string | null): void {
    this.importingTemplate.set(false);
    if (message) {
      this.notice.set(message);
    }
  }

  protected wake(h: Host): Promise<void> {
    return this.run(async () => {
      await this.store.wake(h.id);
      this.notice.set(this.transloco.translate('hosts.wakeSent', { name: h.name }));
    });
  }

  private statusTimer?: ReturnType<typeof setInterval>;

  private readonly menu = inject(ContextMenuService);
  protected readonly sharing = inject(Sharing);

  constructor() {
    void this.run(() => this.store.load());
    // Which hosts answer: now and every minute while the panel is shown.
    void this.store.refreshStatus().catch(() => undefined);
    void this.store.loadClients().catch(() => undefined);
    this.statusTimer = setInterval(() => void this.store.refreshStatus().catch(() => undefined), 60_000);
    // Requests from the command palette.
    effect(() => {
      this.nav.request();
      untracked(() => {
        if (this.nav.take('newHost')) {
          this.newHost();
        } else if (this.nav.take('newGroup')) {
          this.newGroup();
        } else if (this.nav.take('import')) {
          this.importing.set(true);
        }
      });
    });
  }

  protected newGroup(parentId = ROOT): void {
    this.groupDialog.set({ mode: 'create', parentId });
  }

  protected newHost(groupId = ROOT): void {
    this.hostDialog.set({ mode: 'create', groupId });
  }

  protected connect(host: Host): void {
    void this.run(() => this.sessions.openHost(host.id, host.name));
  }

  /** Hosts marked as favorites, by name. */
  protected readonly favorites = computed(() =>
    this.store
      .hosts()
      .filter((h) => h.favorite)
      .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
  );
  /** The last hosts connected to, newest first. */
  protected readonly recent = computed(() =>
    this.store
      .hosts()
      .filter((h) => h.lastUsed)
      .sort((a, b) => b.lastUsed.localeCompare(a.lastUsed))
      .slice(0, RECENT_COUNT),
  );
  /** Hosts shown by the filter ("#tag" or text), for "connect to all". */
  protected readonly filtered = computed(() => this.rows().filter((r) => r.kind === 'host').map((r) => (r.kind === 'host' ? r.host : null!)));
  protected readonly notesOf = signal<Host | null>(null);
  protected readonly notesHtml = signal('');

  /** A host's context menu. */
  protected hostMenu(ev: MouseEvent, h: Host): void {
    this.menu.open(ev, [
      { label: 'menu.connect', action: () => this.connect(h) },
      this.store.clients().mosh ? { label: 'hosts.mosh', action: () => this.mosh(h) } : null,
      h.settings.remoteDesktop ? { label: 'hosts.desktop', action: () => void this.remoteDesktop(h) } : null,
      { label: 'menu.openFiles', action: () => this.nav.openFiles(h.id) },
      h.settings.macAddress ? { label: 'hosts.wake', action: () => void this.wake(h) } : null,
      null,
      { label: 'menu.copyAddress', action: () => void writeClipboard(h.hostname).catch(() => undefined) },
      { label: 'hosts.favorite', action: () => this.toggleFavorite(h) },
      h.notes ? { label: 'hosts.host.notes', action: () => void this.showNotes(h) } : null,
      { label: 'hosts.duplicate', action: () => this.duplicate(h) },
      { label: 'hosts.actions.edit', action: () => this.editHost(h) },
      null,
      { label: 'hosts.actions.delete', danger: true, action: () => this.askDeleteHost(h) },
    ]);
  }

  /** A group's context menu. */
  protected groupMenu(ev: MouseEvent, g: Group): void {
    this.menu.open(ev, [
      { label: 'menu.connectAll', action: () => this.connectGroup(g) },
      null,
      { label: 'menu.newHostHere', action: () => this.newHost(g.id) },
      { label: 'menu.newSubgroup', action: () => this.newGroup(g.id) },
      { label: 'hosts.template.copy', action: () => this.exporting.set(g) },
      ...this.sharing.groupMenu(g, (work) => void this.run(work)),
      { label: 'hosts.actions.edit', action: () => this.editGroup(g) },
      null,
      { label: 'hosts.actions.delete', danger: true, action: () => void this.askDeleteGroup(g) },
    ]);
  }

  protected toggleFavorite(host: Host): void {
    void this.run(() => this.store.setFavorite(host.id, !host.favorite));
  }

  protected duplicate(host: Host): void {
    void this.run(() => this.store.duplicate(host.id, this.transloco.translate('hosts.copySuffix')));
  }

  /** Opens every host of a group (or of the filter) in one tab, in a grid. */
  protected connectAll(hosts: Host[]): void {
    if (hosts.length > 0) {
      void this.run(() => this.sessions.openMany(hosts, hosts.length > 1));
    }
  }

  protected connectGroup(group: Group): void {
    this.connectAll(this.store.hostsIn(group.id));
  }

  protected filterTag(tag: string): void {
    this.filter.set('#' + tag);
  }

  /** Shows a host's notes, rendered from Markdown. */
  protected async showNotes(host: Host): Promise<void> {
    this.notesOf.set(host);
    const { render } = await import('../guide/guide-render');
    this.notesHtml.set(render(host.notes));
  }

  protected editGroup(group: Group): void {
    this.groupDialog.set({ mode: 'edit', group });
  }

  protected editHost(host: Host): void {
    this.hostDialog.set({ mode: 'edit', host });
  }

  protected async askDeleteGroup(group: Group): Promise<void> {
    await this.run(async () => {
      const c = await this.store.groupContents(group.id);
      const message =
        c.groups + c.hosts === 0
          ? this.transloco.translate('hosts.delete.emptyGroup', { name: group.name })
          : this.transloco.translate('hosts.delete.groupWithContents', {
              name: group.name,
              groups: c.groups,
              hosts: c.hosts,
            });
      this.pendingDelete.set({ kind: 'group', group, message });
    });
  }

  protected askDeleteHost(host: Host): void {
    this.pendingDelete.set({
      kind: 'host',
      host,
      message: this.transloco.translate('hosts.delete.host', { name: host.name }),
    });
  }

  protected async confirmDelete(): Promise<void> {
    const p = this.pendingDelete();
    this.pendingDelete.set(null);
    if (!p) {
      return;
    }
    await this.run(() => (p.kind === 'group' ? this.store.deleteGroup(p.group.id) : this.store.deleteHost(p.host.id)));
  }

  protected onFilter(event: Event): void {
    this.filter.set((event.target as HTMLInputElement).value);
  }

  // Drag and drop: hosts and groups can be dropped on a group or on the root.

  protected onDragStart(event: DragEvent, item: DragItem): void {
    this.dragging = item;
    event.dataTransfer?.setData('text/plain', item.id);
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
    }
  }

  protected onDragEnd(): void {
    this.dragging = null;
    this.dropTarget.set(null);
  }

  protected onDragOver(event: DragEvent, groupId: string): void {
    if (!this.canDrop(groupId)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move';
    }
    this.dropTarget.set(groupId);
  }

  protected async onDrop(event: DragEvent, groupId: string): Promise<void> {
    event.preventDefault();
    event.stopPropagation();
    const item = this.dragging;
    this.onDragEnd();
    if (!item || !this.canDrop(groupId, item)) {
      return;
    }
    await this.run(() =>
      item.kind === 'host' ? this.store.moveHost(item.id, groupId) : this.store.moveGroup(item.id, groupId),
    );
  }

  private canDrop(groupId: string, item = this.dragging): boolean {
    if (!item) {
      return false;
    }
    if (item.kind === 'host') {
      return this.store.hosts().find((h) => h.id === item.id)?.groupId !== groupId;
    }
    const group = this.store.groups().find((g) => g.id === item.id);
    return !!group && group.parentId !== groupId && !subtreeIds(this.store.groups(), item.id).has(groupId);
  }

  /** Runs an action and shows its error, translated, under the toolbar. */
  private async run(action: () => Promise<unknown>): Promise<void> {
    this.error.set(null);
    try {
      await this.vaultGate.run(action);
    } catch (err) {
      const e = toTranslatableError(err);
      this.error.set(this.transloco.translate(e.key, e.params));
    }
  }
}
