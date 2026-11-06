import { Injectable, inject } from '@angular/core';

import { ControlApi, OpenRequest } from './backend/control-api';
import { Navigation } from '../layout/navigation';
import { FULL_VIEWS } from '../layout/views';
import { SessionsStore } from '../sessions/sessions.store';

/** Opens hosts, quick connections and files asked by links or the CLI. */
@Injectable({ providedIn: 'root' })
export class OpenRequests {
  private readonly api = inject(ControlApi);
  private readonly sessions = inject(SessionsStore);
  private readonly nav = inject(Navigation);

  async start(): Promise<void> {
    try {
      this.api.onOpen((r) => this.open(r));
      for (const r of await this.api.ready()) {
        this.open(r);
      }
    } catch {
      // Outside the desktop app.
    }
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
