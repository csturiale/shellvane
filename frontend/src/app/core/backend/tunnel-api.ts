import { Injectable } from '@angular/core';
import { Events } from '@wailsio/runtime';

import { TunnelService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { TunnelStatus, TunnelView } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Input, Profile, RemotePort } from '../../../../bindings/github.com/csturiale/shellvane/core/tunnel';
import { TunnelState } from '../../../../bindings/events';

export type { Profile as TunnelProfile, TunnelStatus, TunnelView };
export type TunnelInput = Input;
export type { RemotePort };

@Injectable({ providedIn: 'root' })
export class TunnelApi {
  async list(): Promise<TunnelView[]> {
    return (await TunnelService.List()) ?? [];
  }
  create(input: TunnelInput): Promise<Profile> {
    return TunnelService.Create(input);
  }
  update(id: string, input: TunnelInput): Promise<Profile> {
    return TunnelService.Update(id, input);
  }
  delete(id: string): Promise<void> {
    return TunnelService.Delete(id);
  }
  start(id: string): Promise<void> {
    return TunnelService.Start(id);
  }
  stop(id: string): Promise<void> {
    return TunnelService.Stop(id);
  }
  async listeningPorts(hostId: string): Promise<RemotePort[]> {
    return (await TunnelService.ListeningPorts(hostId)) ?? [];
  }
  forward(hostId: string, port: RemotePort): Promise<Profile> {
    return TunnelService.Forward(hostId, port);
  }
  startAutostart(): Promise<void> {
    return TunnelService.StartAutostart();
  }
  onState(handler: (s: TunnelStatus) => void): () => void {
    return Events.On(TunnelState, (ev) => handler(ev.data));
  }
}
