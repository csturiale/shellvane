import { Component, input, output } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import type { TransferResult, TransferState } from './zmodem-transfer';

/** Progress and outcome of a ZMODEM transfer (rz / sz on the server). */
@Component({
  selector: 'app-transfer-bar',
  imports: [TranslocoDirective],
  templateUrl: './transfer-bar.html',
  styleUrl: './transfer-bar.scss',
})
export class TransferBar {
  readonly state = input<TransferState | null>(null);
  readonly result = input<TransferResult | null>(null);
  readonly cancel = output<void>();
  readonly reveal = output<string>();
  readonly dismiss = output<void>();

  protected percent(s: TransferState): number {
    return s.total > 0 ? Math.min(100, Math.round((s.done / s.total) * 100)) : 0;
  }
}
