import { Component, input, output } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { Modal } from './modal';

@Component({
  selector: 'app-confirm-dialog',
  imports: [TranslocoDirective, Modal],
  templateUrl: './confirm-dialog.html',
  styleUrl: './confirm-dialog.scss',
})
export class ConfirmDialog {
  readonly heading = input.required<string>();
  readonly message = input.required<string>();
  readonly confirmLabel = input.required<string>();
  readonly danger = input(false);
  /** A third choice between Cancel and the confirmation ("" for none). */
  readonly secondaryLabel = input('');
  readonly confirmed = output<void>();
  readonly secondary = output<void>();
  readonly cancelled = output<void>();
}
