import { Component, computed, inject } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideX } from '@lucide/angular';

import { openUrl } from '../core/browser';
import { Navigation } from '../layout/navigation';
import { LanguageService } from '../core/i18n/language.service';
import { UpdatesStore } from './updates.store';

/**
 * The notice shown when a new version is found: install it now (and restart
 * now or at the next start), be reminded tomorrow, skip this version or stop
 * these notices. Settings › Updates keeps every option.
 */
@Component({
  selector: 'app-update-prompt',
  imports: [TranslocoDirective, LucideX],
  templateUrl: './update-prompt.html',
  styleUrl: './update-prompt.scss',
})
export class UpdatePrompt {
  protected readonly store = inject(UpdatesStore);
  private readonly language = inject(LanguageService);
  private readonly nav = inject(Navigation);

  /** Settings › Updates: check again, install, all the options. */
  protected openSettings(): void {
    this.nav.view.set('settings');
  }

  /** The first lines of the release notes, in the UI language or English. */
  protected readonly notes = computed(() => {
    const n = this.store.info()?.notes ?? {};
    const text = n[this.language.current()] ?? n['en'] ?? '';
    const lines = text.split('\n').filter((l) => l.trim());
    return lines.slice(0, 4).join('\n') + (lines.length > 4 ? '\n…' : '');
  });

  protected openPage(): void {
    const page = this.store.info()?.page;
    if (page) {
      void openUrl(page);
    }
  }

  protected stopNotices(): void {
    this.store.setNotify(false);
    this.store.dismiss();
  }
}
