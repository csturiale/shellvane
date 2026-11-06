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

  constructor() {
    afterRenderEffect(() => {
      const menu = this.service.menu();
      const el = this.list()?.nativeElement;
      if (!menu || !el) {
        this.position.set(null);
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

  protected run(item: MenuItem): void {
    this.service.close();
    if (!item.disabled) {
      item.action();
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
    if (this.service.menu() && el && !el.contains(ev.target as Node)) {
      this.service.close();
    }
  }

  /** Arrow keys move between the entries. */
  protected onKeydown(ev: KeyboardEvent): void {
    if (ev.key !== 'ArrowDown' && ev.key !== 'ArrowUp') {
      return;
    }
    ev.preventDefault();
    const buttons = Array.from(this.list()?.nativeElement.querySelectorAll<HTMLElement>('button:not([disabled])') ?? []);
    const at = buttons.indexOf(document.activeElement as HTMLElement);
    const next = ev.key === 'ArrowDown' ? (at + 1) % buttons.length : (at - 1 + buttons.length) % buttons.length;
    buttons[next]?.focus();
  }
}
