import { Injectable, inject, signal } from '@angular/core';

import { AgentStatus, KeyApi, SshKey } from '../core/backend/key-api';
import { readPreference, writePreference } from '../core/preferences';

@Injectable({ providedIn: 'root' })
export class KeysStore {
  private readonly api = inject(KeyApi);
  readonly keys = signal<SshKey[]>([]);
  readonly loaded = signal(false);
  readonly agent = signal<AgentStatus | null>(null);

  /** Starts Shellvane's agent at launch when the user turned it on. */
  async initAgent(): Promise<void> {
    if (readPreference('keys.agent') === 'true') {
      await this.setAgent(true);
    } else {
      this.agent.set(await this.api.agentStatus());
    }
  }

  async setAgent(on: boolean): Promise<void> {
    writePreference('keys.agent', String(on));
    if (on) {
      this.agent.set(await this.api.startAgent());
    } else {
      await this.api.stopAgent();
      this.agent.set(await this.api.agentStatus());
    }
  }

  async load(): Promise<void> {
    this.keys.set(await this.api.list());
    this.loaded.set(true);
  }

  async generate(name: string, kind: string, bits: number, comment: string): Promise<void> {
    await this.api.generate(name, kind, bits, comment);
    await this.load();
  }

  async import(name: string, data: string, passphrase: string): Promise<void> {
    await this.api.import(name, data, passphrase);
    await this.load();
  }

  async delete(id: string): Promise<void> {
    await this.api.delete(id);
    await this.load();
  }

  async setCertificate(id: string, certLine: string): Promise<void> {
    await this.api.setCertificate(id, certLine);
    await this.load();
  }

  deploy(keyId: string, hostId: string): Promise<void> {
    return this.api.deploy(keyId, hostId);
  }

  usage(keyId: string) {
    return this.api.usage(keyId);
  }

  export(keyId: string, format: 'openssh' | 'ppk', passphrase: string): Promise<string> {
    return this.api.export(keyId, format, passphrase);
  }

  async rotate(keyId: string, removeOld: boolean) {
    const r = await this.api.rotate(keyId, removeOld);
    await this.load();
    return r;
  }
}
