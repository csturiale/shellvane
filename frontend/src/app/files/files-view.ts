import { Component, computed, effect, inject, signal, untracked, viewChild } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideX } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { readPreference, writePreference } from '../core/preferences';
import { FileEdit, FilesApi, FolderDiff, LOCAL_FS, TransferStatus } from '../core/backend/files-api';
import { Modal } from '../shared/modal';
import { formatBytes } from '../core/format';
import { LanguageService } from '../core/i18n/language.service';
import { HostsStore } from '../hosts/hosts.store';
import { HostPicker } from '../hosts/host-picker';
import { Navigation } from '../layout/navigation';
import { DraggedEntry, FilePane } from './file-pane';
import { TransfersStore } from './transfers.store';

export type SideName = 'left' | 'right';

/** What one side of the browser shows: this computer or a host. */
interface Side {
  /** "local" or the ID of the host. */
  source: string;
  fs: string;
  title: string;
  home: string;
  shell?: boolean;
}

const LOCAL_SIDE: Side = { source: LOCAL_FS, fs: LOCAL_FS, title: '', home: '' };

/**
 * Two-pane file browser. Each side is this computer or any host, so files
 * can also go from one server to another (through this computer).
 */
@Component({
  selector: 'app-files-view',
  imports: [TranslocoDirective, FilePane, HostPicker, LucideX, Modal],
  templateUrl: './files-view.html',
  styleUrl: './files-view.scss',
})
export class FilesView {
  protected readonly hosts = inject(HostsStore);
  protected readonly transfers = inject(TransfersStore);
  private readonly api = inject(FilesApi);
  private readonly nav = inject(Navigation);
  private readonly language = inject(LanguageService);
  private readonly transloco = inject(TranslocoService);

  protected readonly local = LOCAL_FS;
  protected readonly sideNames: SideName[] = ['left', 'right'];
  protected readonly sides = signal<Record<SideName, Side | null>>({ left: LOCAL_SIDE, right: null });
  protected readonly connecting = signal<SideName | null>(null);
  protected readonly error = signal<string | null>(null);

  private readonly leftPane = viewChild<FilePane>('left');
  private readonly rightPane = viewChild<FilePane>('right');
  private unsubscribeDrop?: () => void;
  private unsubscribeEdit?: () => void;
  protected readonly conflicts = ['overwrite', 'skip', 'rename', 'newer', 'resume'];
  protected readonly conflict = signal(readPreference('files.conflict') ?? 'overwrite');
  /** Files being edited on this computer. */
  protected readonly edits = signal<FileEdit[]>([]);

  /** Folder comparison: direction, differences and those to apply. */
  protected readonly comparing = signal<{ from: SideName; diffs: FolderDiff[] | null; chosen: Set<string>; deleteExtra: boolean } | null>(null);

  protected openCompare(): void {
    this.comparing.set({ from: 'left', diffs: null, chosen: new Set(), deleteExtra: false });
    void this.runCompare();
  }

  protected setDirection(from: SideName): void {
    this.comparing.update((c) => c && { ...c, from, diffs: null, chosen: new Set() });
    void this.runCompare();
  }

  protected toggleDiff(rel: string): void {
    this.comparing.update((c) => {
      if (!c) {
        return c;
      }
      const chosen = new Set(c.chosen);
      if (chosen.has(rel)) {
        chosen.delete(rel);
      } else {
        chosen.add(rel);
      }
      return { ...c, chosen };
    });
  }

  protected setDeleteExtra(on: boolean): void {
    this.comparing.update((c) => {
      if (!c) {
        return c;
      }
      const chosen = new Set(c.chosen);
      for (const d of c.diffs ?? []) {
        if (d.kind === 'extra') {
          if (on) {
            chosen.add(d.rel);
          } else {
            chosen.delete(d.rel);
          }
        }
      }
      return { ...c, deleteExtra: on, chosen };
    });
  }

  private ends(from: SideName) {
    const to: SideName = from === 'left' ? 'right' : 'left';
    const a = this.sides()[from];
    const b = this.sides()[to];
    const pa = this.pane(from);
    const pb = this.pane(to);
    return a && b && pa && pb ? { src: a.fs, srcDir: pa.dir(), dst: b.fs, dstDir: pb.dir(), to } : null;
  }

  private async runCompare(): Promise<void> {
    const c = this.comparing();
    const e = c && this.ends(c.from);
    if (!c || !e) {
      return;
    }
    try {
      const diffs = await this.api.compare(e.src, e.srcDir, e.dst, e.dstDir);
      // New and newer are proposed; older and extra are the user's choice.
      const chosen = new Set(diffs.filter((d) => d.kind === 'new' || d.kind === 'newer').map((d) => d.rel));
      this.comparing.update((cur) => cur && cur.from === c.from ? { ...cur, diffs, chosen } : cur);
    } catch (err) {
      this.comparing.set(null);
      this.showError(err);
    }
  }

  protected async applyCompare(): Promise<void> {
    const c = this.comparing();
    const e = c && this.ends(c.from);
    if (!c || !e || !c.diffs) {
      return;
    }
    // Older files go when chosen: copied like newer ones.
    const chosen = c.diffs.filter((d) => c.chosen.has(d.rel)).map((d) => (d.kind === 'older' ? { ...d, kind: 'newer' } : d));
    this.comparing.set(null);
    try {
      await this.api.syncFolders(e.src, e.srcDir, e.dst, e.dstDir, chosen, c.deleteExtra);
      void this.pane(e.to)?.load(e.dstDir, false);
    } catch (err) {
      this.showError(err);
    }
  }

  protected setConflict(ev: Event): void {
    const v = (ev.target as HTMLSelectElement).value;
    this.conflict.set(v);
    writePreference('files.conflict', v);
  }

  protected stopEdit(id: string): void {
    void this.api.stopEdit(id).catch(() => undefined);
    this.edits.update((list) => list.filter((e) => e.id !== id));
  }

  constructor() {
    if (!this.hosts.loaded()) {
      void this.hosts.load().catch(() => undefined);
    }
    effect(() => {
      const pending = this.nav.filesHost();
      if (pending) {
        untracked(() => {
          const path = this.nav.filesPath();
          this.nav.filesHost.set(null);
          this.nav.filesPath.set('');
          void this.choose('right', pending, path);
        });
      }
    });
    void this.api.edits().then((list) => this.edits.set(list), () => undefined);
    try {
      this.unsubscribeEdit = this.api.onEdit((e) =>
        this.edits.update((list) => (e.state === 'stopped' ? list.filter((x) => x.id !== e.id) : [...list.filter((x) => x.id !== e.id), e])),
      );
    } catch {
      // No event bus outside the desktop app.
    }
    try {
      // Files dropped from the system file manager onto a host's pane.
      this.unsubscribeDrop = this.api.onDrop((d) => {
        const name = this.sideNames.find((n) => this.dropTarget(n) === d.target);
        const side = name && this.sides()[name];
        const dir = name && this.pane(name)?.dir();
        if (side && dir && side.fs !== LOCAL_FS) {
          for (const f of d.files ?? []) {
            void this.api.transfer(LOCAL_FS, f, side.fs, dir).catch((err) => this.showError(err));
          }
        }
      });
    } catch {
      // No event bus outside the desktop app.
    }
  }

  ngOnDestroy(): void {
    this.unsubscribeEdit?.();
    this.unsubscribeDrop?.();
  }

  protected dropTarget(name: SideName): string {
    return `shellvane-drop-${name}`;
  }

  protected titleOf(side: Side): string {
    return side.fs === LOCAL_FS ? this.transloco.translate('files.thisComputer') : side.title;
  }

  /** Switches a side to this computer or to a host. */
  protected async choose(name: SideName, source: string, path = ''): Promise<void> {
    const previous = this.sides()[name];
    if (previous?.source === source) {
      if (path) {
        void this.pane(name)?.load(path);
      }
      return;
    }
    this.error.set(null);
    if (source === LOCAL_FS) {
      this.setSide(name, path ? { ...LOCAL_SIDE, home: path } : LOCAL_SIDE);
    } else {
      this.connecting.set(name);
      try {
        const info = await this.api.connect(source);
        this.setSide(name, { source, fs: info.id, title: info.title, home: path || info.home, shell: info.shell });
      } catch (err) {
        this.showError(err);
        return;
      } finally {
        this.connecting.set(null);
      }
    }
    if (previous && previous.fs !== LOCAL_FS) {
      void this.api.disconnect(previous.fs);
    }
  }

  /** Copies an entry to the directory shown on the other side. */
  protected sendFrom(name: SideName, entry: DraggedEntry): void {
    const other: SideName = name === 'left' ? 'right' : 'left';
    const target = this.sides()[other];
    const pane = this.pane(other);
    if (!target || !pane) {
      this.error.set(this.transloco.translate('files.connectFirst'));
      return;
    }
    void this.api.transfer(entry.fs, entry.path, target.fs, pane.dir()).catch((err) => this.showError(err));
  }

  protected receiveOn(name: SideName, ev: { from: DraggedEntry; dir: string }): void {
    const side = this.sides()[name];
    if (side) {
      void this.api.transfer(ev.from.fs, ev.from.path, side.fs, ev.dir).catch((err) => this.showError(err));
    }
  }

  /** ↑ upload, ↓ download, ⇄ between two hosts. */
  protected direction(t: TransferStatus): string {
    if (t.from === LOCAL_FS) {
      return '↑';
    }
    return t.to === LOCAL_FS ? '↓' : '⇄';
  }

  protected bytes(n: number): string {
    return formatBytes(n, this.language.current());
  }

  protected percent(done: number, total: number): number {
    return total > 0 ? Math.round((done / total) * 100) : 0;
  }

  private pane(name: SideName): FilePane | undefined {
    return name === 'left' ? this.leftPane() : this.rightPane();
  }

  private setSide(name: SideName, side: Side): void {
    this.sides.update((s) => ({ ...s, [name]: side }));
  }

  private showError(err: unknown): void {
    const e = toTranslatableError(err);
    this.error.set(this.transloco.translate(e.key, e.params));
  }
}
