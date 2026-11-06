import { Injectable } from '@angular/core';
import { Events } from '@wailsio/runtime';

import { ControlService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { OpenRequest } from '../../../../bindings/github.com/csturiale/shellvane/app';
import { AppOpen } from '../../../../bindings/events';

export type { OpenRequest };

/** Opens asked from outside the UI: links and the command-line companion. */
@Injectable({ providedIn: 'root' })
export class ControlApi {
  async ready(): Promise<OpenRequest[]> {
    return (await ControlService.Ready()) ?? [];
  }
  onOpen(handler: (r: OpenRequest) => void): () => void {
    return Events.On(AppOpen, (ev) => handler(ev.data));
  }
}
