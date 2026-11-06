import { Injectable, inject, signal } from '@angular/core';

import { Group, GroupInput, Host, HostApi, HostInput, HostStatus, TemplateResult } from '../core/backend/host-api';
import { readPreference, writePreference } from '../core/preferences';
import { Clients, LaunchApi } from '../core/backend/launch-api';

const EXPANDED_KEY = 'hosts.expanded';

/** Client-side state of the host inventory. Every change reloads from Go. */
@Injectable({ providedIn: 'root' })
export class HostsStore {
  private readonly api = inject(HostApi);
  private readonly launch = inject(LaunchApi);

  readonly groups = signal<Group[]>([]);
  readonly hosts = signal<Host[]>([]);
  readonly loaded = signal(false);
  readonly expanded = signal<ReadonlySet<string>>(readExpanded());
  /** Reachability of the hosts' SSH ports, by host ID. */
  readonly status = signal<Record<string, HostStatus>>({});
  /** External clients installed (Mosh, RDP, VNC). */
  readonly clients = signal<Clients>({ mosh: false, rdp: false, vnc: false });

  async loadClients(): Promise<void> {
    this.clients.set(await this.launch.clients());
  }

  remoteDesktop(id: string): Promise<void> {
    return this.launch.remoteDesktop(id);
  }

  /** Probe the hosts' SSH ports to show which ones answer. */
  readonly showStatus = signal(readPreference('hosts.showStatus') !== 'false');

  setShowStatus(on: boolean): void {
    this.showStatus.set(on);
    writePreference('hosts.showStatus', String(on));
    if (!on) {
      this.status.set({});
    }
  }

  async refreshStatus(): Promise<void> {
    if (!this.showStatus()) {
      return;
    }
    const list = await this.api.status([]);
    this.status.set(Object.fromEntries(list.map((s) => [s.hostId, s])));
  }

  wake(id: string): Promise<void> {
    return this.api.wake(id);
  }

  async load(): Promise<void> {
    const inv = await this.api.inventory();
    this.groups.set(inv.groups ?? []);
    this.hosts.set(inv.hosts ?? []);
    this.loaded.set(true);
  }

  async setFavorite(id: string, on: boolean): Promise<void> {
    await this.api.setFavorite(id, on);
    this.hosts.update((list) => list.map((h) => (h.id === id ? { ...h, favorite: on } : h)));
  }

  /** A group, its subgroups and optionally their hosts as shareable text (every group when empty). */
  exportTemplate(groupId: string, withHosts: boolean): Promise<string> {
    return this.api.exportTemplate(groupId, withHosts);
  }

  async importTemplate(parentId: string, text: string): Promise<TemplateResult> {
    const res = await this.api.importTemplate(parentId, text);
    await this.load();
    return res;
  }

  async duplicate(id: string, suffix: string): Promise<void> {
    await this.api.duplicateHost(id, suffix);
    await this.load();
  }

  /** Every host of a group and its subgroups. */
  hostsIn(groupId: string): Host[] {
    const ids = new Set([groupId]);
    let grew = true;
    while (grew) {
      grew = false;
      for (const g of this.groups()) {
        if (ids.has(g.parentId) && !ids.has(g.id)) {
          ids.add(g.id);
          grew = true;
        }
      }
    }
    return this.hosts().filter((h) => ids.has(h.groupId));
  }

  toggle(groupId: string): void {
    this.setExpanded(groupId, !this.expanded().has(groupId));
  }

  setExpanded(groupId: string, open: boolean): void {
    const next = new Set(this.expanded());
    if (open) {
      next.add(groupId);
    } else {
      next.delete(groupId);
    }
    this.expanded.set(next);
    writePreference(EXPANDED_KEY, JSON.stringify([...next]));
  }

  async createGroup(input: GroupInput): Promise<Group> {
    const g = await this.api.createGroup(input);
    this.setExpanded(g.id, true);
    if (g.parentId) {
      this.setExpanded(g.parentId, true);
    }
    await this.load();
    return g;
  }

  async updateGroup(id: string, input: GroupInput): Promise<void> {
    await this.api.updateGroup(id, input);
    await this.load();
  }

  async moveGroup(id: string, parentId: string): Promise<void> {
    await this.api.moveGroup(id, parentId);
    if (parentId) {
      this.setExpanded(parentId, true);
    }
    await this.load();
  }

  groupContents(id: string) {
    return this.api.groupContents(id);
  }

  async deleteGroup(id: string): Promise<void> {
    await this.api.deleteGroup(id);
    this.setExpanded(id, false);
    await this.load();
  }

  async createHost(input: HostInput): Promise<Host> {
    const h = await this.api.createHost(input);
    if (h.groupId) {
      this.setExpanded(h.groupId, true);
    }
    await this.load();
    return h;
  }

  async updateHost(id: string, input: HostInput): Promise<void> {
    await this.api.updateHost(id, input);
    await this.load();
  }

  async moveHost(id: string, groupId: string): Promise<void> {
    await this.api.moveHost(id, groupId);
    if (groupId) {
      this.setExpanded(groupId, true);
    }
    await this.load();
  }

  async deleteHost(id: string): Promise<void> {
    await this.api.deleteHost(id);
    await this.load();
  }

  inherited(groupId: string) {
    return this.api.inherited(groupId);
  }
}

function readExpanded(): Set<string> {
  try {
    const ids: unknown = JSON.parse(readPreference(EXPANDED_KEY) ?? '[]');
    return new Set(Array.isArray(ids) ? ids.filter((id): id is string => typeof id === 'string') : []);
  } catch {
    return new Set();
  }
}
