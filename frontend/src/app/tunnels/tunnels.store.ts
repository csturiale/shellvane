import { Injectable, computed, inject, signal } from '@angular/core';

import { RemotePort, TunnelApi, TunnelInput, TunnelProfile, TunnelStatus, TunnelView } from '../core/backend/tunnel-api';
import { UsageStore } from '../core/usage/usage.store';

@Injectable({ providedIn: 'root' })
export class TunnelsStore {
  private readonly usage = inject(UsageStore);
  private readonly api = inject(TunnelApi);
  readonly tunnels = signal<TunnelView[]>([]);
  readonly activeCount = computed(() => this.tunnels().filter((t) => t.status.state === 'running').length);

  constructor() {
    try {
      this.api.onState((s) => this.apply(s));
    } catch {
      // No event bus outside the desktop app.
    }
  }

  /** Loads profiles and starts the autostart ones; called once at startup. */
  async init(): Promise<void> {
    await this.load();
    await this.api.startAutostart();
  }

  async load(): Promise<void> {
    this.tunnels.set(await this.api.list());
  }

  async save(id: string | null, input: TunnelInput): Promise<void> {
    if (id) {
      await this.api.update(id, input);
    } else {
      await this.api.create(input);
    }
    await this.load();
  }

  async delete(id: string): Promise<void> {
    await this.api.delete(id);
    await this.load();
  }

  start(id: string): Promise<void> {
    this.usage.count('tunnel.start');
    return this.api.start(id);
  }

  stop(id: string): Promise<void> {
    return this.api.stop(id);
  }

  listeningPorts(hostId: string): Promise<RemotePort[]> {
    return this.api.listeningPorts(hostId);
  }

  /** Forwards a detected port (temporary, not saved). */
  async forward(hostId: string, port: RemotePort): Promise<TunnelProfile> {
    const p = await this.api.forward(hostId, port);
    await this.load();
    return p;
  }

  /** Saves a temporary forward as a profile. */
  async keep(t: TunnelView): Promise<void> {
    await this.api.stop(t.profile.id);
    const { kind, bindAddr, destHost, destPort } = t.profile;
    // Keep the port it listened on, so saved links keep working.
    const bindPort = Number(t.status.addr.split(':').pop()) || t.profile.bindPort;
    const created = await this.api.create({ hostId: t.profile.hostId, name: t.profile.name, spec: { kind, bindAddr, bindPort, destHost, destPort }, autostart: false });
    await this.api.start(created.id);
    await this.load();
  }

  /** The running forward of a host's port, if any. */
  forwardOf(hostId: string, port: number): TunnelView | undefined {
    return this.tunnels().find((t) => t.profile.hostId === hostId && t.profile.destPort === port && t.profile.kind === 'local' && t.status.state === 'running');
  }

  private apply(s: TunnelStatus): void {
    this.tunnels.update((list) =>
      list
        .filter((t) => !(t.temporary && t.profile.id === s.id && s.state === 'stopped'))
        .map((t) => (t.profile.id === s.id ? { ...t, status: s } : t)),
    );
  }
}
