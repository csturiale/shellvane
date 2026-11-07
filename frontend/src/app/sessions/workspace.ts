import { Component, computed, inject, signal } from '@angular/core';

const PANE_MIME = 'application/x-shellvane-pane';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideArrowLeftToLine, LucideExternalLink, LucideRadioTower, LucideSquareTerminal, LucideX } from '@lucide/angular';

import { Divider, DropZone, Rect, arrange, dropZone, panesOf } from './layout';
import { colorVar } from '../hosts/colors';
import { Pane, SessionsStore, Tab, labelOf } from './sessions.store';
import { SessionPrefs } from './session-prefs';
import { TerminalPrefs } from '../terminal/terminal-prefs';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { FormsModule } from '@angular/forms';
import { combo, shortcutAction } from './shortcuts';
import { TerminalView } from './terminal-view';
import { WorkspaceMenu } from './workspace-menu';
import { NewTabMenu } from './new-tab-menu';
import { BroadcastBar } from '../broadcast/broadcast-bar';
import { ContextMenuService } from '../shared/context-menu.service';
import { detachedToken } from './detached';

/** Tab bar plus the panes of every tab. Hidden tabs stay mounted. */
@Component({
  selector: 'app-workspace',
  imports: [BroadcastBar, TranslocoDirective, FormsModule, ConfirmDialog, TerminalView, WorkspaceMenu, NewTabMenu, LucideSquareTerminal, LucideX, LucideExternalLink, LucideArrowLeftToLine, LucideRadioTower],
  host: { '(document:keydown)': 'onKeydown($event)' },
  templateUrl: './workspace.html',
  styleUrl: './workspace.scss',
})
export class Workspace {
  protected readonly store = inject(SessionsStore);
  protected readonly prefs = inject(SessionPrefs);
  protected readonly termPrefs = inject(TerminalPrefs);
  private readonly menu = inject(ContextMenuService);
  protected readonly colorVar = colorVar;
  /** This window shows tabs moved out of the main window. */
  protected readonly detachedWindow = detachedToken() !== null;
  protected readonly label = labelOf;

  protected readonly layouts = computed(() => new Map(this.store.tabs().map((t) => [t.key, arrange(t.layout)])));

  protected readonly keys = { splitRight: combo('D'), splitDown: combo('E'), broadcast: combo('B'), newLocal: combo('T') };

  /** Every pane with its tab and rectangle, in creation order (stable DOM). */
  protected readonly placed = computed(() => {
    const out: { pane: Pane; tab: Tab; rect: Rect; active: boolean; count: number }[] = [];
    const where = new Map<number, Tab>();
    for (const t of this.store.tabs()) {
      for (const k of panesOf(t.layout)) {
        where.set(k, t);
      }
    }
    for (const pane of this.store.panes()) {
      const tab = where.get(pane.key);
      let rect = tab && this.layouts().get(tab.key)?.panes.get(pane.key);
      if (tab && rect) {
        let active = tab.key === this.store.activeKey();
        if (tab.maximized !== null) {
          // The maximized pane fills the tab; the others stay mounted, hidden.
          active = active && tab.maximized === pane.key;
          rect = tab.maximized === pane.key ? { x: 0, y: 0, w: 100, h: 100 } : rect;
        }
        out.push({ pane, tab, rect, active, count: tab.maximized !== null ? 1 : panesOf(tab.layout).length });
      }
    }
    return out;
  });

  // Moving panes by dragging their header.
  private dragged: number | null = null;
  protected readonly dropHint = signal<{ target: number; zone: DropZone } | null>(null);
  /** Tab under the pointer; 0 is the empty part of the tab bar (new tab). */
  protected readonly dropTab = signal<number | null>(null);

  protected onPaneDragStart(ev: DragEvent, paneKey: number): void {
    if (!(ev.target as HTMLElement).closest?.('.pane-header')) {
      return;
    }
    this.dragged = paneKey;
    ev.dataTransfer?.setData(PANE_MIME, String(paneKey));
    if (ev.dataTransfer) {
      ev.dataTransfer.effectAllowed = 'move';
    }
  }

  protected onPaneDragOver(ev: DragEvent, target: number): void {
    if (this.dragged === null || this.dragged === target) {
      this.dropHint.set(null);
      return;
    }
    ev.preventDefault();
    const box = (ev.currentTarget as HTMLElement).getBoundingClientRect();
    const zone = dropZone((ev.clientX - box.left) / box.width, (ev.clientY - box.top) / box.height);
    const h = this.dropHint();
    if (!h || h.target !== target || h.zone !== zone) {
      this.dropHint.set({ target, zone });
    }
  }

  protected onPaneDrop(ev: DragEvent, target: number): void {
    const hint = this.dropHint();
    if (this.dragged !== null && hint?.target === target) {
      ev.preventDefault();
      this.store.movePane(this.dragged, target, hint.zone);
    }
    this.endDrag();
  }

  protected onTabDragOver(ev: DragEvent, tabKey: number): void {
    if (this.dragged === null) {
      return;
    }
    ev.preventDefault();
    ev.stopPropagation();
    this.dropTab.set(tabKey);
  }

  protected onTabDrop(ev: DragEvent, tabKey: number): void {
    if (this.dragged !== null) {
      ev.preventDefault();
      ev.stopPropagation();
      if (tabKey === 0) {
        this.store.movePaneToNewTab(this.dragged);
      } else {
        this.store.movePaneToTab(this.dragged, tabKey);
      }
    }
    this.endDrag();
  }

  protected endDrag(): void {
    this.dragged = null;
    this.dropHint.set(null);
    this.dropTab.set(null);
  }

  // Renaming a tab (double click on its title).
  protected readonly renaming = signal<number | null>(null);
  protected renameText = '';
  /** Tab waiting for confirmation before it is closed. */
  protected readonly closing = signal<Tab[] | null>(null);

  protected startRename(tab: Tab, title: string): void {
    this.renameText = tab.title || title;
    this.renaming.set(tab.key);
  }

  protected finishRename(tab: Tab): void {
    if (this.renaming() === tab.key) {
      this.store.renameTab(tab.key, this.renameText);
      this.renaming.set(null);
    }
  }

  /** Closes a tab, asking first when it has connected sessions. */
  /** Moves the tab, sessions included, to a window of its own. */
  protected detach(tab: Tab): void {
    void this.store.detachTab(tab.key).catch(() => undefined);
  }

  /** Gives the tab back to the main window (in a detached window). */
  protected reattach(tab: Tab): void {
    void this.store.reattachTab(tab.key).catch(() => undefined);
  }

  protected close(tab: Tab): void {
    this.closeMany([tab]);
  }

  /** Closes tabs, asking first when some have connected sessions. */
  protected closeMany(tabs: Tab[]): void {
    const connected = tabs.some((tab) => panesOf(tab.layout).some((k) => this.store.pane(k)?.state === 'connected'));
    if (connected && this.termPrefs.confirmClose()) {
      this.closing.set(tabs);
    } else {
      tabs.forEach((tab) => void this.store.closeTab(tab.key));
    }
  }

  protected confirmClose(): void {
    const tabs = this.closing() ?? [];
    this.closing.set(null);
    tabs.forEach((tab) => void this.store.closeTab(tab.key));
  }

  /** The tab's context menu. */
  protected tabMenu(ev: MouseEvent, tab: Tab): void {
    const tabs = this.store.tabs();
    const i = tabs.findIndex((t) => t.key === tab.key);
    const title = this.store.pane(tab.focus)?.title ?? '';
    this.menu.open(ev, [
      { label: 'menu.rename', action: () => this.startRename(tab, title) },
      this.detachedWindow
        ? { label: 'menu.reattach', action: () => this.reattach(tab) }
        : { label: 'sessions.detach', action: () => this.detach(tab) },
      null,
      { label: 'sessions.closeTab', action: () => this.close(tab) },
      { label: 'menu.closeOthers', disabled: tabs.length < 2, action: () => this.closeMany(tabs.filter((t) => t.key !== tab.key)) },
      { label: 'menu.closeLeft', disabled: i <= 0, action: () => this.closeMany(tabs.slice(0, i)) },
      { label: 'menu.closeRight', disabled: i >= tabs.length - 1, action: () => this.closeMany(tabs.slice(i + 1)) },
      { label: 'menu.closeAll', danger: true, action: () => this.closeMany(tabs) },
    ]);
  }

  protected hasBell(tab: Tab): boolean {
    return panesOf(tab.layout).some((k) => this.store.pane(k)?.bell);
  }

  protected paneCount(tab: Tab): number {
    return panesOf(tab.layout).length;
  }

  protected onKeydown(ev: KeyboardEvent): void {
    const action = shortcutAction(ev);
    if (!action || !this.store.active()) {
      return;
    }
    ev.preventDefault();
    switch (action) {
      case 'splitRight':
        void this.store.split('row');
        break;
      case 'splitDown':
        void this.store.split('column');
        break;
      case 'broadcast':
        this.store.toggleBroadcast(this.store.active()!.key);
        break;
      case 'closePane':
        void this.store.closePane(this.store.active()!.focus);
        break;
      case 'focusLeft':
        this.store.moveFocus('left');
        break;
      case 'focusRight':
        this.store.moveFocus('right');
        break;
      case 'focusUp':
        this.store.moveFocus('up');
        break;
      case 'focusDown':
        this.store.moveFocus('down');
        break;
    }
  }

  protected startDrag(ev: PointerEvent, tab: Tab, d: Divider, area: HTMLElement): void {
    ev.preventDefault();
    const target = ev.target as HTMLElement;
    target.setPointerCapture(ev.pointerId);
    const bounds = area.getBoundingClientRect();
    const toRatio = (e: PointerEvent, a: Rect) =>
      d.dir === 'row'
        ? (((e.clientX - bounds.left) / bounds.width) * 100 - a.x) / a.w
        : (((e.clientY - bounds.top) / bounds.height) * 100 - a.y) / a.h;
    const move = (e: PointerEvent) => this.store.resizeSplit(tab.key, d.path, toRatio(e, d.area));
    const up = () => {
      target.removeEventListener('pointermove', move);
      target.removeEventListener('pointerup', up);
    };
    target.addEventListener('pointermove', move);
    target.addEventListener('pointerup', up);
  }
}
