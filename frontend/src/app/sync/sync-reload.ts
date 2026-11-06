import { Injectable, Injector, inject } from '@angular/core';

import { SyncApi } from '../core/backend/sync-api';
import { HostsStore } from '../hosts/hosts.store';
import { TunnelsStore } from '../tunnels/tunnels.store';

/** Reloads what a sync from another device changed. */
@Injectable({ providedIn: 'root' })
export class SyncReload {
  private readonly api = inject(SyncApi);
  private readonly hosts = inject(HostsStore);
  private readonly tunnels = inject(TunnelsStore);
  private readonly injector = inject(Injector);

  start(): void {
    try {
      this.api.onDone(() => {
        void this.hosts.load().catch(() => undefined);
        void this.tunnels.load().catch(() => undefined);
        // Schemes, snippets and the rest reload when their views open.
        void import('../themes/themes.store').then((m) => this.injector.get(m.ThemesStore).load()).catch(() => undefined);
      });
    } catch {
      // Outside the desktop app.
    }
  }
}
