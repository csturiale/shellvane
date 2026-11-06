import { Component, inject, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideCopy, LucideFileText, LucideRefreshCw, LucideTrash2, LucideVideo } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { SessionApi, SessionFile } from '../core/backend/session-api';
import { writeClipboard } from '../core/clipboard';
import { formatBytes } from '../core/format';
import { HelpLink } from '../guide/help-link';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { CastPlayer } from './cast-player';
import { clock } from './cast';

/** Saved session logs and recordings, with a viewer and a player. */
@Component({
  selector: 'app-recordings-view',
  imports: [TranslocoDirective, CastPlayer, ConfirmDialog, HelpLink, LucideVideo, LucideFileText, LucideTrash2, LucideCopy, LucideRefreshCw],
  templateUrl: './recordings-view.html',
  styleUrl: './recordings-view.scss',
})
export class RecordingsView {
  private readonly api = inject(SessionApi);
  private readonly transloco = inject(TranslocoService);

  protected readonly files = signal<SessionFile[]>([]);
  protected readonly dir = signal('');
  protected readonly selected = signal<SessionFile | null>(null);
  protected readonly content = signal<string | null>(null);
  protected readonly deleting = signal<SessionFile | null>(null);
  protected readonly message = signal<string | null>(null);
  protected readonly clock = clock;

  constructor() {
    void this.load();
  }

  protected async load(): Promise<void> {
    try {
      this.files.set(await this.api.sessionFiles());
      this.dir.set(await this.api.sessionFilesDir());
    } catch (err) {
      this.fail(err);
    }
  }

  protected async open(f: SessionFile): Promise<void> {
    this.selected.set(f);
    this.content.set(null);
    try {
      this.content.set(await this.api.readSessionFile(f.path));
    } catch (err) {
      this.fail(err);
    }
  }

  protected async copyPath(f: SessionFile): Promise<void> {
    await writeClipboard(f.path).catch(() => undefined);
    this.message.set(this.transloco.translate('logs.pathCopied'));
  }

  protected async confirmDelete(): Promise<void> {
    const f = this.deleting();
    this.deleting.set(null);
    if (!f) {
      return;
    }
    try {
      await this.api.deleteSessionFile(f.path);
      if (this.selected()?.path === f.path) {
        this.selected.set(null);
        this.content.set(null);
      }
      await this.load();
    } catch (err) {
      this.fail(err);
    }
  }

  protected size(bytes: number): string {
    return formatBytes(bytes, this.transloco.getActiveLang());
  }

  protected date(iso: string): string {
    return new Intl.DateTimeFormat(this.transloco.getActiveLang(), { dateStyle: 'short', timeStyle: 'short' }).format(new Date(iso));
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.message.set(this.transloco.translate(e.key, e.params));
  }
}
