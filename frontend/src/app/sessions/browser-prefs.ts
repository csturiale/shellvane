import { Injectable, signal } from '@angular/core';

import { readPreference, writePreference } from '../core/preferences';
import type { BrowserChoice } from '../core/backend/session-api';

/** Browsers a sign-in page can open in (see app/privatebrowser.go). */
export const BROWSERS = ['chrome', 'edge', 'brave', 'chromium', 'firefox', 'safari'];

export interface BrowserSettings {
  /** '' automatic, 'custom' (command), or one of BROWSERS. */
  browser: string;
  /** The custom command, %s for the link. */
  command: string;
  /** Seconds a login password copied by an environment stays on the clipboard. */
  seconds: number;
  /** Every local terminal uses the browser (BROWSER), not only environment logins. */
  allTerminals: boolean;
  /** In every local terminal: a private window, and opening the sign-in links printed. */
  allPrivate: boolean;
  allLinks: boolean;
}

/** How local terminals open sign-in pages (Settings › Environment logins). */
@Injectable({ providedIn: 'root' })
export class BrowserPrefs {
  readonly settings = signal<BrowserSettings>({
    browser: readPreference('environments.browser') ?? '',
    command: readPreference('environments.browserCommand') ?? '',
    seconds: Number(readPreference('environments.clipboardSeconds') ?? 45),
    allTerminals: readPreference('browser.allTerminals') === 'true',
    allPrivate: readPreference('browser.allPrivate') === 'true',
    allLinks: readPreference('browser.allLinks') === 'true',
  });

  set(change: Partial<BrowserSettings>): void {
    const v = { ...this.settings(), ...change };
    this.settings.set(v);
    writePreference('environments.browser', v.browser);
    writePreference('environments.browserCommand', v.command);
    writePreference('environments.clipboardSeconds', String(v.seconds));
    writePreference('browser.allTerminals', String(v.allTerminals));
    writePreference('browser.allPrivate', String(v.allPrivate));
    writePreference('browser.allLinks', String(v.allLinks));
  }

  /** The choice for a terminal: browser ('' = the settings'), private window. */
  choice(browser: string, isPrivate: boolean): BrowserChoice {
    const s = this.settings();
    return { id: browser || s.browser, command: s.command, private: isPrivate };
  }
}
