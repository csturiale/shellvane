import { Injectable } from '@angular/core';

import { UsageService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { UsagePayload, UsageReport } from '../../../../bindings/github.com/csturiale/shellvane/app';

export type { UsagePayload, UsageReport };

/** The opt-in usage report. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class UsageApi {
  payload(report: UsageReport): Promise<UsagePayload> {
    return UsageService.Payload(report);
  }
  send(report: UsageReport): Promise<void> {
    return UsageService.Send(report);
  }
}
