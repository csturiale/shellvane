import { Injectable } from '@angular/core';
import { Events } from '@wailsio/runtime';

import { SyncService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { SyncSetup, SyncStatus as GenStatus } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { CloudFolder, DeviceInfo, Report } from '../../../../bindings/github.com/csturiale/shellvane/core/sync';
import { SyncDone } from '../../../../bindings/events';

export type { CloudFolder, DeviceInfo, Report as SyncReport, SyncSetup };

export interface SyncStatus extends Omit<GenStatus, 'devices'> {
  devices: DeviceInfo[];
}

/** End-to-end encrypted sync through a shared folder, WebDAV or Git. */
@Injectable({ providedIn: 'root' })
export class SyncApi {
  async status(): Promise<SyncStatus> {
    const s = await SyncService.Status();
    return { ...s, devices: s.devices ?? [] };
  }
  async cloudFolders(): Promise<CloudFolder[]> {
    return (await SyncService.CloudFolders()) ?? [];
  }
  setup(setup: SyncSetup): Promise<Report> {
    return SyncService.Setup(setup);
  }
  syncNow(): Promise<Report> {
    return SyncService.SyncNow();
  }
  setAuto(on: boolean): Promise<void> {
    return SyncService.SetAuto(on);
  }
  disable(): Promise<void> {
    return SyncService.Disable();
  }
  onDone(handler: (r: Report) => void): () => void {
    return Events.On(SyncDone, (ev) => handler(ev.data));
  }
}
