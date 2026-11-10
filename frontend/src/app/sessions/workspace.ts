import { Component, DestroyRef, computed, inject, signal } from '@angular/core';

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

  // Moving panes by dragging their header. Pointer events, not HTML drag
  // and drop: in the macOS app the web view's native file-drop layer gets
  // in the way of HTML drags, and pointer events behave the same everywhere.
  private drag: { pane: number; pointer: number; x: number; y: number; moving: boolean } | null = null;
  protected readonly dropHint = signal<{ target: number; zone: DropZone } | null>(null);
  /** Tab under the pointer; 0 is the empty part of the tab bar (new tab). */
  protected readonly dropTab = signal<number | null>(null);
  /** The dragged pane's title next to the pointer. */
  protected readonly dragGhost = signal<{ title: string; x: number; y: number } | null>(null);
  /** Hovering a tab while dragging opens it, to drop at a precise place. */
  private hoverTimer: ReturnType<typeof setTimeout> | undefined;
  private hoverTab: number | null = null;

  constructor() {
    inject(DestroyRef).onDestroy(() => this.cancelDrag());
  }

  protected onPanePointerDown(ev: PointerEvent, paneKey: number): void {
    const el = ev.target as HTMLElement;
    if (ev.button !== 0 || !el.closest?.('.pane-header') || el.closest('button, input, select, a, [role="button"]')) {
      return;
    }
    this.drag = { pane: paneKey, pointer: ev.pointerId, x: ev.clientX, y: ev.clientY, moving: false };
    document.addEventListener('pointermove', this.onPointerMove);
    document.addEventListener('pointerup', this.onPointerUp);
    document.addEventListener('pointercancel', this.cancelDrag);
    document.addEventListener('keydown', this.onDragKey, true);
  }

  private readonly onPointerMove = (ev: PointerEvent): void => {
    const d = this.drag;
    if (!d || ev.pointerId !== d.pointer) {
      return;
    }
    if (!d.moving) {
      if (Math.hypot(ev.clientX - d.x, ev.clientY - d.y) < 6) {
        return;
      }
      d.moving = true;
      document.body.classList.add('pane-dragging');
      getSelection()?.removeAllRanges();
    }
    ev.preventDefault();
    this.dragGhost.set({ title: this.store.pane(d.pane)?.title ?? '', x: ev.clientX, y: ev.clientY });
    this.track(ev.clientX, ev.clientY);
  };

  /** What is under the pointer: a pane (and which side), a tab, the bar. */
  private track(x: number, y: number): void {
    const d = this.drag!;
    const under = document.elementFromPoint(x, y) as HTMLElement | null;
    const tabEl = under?.closest<HTMLElement>('[data-tab-key]');
    const barEl = under?.closest<HTMLElement>('.tabs');
    const paneEl = under?.closest<HTMLElement>('[data-pane-key]');
    if (tabEl || barEl) {
      const tabKey = tabEl ? Number(tabEl.dataset['tabKey']) : 0;
      this.dropHint.set(null);
      this.dropTab.set(tabKey);
      this.hoverOpen(tabKey);
      return;
    }
    this.hoverOpen(null);
    this.dropTab.set(null);
    const target = paneEl ? Number(paneEl.dataset['paneKey']) : null;
    if (target === null || target === d.pane) {
      this.dropHint.set(null);
      return;
    }
    const box = paneEl!.getBoundingClientRect();
    const zone = dropZone((x - box.left) / box.width, (y - box.top) / box.height);
    const h = this.dropHint();
    if (!h || h.target !== target || h.zone !== zone) {
      this.dropHint.set({ target, zone });
    }
  }

  /** Opens a tab hovered for a moment during a drag (not the new-tab area). */
  private hoverOpen(tabKey: number | null): void {
    if (tabKey === this.hoverTab) {
      return;
    }
    clearTimeout(this.hoverTimer);
    this.hoverTab = tabKey;
    if (tabKey && tabKey !== this.store.activeKey()) {
      this.hoverTimer = setTimeout(() => {
        if (this.drag?.moving && this.hoverTab === tabKey) {
          this.store.activate(tabKey);
        }
      }, 600);
    }
  }

  private readonly onPointerUp = (ev: PointerEvent): void => {
    const d = this.drag;
    if (!d || ev.pointerId !== d.pointer) {
      return;
    }
    if (d.moving) {
      const hint = this.dropHint();
      const tab = this.dropTab();
      if (tab === 0) {
        this.store.movePaneToNewTab(d.pane);
      } else if (tab !== null) {
        this.store.movePaneToTab(d.pane, tab);
      } else if (hint) {
        this.store.movePane(d.pane, hint.target, hint.zone);
      }
    }
    this.cancelDrag();
  };

  private readonly onDragKey = (ev: KeyboardEvent): void => {
    if (ev.key === 'Escape' && this.drag?.moving) {
      ev.preventDefault();
      ev.stopPropagation();
      this.cancelDrag();
    }
  };

  protected readonly cancelDrag = (): void => {
    document.removeEventListener('pointermove', this.onPointerMove);
    document.removeEventListener('pointerup', this.onPointerUp);
    document.removeEventListener('pointercancel', this.cancelDrag);
    document.removeEventListener('keydown', this.onDragKey, true);
    document.body.classList.remove('pane-dragging');
    clearTimeout(this.hoverTimer);
    this.hoverTab = null;
    this.drag = null;
    this.dragGhost.set(null);
    this.dropHint.set(null);
    this.dropTab.set(null);
  };

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
