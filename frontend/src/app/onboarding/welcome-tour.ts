import { Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { TranslatableError, toTranslatableError } from '../core/backend/errors';
import { ImportApi } from '../core/backend/import-api';
import type { ImportSource } from '../core/backend/import-api';
import { LanguageService } from '../core/i18n/language.service';
import type { Language } from '../core/i18n/languages';
import { Navigation } from '../layout/navigation';
import { Modal } from '../shared/modal';
import { VaultStore } from '../vault/vault.store';

export type TourStep = 'welcome' | 'vault' | 'import' | 'done';

/** First-run tour: language, vault, importing hosts, then the guide. */
import { combo } from '../sessions/shortcuts';
@Component({
  selector: 'app-welcome-tour',
  imports: [FormsModule, Modal, TranslocoDirective],
  templateUrl: './welcome-tour.html',
  styleUrls: ['../shared/form.scss', './welcome-tour.scss'],
})
export class WelcomeTour {
  protected readonly paletteKeys = combo('K');
  readonly closed = output<void>();

  protected readonly language = inject(LanguageService);
  protected readonly vault = inject(VaultStore);
  private readonly importApi = inject(ImportApi);
  private readonly nav = inject(Navigation);

  protected readonly steps: TourStep[] = ['welcome', 'vault', 'import', 'done'];
  protected readonly step = signal<TourStep>('welcome');
  protected readonly index = computed(() => this.steps.indexOf(this.step()));
  protected readonly sources = signal<ImportSource[]>([]);
  protected readonly busy = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);
  protected password = '';
  protected confirm = '';
  protected keychain = false;

  constructor() {
    void this.importApi
      .sources()
      .then((list) => this.sources.set(list.filter((s) => s.found)))
      .catch(() => undefined);
  }

  protected setLanguage(lang: string): void {
    this.language.set(lang as Language);
  }

  protected next(): void {
    this.error.set(null);
    this.step.set(this.steps[Math.min(this.index() + 1, this.steps.length - 1)]);
  }

  protected back(): void {
    this.error.set(null);
    this.step.set(this.steps[Math.max(this.index() - 1, 0)]);
  }

  protected async createVault(): Promise<void> {
    if (this.password !== this.confirm) {
      this.error.set({ key: 'vault.mismatch', params: {} });
      return;
    }
    this.busy.set(true);
    this.error.set(null);
    try {
      await this.vault.setup(this.password);
      if (this.keychain) {
        // Optional: a missing keychain must not stop the tour.
        await this.vault.setKeychain(true).catch(() => undefined);
      }
      this.password = this.confirm = '';
      this.next();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.busy.set(false);
    }
  }

  /** Opens the import dialog of the Hosts panel and ends the tour there. */
  protected importHosts(): void {
    this.closed.emit();
    this.nav.ask('hosts', 'import');
  }

  protected newHost(): void {
    this.closed.emit();
    this.nav.ask('hosts', 'newHost');
  }

  protected openGuide(): void {
    this.closed.emit();
    this.nav.openGuide('getting-started');
  }
}
