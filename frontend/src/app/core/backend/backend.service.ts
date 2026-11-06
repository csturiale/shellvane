import { Injectable, signal } from '@angular/core';

import { AppService, type Info } from '../../../../bindings/github.com/csturiale/shellvane/app';

/**
 * Thin wrapper over the generated Wails bindings. Components talk to this
 * service, never to the bindings directly, so they can run (and be tested)
 * outside the desktop app.
 */
@Injectable({ providedIn: 'root' })
export class BackendService {
  /** False when the frontend runs in a plain browser, e.g. `ng serve`. */
  readonly connected = signal(true);

  async info(): Promise<Info | null> {
    try {
      return await AppService.Info();
    } catch {
      this.connected.set(false);
      return null;
    }
  }

  async setLanguage(lang: string): Promise<void> {
    try {
      await AppService.SetLanguage(lang);
    } catch {
      this.connected.set(false);
    }
  }
}
