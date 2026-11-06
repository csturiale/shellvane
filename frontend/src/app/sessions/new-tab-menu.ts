import { Component, ElementRef, computed, inject, signal, viewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucidePlus } from '@lucide/angular';

import { Host } from '../core/backend/host-api';
import { HostsStore } from '../hosts/hosts.store';
import { groupPaths } from '../hosts/tree';
import { SessionsStore } from './sessions.store';

/** An entry of the menu. */
export interface NewTabEntry {
  kind: 'local' | 'target' | 'host';
  label: string;
  detail: string;
  /** Section heading shown above the entry (favorites, recent, results). */
  section?: string;
  host?: Host;
}

/** Recent hosts listed with an empty search, and hosts listed at most. */
const RECENT = 6;
const MAX_RESULTS = 50;

/** "user@host", "host:port", telnet:// or serial:// typed in the search. */
export function looksLikeTarget(text: string): boolean {
  const t = text.trim();
  return /^(telnet|serial):\/\//i.test(t) || /^[^\s@]+@[^\s@]+$/.test(t) || /^[\w.-]+:\d{1,5}$/.test(t);
}

/**
 * The "+" after the tabs: a new tab with a local terminal, a saved host
 * (favorites and recent first, or a search) or an address typed in.
 */
@Component({
  selector: 'app-new-tab-menu',
  imports: [TranslocoDirective, FormsModule, LucidePlus],
  host: { '(document:pointerdown)': 'outside($event)' },
  templateUrl: './new-tab-menu.html',
  styleUrl: './new-tab-menu.scss',
})
export class NewTabMenu {
  private readonly sessions = inject(SessionsStore);
  private readonly hosts = inject(HostsStore);
  private readonly el = inject(ElementRef<HTMLElement>);
  private readonly search = viewChild<ElementRef<HTMLInputElement>>('search');

  protected readonly open = signal(false);
  protected readonly pos = signal({ top: 0, left: 0 });
  protected readonly query = signal('');
  protected readonly active = signal(0);

  protected readonly entries = computed<NewTabEntry[]>(() => {
    const q = this.query().trim();
    const paths = new Map(groupPaths(this.hosts.groups()).map((g) => [g.id, g.path]));
    const entry = (h: Host, section?: string): NewTabEntry => ({
      kind: 'host',
      label: h.name,
      detail: [h.hostname, paths.get(h.groupId)].filter(Boolean).join(' · '),
      section,
      host: h,
    });
    const out: NewTabEntry[] = [{ kind: 'local', label: 'local', detail: '' }];
    if (q && looksLikeTarget(q)) {
      out.push({ kind: 'target', label: q, detail: '' });
    }
    const all = this.hosts.hosts();
    if (!q) {
      const favorites = all.filter((h) => h.favorite).sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
      const recent = all
        .filter((h) => h.lastUsed && !h.favorite)
        .sort((a, b) => b.lastUsed.localeCompare(a.lastUsed))
        .slice(0, RECENT);
      favorites.forEach((h, i) => out.push(entry(h, i === 0 ? 'favorites' : undefined)));
      recent.forEach((h, i) => out.push(entry(h, i === 0 ? 'recent' : undefined)));
      // Then the other hosts, by name: the search narrows long lists.
      const listed = new Set([...favorites, ...recent].map((h) => h.id));
      all
        .filter((h) => !listed.has(h.id))
        .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
        .slice(0, MAX_RESULTS)
        .forEach((h, i) => out.push(entry(h, i === 0 ? 'hosts' : undefined)));
      return out;
    }
    const words = q.toLowerCase().split(/\s+/);
    all
      .filter((h) => {
        const text = `${h.name} ${h.hostname} ${paths.get(h.groupId) ?? ''} ${(h.tags ?? []).join(' ')}`.toLowerCase();
        return words.every((w) => text.includes(w));
      })
      .sort((a, b) => Number(b.favorite) - Number(a.favorite) || a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
      .slice(0, MAX_RESULTS)
      .forEach((h, i) => out.push(entry(h, i === 0 ? 'hosts' : undefined)));
    return out;
  });

  protected toggle(ev: MouseEvent): void {
    const box = (ev.currentTarget as HTMLElement).getBoundingClientRect();
    this.pos.set({ top: box.bottom + 4, left: Math.max(8, Math.min(box.left, globalThis.innerWidth - 376)) });
    this.open.update((o) => !o);
    if (this.open()) {
      this.query.set('');
      this.active.set(0);
      setTimeout(() => this.search()?.nativeElement.focus());
    }
  }

  protected setQuery(q: string): void {
    this.query.set(q);
    // A typed address goes first after "local", a search to its results.
    this.active.set(q.trim() ? 1 : 0);
    if (this.active() >= this.entries().length) {
      this.active.set(0);
    }
  }

  protected choose(e: NewTabEntry): void {
    this.open.set(false);
    switch (e.kind) {
      case 'local':
        void this.sessions.openLocal();
        break;
      case 'target':
        void this.sessions.openTarget(e.label);
        break;
      case 'host':
        void this.sessions.openHost(e.host!.id, e.host!.name);
        break;
    }
  }

  protected onKeydown(ev: KeyboardEvent): void {
    const list = this.entries();
    switch (ev.key) {
      case 'ArrowDown':
        this.active.update((i) => Math.min(i + 1, list.length - 1));
        break;
      case 'ArrowUp':
        this.active.update((i) => Math.max(i - 1, 0));
        break;
      case 'Enter':
        if (list[this.active()]) {
          this.choose(list[this.active()]);
        }
        break;
      case 'Escape':
        this.open.set(false);
        break;
      default:
        return;
    }
    ev.preventDefault();
    setTimeout(() => this.el.nativeElement.querySelector('.entry.active')?.scrollIntoView({ block: 'nearest' }));
  }

  protected outside(ev: PointerEvent): void {
    if (this.open() && !this.el.nativeElement.contains(ev.target as Node)) {
      this.open.set(false);
    }
  }
}
