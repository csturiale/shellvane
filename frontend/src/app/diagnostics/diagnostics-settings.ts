import { Component, inject, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideRefreshCw } from '@lucide/angular';

import { BundleFile, DiagnosticsApi, Handshake } from '../core/backend/diagnostics-api';
import { toTranslatableError } from '../core/backend/errors';
import { LanguageService } from '../core/i18n/language.service';
import { allPreferences } from '../core/preferences';
import { HelpLink } from '../guide/help-link';
import { Modal } from '../shared/modal';
import { HandshakeView } from './handshake-view';
import { SessionsStore } from '../sessions/sessions.store';
import { Navigation } from '../layout/navigation';

/** Settings section: recent connection attempts and the support bundle. */
@Component({
  selector: 'app-diagnostics-settings',
  imports: [TranslocoDirective, HandshakeView, Modal, HelpLink, LucideRefreshCw],
  templateUrl: './diagnostics-settings.html',
  styleUrl: './diagnostics-settings.scss',
})
export class DiagnosticsSettings {
  private readonly api = inject(DiagnosticsApi);
  private readonly sessions = inject(SessionsStore);
  private readonly nav = inject(Navigation);

  /** Opens a tab that measures the terminal on this system. */
  protected async runBenchmark(): Promise<void> {
    this.nav.view.set('hosts');
    await this.sessions.openBenchmark(8);
  }
  private readonly language = inject(LanguageService);
  private readonly transloco = inject(TranslocoService);

  protected readonly handshakes = signal<Handshake[]>([]);
  protected readonly open = signal<string | null>(null);
  protected readonly preview = signal<BundleFile[] | null>(null);
  protected readonly file = signal(0);
  protected readonly saved = signal<string | null>(null);
  protected readonly error = signal<string | null>(null);

  constructor() {
    void this.refresh();
  }

  protected async refresh(): Promise<void> {
    try {
      this.handshakes.set(await this.api.handshakes());
    } catch (err) {
      this.fail(err);
    }
  }

  protected time(h: Handshake): string {
    return new Intl.DateTimeFormat(this.language.current(), { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(h.started));
  }

  protected async showPreview(): Promise<void> {
    try {
      this.file.set(0);
      this.preview.set(await this.api.preview(allPreferences()));
    } catch (err) {
      this.fail(err);
    }
  }

  protected async save(): Promise<void> {
    try {
      this.saved.set(await this.api.save(allPreferences()));
      this.preview.set(null);
    } catch (err) {
      this.fail(err);
    }
  }

  protected reveal(path: string): void {
    void this.api.reveal(path).catch((err) => this.fail(err));
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.error.set(this.transloco.translate(e.key, e.params));
  }
}
