import { Injectable } from '@angular/core';

import { LaunchService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Clients, SessionInfo } from '../../../../bindings/github.com/csturiale/shellvane/app';

export type { Clients };

/** External clients: Mosh, remote desktop, VNC. */
@Injectable({ providedIn: 'root' })
export class LaunchApi {
  clients(): Promise<Clients> {
    return LaunchService.Clients();
  }
  openMosh(hostId: string, cols: number, rows: number): Promise<SessionInfo> {
    return LaunchService.OpenMosh(hostId, cols, rows);
  }
  remoteDesktop(hostId: string): Promise<void> {
    return LaunchService.RemoteDesktop(hostId);
  }
}
