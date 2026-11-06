import { Injectable } from '@angular/core';

import { HostService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { HostStatus } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type {
  Group,
  GroupContents,
  GroupInput,
  Host,
  HostInput,
  Inventory,
  TemplateResult,
} from '../../../../bindings/github.com/csturiale/shellvane/core/hosts';
import type { Resolved } from '../../../../bindings/github.com/csturiale/shellvane/core/settings';

export type { Group, GroupContents, GroupInput, Host, HostInput, HostStatus, Inventory, Resolved, TemplateResult };
export type { Settings } from '../../../../bindings/github.com/csturiale/shellvane/core/settings';

/** Host inventory operations. Tests replace this class with an in-memory fake. */
@Injectable({ providedIn: 'root' })
export class HostApi {
  inventory(): Promise<Inventory> {
    return HostService.Inventory();
  }
  exportTemplate(groupId: string, withHosts: boolean): Promise<string> {
    return HostService.ExportTemplate(groupId, withHosts);
  }
  importTemplate(parentId: string, text: string): Promise<TemplateResult> {
    return HostService.ImportTemplate(parentId, text);
  }
  createGroup(input: GroupInput): Promise<Group> {
    return HostService.CreateGroup(input);
  }
  updateGroup(id: string, input: GroupInput): Promise<Group> {
    return HostService.UpdateGroup(id, input);
  }
  moveGroup(id: string, parentId: string): Promise<void> {
    return HostService.MoveGroup(id, parentId);
  }
  groupContents(id: string): Promise<GroupContents> {
    return HostService.GroupContents(id);
  }
  deleteGroup(id: string): Promise<void> {
    return HostService.DeleteGroup(id);
  }
  setFavorite(id: string, on: boolean): Promise<void> {
    return HostService.SetFavorite(id, on);
  }
  async status(ids: string[]): Promise<HostStatus[]> {
    return (await HostService.Status(ids)) ?? [];
  }
  wake(id: string): Promise<void> {
    return HostService.Wake(id);
  }
  duplicateHost(id: string, suffix: string): Promise<Host> {
    return HostService.DuplicateHost(id, suffix);
  }
  createHost(input: HostInput): Promise<Host> {
    return HostService.CreateHost(input);
  }
  updateHost(id: string, input: HostInput): Promise<Host> {
    return HostService.UpdateHost(id, input);
  }
  moveHost(id: string, groupId: string): Promise<void> {
    return HostService.MoveHost(id, groupId);
  }
  deleteHost(id: string): Promise<void> {
    return HostService.DeleteHost(id);
  }
  inherited(groupId: string): Promise<Resolved> {
    return HostService.Inherited(groupId);
  }
}
