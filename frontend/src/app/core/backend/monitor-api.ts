import { Injectable } from '@angular/core';

import { MonitorService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Stats } from '../../../../bindings/github.com/csturiale/shellvane/core/monitor';

export type { Stats as ServerStats };

/** Server figures read over SSH. */
@Injectable({ providedIn: 'root' })
export class MonitorApi {
  sample(hostId: string): Promise<Stats> {
    return MonitorService.Sample(hostId);
  }
}
