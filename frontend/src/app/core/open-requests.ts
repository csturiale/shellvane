import { Injectable, inject } from '@angular/core';

import { Events } from '@wailsio/runtime';

import { ProceduresReceived } from '../../../bindings/events';
import { ControlApi, OpenRequest } from './backend/control-api';
import { ProcedureApi } from './backend/procedure-api';
import { Navigation } from '../layout/navigation';
import { FULL_VIEWS } from '../layout/views';
import { SessionsStore } from '../sessions/sessions.store';

/** Opens hosts, quick connections and files asked by links or the CLI. */
@Injectable({ providedIn: 'root' })
export class OpenRequests {
  private readonly api = inject(ControlApi);
  private readonly sessions = inject(SessionsStore);
  private readonly nav = inject(Navigation);
  private readonly procedures = inject(ProcedureApi);

  async start(): Promise<void> {
    try {
      this.api.onOpen((r) => this.open(r));
      for (const r of await this.api.ready()) {
        this.open(r);
      }
      // Pages sent from the browser ("Send to Shellvane") open in Procedures.
      Events.On(ProceduresReceived, (ev) => this.procedure(ev.data));
      const received = await this.procedures.takeReceived();
      if (received) {
        this.procedure(received);
      }
    } catch {
      // Outside the desktop app.
    }
  }

  procedure(path: string): void {
    this.nav.view.set('procedures');
    this.nav.procedure.set(path);
  }

  open(r: OpenRequest): void {
    if (r.kind === 'files') {
      this.nav.openFiles(r.hostId, r.path);
      return;
    }
    if (FULL_VIEWS.includes(this.nav.view())) {
      this.nav.view.set('hosts');
    }
    if (r.kind === 'host') {
      void this.sessions.openHost(r.hostId, r.name);
    } else {
      void this.sessions.openTarget(r.spec);
    }
  }
}
