import { Injectable } from '@angular/core';

import { ProcedureService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { ProcedureList } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Doc, Source } from '../../../../bindings/github.com/csturiale/shellvane/core/procedures';

export type { Doc, ProcedureList, Source };

/** Markdown procedures from Git repositories and folders. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class ProcedureApi {
  list(src: Source): Promise<ProcedureList> {
    return ProcedureService.List(src);
  }
  sync(src: Source): Promise<ProcedureList> {
    return ProcedureService.Sync(src);
  }
  read(src: Source, path: string): Promise<string> {
    return ProcedureService.Read(src, path);
  }
  /** The folder source of the pages received from the browser or pasted. */
  inbox(): Promise<Source> {
    return ProcedureService.Inbox();
  }
  saveInbox(title: string, source: string, markdown: string): Promise<Doc> {
    return ProcedureService.SaveInbox(title, source, markdown);
  }
  deleteInbox(path: string): Promise<void> {
    return ProcedureService.DeleteInbox(path);
  }
  takeReceived(): Promise<string> {
    return ProcedureService.TakeReceived();
  }
}
