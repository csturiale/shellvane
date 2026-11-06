import { Injectable } from '@angular/core';
import { Events } from '@wailsio/runtime';

import { FilesService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { DroppedFiles, FileEdit, RemoteFSInfo, TransferStatus } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Diff as FolderDiff, Entry, Preview as FilePreview } from '../../../../bindings/github.com/csturiale/shellvane/core/files';
import { FilesDropped, FilesEdit as FileEditEvent, TransferProgress } from '../../../../bindings/events';
import { readPreference } from '../preferences';

export type { DroppedFiles, Entry as FileEntry, FileEdit, FilePreview, FolderDiff, RemoteFSInfo, TransferStatus };
export const LOCAL_FS = 'local';

@Injectable({ providedIn: 'root' })
export class FilesApi {
  connect(hostId: string): Promise<RemoteFSInfo> {
    return FilesService.Connect(hostId);
  }
  disconnect(id: string): Promise<void> {
    return FilesService.Disconnect(id);
  }
  home(fs: string): Promise<string> {
    return FilesService.Home(fs);
  }
  async list(fs: string, dir: string): Promise<Entry[]> {
    return (await FilesService.List(fs, dir)) ?? [];
  }
  parent(fs: string, path: string): Promise<string> {
    return FilesService.Parent(fs, path);
  }
  join(fs: string, dir: string, name: string): Promise<string> {
    return FilesService.Join(fs, dir, name);
  }
  mkdir(fs: string, path: string): Promise<void> {
    return FilesService.Mkdir(fs, path);
  }
  rename(fs: string, from: string, to: string): Promise<void> {
    return FilesService.Rename(fs, from, to);
  }
  chmod(fs: string, path: string, mode: number, recursive: boolean): Promise<void> {
    return FilesService.Chmod(fs, path, mode, recursive);
  }
  remove(fs: string, path: string): Promise<void> {
    return FilesService.Remove(fs, path);
  }
  /** Copies with the conflict policy chosen in the Files view. */
  transfer(from: string, src: string, to: string, dstDir: string): Promise<string> {
    return FilesService.Transfer(from, src, to, dstDir, { conflict: readPreference('files.conflict') ?? 'overwrite' });
  }
  chown(fs: string, path: string, uid: number, gid: number): Promise<void> {
    return FilesService.Chown(fs, path, uid, gid);
  }
  async compare(fromFs: string, srcDir: string, toFs: string, dstDir: string): Promise<FolderDiff[]> {
    return (await FilesService.Compare(fromFs, srcDir, toFs, dstDir)) ?? [];
  }
  syncFolders(fromFs: string, srcDir: string, toFs: string, dstDir: string, diffs: FolderDiff[], deleteExtra: boolean): Promise<number> {
    return FilesService.SyncFolders(fromFs, srcDir, toFs, dstDir, diffs, deleteExtra);
  }
  preview(fs: string, path: string): Promise<FilePreview> {
    return FilesService.Preview(fs, path);
  }
  symlink(fs: string, target: string, link: string): Promise<void> {
    return FilesService.Symlink(fs, target, link);
  }
  edit(fs: string, path: string): Promise<FileEdit> {
    return FilesService.Edit(fs, path);
  }
  stopEdit(id: string): Promise<void> {
    return FilesService.StopEdit(id);
  }
  async edits(): Promise<FileEdit[]> {
    return (await FilesService.Edits()) ?? [];
  }
  revealLocal(path: string): Promise<void> {
    return FilesService.RevealLocal(path);
  }
  onEdit(handler: (e: FileEdit) => void): () => void {
    return Events.On(FileEditEvent, (ev) => handler(ev.data));
  }
  cancel(id: string): Promise<void> {
    return FilesService.CancelTransfer(id);
  }
  clearFinished(): Promise<void> {
    return FilesService.ClearFinished();
  }
  onTransfer(handler: (s: TransferStatus) => void): () => void {
    return Events.On(TransferProgress, (ev) => handler(ev.data));
  }
  onDrop(handler: (d: DroppedFiles) => void): () => void {
    return Events.On(FilesDropped, (ev) => handler(ev.data));
  }
}
