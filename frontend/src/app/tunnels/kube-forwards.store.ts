import { Injectable, inject, signal } from '@angular/core';

import { KubeForward, KubeForwardApi, KubeForwardSpec, KubeTarget } from '../core/backend/kube-forward-api';
import { KubeSelection } from '../core/backend/session-api';

/** The cluster a port-forward is chosen for: a terminal's context and route. */
export interface KubeScope {
  selection: KubeSelection;
  gcloud: string;
  label: string;
}

/** Kubernetes port-forwards: services or pods on a local port, kept up by the app. */
@Injectable({ providedIn: 'root' })
export class KubeForwardsStore {
  private readonly api = inject(KubeForwardApi);
  readonly list = signal<KubeForward[]>([]);
  private listening = false;

  async load(): Promise<void> {
    if (!this.listening) {
      this.listening = true;
      try {
        this.api.onChange((list) => this.list.set(list));
      } catch {
        // Outside the desktop app.
      }
    }
    this.list.set(await this.api.list());
  }

  targets(scope: KubeScope): Promise<KubeTarget[]> {
    return this.api.targets(scope.selection, scope.gcloud);
  }

  async start(spec: KubeForwardSpec): Promise<KubeForward> {
    const f = await this.api.start(spec);
    this.list.update((l) => [...l.filter((x) => x.id !== f.id), f]);
    return f;
  }

  async stop(id: string): Promise<void> {
    await this.api.stop(id);
    this.list.update((l) => l.filter((x) => x.id !== id));
  }
}
