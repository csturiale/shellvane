import { Injectable } from '@angular/core';

import { UpdateService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { UpdateInfo as GenInfo } from '../../../../bindings/github.com/csturiale/shellvane/app';

export interface UpdateInfo extends Omit<GenInfo, 'notes'> {
  notes: Record<string, string>;
}

/** Signed updates. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class UpdateApi {
  async check(): Promise<UpdateInfo> {
    const i = await UpdateService.Check();
    return { ...i, notes: (i.notes ?? {}) as Record<string, string> };
  }
  install(): Promise<void> {
    return UpdateService.Install();
  }
  restart(): Promise<void> {
    return UpdateService.Restart();
  }
}
