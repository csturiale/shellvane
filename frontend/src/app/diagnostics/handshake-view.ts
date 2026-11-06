import { Component, inject, input, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { Handshake, DiagnosticsApi, TraceEvent } from '../core/backend/diagnostics-api';
import { writeClipboard } from '../core/clipboard';

/** The steps of one connection attempt, with "copy report". */
@Component({
  selector: 'app-handshake-view',
  imports: [TranslocoDirective],
  templateUrl: './handshake-view.html',
  styleUrl: './handshake-view.scss',
})
export class HandshakeView {
  readonly handshake = input.required<Handshake>();
  private readonly api = inject(DiagnosticsApi);
  protected readonly copied = signal(false);

  protected details(e: TraceEvent): [string, string][] {
    return Object.entries(e.detail).sort(([a], [b]) => a.localeCompare(b));
  }

  protected async copy(): Promise<void> {
    await writeClipboard(await this.api.report(this.handshake().id));
    this.copied.set(true);
    setTimeout(() => this.copied.set(false), 2000);
  }
}
