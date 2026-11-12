import { Component, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { Modal } from '../shared/modal';
import { SavedSource } from './procedures.store';

/** Adds or edits a source of procedures: a Git repository or a folder. */
@Component({
  selector: 'app-procedure-source-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './source-dialog.html',
  styleUrl: '../shared/form.scss',
})
export class SourceDialog {
  /** null adds one. */
  readonly source = input<SavedSource | null>(null);
  readonly saved = output<Omit<SavedSource, 'id'> & { id?: string }>();
  readonly removed = output<string>();
  readonly closed = output<void>();

  protected name = '';
  protected kind = 'git';
  protected url = '';
  protected branch = '';
  protected folder = '';

  ngOnInit(): void {
    const s = this.source();
    if (s) {
      [this.name, this.kind, this.url, this.branch, this.folder] = [s.name, s.kind, s.url ?? '', s.branch ?? '', s.folder ?? ''];
    }
  }

  protected valid(): boolean {
    return this.kind === 'git' ? !!this.url.trim() : !!this.folder.trim();
  }

  protected submit(): void {
    if (!this.valid()) {
      return;
    }
    const url = this.url.trim();
    const fallback = this.kind === 'git' ? url.replace(/\.git$/, '').split(/[/:]/).pop() : this.folder.trim().split(/[/\\]/).pop();
    this.saved.emit({
      id: this.source()?.id,
      name: this.name.trim() || fallback || '?',
      kind: this.kind,
      url: this.kind === 'git' ? url : '',
      branch: this.kind === 'git' ? this.branch.trim() : '',
      folder: this.folder.trim(),
    });
  }
}
