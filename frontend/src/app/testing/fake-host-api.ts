import type {
  Group,
  GroupContents,
  GroupInput,
  Host,
  HostInput,
  Inventory,
  Resolved,
} from '../core/backend/host-api';
import { subtreeIds } from '../hosts/tree';

/** In-memory HostApi for tests. Mirrors the Go rules that matter to the UI. */
export class FakeHostApi {
  groups: Group[] = [];
  hosts: Host[] = [];
  private nextId = 1;

  async inventory(): Promise<Inventory> {
    return { groups: structuredClone(this.groups), hosts: structuredClone(this.hosts) };
  }
  async createGroup(input: GroupInput): Promise<Group> {
    if (!input.name.trim()) {
      throw codedError('hosts.nameRequired');
    }
    const g: Group = { id: `g${this.nextId++}`, ...input, name: input.name.trim() };
    this.groups.push(g);
    return g;
  }
  async updateGroup(id: string, input: GroupInput): Promise<Group> {
    const g = this.groups.find((x) => x.id === id)!;
    Object.assign(g, input);
    return g;
  }
  async moveGroup(id: string, parentId: string): Promise<void> {
    if (subtreeIds(this.groups, id).has(parentId)) {
      throw codedError('hosts.groupCycle');
    }
    this.groups.find((g) => g.id === id)!.parentId = parentId;
  }
  async groupContents(id: string): Promise<GroupContents> {
    const ids = subtreeIds(this.groups, id);
    return { groups: ids.size - 1, hosts: this.hosts.filter((h) => ids.has(h.groupId)).length };
  }
  async deleteGroup(id: string): Promise<void> {
    const ids = subtreeIds(this.groups, id);
    this.groups = this.groups.filter((g) => !ids.has(g.id));
    this.hosts = this.hosts.filter((h) => !ids.has(h.groupId));
  }
  async createHost(input: HostInput): Promise<Host> {
    if (!input.hostname.trim()) {
      throw codedError('hosts.hostnameRequired');
    }
    const h: Host = { id: `h${this.nextId++}`, ...input, name: input.name.trim() || input.hostname, lastUsed: "" };
    this.hosts.push(h);
    return h;
  }
  async updateHost(id: string, input: HostInput): Promise<Host> {
    const h = this.hosts.find((x) => x.id === id)!;
    Object.assign(h, input, { name: input.name.trim() || input.hostname });
    return h;
  }
  async moveHost(id: string, groupId: string): Promise<void> {
    this.hosts.find((h) => h.id === id)!.groupId = groupId;
  }
  async deleteHost(id: string): Promise<void> {
    this.hosts = this.hosts.filter((h) => h.id !== id);
  }
  async inherited(): Promise<Resolved> {
    return { settings: { port: 22 }, source: { port: '' } };
  }
}

/** Shape of a rejected Wails call carrying a Go errs.Payload. */
export function codedError(code: string, params: Record<string, unknown> = {}): Error {
  return Object.assign(new Error(code), { cause: { code, params } });
}
