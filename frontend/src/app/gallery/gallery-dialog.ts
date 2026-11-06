import { Component, OnInit, inject, input, output, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { toTranslatableError } from '../core/backend/errors';
import { GalleryApi, GalleryIndex } from '../core/backend/gallery-api';
import { openUrl } from '../core/browser';
import { SITE_URL } from '../core/site';
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
  protected readonly tab = signal<'snippets' | 'themes'>('snippets');
  protected readonly index = signal<GalleryIndex | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly message = signal<string | null>(null);
  protected readonly busy = signal<string | null>(null);
  protected readonly done = signal<Set<string>>(new Set());

  async ngOnInit(): Promise<void> {
    this.tab.set(this.start());
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
      this.done.update((s) => new Set(s).add(file));
      this.message.set(this.transloco.translate(kind === 'snippets' ? 'gallery.installedSnippets' : 'gallery.installedTheme', { name, count: r.added }));
      this.installed.emit(kind);
    } catch (err) {
      this.fail(err);
    } finally {
      this.busy.set(null);
    }
  }

  protected propose(): void {
    void openUrl(`${SITE_URL}/gallery/#share`);
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.error.set(this.transloco.translate(e.key, e.params));
  }
}
