import { Injectable, computed, inject, signal } from '@angular/core';

import { ImportResult, Snippet, SnippetApi, SnippetInput } from '../core/backend/snippet-api';

@Injectable({ providedIn: 'root' })
export class SnippetsStore {
  private readonly api = inject(SnippetApi);
  readonly snippets = signal<Snippet[]>([]);
  readonly folders = computed(() => [...new Set(this.snippets().map((s) => s.folder).filter(Boolean))].sort());

  /** A snippet the command palette asked to run: the Snippets panel asks
   * for its variables (or confirmation), then runs it. */
  readonly pendingRun = signal<string | null>(null);

  async load(): Promise<void> {
    this.snippets.set(await this.api.list());
  }

  async save(id: string | null, input: SnippetInput): Promise<void> {
    if (id) {
      await this.api.update(id, input);
    } else {
      await this.api.create(input);
    }
    await this.load();
  }

  /** A Shellvane snippets file with the given snippets (all when empty). */
  export(ids: string[] = []): Promise<string> {
    return this.api.export(ids);
  }

  async import(fileName: string, content: string): Promise<ImportResult> {
    const r = await this.api.import(fileName, content);
    await this.load();
    return r;
  }

  async delete(id: string): Promise<void> {
    await this.api.delete(id);
    await this.load();
  }
}
