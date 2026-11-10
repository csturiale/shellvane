import { Component, inject, signal, Type } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideMonitor, LucideMoon, LucideSun } from '@lucide/angular';

import { LanguageService } from '../core/i18n/language.service';
import { Language } from '../core/i18n/languages';
import { ThemeMode, ThemeService, UI_SCALES } from '../core/theme/theme.service';
import { Program, SessionApi } from '../core/backend/session-api';
import { EnvironmentsStore } from '../environments/environments.store';
import { SessionPrefs } from '../sessions/session-prefs';
import { TerminalSettings } from '../terminal/terminal-settings';
import { VaultSettings } from '../vault/vault-settings';
import { HelpLink } from '../guide/help-link';
import { SyncSettings } from '../sync/sync-settings';
import { UsageSettings } from '../core/usage/usage-settings';
import { NetworkSettings } from './network/network-settings';
import { UpdateSettings } from '../updates/update-settings';
import { DiagnosticsSettings } from '../diagnostics/diagnostics-settings';
import { HostsStore } from '../hosts/hosts.store';
import { DangerousCommands } from '../broadcast/dangerous-commands';

import { combo } from '../sessions/shortcuts';
import { NgComponentOutlet } from '@angular/common';

import { EXTENSIONS } from '../extensions/extensions';
@Component({
  selector: 'app-settings-page',
  imports: [NetworkSettings, NgComponentOutlet, HelpLink, TranslocoDirective, LucideMonitor, LucideMoon, LucideSun, TerminalSettings, VaultSettings, DiagnosticsSettings, UpdateSettings, UsageSettings, SyncSettings],
  templateUrl: './settings-page.html',
  styleUrl: './settings-page.scss',
})
export class SettingsPage {
  /** Settings sections added by extensions, in order, once loaded. */
  protected readonly extensionSettings = signal<Type<unknown>[]>([]);
  protected readonly newLocalKeys = combo('T');
  protected readonly uiScales = UI_SCALES;
  protected readonly theme = inject(ThemeService);
  protected readonly language = inject(LanguageService);
  protected readonly sessionPrefs = inject(SessionPrefs);
  protected readonly environments = inject(EnvironmentsStore);
  protected readonly danger = inject(DangerousCommands);
  protected readonly hosts = inject(HostsStore);
  protected readonly shells = signal<Program[]>([]);

  constructor() {
    void Promise.all(EXTENSIONS.flatMap((e) => e.settings ?? []).map((load) => load())).then((list) => this.extensionSettings.set(list));
    void inject(SessionApi)
      .localShells()
      .then((list) => this.shells.set(list))
      .catch(() => undefined);
  }

  protected onShell(event: Event): void {
    const p = this.sessionPrefs;
    p.setLocal((event.target as HTMLSelectElement).value, p.localKubeContext(), p.localKubeNamespace());
  }

  protected onLocalKube(event: Event, field: 'context' | 'namespace'): void {
    const p = this.sessionPrefs;
    const value = (event.target as HTMLInputElement).value;
    p.setLocal(p.localShell(), field === 'context' ? value : p.localKubeContext(), field === 'namespace' ? value : p.localKubeNamespace());
  }

  protected onLanguageChange(event: Event): void {
    this.language.set((event.target as HTMLSelectElement).value as Language);
  }
}
