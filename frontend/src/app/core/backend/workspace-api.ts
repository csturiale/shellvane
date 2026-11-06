import { Injectable } from '@angular/core';

import { WorkspaceService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Workspace } from '../../../../bindings/github.com/csturiale/shellvane/core/workspaces';

export type { Workspace };

/** Saved workspaces. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class WorkspaceApi {
  async list(): Promise<Workspace[]> {
    return (await WorkspaceService.List()) ?? [];
  }
  save(name: string, layout: string): Promise<Workspace> {
    return WorkspaceService.Save(name, layout);
  }
  delete(id: string): Promise<void> {
    return WorkspaceService.Delete(id);
  }
  saveLast(layout: string): Promise<void> {
    return WorkspaceService.SaveLast(layout);
  }
  last(): Promise<string> {
    return WorkspaceService.Last();
  }
}
