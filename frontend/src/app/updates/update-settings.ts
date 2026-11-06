import { Component, computed, inject } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { openUrl } from '../core/browser';
import { SITE_URL } from '../core/site';
import { LanguageService } from '../core/i18n/language.service';
import { UpdatesStore } from './updates.store';

/** Updates section of the settings page. */
@Component({
  selector: 'app-update-settings',
  imports: [TranslocoDirective],
  templateUrl: './update-settings.html',
  styleUrl: './update-settings.scss',
})
export class UpdateSettings {
  /** Shellvane is free; donations keep it going. */
  protected support(): void {
    void openUrl(`${SITE_URL}/support/`);
  }

  protected readonly store = inject(UpdatesStore);
  private readonly language = inject(LanguageService);

  /** Release notes in the UI language, else in English. */
  protected readonly notes = computed(() => {
    const n = this.store.info()?.notes ?? {};
    return n[this.language.current()] ?? n['en'] ?? '';
  });

  protected openPage(url: string): void {
    void openUrl(url);
  }
}
