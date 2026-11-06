import { Component, ElementRef, computed, inject, input, output, signal, viewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideChevronDown } from '@lucide/angular';

import { HostsStore } from './hosts.store';
import { groupPaths } from './tree';

/** An entry of the picker. */
export interface PickerItem {
  id: string;
  name: string;
  detail: string;
  group: string;
}

/** Most entries listed at once: the search narrows the rest. */
const MAX_SHOWN = 200;

/**
 * A searchable host picker, for lists too long for a select: it filters by
 * name, address, user, group path and tags.
 */
@Component({
  selector: 'app-host-picker',
  imports: [TranslocoDirective, FormsModule, LucideChevronDown],
  host: {
    '(document:pointerdown)': 'outside($event)',
  },
  templateUrl: './host-picker.html',
  styleUrl: './host-picker.scss',
})
export class HostPicker {
  private readonly store = inject(HostsStore);
  private readonly el = inject(ElementRef<HTMLElement>);

  /** The chosen ID, or "" for none. */
  readonly value = input('');
  /** Extra entries listed first (e.g. this computer). */
  readonly extra = input<PickerItem[]>([]);
  readonly placeholder = input('');
  readonly label = input('');
  readonly disabled = input(false);
  readonly picked = output<string>();

  protected readonly open = signal(false);
  protected readonly query = signal('');
  protected readonly active = signal(0);
  private readonly search = viewChild<ElementRef<HTMLInputElement>>('search');
  private readonly listEl = viewChild<ElementRef<HTMLElement>>('list');

  private readonly all = computed<PickerItem[]>(() => {
    const paths = new Map(groupPaths(this.store.groups()).map((g) => [g.id, g.path]));
    const hosts = this.store
      .hosts()
      .map((h) => ({
        id: h.id,
        name: h.name,
        detail: h.hostname,
        group: paths.get(h.groupId) ?? '',
        tags: (h.tags ?? []).join(' '),
      }))
      .sort((a, b) => a.group.localeCompare(b.group) || a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
    return [...this.extra(), ...hosts];
  });

  protected readonly items = computed(() => {
    const words = this.query().toLowerCase().split(/\s+/).filter(Boolean);
    const list = this.all().filter((item) => {
      const text = `${item.name} ${item.detail} ${item.group} ${(item as { tags?: string }).tags ?? ''}`.toLowerCase();
      return words.every((w) => text.includes(w));
    });
    return { shown: list.slice(0, MAX_SHOWN), more: Math.max(0, list.length - MAX_SHOWN) };
  });

  protected readonly current = computed(() => this.all().find((i) => i.id === this.value()) ?? null);

  protected toggle(): void {
    if (this.disabled()) {
      return;
    }
    this.open.update((o) => !o);
    if (this.open()) {
      this.query.set('');
      this.active.set(0);
      setTimeout(() => this.search()?.nativeElement.focus());
    }
  }

  protected choose(item: PickerItem): void {
    this.open.set(false);
    if (item.id !== this.value()) {
      this.picked.emit(item.id);
    }
  }

  protected setQuery(q: string): void {
    this.query.set(q);
    this.active.set(0);
  }

  protected onKeydown(ev: KeyboardEvent): void {
    const shown = this.items().shown;
    switch (ev.key) {
      case 'ArrowDown':
        this.active.update((i) => Math.min(i + 1, shown.length - 1));
        break;
      case 'ArrowUp':
        this.active.update((i) => Math.max(i - 1, 0));
        break;
      case 'Enter':
        if (shown[this.active()]) {
          this.choose(shown[this.active()]);
        }
        break;
      case 'Escape':
        this.open.set(false);
        break;
      default:
        return;
    }
    ev.preventDefault();
    setTimeout(() => this.listEl()?.nativeElement.querySelector('.active')?.scrollIntoView({ block: 'nearest' }));
  }

  protected outside(ev: PointerEvent): void {
    if (this.open() && !this.el.nativeElement.contains(ev.target as Node)) {
      this.open.set(false);
    }
  }
}
