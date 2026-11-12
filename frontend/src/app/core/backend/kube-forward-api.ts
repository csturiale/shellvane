import { Injectable } from '@angular/core';
import { Events } from '@wailsio/runtime';

import { KubeForwardService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { KubeForward, KubeForwardSpec, KubeTarget } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Selection } from '../../../../bindings/github.com/csturiale/shellvane/core/kube';
import { KubeForwards } from '../../../../bindings/events';

export type { KubeForward, KubeForwardSpec, KubeTarget };

/** Kubernetes port-forwards run by the app. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class KubeForwardApi {
  async targets(sel: Selection, gcloud: string): Promise<KubeTarget[]> {
    return (await KubeForwardService.Targets(sel, gcloud)) ?? [];
  }
  start(spec: KubeForwardSpec): Promise<KubeForward> {
    return KubeForwardService.Start(spec);
  }
  stop(id: string): Promise<void> {
    return KubeForwardService.Stop(id);
  }
  async list(): Promise<KubeForward[]> {
    return (await KubeForwardService.List()) ?? [];
  }
  onChange(handler: (list: KubeForward[]) => void): () => void {
    return Events.On(KubeForwards, (ev) => handler(ev.data ?? []));
  }
}
