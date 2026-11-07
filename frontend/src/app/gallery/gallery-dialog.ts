import { Component, OnInit, computed, inject, input, output, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { toTranslatableError } from '../core/backend/errors';
import { GalleryApi, GalleryIndex } from '../core/backend/gallery-api';
import { Navigation } from '../layout/navigation';
import { SnippetsStore } from '../snippets/snippets.store';
import { ThemesStore } from '../themes/themes.store';
import { Modal } from '../shared/modal';

/**
 * The community gallery: snippet collections and terminal color schemes
 * shared on termoraplus.com, installed in one click. The index is signed
 * by the project and each file checked against it.
 */
@Component({
  selector: 'app-gallery-dialog',
  imports: [TranslocoDirective, Modal],
  templateUrl: './gallery-dialog.html',
  styleUrl: './gallery-dialog.scss',
})
export class GalleryDialog implements OnInit {
  /** The tab shown first. */
  readonly start = input<'snippets' | 'themes'>('snippets');
  readonly closed = output<void>();
  /** Emitted after an install, so the lists reload. */
  readonly installed = output<'snippets' | 'themes'>();
  private readonly api = inject(GalleryApi);
  private readonly transloco = inject(TranslocoService);
  private readonly nav = inject(Navigation);
  protected readonly tab = signal<'snippets' | 'themes'>('snippets');
  protected readonly index = signal<GalleryIndex | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly message = signal<string | null>(null);
  protected readonly busy = signal<string | null>(null);
  private readonly snippetsStore = inject(SnippetsStore);
  private readonly themesStore = inject(ThemesStore);

  /**
   * Files already in the app: a color scheme whose name is among the
   * user's schemes, a collection whose snippets are all in its folder.
   */
  protected readonly done = computed(() => {
    const idx = this.index();
    const out = new Set<string>();
    if (!idx) {
      return out;
    }
    const themes = new Set(this.themesStore.themes().map((t) => t.name));
    for (const th of idx.themes ?? []) {
      if (themes.has(th.name)) {
        out.add(th.file);
      }
    }
    const snippets = new Set(this.snippetsStore.snippets().map((s) => `${s.folder}\u0000${s.name}`));
    for (const s of idx.snippets ?? []) {
      const names = s.names ?? [];
      if (names.length > 0 && names.every((n) => snippets.has(`${s.name}\u0000${n}`))) {
        out.add(s.file);
      }
    }
    return out;
  });

  async ngOnInit(): Promise<void> {
    this.tab.set(this.start());
    void this.snippetsStore.load().catch(() => undefined);
    void this.themesStore.load().catch(() => undefined);
    try {
      this.index.set(await this.api.gallery());
    } catch (err) {
      this.fail(err);
    }
  }

  protected async install(kind: 'snippets' | 'themes', file: string, name: string): Promise<void> {
    this.busy.set(file);
    this.error.set(null);
    try {
      const r = await this.api.install(kind, file);
      await (kind === 'snippets' ? this.snippetsStore.load() : this.themesStore.load()).catch(() => undefined);
      this.message.set(this.transloco.translate(kind === 'snippets' ? 'gallery.installedSnippets' : 'gallery.installedTheme', { name, count: r.added }));
      this.installed.emit(kind);
    } catch (err) {
      this.fail(err);
    } finally {
      this.busy.set(null);
    }
  }

  /** The guide's Contribute topic, on the open tab's section. */
  protected propose(): void {
    this.nav.openGuide('contribute', this.tab());
    this.closed.emit();
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.error.set(this.transloco.translate(e.key, e.params));
  }
}
