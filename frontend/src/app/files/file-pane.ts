import { Component, effect, inject, input, output, signal, untracked } from '@angular/core';
import { readPreference, writePreference } from '../core/preferences';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import {
  LucideArrowLeftRight,
  LucideEye,
  LucideArrowUp,
  LucideFile,
  LucideFilePen,
  LucideLink,
  LucideStar,
  LucideFolder,
  LucideFolderPlus,
  LucidePencil,
  LucideRefreshCw,
  LucideShieldCheck,
  LucideTrash2,
} from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { FileEntry, FilePreview, FilesApi } from '../core/backend/files-api';
import { Modal } from '../shared/modal';
import { formatBytes } from '../core/format';
import { LanguageService } from '../core/i18n/language.service';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { PermissionsDialog } from './permissions-dialog';
import { toSymbolic } from './permissions';
import { TransfersStore } from './transfers.store';

/** Data carried when an entry is dragged to the other pane. */
export interface DraggedEntry {
  fs: string;
  path: string;
}

/** One side of the file browser: the local disk or a remote host. */
@Component({
  selector: 'app-file-pane',
  imports: [
    FormsModule,
    TranslocoDirective,
    ConfirmDialog,
    LucideArrowUp,
    LucideRefreshCw,
    LucideFilePen,
    LucideLink,
    LucideStar,
    LucideFolderPlus,
    LucideFolder,
    LucideFile,
    LucidePencil,
    LucideTrash2,
    LucideArrowLeftRight,
    LucideShieldCheck,
    PermissionsDialog,
    Modal,
    LucideEye,
  ],
  templateUrl: './file-pane.html',
  styleUrl: './file-pane.scss',
})
export class FilePane {
  /** File system ID: "local" or a remote session. */
  readonly fs = input.required<string>();
  readonly title = input.required<string>();
  /** Starting directory; the home directory when empty. */
  readonly start = input('');
  /** Element ID for files dropped from the system file manager. */
  readonly dropTarget = input('');
  /** The user wants an entry copied to the other pane. */
  readonly send = output<DraggedEntry>();
  /** An entry from the other pane was dropped here. */
  readonly receive = output<{ from: DraggedEntry; dir: string }>();

  private readonly api = inject(FilesApi);
  private readonly transfers = inject(TransfersStore);
  private readonly language = inject(LanguageService);
  private readonly transloco = inject(TranslocoService);

  readonly dir = signal('');
  protected readonly entries = signal<FileEntry[]>([]);
  protected readonly selected = signal<string | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly loading = signal(false);
  protected readonly renaming = signal<string | null>(null);
  protected readonly deleting = signal<FileEntry | null>(null);
  protected readonly chmodEntry = signal<FileEntry | null>(null);
  protected readonly symbolic = toSymbolic;
  protected readonly dropping = signal(false);
  /** The server has no SFTP: shell commands are used. */
  readonly shell = input(false);
  /** Saved folders, per host ("local" for this computer). */
  readonly bookmarkKey = input('');
  protected readonly bookmarks = signal<string[]>([]);
  protected readonly linking = signal(false);
  protected linkTarget = '';
  protected linkName = '';
  protected pathInput = '';
  protected folderName = '';
  protected renameName = '';

  constructor() {
    effect(() => {
      const fs = this.fs();
      const start = this.start();
      untracked(() => void this.init(fs, start));
    });
    // Refresh after transfers complete: a new file may have landed here.
    effect(() => {
      this.transfers.completed();
      untracked(() => this.dir() && void this.load(this.dir(), false));
    });
  }

  private async init(fs: string, start: string): Promise<void> {
    await this.run(async () => this.load(start || (await this.api.home(fs))));
  }

  async load(dir: string, clearSelection = true): Promise<void> {
    this.loading.set(true);
    try {
      this.entries.set(await this.api.list(this.fs(), dir));
      this.dir.set(dir);
      this.pathInput = dir;
      if (clearSelection) {
        this.selected.set(null);
      }
      this.error.set(null);
    } catch (err) {
      this.showError(err);
    } finally {
      this.loading.set(false);
    }
  }

  ngOnInit(): void {
    this.bookmarks.set(this.readBookmarks());
  }

  private readBookmarks(): string[] {
    try {
      return JSON.parse(readPreference('files.bookmarks.' + this.bookmarkKey()) ?? '[]') as string[];
    } catch {
      return [];
    }
  }

  protected toggleBookmark(): void {
    const dir = this.dir();
    const list = this.bookmarks().includes(dir) ? this.bookmarks().filter((b) => b !== dir) : [...this.bookmarks(), dir].sort();
    this.bookmarks.set(list);
    writePreference('files.bookmarks.' + this.bookmarkKey(), JSON.stringify(list));
  }

  protected goBookmark(ev: Event): void {
    const select = ev.target as HTMLSelectElement;
    if (select.value) {
      void this.load(select.value);
    }
    select.value = '';
  }

  protected async createLink(): Promise<void> {
    if (!this.linkTarget.trim() || !this.linkName.trim()) {
      return;
    }
    try {
      const link = await this.api.join(this.fs(), this.dir(), this.linkName.trim());
      await this.api.symlink(this.fs(), this.linkTarget.trim(), link);
      this.linking.set(false);
      this.linkTarget = this.linkName = '';
      await this.load(this.dir(), false);
    } catch (err) {
      this.showError(err);
    }
  }

  /** Opens the file in this computer's default application; saves upload. */
  protected async editFile(e: FileEntry): Promise<void> {
    try {
      await this.api.edit(this.fs(), e.path);
    } catch (err) {
      this.showError(err);
    }
  }

  protected readonly preview = signal<{ name: string; data: FilePreview } | null>(null);

  /** Shows the beginning of a text file, or an image. */
  protected async showPreview(e: FileEntry): Promise<void> {
    try {
      this.preview.set({ name: e.name, data: await this.api.preview(this.fs(), e.path) });
    } catch (err) {
      this.showError(err);
    }
  }

  protected open(e: FileEntry): void {
    if (e.isDir) {
      void this.load(e.path);
    } else {
      this.send.emit({ fs: this.fs(), path: e.path });
    }
  }

  protected async up(): Promise<void> {
    await this.run(async () => this.load(await this.api.parent(this.fs(), this.dir())));
  }

  protected async mkdir(): Promise<void> {
    const name = this.folderName.trim();
    if (!name) {
      return;
    }
    await this.run(async () => {
      await this.api.mkdir(this.fs(), await this.api.join(this.fs(), this.dir(), name));
      this.folderName = '';
      await this.load(this.dir());
    });
  }

  protected startRename(e: FileEntry): void {
    this.renaming.set(e.path);
    this.renameName = e.name;
  }

  protected async rename(e: FileEntry): Promise<void> {
    const name = this.renameName.trim();
    if (this.renaming() !== e.path) {
      return; // already handled (Enter, then blur)
    }
    this.renaming.set(null);
    if (!name || name === e.name) {
      return;
    }
    await this.run(async () => {
      await this.api.rename(this.fs(), e.path, await this.api.join(this.fs(), this.dir(), name));
      await this.load(this.dir());
    });
  }

  protected async confirmDelete(): Promise<void> {
    const e = this.deleting();
    this.deleting.set(null);
    if (e) {
      await this.run(async () => {
        await this.api.remove(this.fs(), e.path);
        await this.load(this.dir());
      });
    }
  }

  protected size(e: Pick<FileEntry, 'size' | 'isDir'>): string {
    return e.isDir ? '' : formatBytes(e.size, this.language.current());
  }

  protected date(e: FileEntry): string {
    return new Intl.DateTimeFormat(this.language.current(), { dateStyle: 'short', timeStyle: 'short' }).format(new Date(e.modTime));
  }

  // Drag between panes.

  protected dragStart(ev: DragEvent, e: FileEntry): void {
    ev.dataTransfer?.setData('application/x-shellvane-file', JSON.stringify({ fs: this.fs(), path: e.path }));
    if (ev.dataTransfer) {
      ev.dataTransfer.effectAllowed = 'copy';
    }
  }

  protected dragOver(ev: DragEvent): void {
    if (ev.dataTransfer?.types.includes('application/x-shellvane-file')) {
      ev.preventDefault();
      this.dropping.set(true);
    }
  }

  protected drop(ev: DragEvent): void {
    this.dropping.set(false);
    const raw = ev.dataTransfer?.getData('application/x-shellvane-file');
    if (!raw) {
      return; // files from the system file manager arrive through Go
    }
    ev.preventDefault();
    const from = JSON.parse(raw) as DraggedEntry;
    if (from.fs !== this.fs()) {
      this.receive.emit({ from, dir: this.dir() });
    }
  }

  private async run(action: () => Promise<void>): Promise<void> {
    try {
      await action();
    } catch (err) {
      this.showError(err);
    }
  }

  private showError(err: unknown): void {
    const e = toTranslatableError(err);
    this.error.set(this.transloco.translate(e.key, e.params));
  }
}
