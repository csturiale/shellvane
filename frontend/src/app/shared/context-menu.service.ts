import { Injectable, signal } from '@angular/core';

/** An entry of a context menu; a null entry is a separator. */
export interface MenuItem {
  /** Translation key. */
  label: string;
  params?: Record<string, unknown>;
  action: () => void;
  disabled?: boolean;
  danger?: boolean;
}

export interface OpenMenu {
  x: number;
  y: number;
  items: (MenuItem | null)[];
}

/** Shows one context menu at a time, rendered by <app-context-menu>. */
@Injectable({ providedIn: 'root' })
export class ContextMenuService {
  readonly menu = signal<OpenMenu | null>(null);

  /** Opens a menu at the pointer of a contextmenu event. */
  open(ev: MouseEvent, items: (MenuItem | null)[]): void {
    ev.preventDefault();
    ev.stopPropagation();
    this.menu.set({ x: ev.clientX, y: ev.clientY, items: tidy(items) });
  }

  close(): void {
    this.menu.set(null);
  }
}

/** Drops leading, trailing and repeated separators. */
export function tidy(items: (MenuItem | null)[]): (MenuItem | null)[] {
  const out: (MenuItem | null)[] = [];
  for (const item of items) {
    if (item === null && (out.length === 0 || out[out.length - 1] === null)) {
      continue;
    }
    out.push(item);
  }
  while (out.length > 0 && out[out.length - 1] === null) {
    out.pop();
  }
  return out;
}
