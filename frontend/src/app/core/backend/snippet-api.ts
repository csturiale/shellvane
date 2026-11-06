import { Injectable } from '@angular/core';

import { SnippetService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { ImportResult, Input, Snippet } from '../../../../bindings/github.com/csturiale/shellvane/core/snippets';

export type { ImportResult, Snippet };
export type SnippetInput = Input;

@Injectable({ providedIn: 'root' })
export class SnippetApi {
  async list(): Promise<Snippet[]> {
    return (await SnippetService.List()) ?? [];
  }
  create(input: SnippetInput): Promise<Snippet> {
    return SnippetService.Create(input);
  }
  update(id: string, input: SnippetInput): Promise<Snippet> {
    return SnippetService.Update(id, input);
  }
  delete(id: string): Promise<void> {
    return SnippetService.Delete(id);
  }
  export(ids: string[]): Promise<string> {
    return SnippetService.Export(ids);
  }
  import(fileName: string, content: string): Promise<ImportResult> {
    return SnippetService.Import(fileName, content);
  }
}
