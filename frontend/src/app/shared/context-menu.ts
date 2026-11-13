import { Component, ElementRef, HostListener, afterRenderEffect, inject, signal, viewChild } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { ContextMenuService, MenuItem } from './context-menu.service';

/** The context menu opened through ContextMenuService. */
@Component({
  selector: 'app-context-menu',
  imports: [TranslocoDirective],
  templateUrl: './context-menu.html',
  styleUrl: './context-menu.scss',
})
export class ContextMenu {
  protected readonly service = inject(ContextMenuService);
  private readonly list = viewChild<ElementRef<HTMLElement>>('list');
  /** Position after keeping the menu inside the window. */
  protected readonly position = signal<{ x: number; y: number } | null>(null);
  /** The entry whose second level is open, and where that level goes. */
  protected readonly sub = signal<{ index: number; x: number; y: number; left: boolean } | null>(null);
  private readonly subList = viewChild<ElementRef<HTMLElement>>('subList');

  constructor() {
    afterRenderEffect(() => {
      const menu = this.service.menu();
      const el = this.list()?.nativeElement;
      if (!menu || !el) {
        this.position.set(null);
        this.sub.set(null);
        return;
      }
      const r = el.getBoundingClientRect();
      const x = Math.max(4, Math.min(menu.x, window.innerWidth - r.width - 4));
      const y = Math.max(4, Math.min(menu.y, window.innerHeight - r.height - 4));
      const cur = this.position();
      if (!cur || cur.x !== x || cur.y !== y) {
        this.position.set({ x, y });
      }
      (el.querySelector('button:not([disabled])') as HTMLElement | null)?.focus();
    });
  }

  protected run(item: MenuItem, index: number, ev?: Event): void {
    if (item.children) {
      this.openSub(index, ev?.currentTarget as HTMLElement | undefined, true);
      return;
    }
    this.service.close();
    this.sub.set(null);
    if (!item.disabled) {
      item.action();
    }
  }

  /** Opens the second level of an entry beside it (on the left when there is no room). */
  protected openSub(index: number, button: HTMLElement | undefined, focus = false): void {
    if (!button) {
      return;
    }
    const r = button.getBoundingClientRect();
    const left = r.right + 200 > window.innerWidth;
    this.sub.set({ index, x: left ? r.left : r.right, y: r.top - 5, left });
    if (focus) {
      setTimeout(() => (this.subList()?.nativeElement.querySelector('button:not([disabled])') as HTMLElement | null)?.focus());
    }
  }

  protected windowWidth(): number {
    return window.innerWidth;
  }

  /** Hovering an entry opens its second level, or closes the one open. */
  protected hover(item: MenuItem, index: number, ev: Event): void {
    if (item.children) {
      this.openSub(index, ev.currentTarget as HTMLElement);
    } else if (this.sub()) {
      this.sub.set(null);
    }
  }

  @HostListener('document:keydown.escape')
  @HostListener('window:blur')
  @HostListener('window:resize')
  protected dismiss(): void {
    if (this.service.menu()) {
      this.service.close();
    }
  }

  @HostListener('document:pointerdown', ['$event'])
  protected outside(ev: PointerEvent): void {
    const el = this.list()?.nativeElement;
    const sub = this.subList()?.nativeElement;
    if (this.service.menu() && el && !el.contains(ev.target as Node) && !sub?.contains(ev.target as Node)) {
      this.service.close();
      this.sub.set(null);
    }
  }

  /** Arrow keys move between the entries; right opens a second level, left goes back. */
  protected onKeydown(ev: KeyboardEvent, level: 'main' | 'sub' = 'main'): void {
    const box = (level === 'sub' ? this.subList() : this.list())?.nativeElement;
    if (level === 'main' && ev.key === 'ArrowRight') {
      const at = document.activeElement as HTMLElement;
      const index = Number(at?.dataset['index']);
      const item = this.service.menu()?.items[index];
      if (item?.children) {
        ev.preventDefault();
        this.openSub(index, at, true);
      }
      return;
    }
    if (level === 'sub' && ev.key === 'ArrowLeft') {
      ev.preventDefault();
      const index = this.sub()?.index;
      this.sub.set(null);
      (this.list()?.nativeElement.querySelector(`button[data-index="${index}"]`) as HTMLElement | null)?.focus();
      return;
    }
    if (ev.key !== 'ArrowDown' && ev.key !== 'ArrowUp') {
      return;
    }
    ev.preventDefault();
    const buttons = Array.from(box?.querySelectorAll<HTMLElement>('button:not([disabled])') ?? []);
    const at = buttons.indexOf(document.activeElement as HTMLElement);
    const next = ev.key === 'ArrowDown' ? (at + 1) % buttons.length : (at - 1 + buttons.length) % buttons.length;
    buttons[next]?.focus();
  }
}
