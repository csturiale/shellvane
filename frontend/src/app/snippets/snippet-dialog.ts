import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { Snippet } from '../core/backend/snippet-api';
import { Modal } from '../shared/modal';
import { SnippetsStore } from './snippets.store';
import { userVariables } from './variables';

@Component({
  selector: 'app-snippet-dialog',
  imports: [NgComponentOutlet, FormsModule, TranslocoDirective, Modal],
  templateUrl: './snippet-dialog.html',
  styleUrls: ['../shared/form.scss', './snippet-dialog.scss'],
})
export class SnippetDialog {
  protected readonly sharing = inject(Sharing);
  readonly snippet = input<Snippet | null>(null);
  readonly closed = output<void>();

  protected readonly store = inject(SnippetsStore);
  protected name = '';
  protected folder = '';
  protected readonly body = signal('');
  protected run = true;
  protected readonly variables = computed(() => userVariables(this.body()).map((v) => v.name));
  protected readonly error = signal<TranslatableError | null>(null);
  protected readonly saving = signal(false);
  /** Bound as text: braces would be interpolated by templates and translations. */
  protected readonly examples = {
    ask: '{{name}}',
    choose: '{{name:a|b}}',
    system: '{{host}}, {{user}}, {{port}}, {{date}}, {{time}}',
  };

  ngOnInit(): void {
    const s = this.snippet();
    if (s) {
      this.name = s.name;
      this.folder = s.folder;
      this.body.set(s.body);
      this.run = s.run;
    }
  }

  protected async save(): Promise<void> {
    this.saving.set(true);
    this.error.set(null);
    try {
      await this.store.save(this.snippet()?.id ?? null, {
        name: this.name,
        folder: this.folder,
        body: this.body(),
        run: this.run,
      });
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.saving.set(false);
    }
  }
}
