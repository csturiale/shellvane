import { Component, inject, input, output } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideFile, LucideFolderOpen } from '@lucide/angular';

import { DialogApi } from '../core/backend/dialog-api';

/**
 * A button next to a field that takes a path on this computer: it opens the
 * system's chooser (near the path already typed) and emits the one chosen.
 */
@Component({
  selector: 'app-path-picker',
  imports: [TranslocoDirective, LucideFile, LucideFolderOpen],
  templateUrl: './path-picker.html',
  styleUrl: './path-picker.scss',
})
export class PathPicker {
  private readonly api = inject(DialogApi);
  private readonly transloco = inject(TranslocoService);
  readonly kind = input<'file' | 'folder'>('file');
  /** The field's value: where the chooser opens. */
  readonly current = input('');
  /** The chooser's title; a generic one by default. */
  readonly title = input('');
  readonly picked = output<string>();

  protected async choose(): Promise<void> {
    const title = this.title() || this.transloco.translate(this.kind() === 'folder' ? 'common.chooseFolder' : 'common.chooseFile');
    try {
      const path = this.kind() === 'folder' ? await this.api.chooseFolder(title, this.current()) : await this.api.chooseFile(title, this.current());
      if (path) {
        this.picked.emit(path);
      }
    } catch {
      // Outside the desktop app: the path is typed.
    }
  }
}
